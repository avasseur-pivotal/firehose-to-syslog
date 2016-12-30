# This is a fork

This is a fork from https://github.com/cloudfoundry-community/firehose-to-syslog

Added & changed:
- not using Logrus syslog but using a custom implementation derived from https://github.com/papertrail/remote_syslog2/tree/master/syslog
- this is to implement optional RFC5424 structured data field in addition to the message field
- adding --filter-path=...file.txt that contains a yml configuration file for the regexp matching against orgName/spaceName/appName and the structureddata to add
- optional skipping syslog 
- optional intercept LogMessage to a rolling logfile, one file per org, intercepting prefix (and removing it before logging)

If --filter-path is used then:
unless there is a wildcard .* :
- logs that do not have appName (found from app id and cache) are discarded
- orgName/spaceName/appName is matched to the regexp in order till a match - else discarded
- if matched, and if regexp is associated with a structureddata, the structureddata is added to the syslog message in RFC5424 format
If there is a wildcard .* then all logs including those without app id are accepted and enriched with the provided ".* TAB structureddata" from the filter file


# Example command
You can test using PCFdev and using `cf push -o cloudfoundry/lattice-app lattice`

```
./firehose-to-syslog \
--api-endpoint https://api.local.pcfdev.io --skip-ssl-validation --user=admin --password=admin \
--syslog-server=localhost:5001 \
-subscription-id="exlog" \
--events=LogMessage --filter-path=rfc5424/rfc5424.yml \
--debug
```



# Example yml
See rfc5424/ folder
```
# Yml formatted file
#
# "rfc5424" defines a set of Org/Space/AppName regexp to capture
# - if "meta" is present, it will be added as RFC5424 structured data
# - if a rule has "intercept" present (and if there is an "intercept"
#   configuration) then the LogMessage who are prefixed with it will be
#   captured and written to the configured intercept filename
# - the optional "skip-syslog" can be used to only have intercept to log
#   and no syslog out.
#
# "intercept" defines the configuration of log files for the intercepted
# LogMessage. The "filename" %s is substituted with the Org id
#
---
rfc5424:
  - rule: demo1
    space: "^France-org/development/.*"
  - rule: demo2
    space: "^France-org/docker/lattice$"
    meta: '[xx@123 code="lattice"]'
    intercept: "Lattice-"
  - rule: catchAll
    space: .*
#    meta: '[meta sequenceid=""][xx@123 code="1CF"]'
    skip-syslog: false
#
# filename must have a %s that will be replaced by the org id
#
intercept:
  filename: /tmp/firehose-%s.log
  sizeMB: 1
  backup: 2
  maxDays: 1
```




#Disclaimer

This is V2 if you encounter any trouble with this version please use the 1.X.X

# Firehose-to-syslog

This nifty util aggregates all the events from the firehose feature in
CloudFoundry.

	./firehose-to-syslog \
              --api-endpoint="https://api.10.244.0.34.xip.io" \
              --skip-ssl-validation \
              --debug
	....
	....
	{"cf_app_id":"c5cb762b-b7bb-44b6-97d1-2b612d4baba9","cf_app_name":"lattice","cf_org_id":"fb5777e6-e234-4832-8844-773114b505b0","cf_org_name":"GWENN","cf_origin":"firehose","cf_space_id":"3c910823-22e7-41ff-98de-094759594398","cf_space_name":"GWENN-SPACE","event_type":"LogMessage","level":"info","message_type":"OUT","msg":"Lattice-app. Says Hello. on index: 0","origin":"rep","source_instance":"0","source_type":"APP","time":"2015-06-12T11:46:11+09:00","timestamp":1434077171244715915}

# Options

```
usage: firehose-to-syslog --api-endpoint=API-ENDPOINT [<flags>]

Flags:
  --help                         Show context-sensitive help (also try --help-long and --help-man).
  --debug                        Enable debug mode. This disables forwarding to syslog
  --api-endpoint=API-ENDPOINT    Api endpoint address. For bosh-lite installation of CF: https://api.10.244.0.34.xip.io
  --doppler-endpoint=DOPPLER-ENDPOINT
                                 Overwrite default doppler endpoint return by /v2/info
  --syslog-server=SYSLOG-SERVER  Syslog server.
  --syslog-protocol="tcp"        Syslog protocol (tcp/udp).
  --subscription-id="firehose"   Id for the subscription.
  --user="admin"                 Admin user.
  --password="admin"             Admin password.
  --skip-ssl-validation          Please don't
  --fh-keep-alive=25s            Keep Alive duration for the firehose consumer
  --log-event-totals             Logs the counters for all selected events since nozzle was last started.
  --log-event-totals-time=30s    How frequently the event totals are calculated (in sec).
  --events="LogMessage"          Comma separated list of events you would like. Valid options are Error, ContainerMetric,
                                 HttpStart, HttpStop, HttpStartStop, LogMessage, ValueMetric, CounterEvent
  --boltdb-path="my.db"          Bolt Database path
  --cc-pull-time=60s             CloudController Polling time in sec
  --extra-fields=""              Extra fields you want to annotate your events with, example:
                                 '--extra-fields=env:dev,something:other
  --mode-prof=""                 Enable profiling mode, one of [cpu, mem, block]
  --path-prof=""                 Set the Path to write profiling file
  --log-formatter-type=LOG-FORMATTER-TYPE
                                 Log formatter type to use. Valid options are text, json. If none provided, defaults to json.
  --version                      Show application version.
```

** !!! **--events** Please use --help to get last updated event.


#Endpoint definition

