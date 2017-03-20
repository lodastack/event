all: build

fmt:
	gofmt -l -w -s */

build: fmt 
	cd cmd/event && go build -v

install: fmt
	cd cmd/event && go install

clean:
	cd cmd/event && go clean