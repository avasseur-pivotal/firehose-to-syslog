package logging

import "time"
import "os"
import "bufio"
import "io/ioutil"

import "fmt"
import "sync"
import "strings"
import "github.com/cloudfoundry-community/firehose-to-syslog/syslog"
import "github.com/Sirupsen/logrus"
import "gopkg.in/natefinch/lumberjack.v2"
import "gopkg.in/yaml.v2"

type InterceptConfig struct {
	Intercept InterceptConfigItem `yaml:"intercept"`
}

type InterceptConfigItem struct {
	FileName string `yaml:"filename"`
	SizeMB   int    `yaml:"sizeMB"`
	Backup   int    `yaml:"backup"`
	MaxDays  int    `yaml:"maxDays"`
}

func LoadInterceptYaml(path string) (*InterceptConfigItem, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer f.Close()
	reader := bufio.NewReader(f)
	contents, _ := ioutil.ReadAll(reader)

	config := InterceptConfig{}
	err = yaml.Unmarshal(contents, &config)
	if err != nil {
		return nil, err
	}
	return &config.Intercept, nil
}

type LoggingSyslog struct {
	Logger                            *syslog.Logger
	LogrusLogger                      *logrus.Logger
	syslogServer                      string
	debugFlag                         bool
	logFormatterType                  string
	syslogProtocol                    string
	capturedLogMessageOrgLoggers      map[interface{}]lumberjack.Logger
	capturedLogMessageOrgLoggersMutex sync.RWMutex
	intercept                         *InterceptConfigItem
}

func NewLoggingSyslog(SyslogServerFlag string, SysLogProtocolFlag string, LogFormatterFlag string, DebugFlag bool, InterceptConfigPath string) Logging {
	var config *InterceptConfigItem
	//Parse filter intercept config if needed
	if InterceptConfigPath != "" {
		config, _ = LoadInterceptYaml(InterceptConfigPath)
	} else {
		config = nil
	}

	return &LoggingSyslog{
		LogrusLogger:                 logrus.New(),
		syslogServer:                 SyslogServerFlag,
		logFormatterType:             LogFormatterFlag,
		syslogProtocol:               SysLogProtocolFlag,
		debugFlag:                    DebugFlag,
		capturedLogMessageOrgLoggers: make(map[interface{}]lumberjack.Logger),
		intercept:                    config,
	}

}

func (l *LoggingSyslog) Connect() bool {
	l.LogrusLogger.Formatter = GetLogFormatter(l.logFormatterType)

	connectTimeout := time.Duration(10) * time.Second
	writeTimeout := time.Duration(5) * time.Second
	logger, err := syslog.Dial("doppler", l.syslogProtocol, l.syslogServer, nil /*tls cert*/, connectTimeout, writeTimeout, 0 /*tcp max line length*/)
	if err != nil {
		LogError("Could not connect to syslog endpoint", err)
		return false
	} else {
		LogStd(fmt.Sprintf("Connected to syslog endpoint %s://%s", l.syslogProtocol, l.syslogServer), l.debugFlag)
		l.Logger = logger
		return true
	}
}

func (l *LoggingSyslog) ShipEvents(eventFields map[string]interface{}, aMessage string) {
	// remove structured metadata prefixed fields in the message if it was added
	var sds string
	if eventFields["rfc5424_structureddata"] != nil {
		sds = eventFields["rfc5424_structureddata"].(string)
		delete(eventFields, "rfc5424_structureddata")
	}
	var prefix string
	if eventFields["intercept_prefix"] != nil {
		prefix = eventFields["intercept_prefix"].(string)
		delete(eventFields, "intercept_prefix")
	}
	var skipSyslog bool
	if eventFields["intercept_skipsyslog"] != nil {
		skipSyslog = eventFields["intercept_skipsyslog"].(bool)
		delete(eventFields, "intercept_skipsyslog")
	}

	entry := l.LogrusLogger.WithFields(eventFields)
	entry.Message = aMessage
	formatted, _ := entry.String()

	//fmt.Fprintf(os.Stdout, "ShipEvents [%s] %s -| %s", aMessage, eventFields["event_type"], formatted)
	//TODO debug log of some kind?

	packet := syslog.Packet{
		Severity: syslog.SevInfo,
		Facility: syslog.LogLocal5,
		Hostname: "dopplerhostname", //TODO could get local machine name
		Tag:      "pcflog",          //TODO could get proc id - doppler[pid]
		//TODO on UDP it will be truncated to 1K
		//Time: eventFields["timestamp"],
		Time:           time.Now(),
		StructuredData: sds,       //[xxx yy="zz" uu="tt"][other@123 code="abc"]
		Message:        formatted, //For LogMessage, the stdout/stderr will be in "msg:" which comes from Logrus entry.Message = aMessage
	}

	if !skipSyslog {
		l.Logger.Write(packet)
	}

	if l.intercept != nil && prefix != "" {
		if eventFields["cf_app_id"] != "" && eventFields["cf_org_id"] != "" {
			if strings.HasPrefix(aMessage, prefix) {
				l.capturedLogMessageOrgLoggersMutex.RLock()
				lb, exists := l.capturedLogMessageOrgLoggers[eventFields["cf_org_id"]]
				if !exists {
					LogStd(fmt.Sprintf("new Logger for Org %s %s\n", eventFields["cf_org_name"], eventFields["cf_org_id"]), l.debugFlag)
					l.capturedLogMessageOrgLoggersMutex.RUnlock()
					l.capturedLogMessageOrgLoggersMutex.Lock()
					lb = lumberjack.Logger{
						Filename:   fmt.Sprintf(l.intercept.FileName, eventFields["cf_org_id"]),
						MaxSize:    l.intercept.SizeMB,  // megabytes
						MaxBackups: l.intercept.Backup,  // keep 2 archive + current - firehose-2016-12-05T09-48-47.802.log
						MaxAge:     l.intercept.MaxDays, //days
					}
					l.capturedLogMessageOrgLoggers[eventFields["cf_org_id"]] = lb
					l.capturedLogMessageOrgLoggersMutex.Unlock()
				} else {
					//fmt.Fprintf(os.Stdout, "cur Logger %s %s\n", eventFields["cf_org_id"], l.capturedLogMessageOrgLoggers[eventFields["cf_org_id"]])
					l.capturedLogMessageOrgLoggersMutex.RUnlock()
				}

				//TODO we would need to remove the prefix ?
				lb.Write([]byte(strings.TrimPrefix(aMessage, prefix)))
				lb.Write([]byte("\n"))
			}
		}
	}

	/*
		lb := &lumberjack.Logger{
			Filename:   "/tmp/firehose.log",
			MaxSize:    1, // megabytes
			MaxBackups: 2, // keep 2 archive + current - firehose-2016-12-05T09-48-47.802.log
			MaxAge:     1, //days
		}
	*/

}
