#!/bin/bash

make
mkdir -p  ${BUILD_ROOT}/bin && mv cmd/event/event ${BUILD_ROOT}/bin/.
