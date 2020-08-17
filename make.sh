#!/bin/bash -xv

mkdir -pv bin
GOARCH=arm GOARM=7 go build -o ./bin/ipv6thingy-armv7l
GOARCH=arm GOARM=6 go build -o ./bin/ipv6thingy-armipv6thingyl
GOARCH=amd64 go build -o ./bin/ipv6thingy-x86_64
GOARCH=amd64 go build -o ./bin/ipv6thingy-x86_64
GOARCH=386 go build -o ./bin/ipv6thingy-386





