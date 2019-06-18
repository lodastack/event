all: build

fmt:
	gofmt -l -w -s */

dep: fmt
	export GO111MODULE=off
	gdm restore

build: dep
	cd cmd/event && go build -v

install: fmt
	cd cmd/event && go install

clean:
	cd cmd/event && go clean
