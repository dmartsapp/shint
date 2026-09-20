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

# ---------------------------------------------------------------------------
# Checks. The Makefile is the one place to build and test from: nothing in CI
# or the release checklist should run these commands any other way.
#
#   make check       everything that needs no network - run before every push
#   make test-live   the smoke test against real hosts (internet + ICMP)
#   make test-full   check + test-live - run before a release
#
# The tools are run from PATH; override a path with e.g.
#   make check GOLANGCI_LINT=/path/to/golangci-lint
# ---------------------------------------------------------------------------
GOLANGCI_LINT ?= golangci-lint
GOVULNCHECK ?= govulncheck
ACTIONLINT ?= actionlint
# Keep in step with the golangci-lint version pinned in .github/workflows/*.yaml.
GOLANGCI_LINT_VERSION = 2.13.2

.PHONY: help check test-full test-live tools fmt-check vet test test-go test-battery lint vuln docs-check workflows release-check

help:
	echo "make check       fmt, vet, race tests, black-box battery, lint, vulncheck, docs and workflow checks (no network)"
	echo "make test-live   smoke test against real hosts (needs the internet and ICMP)"
	echo "make test-full   check + test-live"
	echo "make release-check   on a release branch, after the release commit: is it safe to fast-forward main and tag?"
	echo "make test        the Go tests (race detector), then the black-box battery in test/battery"
	echo "make test-go     just the Go tests, with the race detector"
	echo "make test-battery  just the black-box battery (BATTERY_ARGS=\"--group F\" to pick, --list to see the cases)"
	echo "make <platform>  build for one platform (linux-amd64, darwin-arm64, ...); all-platforms builds every one"

check: tools fmt-check vet test lint vuln docs-check workflows
	echo "==> all checks passed"

test-full: check test-live
	echo "==> full test run passed"

# Fail early, with the install command, rather than half-way through a run.
tools:
	missing=0; \
	command -v $(GOLANGCI_LINT) >/dev/null 2>&1 || { echo "missing golangci-lint $(GOLANGCI_LINT_VERSION): go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v$(GOLANGCI_LINT_VERSION)"; missing=1; }; \
	command -v $(GOVULNCHECK) >/dev/null 2>&1 || { echo "missing govulncheck: go install golang.org/x/vuln/cmd/govulncheck@latest"; missing=1; }; \
	command -v $(ACTIONLINT) >/dev/null 2>&1 || { echo "missing actionlint: go install github.com/rhysd/actionlint/cmd/actionlint@latest"; missing=1; }; \
	command -v python3 >/dev/null 2>&1 || { echo "missing python3 (the documentation generator and workflow checks use it)"; missing=1; }; \
	[ $$missing -eq 0 ] || exit 1; \
	$(GOLANGCI_LINT) --version | grep -q "version $(GOLANGCI_LINT_VERSION) " || { echo "golangci-lint must be v$(GOLANGCI_LINT_VERSION), as CI uses; found: $$($(GOLANGCI_LINT) --version | head -n1)"; exit 1; }

fmt-check:
	echo "==> gofmt"
	unformatted="$$(gofmt -l .)"; [ -z "$$unformatted" ] || { echo "not gofmt-formatted (run gofmt -w):"; echo "$$unformatted"; exit 1; }

# Also vets for the other operating systems, which compiles their test files
# too: a test that only builds on Unix (issue #13) fails here, not in a user's
# hands.
vet:
	echo "==> go vet"
	go vet ./...
	for os in windows freebsd solaris; do echo "    go vet (GOOS=$$os)"; GOOS=$$os GOARCH=amd64 go vet ./... || exit 1; done

test: test-go test-battery

test-go:
	echo "==> go test -race"
	go test -race ./...

# About 350 black-box cases: the real binary, run as a user runs it, against loopback servers that
# misbehave in every way the tests can think of (test/battery, and docs: Testing). A case for a bug
# that is still open must keep failing and is listed in test/battery/known_issues.py; the run fails
# on anything else, and when a listed bug is fixed. Needs python3; ping cases need unprivileged ICMP
# and are skipped without it. BATTERY_ARGS passes options through (--group, --only, --list, --verbose).
BATTERY_ARGS ?=
test-battery:
	echo "==> black-box battery"
	tmp="$$(mktemp -d)"; trap 'rm -rf "$$tmp"' EXIT; \
	CGO_ENABLED=0 go build -o "$$tmp/shint" . && python3 test/battery/run.py --bin "$$tmp/shint" $(BATTERY_ARGS)

lint:
	echo "==> golangci-lint"
	$(GOLANGCI_LINT) run ./...

vuln:
	echo "==> govulncheck"
	$(GOVULNCHECK) ./...

# The documentation site: generator tests, then "the committed pages are
# current and every link resolves".
docs-check:
	echo "==> documentation"
	python3 docs/test_build.py
	python3 docs/build.py --check

# The release pipeline. Workflows only run on a release tag, so they cannot be
# tried out any other way: the trigger rule, the scripts they call (against a
# fake gh, and so on) and actionlint.
workflows:
	echo "==> workflows"
	python3 .github/scripts/test_check_workflow_triggers.py
	python3 .github/scripts/check-workflow-triggers.py
	bash .github/scripts/test-verify-release-tag.sh
	bash .github/scripts/test-write-ci-failure-issue.sh
	bash .github/scripts/test-write-checksums.sh
	bash .github/scripts/test-release-check.sh
	python3 .github/scripts/test_notify_slack.py
	$(ACTIONLINT) .github/workflows/*.yaml

# Needs the internet and unprivileged ICMP; builds ./shint, runs one check per
# command against real hosts, and removes the binary again.
test-live:
	echo "==> live smoke test"
	bash basic_module_test.sh

# Release day preflight, on the release branch after the release commit: the
# branch is on top of main, main's readme.md is untouched (a release branch's
# README never reaches main), the version, changelog and release commit agree,
# and the tag is free. Not part of `check`: it only makes sense on a finished
# release branch. See docs/src/tech-release.md.
release-check:
	bash .github/scripts/release-check.sh
