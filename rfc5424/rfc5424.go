package rfc5424

import (
	"bufio"
	"io/ioutil"
	"os"
	"regexp"

	"gopkg.in/yaml.v2"

	"github.com/cloudfoundry-community/firehose-to-syslog/logging"
)

type Rfc5424Config struct {
	Items []Rfc5424ConfigItem `yaml:"rfc5424"`
}

type Rfc5424ConfigItem struct {
	Rule       string `yaml:"rule"`
	Space      string `yaml:"space"`
	Meta       string `yaml:"meta"`        // RFC5424 structured data to add
	SkipSyslog bool   `yaml:"skip-syslog"` // defaults to false
	Intercept  string `yaml:"intercept"`
}

type AppFilterStructuredData struct {
	Config  Rfc5424ConfigItem
	Matcher *regexp.Regexp
}

func LoadFilterYaml(path string) (*[]AppFilterStructuredData, string, error) {
	var filter []AppFilterStructuredData
	var defaultsd string

	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}

	defer f.Close()
	reader := bufio.NewReader(f)
	contents, _ := ioutil.ReadAll(reader)

	config := Rfc5424Config{}
	err = yaml.Unmarshal(contents, &config)
	if err != nil {
		return nil, "", err
	}

	for _, item := range config.Items {
		logging.LogStd("Loading rule "+item.Rule, true)

		rx, err := regexp.Compile(item.Space)
		if err != nil {
			logging.LogError("Cannot compile regexp", err)
			return nil, "", err
		}
		sd := ""
		if item.Meta != "" {
			sd = item.Meta
		}
		filter = append(filter, AppFilterStructuredData{
			Config:  item,
			Matcher: rx,
		})

		// special case for ".*" with structured data
		if item.Space == ".*" && sd != "" {
			defaultsd = sd
		}
	}
	return &filter, defaultsd, nil

}

// DEPRECATED
// loads from file at path
// regexp TAB structureddata
// example:
// org.*prod/^space$/.*		[stuff@id foo="bar"]
// will ignore lines starting with #
/*
func LoadFilter(path string) (*[]AppFilterStructuredData, string, error) {
	var filter []AppFilterStructuredData
	var defaultsd string

	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}

	defer f.Close()
	reader := bufio.NewReader(f)
	contents, _ := ioutil.ReadAll(reader)
	lines := strings.Split(string(contents), "\n")
	for _, line := range lines {
		if len(strings.TrimSpace(line)) > 0 && !strings.HasPrefix(line, "#") {
			cols := strings.Split(line, "\t")

			rx, err := regexp.Compile(cols[0])
			if err != nil {
				logging.LogError("Cannot compile regexp", err)
				return nil, "", err
			}

			sd := ""
			if len(cols) > 1 {
				sd = cols[1]
			}
			filter = append(filter, AppFilterStructuredData{
				match:          cols[0],
				Matcher:        rx,
				StructuredData: sd,
			})

			// special case for ".*" with structured data
			if cols[0] == ".*" && sd != "" {
				defaultsd = sd
			}
		}
	}
	return &filter, defaultsd, nil
}
*/
