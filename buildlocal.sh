BUILD_NUMBER=1.0.`date "+%Y%m%d%H%M"`


GOARCH=amd64 GOOS=darwin go build --ldflags="-X main.version=${BUILD_NUMBER}" -o firehose-to-syslog

CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build --ldflags="-X main.version=${BUILD_NUMBER}" -o firehose-to-syslog_linux_amd64


