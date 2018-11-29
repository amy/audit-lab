#!/usr/bin/env bash

GOOS=linux go build .

docker build -t aunem/audit-proxy:latest .

docker push aunem/audit-proxy:latest