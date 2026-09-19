//go:build race

package handlers

// raceDetectorEnabled lets tests skip a known, unfixable-from-here upstream
// race rather than let it fail every `-race` run. See icmp_test.go.
const raceDetectorEnabled = true
