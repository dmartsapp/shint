BINARY=shint
COMMIT=$(shell git rev-list -1 HEAD)
VERSION=$(or $(shell git tag --contains $(COMMIT) 2>/dev/null | head -n1),dev)
VERSIONSTR="$(VERSION)-$(shell git show --no-patch --format="%cd" --date='format:%d%m%Y%H%M%S' $(COMMIT))"
LDFLAGS=-ldflags "-X main.Version=$(VERSIONSTR) -s -w"
BUILDFLAGS=-buildvcs=true -trimpath $(LDFLAGS)
MAKEFLAGS += --silent

.PHONY: all all-platforms clean run \
	linux darwin windows freebsd openbsd netbsd solaris android aix illumos \
	linux-amd64 linux-arm64 linux-arm linux-ppc64le darwin-amd64 darwin-arm64 windows-amd64 windows-arm64 \
	freebsd-amd64 freebsd-arm64 openbsd-amd64 openbsd-arm64 netbsd-amd64 netbsd-arm64 \
	solaris-amd64 android-arm64 aix-ppc64 illumos-amd64 no-dirty

run:
	CGO_ENABLED=0 go build -trimpath -ldflags "-X main.Version=$(VERSIONSTR)" -o $(BINARY) main.go
	go run -ldflags "-X main.Version=$(VERSIONSTR)" main.go

# Common desktop triad for quick local builds.
all: linux darwin windows

# Every platform the CI release workflow builds.
all-platforms: all freebsd openbsd netbsd solaris android aix illumos

windows: windows-amd64 windows-arm64

# arm (32-bit, GOARM=6) covers the older boards a 64-bit-only build leaves out - Raspberry
# Pi 1/Zero and a lot of embedded/IoT Linux are still 32-bit userlands; GOARM=6 keeps it
# running on ARMv6 hardware too, forward-compatible with ARMv7. ppc64le is IBM Power Linux.
linux: linux-amd64 linux-arm64 linux-arm linux-ppc64le

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

# Go only supports aix/ppc64 (IBM POWER); there is no aix/amd64 or aix/arm64 port.
aix: aix-ppc64

# illumos (OmniOS, SmartOS, ...) is its own GOOS, distinct from solaris - and, like solaris,
# Go only supports it on amd64.
illumos: illumos-amd64

windows-arm64:
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -o bin/$(BINARY).windows.arm64.exe $(BUILDFLAGS) main.go

windows-amd64:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/$(BINARY).windows.amd64.exe $(BUILDFLAGS) main.go

linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/$(BINARY).linux-arm64 $(BUILDFLAGS) main.go

linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/$(BINARY).linux-amd64 $(BUILDFLAGS) main.go

linux-arm:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build -o bin/$(BINARY).linux-arm $(BUILDFLAGS) main.go

linux-ppc64le:
	CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le go build -o bin/$(BINARY).linux-ppc64le $(BUILDFLAGS) main.go

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

aix-ppc64:
	CGO_ENABLED=0 GOOS=aix GOARCH=ppc64 go build -o bin/$(BINARY).aix-ppc64 $(BUILDFLAGS) main.go

illumos-amd64:
	CGO_ENABLED=0 GOOS=illumos GOARCH=amd64 go build -o bin/$(BINARY).illumos-amd64 $(BUILDFLAGS) main.go

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

.PHONY: help check check-quick attest hooks test-full test-live tools tools-quick fmt-check vet vet-host test test-go test-quick test-battery lint vuln docs-check workflows release-check readme-reconcile

help:
	echo "make check       fmt, vet, race tests, black-box battery, lint, vulncheck, docs and workflow checks (no network)"
	echo "make attest      make check, then sign a report of it and attach it to the commit as a git note, so CI on main runs a short subset instead of everything"
	echo "make hooks       install the pre-push hook, which runs make attest when you push main or a release tag without a signed report"
	echo "make check-quick the short subset CI runs when that status exists: fmt, vet, quick tests, CLI battery groups, vulncheck, docs"
	echo "make test-live   smoke test against real hosts (needs the internet and ICMP)"
	echo "make test-full   check + test-live"
	echo "make release-check   on a release branch, after the release commit: is it safe to fast-forward main and tag?"
	echo "make readme-reconcile   after a release: a branch readme/main-<tag> with main's README brought up to date (TAG=vX.Y.Z, PUSH=1 to push and open the pull request)"
	echo "make test        the Go tests (race detector), then the black-box battery in test/battery"
	echo "make test-go     just the Go tests, with the race detector"
	echo "make test-battery  just the black-box battery (BATTERY_ARGS=\"--group F\" to pick, --list to see the cases)"
	echo "make <platform>  build for one platform (linux-amd64, darwin-arm64, ...); all-platforms builds every one"

