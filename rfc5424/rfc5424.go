package rfc5424

import (
	"bufio"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"github.com/cloudfoundry-community/firehose-to-syslog/logging"
)

type AppFilterStructuredData struct {
	match          string
	Matcher        *regexp.Regexp
	StructuredData string
}

// loads from file at path
// regexp TAB structureddata
// example:
// org.*prod/^space$/.*		[stuff@id foo="bar"]
// will ignore lines starting with #
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
