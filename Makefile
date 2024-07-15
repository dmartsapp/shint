BINARY=telnet
VERSION=$(shell git rev-list -1 HEAD)
run:
	go run -ldflags "-X main.Version=$(VERSION)" main.go

all: linux-x64 linux-arm64 darwin-x64 darwin-arm64 windows

windows: windows-x64 windows-arm64

windows-arm64:
	GOOS=windows GOARCH=arm64 go build -o bin/$(BINARY).windows.arm64.exe -tags windows -ldflags "-X main.Version=$(VERSION)" main.go

windows-x64:
	GOOS=windows GOARCH=amd64 go build -o bin/$(BINARY).windows.x64.exe -tags windows -ldflags "-X main.Version=$(VERSION)" main.go

linux-arm64:
	GOOS=windows GOARCH=amd64 go build -o bin/$(BINARY).linux-arm64 -ldflags "-X main.Version=$(VERSION)" main.go

linux-x64:
	GOOS=windows GOARCH=amd64 go build -o bin/$(BINARY).linux-x64 -ldflags "-X main.Version=$(VERSION)" main.go

darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -o bin/$(BINARY).darwin-arm64 -ldflags "-X main.Version=$(VERSION)" main.go

darwin-x64:
	GOOS=darwin GOARCH=amd64 go build -o bin/$(BINARY).darwin-x64 -ldflags "-X main.Version=$(VERSION)" main.go

clean:
	rm bin/*