We use [gocf-client](https://github.com/cloudfoundry-community/go-cfclient) which will call the CF endpoint /v2/info to get Auth., doppler endpoint.

But for doppler endpoint you can overwrite it with ``` --doppler-address ``` as we know some people use different endpoint.

# Event documentation

See the [dropsonde protocol documentation](https://github.com/cloudfoundry/dropsonde-protocol/tree/master/events) for details on what data is sent as part of each event.

# Caching
We use [boltdb](https://github.com/boltdb/bolt) for caching application name, org and space name.

We have 3 caching strategies:
* Pull all application data on start.
* Pull application data if not cached yet.
* Pull all application data every "cc-pull-time".

# To test and build


    # Setup repo
    go get github.com/cloudfoundry-community/firehose-to-syslog
    cd $GOPATH/src/github.com/cloudfoundry-community/firehose-to-syslog

    # Test
	ginkgo -r .

    # Build binary
    godep go build

# Deploy with Bosh

[logsearch-for-cloudfoundry](https://github.com/logsearch/logsearch-for-cloudfoundry)

# Run against a bosh-lite CF deployment

    godep go run main.go \
		--debug \
		--skip-ssl-validation \
		--api-endpoint="https://api.10.244.0.34.xip.io"

# Parsing the logs with Logstash

[logsearch-for-cloudfoundry](https://github.com/logsearch/logsearch-for-cloudfoundry)


# Docker (tested with docker 1.7.1 / Kitematic)
We use DockerInDocker to built the image
Since is around 7MG

* For Github Master branch Image
```bash
# Make the image
make docker-final

#Run the image
docker run getourneau/firehose-to-syslog

```

* For development
```bash
#Build the image
make docker-dev

#Run the image
docker run getourneau/firehose-to-syslog-dev
```


# Devel

This is a
[Git Flow](http://nvie.com/posts/a-successful-git-branching-model/)
project. Please fork and branch your features from develop.

# Profiling

To enable CPU Profiling you just need to add the profiling path ex ``` --mode-prof=cpu```

Run your program for some time and after that you can use the pprof tool
```bash
go tool pprof YOUR_EXECUTABLE cpu.pprof

(pprof) top 10
110ms of 110ms total (  100%)
Showing top 10 nodes out of 44 (cum >= 20ms)
      flat  flat%   sum%        cum   cum%
      30ms 27.27% 27.27%       30ms 27.27%  syscall.Syscall
      20ms 18.18% 45.45%       20ms 18.18%  ExternalCode
      20ms 18.18% 63.64%       20ms 18.18%  runtime.futex
      10ms  9.09% 72.73%       10ms  9.09%  adjustpointers
      10ms  9.09% 81.82%       10ms  9.09%  bytes.func·001
      10ms  9.09% 90.91%       20ms 18.18%  io/ioutil.readAll
      10ms  9.09%   100%       10ms  9.09%  runtime.epollwait
         0     0%   100%       60ms 54.55%  System
         0     0%   100%       20ms 18.18%  bufio.(*Reader).Read
         0     0%   100%       20ms 18.18%  bufio.(*Reader).fill
```

# Push as an App to Cloud Foundry

1. Create doppler.firehose enabled user

		uaac target https://uaa.[your cf system domain] --skip-ssl-validation
		uaac token client get admin -s [your admin-secret]
		cf create-user [firehose user] [firehose password]
		uaac member add cloud_controller.admin [your firehose user]
		uaac member add doppler.firehose [your firehose user]

1. Download the latest release of  firehose-to-syslog.

		git clone https://github.com/cloudfoundry-community/firehose-to-syslog
		cd firehose-to-syslog

1. Utilize the CF cli to authenticate with your PCF instance.

		cf login -a https://api.[your cf system domain] -u [your id] --skip-ssl-validation

1. Push firehose-to-syslog.

		cf push firehose-to-syslog --no-start

1. Set environment variables with cf cli or in the [manifest.yml](./manifest.yml).

		cf set-env firehose-to-syslog API_ENDPOINT https://api.[your cf system domain]
		cf set-env firehose-to-syslog DOPPLER_ENDPOINT wss://doppler.[your cf system domain]:443
		cf set-env firehose-to-syslog SYSLOG_ENDPOINT [Your Syslog IP]:514
		cf set-env firehose-to-syslog LOG_EVENT_TOTALS true
		cf set-env firehose-to-syslog LOG_EVENT_TOTALS_TIME "10s"
		cf set-env firehose-to-syslog SKIP_SSL_VALIDATION true
		cf set-env firehose-to-syslog FIREHOSE_SUBSCRIPTION_ID firehose-to-syslog
		cf set-env firehose-to-syslog FIREHOSE_USER  [your doppler.firehose enabled user]
		cf set-env firehose-to-syslog FIREHOSE_PASSWORD  [your doppler.firehose enabled user password]
		cf set-env firehose-to-syslog LOG_FORMATTER_TYPE [Log formatter type to use. Valid options are : text, json]

1. Turn off the health check if you're staging to Diego.

		cf set-health-check firehose-to-syslog none

1. Push the app.

		cf push firehose-to-syslog --no-route

	If you are using the offline version of the go buildpack and your app fails to stage then open up the Godeps/Godeps.json file and change the `GoVersion` from `go1.5.3` to `go1.5` and repush.

# Contributors

* [Ed King](https://github.com/teddyking) - Added support to skip ssl
validation.
* [Mark Alston](https://github.com/malston) - Added support for more
  events and general code cleaup.
* [Etourneau Gwenn](https://github.com/shinji62) - Added validation of
  selected events and general code cleanup, caching system..
