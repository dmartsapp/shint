BINARY=shint
COMMIT=$(shell git rev-list -1 HEAD)
VERSION=$(or $(shell git tag --contains $(COMMIT) 2>/dev/null | head -n1),dev)
VERSIONSTR="$(VERSION)-$(shell git show --no-patch --format="%cd" --date='format:%d%m%Y%H%M%S' $(COMMIT))"
LDFLAGS=-ldflags "-X main.Version=$(VERSIONSTR) -s -w"
BUILDFLAGS=-buildvcs=true -trimpath $(LDFLAGS)
MAKEFLAGS += --silent

.PHONY: all all-platforms clean run \
	linux darwin windows freebsd openbsd netbsd solaris android \
	linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64 windows-arm64 \
	freebsd-amd64 freebsd-arm64 openbsd-amd64 openbsd-arm64 netbsd-amd64 netbsd-arm64 \
	solaris-amd64 android-arm64 no-dirty

run:
	CGO_ENABLED=0 go build -trimpath -ldflags "-X main.Version=$(VERSIONSTR)" -o $(BINARY) main.go
	go run -ldflags "-X main.Version=$(VERSIONSTR)" main.go

# Common desktop triad for quick local builds.
all: linux darwin windows

# Every platform the CI release workflow builds.
all-platforms: all freebsd openbsd netbsd solaris android

windows: windows-amd64 windows-arm64

linux: linux-amd64 linux-arm64

darwin: darwin-amd64 darwin-arm64

freebsd: freebsd-amd64 freebsd-arm64

openbsd: openbsd-amd64 openbsd-arm64

netbsd: netbsd-amd64 netbsd-arm64

# Go only supports solaris/amd64.
solaris: solaris-amd64

# android/amd64 needs cgo (external linking) for its libc syscall shims,
# which would defeat the point of a small static CGO_ENABLED=0 binary.
# android/arm64 covers real devices (and Termux) and builds fine without it.
android: android-arm64

windows-arm64:
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -o bin/$(BINARY).windows.arm64.exe $(BUILDFLAGS) main.go

windows-amd64:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/$(BINARY).windows.amd64.exe $(BUILDFLAGS) main.go

linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/$(BINARY).linux-arm64 $(BUILDFLAGS) main.go

linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/$(BINARY).linux-amd64 $(BUILDFLAGS) main.go

darwin-arm64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o bin/$(BINARY).darwin-arm64 $(BUILDFLAGS) main.go

darwin-amd64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o bin/$(BINARY).darwin-amd64 $(BUILDFLAGS) main.go

freebsd-amd64:
	CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build -o bin/$(BINARY).freebsd-amd64 $(BUILDFLAGS) main.go

freebsd-arm64:
	CGO_ENABLED=0 GOOS=freebsd GOARCH=arm64 go build -o bin/$(BINARY).freebsd-arm64 $(BUILDFLAGS) main.go

openbsd-amd64:
	CGO_ENABLED=0 GOOS=openbsd GOARCH=amd64 go build -o bin/$(BINARY).openbsd-amd64 $(BUILDFLAGS) main.go

openbsd-arm64:
	CGO_ENABLED=0 GOOS=openbsd GOARCH=arm64 go build -o bin/$(BINARY).openbsd-arm64 $(BUILDFLAGS) main.go

netbsd-amd64:
	CGO_ENABLED=0 GOOS=netbsd GOARCH=amd64 go build -o bin/$(BINARY).netbsd-amd64 $(BUILDFLAGS) main.go

netbsd-arm64:
	CGO_ENABLED=0 GOOS=netbsd GOARCH=arm64 go build -o bin/$(BINARY).netbsd-arm64 $(BUILDFLAGS) main.go

solaris-amd64:
	CGO_ENABLED=0 GOOS=solaris GOARCH=amd64 go build -o bin/$(BINARY).solaris-amd64 $(BUILDFLAGS) main.go

android-arm64:
	CGO_ENABLED=0 GOOS=android GOARCH=arm64 go build -o bin/$(BINARY).android-arm64 $(BUILDFLAGS) main.go

clean:
	rm -f bin/*

no-dirty:
	git diff --exit-code
