# nats-load-traffic

Load test [NATS](https://docs.nats.io/nats-concepts/overview) with configurable
traffic patterns.

## Description

This application enables various load profiles to be constructed and applied
to a NATS server or cluster. Setup multiple controlled and concurrent submission
loads to test NATS performance or downstream consumer/workers performance.

Additionally, this app collects metrics about submissions and enables them to be
forwarded to a prometheus remote write server for further analysis.

## Getting Started

### Examples

Please see [/examples](https://github.com/evanofslack/nats-load-traffic/tree/main/example) for a demo

### Running

The app can be run from a [pre-built docker container](https://hub.docker.com/r/evanofslack/nats-load-traffic/tags)

```yaml
version: "3.7"
services:
  nats-load-traffic:
    container_name: nats-load-traffic
    image: evanofslack/nats-load-traffic:latest
    restart: unless-stopped
    volumes:
      - ./config:/config
    command: -c /config/config.yaml -m /config/msg.json
```

Alternatively, build the executable from source

```bash
git clone https://github.com/evanofslack/nats-load-traffic
cd nats-load-traffic/cmd/nats-load-traffic
go build ./...
```

### Parameters

Runtime parameters can be configured through flags:

```bash
usage: nats-load-traffic [-c config path] [-p payload path]
  -c string
        path to config file (default "config.yaml")
  -p string
        path to message payload file (default "msg.json")

```

### Configuration

Configuration file is a yaml formatted file. It provides the endpoints for
the nats server and the remote write prometheus server (if enabled).

The app can take any number of `profiles`. It will run concurrent load for each
provided profile.

```yaml
remote_write:
  enabled: true
  url: http://localhost:9090/api/v1/write
  interval: 5s

nats:
  url: nats://localhost:4222

profiles:
  - name: job-1     # human readable profile name
    subject: test.1 # nats subject to submit to
    load: constant  # type of load (constant, periodic, random, rare)
    rate: 600       # max submit rate (msg/sec)
    duration: 60m   # how long to run the load

  - name: job-2
    subject: test.2
    load: random
    rate: 600
    duration: 60m
```

### Metrics

If remote write is enabled, all publish metrics will be sent to the
prometheus remote write target. There is one metric with a couple
of labels that is recorded: `nats_load_traffic_submissions_total`

Here is an example plot of the metric (which also exhibits the four different
load profiles): `rate(nats_load_traffic_submissions_total{result="success"}[30s])`

<img width="850" alt="grafana plot" src="https://github.com/evanofslack/nats-load-traffic/assets/51209817/676d1f63-e3d1-4952-ac6c-f45ab183331e">
