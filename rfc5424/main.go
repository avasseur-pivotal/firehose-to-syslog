// main
package rfc5424

import (
	"fmt"

	"gopkg.in/mcuadros/go-syslog.v2"
)

// https://github.com/crewjam/rfc5424
// https://github.com/papertrail/remote_syslog2/tree/master/syslog

// echo '<165>1 2003-10-11T22:14:15.003Z mymachine.example.com evntslog - ID47 [exampleSDID@32473 iut="3" eventSource="Application" eventID="1011"] An application event log entry...' | nc -u localhost 5140
func main() {
	fmt.Println("RFC5424 UDP syslog server")

	channel := make(syslog.LogPartsChannel)
	handler := syslog.NewChannelHandler(channel)

	server := syslog.NewServer()
	server.SetFormat(syslog.RFC5424)
	server.SetHandler(handler)
	server.ListenUDP("0.0.0.0:5140")
	server.ListenTCP("0.0.0.0:5140")
	server.Boot()

	go func(channel syslog.LogPartsChannel) {
		for logParts := range channel {
			fmt.Println(logParts)
		}
	}(channel)

	server.Wait()
}
