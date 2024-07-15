BINARY=telnet
VERSION=$(shell git rev-list -1 HEAD)
VERSIONSTR="$(VERSION)-$(shell git show --no-patch --format="%cd" --date='format:%d%m%Y%H%M%S' $(VERSION))"
LDFLAGS=-ldflags "-X main.Version=$(VERSIONSTR)"
BUILDFLAGS=-buildvcs=true $(LDFLAGS)
run:
	go run -ldflags "-X main.Version=$(VERSIONSTR)" main.go

all: linux darwin windows

windows: windows-x64 windows-arm64

linux: linux-x64 linux-arm64

darwin: darwin-x64 darwin-arm64

windows-arm64:
	GOOS=windows GOARCH=arm64 go build -o bin/$(BINARY).windows.arm64.exe $(BUILDFLAGS) main.go

windows-x64:
	GOOS=windows GOARCH=amd64 go build -o bin/$(BINARY).windows.x64.exe $(BUILDFLAGS) main.go

linux-arm64:
	GOOS=windows GOARCH=amd64 go build -o bin/$(BINARY).linux-arm64 $(BUILDFLAGS) main.go

linux-x64:
	GOOS=windows GOARCH=amd64 go build -o bin/$(BINARY).linux-x64 $(BUILDFLAGS) main.go

darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -o bin/$(BINARY).darwin-arm64 $(BUILDFLAGS) main.go

darwin-x64:
	GOOS=darwin GOARCH=amd64 go build -o bin/$(BINARY).darwin-x64 $(BUILDFLAGS) main.go

clean:
	rm bin/*
