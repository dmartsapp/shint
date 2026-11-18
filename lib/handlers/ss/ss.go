package ss

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os/user"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dmartsapp/shint/v4/lib"
	"github.com/dmartsapp/shint/v4/lib/handlers/internal/term"
)

const ssModule = "ss"

// Query is what ss narrows its list to. The zero value lists every socket.
type Query struct {
	TCP, UDP  bool   // only TCP, only UDP; neither (or both) is both
	Listening bool   // only listening sockets: TCP in LISTEN, UDP with no peer
	Ports     []int  // only sockets with one of these ports at either end
	PortsText string // Ports as given back (lib.PortsText), for input_params
	PID       int    // only sockets this process holds; 0 for any
}

// readSockets is where the sockets come from: the platform's socket tables
// (ss_linux.go, ss_windows.go; ss_other.go says the platform is not
// supported). It is a variable so tests can substitute a table.
var readSockets = platformSockets

// userName is the name of a user ID, or "" when it has none this machine
// knows.
func userName(uid uint32) string {
	u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
	if err != nil {
		return ""
	}
	return u.Username
}

// protocolText is the protocols asked for: "tcp", "udp" or "tcp,udp".
func (q Query) protocolText() string {
	switch {
	case q.TCP && !q.UDP:
		return "tcp"
	case q.UDP && !q.TCP:
		return "udp"
	}
	return "tcp,udp"
}

// isListening is whether a socket waits for others: a TCP socket in LISTEN,
// or a UDP socket with no peer (on Windows, every UDP socket: its table lists
// only bound endpoints).
func isListening(s lib.SSStats) bool {
	if s.Protocol == "udp" {
		return s.State == "UNCONN" || s.State == ""
	}
	return s.State == "LISTEN"
}

// matchesButPID is everything Query asks except the process.
func (q Query) matchesButPID(s lib.SSStats) bool {
	if want := q.protocolText(); want != "tcp,udp" && s.Protocol != want {
		return false
	}
	switch lib.NetworkType {
	case "ip4":
		if s.Family != "ipv4" {
			return false
		}
	case "ip6":
		if s.Family != "ipv6" {
			return false
		}
	}
	if q.Listening && !isListening(s) {
		return false
	}
	if len(q.Ports) > 0 {
		_, local := slices.BinarySearch(q.Ports, s.LocalPort)
		_, remote := slices.BinarySearch(q.Ports, s.RemotePort)
		peer := remote && s.RemoteAddress != "" // a socket with no peer has no remote port
		if !local && !peer {
			return false
		}
	}
	return true
}

func (q Query) holds(s lib.SSStats) bool {
	if q.PID == 0 {
		return true
	}
	for _, p := range s.Processes {
		if p.PID == q.PID {
			return true
		}
	}
	return false
}

// describe is the query in words, for "no socket matches".
func (q Query) describe() string {
	var parts []string
	if p := q.protocolText(); p != "tcp,udp" {
		parts = append(parts, p)
	}
	switch lib.NetworkType {
	case "ip4":
		parts = append(parts, "IPv4")
	case "ip6":
		parts = append(parts, "IPv6")
	}
	if q.Listening {
		parts = append(parts, "listening")
	}
	if len(q.Ports) == 1 {
		parts = append(parts, "port "+q.PortsText)
	} else if len(q.Ports) > 1 {
		parts = append(parts, "ports "+q.PortsText)
	}
	if q.PID != 0 {
		parts = append(parts, "held by process "+strconv.Itoa(q.PID))
	}
	return strings.Join(parts, ", ")
}

// sortSockets puts TCP before UDP, listening sockets first, then by local
// port and address and by peer, so the same machine lists the same way twice.
func sortSockets(stats []lib.SSStats) {
	slices.SortStableFunc(stats, func(a, b lib.SSStats) int {
		return cmp.Or(
			cmp.Compare(a.Protocol, b.Protocol),
			cmp.Compare(boolRank(!isListening(a)), boolRank(!isListening(b))),
			cmp.Compare(a.LocalPort, b.LocalPort),
			cmp.Compare(a.LocalAddress, b.LocalAddress),
			cmp.Compare(a.RemoteAddress, b.RemoteAddress),
			cmp.Compare(a.RemotePort, b.RemotePort),
		)
	})
}

