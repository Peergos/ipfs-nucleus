#! /bin/sh
NAME=ipfs-nucleus
GOOS=linux GOARCH=amd64 bash -c 'go build -o '$NAME'-$GOOS-$GOARCH daemon/ipfs-nucleus.go'
GOOS=darwin GOARCH=amd64 bash -c 'go build -o '$NAME'-$GOOS-$GOARCH daemon/ipfs-nucleus.go'
GOOS=windows GOARCH=amd64 bash -c 'go build -o '$NAME'-$GOOS-$GOARCH daemon/ipfs-nucleus.go'
GOOS=linux GOARCH=arm64 bash -c 'go build -o '$NAME'-$GOOS-$GOARCH daemon/ipfs-nucleus.go'
GOOS=darwin GOARCH=arm64 bash -c 'go build -o '$NAME'-$GOOS-$GOARCH daemon/ipfs-nucleus.go'
GOOS=linux GOARCH=arm bash -c 'go build -o '$NAME'-$GOOS-$GOARCH daemon/ipfs-nucleus.go'
