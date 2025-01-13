#!/bin/bash

set pipefile -ex

export SERVICE_PORT=8080
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4319
export OTEL_EXPORTER_OTLP_INSECURE=true
export OTEL_RESOURCE_ATTRIBUTES="service.name=go-example,service.namespace=example-team"

./build/go-example
