#!/bin/bash

export GO111MODULE=off
make
mkdir -p  ${BUILD_ROOT}/bin && mv cmd/event/event ${BUILD_ROOT}/bin/.