check: tools fmt-check vet test lint vuln docs-check workflows
	echo "==> all checks passed"

test-full: check test-live
	echo "==> full test run passed"

# Run the whole of make check on this machine, sign a report of it with your SSH key and attach
# it to the commit as a git note (refs/notes/checks), which it pushes. The Check workflow, on a
# push to main, verifies the signature (against .github/allowed_signers), that the report is for
# the tree pushed, complete and recent - and only then runs check-quick instead of check. Run it
# on a clean tree at the commit that will be merged: a fast-forward merge keeps commit ids, so
# the note on the branch tip is the note on main's tip. See .github/scripts/check-report.py.
attest:
	bash .github/scripts/attest.sh

# The pre-push hook: pushing main or a release tag with no valid signed report runs make attest
# first. A convenience on this machine; the gate is CI, which runs everything without a report.
hooks:
	git config core.hooksPath .githooks
	echo "pre-push hook installed (git config core.hooksPath .githooks). Skip it once with SHINT_SKIP_ATTEST=1."

# What CI runs when the whole suite has already passed here (a valid local check receipt):
# what is cheap, what can differ on Linux, and what goes stale - a vulnerability database
# gets new entries every day, so govulncheck always runs fresh. Left to the receipt: the
# race detector, the lib/handlers tests, the full battery, golangci-lint (also run again by
# the release's own Lint workflow and gate), the other operating systems' vet and the
# workflow tests. About a minute, against three and a half for make check.
QUICK_BATTERY_ARGS = --group A --group B --group C --group D
check-quick: tools-quick fmt-check vet-host test-quick vuln docs-check
	$(MAKE) --no-print-directory test-battery BATTERY_ARGS="$(QUICK_BATTERY_ARGS)"
	echo "==> quick checks passed"

tools-quick:
	missing=0; \
	command -v $(GOVULNCHECK) >/dev/null 2>&1 || { echo "missing govulncheck: go install golang.org/x/vuln/cmd/govulncheck@latest"; missing=1; }; \
	command -v python3 >/dev/null 2>&1 || { echo "missing python3"; missing=1; }; \
	[ $$missing -eq 0 ] || exit 1

vet-host:
	echo "==> go vet"
	go vet ./...

test-quick:
	echo "==> go test (the CLI tests and lib)"
	go test -count=1 . ./lib

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
	# Go's int is 32 bits here, unlike every other platform above (and unlike every other
	# shint target except linux/arm itself): a boundary-test sentinel like math.MaxInt64
	# does not even compile as an int on this one. Adding it once (issue: none yet filed;
	# found by vetting a real release target for the first time) is what catches the next one.
	echo "    go vet (GOOS=linux GOARCH=arm)"; GOOS=linux GOARCH=arm GOARM=6 go vet ./... || exit 1

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
# fake gh, and so on) and actionlint. actionlint runs without shellcheck, so the
# result does not depend on whether the machine has it: CI runners do and most
# laptops do not, and the style findings it reports in the older workflow scripts
# (unquoted variables and the like) are tracked in issue #51 - turn it back on
# when they are fixed.
workflows:
	echo "==> workflows"
	python3 .github/scripts/test_check_workflow_triggers.py
	python3 .github/scripts/check-workflow-triggers.py
	bash .github/scripts/test-verify-release-tag.sh
	bash .github/scripts/test-write-ci-failure-issue.sh
	bash .github/scripts/test-write-checksums.sh
	bash .github/scripts/test-release-check.sh
	python3 .github/scripts/test_notify_slack.py
	python3 .github/scripts/test_readme_reconcile.py
	bash .github/scripts/test-readme-release.sh
	python3 .github/scripts/test_check_report.py
	bash .github/scripts/test-attest.sh
	bash .github/scripts/test-changelog-section.sh
	$(ACTIONLINT) -shellcheck= .github/workflows/*.yaml

# After a release: a branch off main with its README reconciled with the release tags,
# the changelog, the milestones and the binary's --help (see docs: Releases and tagging).
# Nothing is merged. The release workflow "README Reconcile" does the same after every tag.
#   make readme-reconcile TAG=v4.2.0            branch + commit, for you to read and push
#   make readme-reconcile TAG=v4.2.0 PUSH=1     ... and push it and open the pull request
readme-reconcile:
	TAG="$(TAG)" PUSH="$(PUSH)" bash .github/scripts/readme-release.sh

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
