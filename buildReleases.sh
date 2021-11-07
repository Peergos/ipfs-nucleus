#! /bin/sh
NAME=ipfs
VERSION=v0.1.0

GOOS=linux GOARCH=amd64 bash -c 'go build -o releases/$VERSION/$GOOS-$GOARCH/'$NAME' daemon/ipfs-nucleus.go'
GOOS=darwin GOARCH=amd64 bash -c 'go build -o releases/$VERSION/$GOOS-$GOARCH/'$NAME' daemon/ipfs-nucleus.go'
GOOS=windows GOARCH=amd64 bash -c 'go build -o releases/$VERSION/$GOOS-$GOARCH/'$NAME'.exe daemon/ipfs-nucleus.go'
GOOS=linux GOARCH=arm64 bash -c 'go build -o releases/$VERSION/$GOOS-$GOARCH/'$NAME' daemon/ipfs-nucleus.go'
GOOS=darwin GOARCH=arm64 bash -c 'go build -o releases/$VERSION/$GOOS-$GOARCH/'$NAME' daemon/ipfs-nucleus.go'
GOOS=linux GOARCH=arm bash -c 'go build -o releases/$VERSION/$GOOS-$GOARCH/'$NAME' daemon/ipfs-nucleus.go'
