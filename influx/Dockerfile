# Copyright 2022 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

FROM golang:1.18 AS gobuilder

WORKDIR /app

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading
# them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

RUN go build -o setup

FROM marketplace.gcr.io/google/influxdb2:latest

COPY --from=gobuilder /app/setup /setup

# For now, generate a self-signed cert to use. Not for production use!
RUN openssl req -x509 -nodes -newkey rsa:2048 \
  -keyout /etc/ssl/influxdb-selfsigned.key \
  -out /etc/ssl/influxdb-selfsigned.crt \
  -days 30 \
  -subj '/CN=localhost'

# Run our setup application in the background and the parent Influx entrypoint
# in the foreground.
ENTRYPOINT ["/bin/sh"]
CMD ["-c", "/setup & /docker-entrypoint.sh influxd --tls-cert=/etc/ssl/influxdb-selfsigned.crt --tls-key=/etc/ssl/influxdb-selfsigned.key --http-bind-address=:443"]
