# Copyright 2020-present Open Networking Foundation
# SPDX-License-Identifier: Apache-2.0

FROM golang:1.21.3-bookworm as builder

WORKDIR /go/src/app
COPY . .

RUN go get -d -v ./...
RUN go install -v ./...

FROM bitnami/minideb:buster
COPY --from=builder /go/bin/dbuf /usr/local/bin
COPY --from=builder /go/bin/dbuf-client /usr/local/bin

EXPOSE 10000
CMD ["dbuf"]
