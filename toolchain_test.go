package main

import (
	"os"
	"regexp"
	"testing"
)

// go.mod's go line is the one place the Go release is chosen: every workflow
// builds with it (go-version-file: go.mod) and the docs fill it in ({{go}}).
// The Dockerfile cannot read it, so this keeps its builder image in step - the
// container images must not be built with another Go than the release binaries
// (a Go patch can be a security fix: 1.27.1 -> 1.27.3 was).
func TestTheDockerfileBuildsWithTheGoOfGoMod(t *testing.T) {
	mod, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	docker, err := os.ReadFile("Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	goLine := regexp.MustCompile(`(?m)^go (\S+)\s*$`).FindSubmatch(mod)
	from := regexp.MustCompile(`(?m)^FROM golang:(\S+?)-alpine\b`).FindSubmatch(docker)
	if goLine == nil || from == nil {
		t.Fatalf("could not find go.mod's go line (%q) or the Dockerfile's golang builder image (%q)", goLine, from)
	}
	if string(from[1]) != string(goLine[1]) {
		t.Errorf("the Dockerfile builds with golang:%s-alpine, but go.mod names go %s: make them the same", from[1], goLine[1])
	}
}
