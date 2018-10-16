#!/usr/bin/env bash
cd `dirname $0`

PROGRAM_NAME="fastcgi"

CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o ./${PROGRAM_NAME}.mac

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ./${PROGRAM_NAME}.linux