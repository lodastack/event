all: build

fmt:
	gofmt -l -w -s */

build: fmt
	export GO111MODULE=on
	export GOPROXY=https://goproxy.io,direct
	cd cmd/event && go build -v

install: fmt
	cd cmd/event && go install

clean:
	cd cmd/event && go clean
