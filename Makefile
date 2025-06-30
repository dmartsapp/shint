BINARY=telnet
COMMIT=$(shell git rev-list -1 HEAD)
VERSION=$(shell git tag --contains $(COMMIT))
VERSIONSTR="$(VERSION)-$(shell git show --no-patch --format="%cd" --date='format:%d%m%Y%H%M%S' $(VERSION))"
LDFLAGS=-ldflags "-X main.Version=$(VERSIONSTR) -s -w"
CGO_DISABLED="CGO_ENABLED=0"
BUILDFLAGS=-buildvcs=true $(LDFLAGS)
MAKEFLAGS += --silent
.PHONY: all clean run
run:
	go build -ldflags "-X main.Version=$(VERSIONSTR)" -o telnet main.go
	go run -ldflags "-X main.Version=$(VERSIONSTR)" main.go

all: linux darwin windows

windows: windows-x64 windows-arm64

linux: linux-x64 linux-arm64

darwin: darwin-x64 darwin-arm64

windows-arm64:
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -o bin/$(BINARY).windows.arm64 $(BUILDFLAGS) main.go

windows-x64:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/$(BINARY).windows.x64 $(BUILDFLAGS) main.go

linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/$(BINARY).linux-arm64 $(BUILDFLAGS) main.go

linux-x64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/$(BINARY).linux-x64 $(BUILDFLAGS) main.go

darwin-arm64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o bin/$(BINARY).darwin-arm64 $(BUILDFLAGS) main.go

darwin-x64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o bin/$(BINARY).darwin-x64 $(BUILDFLAGS) main.go

clean:
	rm bin/*

.PHONY: no-dirty
no-dirty:
	git diff --exit-code
