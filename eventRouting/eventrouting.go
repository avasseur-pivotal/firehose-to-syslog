package eventRouting

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/cloudfoundry-community/firehose-to-syslog/caching"
	fevents "github.com/cloudfoundry-community/firehose-to-syslog/events"
	"github.com/cloudfoundry-community/firehose-to-syslog/extrafields"
	"github.com/cloudfoundry-community/firehose-to-syslog/logging"
	"github.com/cloudfoundry-community/firehose-to-syslog/rfc5424"
	"github.com/cloudfoundry/sonde-go/events"
)

type EventRouting struct {
	CachingClient       caching.Caching
	selectedEvents      map[string]bool
	selectedEventsCount map[string]uint64
	mutex               *sync.Mutex
	log                 logging.Logging
	ExtraFields         map[string]string
	filterRFC5424       *[]rfc5424.AppFilterStructuredData
}

func NewEventRouting(caching caching.Caching, logging logging.Logging) *EventRouting {
	return &EventRouting{
		CachingClient:       caching,
		selectedEvents:      make(map[string]bool),
		selectedEventsCount: make(map[string]uint64),
		log:                 logging,
		mutex:               &sync.Mutex{},
		ExtraFields:         make(map[string]string),
	}
}

func (e *EventRouting) GetSelectedEvents() map[string]bool {
	return e.selectedEvents
}

func (e *EventRouting) RouteEvent(msg *events.Envelope) {

	eventType := msg.GetEventType()

	if e.selectedEvents[eventType.String()] {
		var event *fevents.Event
		switch eventType {
		case events.Envelope_HttpStart:
			event = fevents.HttpStart(msg)
		case events.Envelope_HttpStop:
			event = fevents.HttpStop(msg)
		case events.Envelope_HttpStartStop:
			event = fevents.HttpStartStop(msg)
		case events.Envelope_LogMessage:
			event = fevents.LogMessage(msg)
		case events.Envelope_ValueMetric:
			event = fevents.ValueMetric(msg)
		case events.Envelope_CounterEvent:
			event = fevents.CounterEvent(msg)
		case events.Envelope_Error:
			event = fevents.ErrorEvent(msg)
		case events.Envelope_ContainerMetric:
			event = fevents.ContainerMetric(msg)
		}

		event.AnnotateWithEnveloppeData(msg)

		event.AnnotateWithMetaData(e.ExtraFields)

		// default accept unless filter in place
		accept := (e.filterRFC5424 == nil)
		if _, hasAppId := event.Fields["cf_app_id"]; hasAppId {
			event.AnnotateWithAppData(e.CachingClient)
			// filter is in place and a real app name was found (else it is just RTR CC api)
			if e.filterRFC5424 != nil && event.Fields["cf_app_name"] != nil {
				orgspaceapp := strings.Join([]string{event.Fields["cf_org_name"].(string), event.Fields["cf_space_name"].(string), event.Fields["cf_app_name"].(string)}, "/")
				for _, filter := range *e.filterRFC5424 {
					if filter.Matcher.MatchString(orgspaceapp) {
						accept = true
						event.Fields["rfc5424_structureddata"] = filter.StructuredData
						break
					}
				}
			}
		}

		if accept {
			e.mutex.Lock()
			e.log.ShipEvents(event.Fields, event.Msg)
			e.selectedEventsCount[eventType.String()]++
			e.mutex.Unlock()
		}
	}
}

func (e *EventRouting) SetupEventRouting(wantedEvents string, filterRFC5424path string) error {
	e.selectedEvents = make(map[string]bool)
	if wantedEvents == "" {
		e.selectedEvents["LogMessage"] = true
	} else {
		for _, event := range strings.Split(wantedEvents, ",") {
			if e.isAuthorizedEvent(strings.TrimSpace(event)) {
				e.selectedEvents[strings.TrimSpace(event)] = true
				logging.LogStd(fmt.Sprintf("Event Type [%s] is included in the fireshose!", event), false)
			} else {
				return fmt.Errorf("Rejected Event Name [%s] - Valid events: %s", event, GetListAuthorizedEventEvents())
			}
		}
	}

	//RFC5424 filter if required
	if filterRFC5424path != "" {
		f, err := rfc5424.LoadFilter(filterRFC5424path)
		if err != nil {
			logging.LogError(fmt.Sprintf("Could not read filter file %s", filterRFC5424path), err)
			return err
		}
		logging.LogStd(fmt.Sprintf("Setup %d filters RFC5424 structured data", len(*f)), true)
		e.filterRFC5424 = f
	}
	return nil
}

func (e *EventRouting) SetExtraFields(extraEventsString string) {
	// Parse extra fields from cmd call
	extraFields, err := extrafields.ParseExtraFields(extraEventsString)
	if err != nil {
		logging.LogError("Error parsing extra fields: ", err)
		os.Exit(1)
	}
	e.ExtraFields = extraFields
}

func (e *EventRouting) isAuthorizedEvent(wantedEvent string) bool {
	for _, authorizeEvent := range events.Envelope_EventType_name {
		if wantedEvent == authorizeEvent {
			return true
		}
	}
	return false
}

func GetListAuthorizedEventEvents() (authorizedEvents string) {
	arrEvents := []string{}
	for _, listEvent := range events.Envelope_EventType_name {
		arrEvents = append(arrEvents, listEvent)
	}
	sort.Strings(arrEvents)
	return strings.Join(arrEvents, ", ")
}

func (e *EventRouting) GetTotalCountOfSelectedEvents() uint64 {
	var total = uint64(0)
	for _, count := range e.GetSelectedEventsCount() {
		total += count
	}
	return total
}

func (e *EventRouting) GetSelectedEventsCount() map[string]uint64 {
	return e.selectedEventsCount
}

func (e *EventRouting) LogEventTotals(logTotalsTime time.Duration) {
	firehoseEventTotals := time.NewTicker(logTotalsTime)
	count := uint64(0)
	startTime := time.Now()
	totalTime := startTime

	go func() {
		for range firehoseEventTotals.C {
			elapsedTime := time.Since(startTime).Seconds()
			totalElapsedTime := time.Since(totalTime).Seconds()
			startTime = time.Now()
			event, lastCount := e.getEventTotals(totalElapsedTime, elapsedTime, count)
			count = lastCount
			e.log.ShipEvents(event.Fields, event.Msg)
		}
	}()
}

func (e *EventRouting) getEventTotals(totalElapsedTime float64, elapsedTime float64, lastCount uint64) (*fevents.Event, uint64) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	totalCount := e.GetTotalCountOfSelectedEvents()
	sinceLastTime := float64(int(elapsedTime*10)) / 10
	fields := logrus.Fields{
		"total_count":   totalCount,
		"by_sec_Events": int((totalCount - lastCount) / uint64(sinceLastTime)),
	}

	for eventtype, count := range e.GetSelectedEventsCount() {
		fields[eventtype] = count
	}

	event := &fevents.Event{
		Type:   "firehose_to_syslog_stats",
		Msg:    "Statistic for firehose to syslog",
		Fields: fields,
	}
	event.AnnotateWithMetaData(map[string]string{})
	return event, totalCount
}
