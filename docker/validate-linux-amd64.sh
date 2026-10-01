#!/bin/sh
set -eux

test "$(uname -m)" = x86_64
test "$(go env GOOS)" = linux
test "$(go env GOARCH)" = amd64
test "$(go env CGO_ENABLED)" = 1

python3 -m unittest discover -s deps -p 'test_*.py'
go test . -run '^Test' -count=1
go test -race . -run '^Test' -count=1
GOEXPERIMENT=cgocheck2 go test . -run '^Test' -count=1
go test ./...
go vet ./...
go test --tags leakcheck . -run '^Test' -count=1
