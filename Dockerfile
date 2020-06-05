FROM golang:1.14 AS build

COPY . /src/project
WORKDIR /src/project

RUN export CGO_ENABLED=0 &&\
    export GOPROXY=https://goproxy.io &&\
    make &&\
    cp cmd/event/event /event &&\
    cp etc/event.sample.conf /event.conf

FROM debian:10
RUN apt-get update && apt-get install -y ca-certificates
COPY --from=build /event /event
COPY --from=build /event.conf /etc/event.conf

EXPOSE 8001

CMD ["/event", "-c", "/etc/event.conf"]
