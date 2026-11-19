package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Every flag a command takes is on that command's documentation page - a
// subcommand's on its parent's (ip route on ip.md, listen tcp on listen.md) -
// and every flag all commands share is in usage.md. The commands and their
// flags come from --help itself, so a new command or flag is checked without
// touching this test. After v4.4.0, --interface was on the usage page only and
// missing from all nine pages of the commands that take it.
func TestEveryFlagIsDocumented(t *testing.T) {
	flagRe := regexp.MustCompile(`(?m)^\s+(?:-\w, )?(--[a-z0-9-]+)`)
	commandRe := regexp.MustCompile(`(?m)^  ([a-z0-9-]+)\s`)
	section := func(help, heading string) string {
		i := strings.Index(help, "\n"+heading+"\n")
		if i < 0 {
			return ""
		}
		rest := help[i+len(heading)+2:]
		if j := strings.Index(rest, "\n\n"); j >= 0 {
			rest = rest[:j]
		}
		return rest
	}
	help := func(args ...string) string {
		_, stdout, stderr := runShint(t, append(args, "--help")...)
		return stdout + stderr
	}
	page := func(name string) string {
		b, err := os.ReadFile(filepath.Join("docs", "src", name+".md"))
		if err != nil {
			t.Fatalf("docs/src/%s.md: %v", name, err)
		}
		return string(b)
	}
	flags := func(text string) []string {
		var out []string
		for _, m := range flagRe.FindAllStringSubmatch(text, -1) {
			if m[1] != "--help" && m[1] != "--version" {
				out = append(out, m[1])
			}
		}
		return out
	}

	root := help()
	usage := page("usage")
	for _, f := range flags(section(root, "Flags:")) {
		if !strings.Contains(usage, f) {
			t.Errorf("%s, shared by every command, is not in docs/src/usage.md", f)
		}
	}

	var check func(doc, docName string, args ...string)
	check = func(doc, docName string, args ...string) {
		h := help(args...)
		for _, f := range flags(section(h, "Flags:")) {
			if !strings.Contains(doc, f) {
				t.Errorf("shint %s %s is not in docs/src/%s.md", strings.Join(args, " "), f, docName)
			}
		}
		for _, m := range commandRe.FindAllStringSubmatch(section(h, "Available Commands:"), -1) {
			check(doc, docName, append(args, m[1])...)
		}
	}
	commands := 0
	for _, m := range commandRe.FindAllStringSubmatch(section(root, "Available Commands:"), -1) {
		if m[1] == "help" || m[1] == "completion" {
			continue
		}
		commands++
		check(page(m[1]), m[1], m[1])
	}
	if commands < 14 {
		t.Fatalf("found %d commands in --help, expected at least 14: the help format changed?", commands)
	}
}
