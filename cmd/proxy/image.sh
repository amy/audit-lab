#!/usr/bin/env bash

echo "building bin"
GOOS=linux go build .

echo "building docker image"
docker build -t aunem/audit-proxy:latest .

echo "pushing docker image"
docker push aunem/audit-proxy:latest