func boolRank(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Run lists this machine's TCP and UDP sockets that the query asks
// for, one record per socket with the processes holding it (#68). It reads
// the operating system's socket tables and sends nothing.
//
// It reports true when it listed at least one socket. A table that cannot be
// read, a platform it does not support, and a query nothing matches are
// failed checks: "ss -l --ports 5432" fails when nothing listens on 5432.
func Run(jsonoutput *bool, q Query) (ok bool) {
	start := time.Now()
	all, err := readSockets()

	stats := make([]lib.SSStats, 0, len(all))
	hidden := 0
	for _, s := range all {
		if !q.matchesButPID(s) {
			continue
		}
		if s.OwnerHidden {
			hidden++
		}
		if q.holds(s) {
			stats = append(stats, s)
		}
	}
	sortSockets(stats)

	failure := ""
	switch asked := q.describe(); {
	case err != nil:
		failure = err.Error()
	case len(stats) == 0 && asked != "":
		failure = "no socket matches: " + asked
		if q.PID != 0 && hidden > 0 {
			failure += fmt.Sprintf(" (the processes of %d sockets could not be seen; run as root to include them)", hidden)
		}
	case len(stats) == 0:
		failure = "this machine reports no TCP or UDP sockets"
	}

	if *jsonoutput {
		end := time.Now()
		output := lib.LocalJSONOutput{
			InputParams: lib.InputParams{Mode: ssModule, Protocol: q.protocolText(), Ports: q.PortsText, Listening: q.Listening, PID: q.PID},
			ModuleName:  ssModule,
			Stats:       stats,
			Error:       failure,
			StartTime:   start.UnixMicro(), EndTime: end.UnixMicro(), TotalTimeTaken: end.Sub(start).Microseconds(),
		}
		JS, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(JS))
		return failure == ""
	}

	if failure != "" {
		fmt.Println(lib.LogWithTimestamp(ssModule, "failed "+lib.Fields("error", failure), true))
		return false
	}
	tcp, listening, shownHidden := 0, 0, 0
	for _, s := range stats {
		if s.Protocol == "tcp" {
			tcp++
		}
		if isListening(s) {
			listening++
		}
		if s.OwnerHidden {
			shownHidden++
		}
		fmt.Println(lib.LogWithTimestamp(ssModule, "socket "+lib.Fields(socketFields(s)...), false))
	}
	summary := []any{"sockets", len(stats), "tcp", tcp, "udp", len(stats) - tcp, "listening", listening}
	if shownHidden > 0 {
		summary = append(summary, "owners_hidden", shownHidden)
	}
	fmt.Println(lib.LogWithTimestamp(ssModule, "done "+lib.Fields(append(summary, "total_time", time.Since(start))...), false))
	if shownHidden > 0 {
		fmt.Println(lib.LogWithLevel(ssModule, lib.LevelWarn, "owners hidden "+lib.Fields("sockets", shownHidden,
			"warning", "these sockets belong to processes this user may not inspect, so which process holds them is not shown; run as root to see it")))
	}
	return true
}

// socketFields is one socket as the key=value pairs of its text line. A
// process name is set by the process itself, so it is printed escaped.
func socketFields(s lib.SSStats) []any {
	f := []any{"proto", s.Protocol}
	if s.State != "" {
		f = append(f, "state", s.State)
	}
	f = append(f, "local", hostPort(s.LocalAddress, s.LocalPort))
	if s.RemoteAddress != "" {
		f = append(f, "remote", hostPort(s.RemoteAddress, s.RemotePort))
	}
	if s.RecvQ != nil && *s.RecvQ != 0 {
		f = append(f, "recv_q", *s.RecvQ)
	}
	if s.SendQ != nil && *s.SendQ != 0 {
		f = append(f, "send_q", *s.SendQ)
	}
	switch {
	case len(s.Processes) == 1:
		p := s.Processes[0]
		f = append(f, "process", processName(p.Name), "pid", p.PID)
		if p.FD != nil {
			f = append(f, "fd", *p.FD)
		}
	case len(s.Processes) > 1:
		list := make([]string, 0, len(s.Processes))
		for _, p := range s.Processes {
			item := fmt.Sprintf("%s(pid=%d", processName(p.Name), p.PID)
			if p.FD != nil {
				item += fmt.Sprintf(",fd=%d", *p.FD)
			}
			list = append(list, item+")")
		}
		f = append(f, "processes", "["+strings.Join(list, ",")+"]")
	case s.OwnerHidden:
		f = append(f, "owner", "hidden")
	}
	switch {
	case s.User != "":
		f = append(f, "user", term.Escape([]byte(s.User)))
	case s.UID != nil:
		f = append(f, "uid", *s.UID)
	}
	return f
}

// processName is a process name fit to print: escaped, and "?" when it could
// not be read (the process exited while the list was made).
func processName(name string) string {
	if name == "" {
		return "?"
	}
	return term.Escape([]byte(name))
}

// hostPort is address:port, with an IPv6 address in brackets.
func hostPort(addr string, port int) string {
	if strings.Contains(addr, ":") {
		return "[" + addr + "]:" + strconv.Itoa(port)
	}
	return addr + ":" + strconv.Itoa(port)
}
