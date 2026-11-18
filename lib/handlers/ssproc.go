package handlers

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/dmartsapp/shint/v4/lib"
)

// This file reads Linux's socket tables from /proc. It has no build tag, so
// its tests run everywhere against a made-up /proc; ss_linux.go is what points
// it at the real one.

// procTables are the socket tables of /proc/net. A missing IPv6 table (a
// kernel built without IPv6) is skipped; a missing IPv4 one means /proc is
// not what it should be.
var procTables = []struct {
	file             string
	protocol, family string
	optional         bool
}{
	{"net/tcp", "tcp", "ipv4", false},
	{"net/tcp6", "tcp", "ipv6", true},
	{"net/udp", "udp", "ipv4", false},
	{"net/udp6", "udp", "ipv6", true},
}

// linuxTCPStates are the kernel's TCP state numbers (include/net/tcp_states.h).
var linuxTCPStates = map[uint64]string{
	1: "ESTABLISHED", 2: "SYN-SENT", 3: "SYN-RECV", 4: "FIN-WAIT-1", 5: "FIN-WAIT-2", 6: "TIME-WAIT",
	7: "CLOSE", 8: "CLOSE-WAIT", 9: "LAST-ACK", 10: "LISTEN", 11: "CLOSING", 12: "NEW-SYN-RECV",
}

// procSocket is a socket table row with what it takes to find its process:
// the socket's inode (0 when no process holds it, as in TIME-WAIT) and the
// user it belongs to.
type procSocket struct {
	stat  lib.SSStats
	inode uint64
	uid   uint32
}

// procUserName turns a user ID into a name. It is a variable so tests do not
// depend on the machine's users; in the program it is os/user, which without
// cgo reads /etc/passwd.
var procUserName = userName

// readProcSockets reads the socket tables under root (/proc) and attaches to
// each socket the processes holding it, found through root/<pid>/fd. order is
// the byte order of the kernel that wrote the tables (binary.NativeEndian).
// euid is who is reading: a socket whose process cannot be found is marked
// OwnerHidden when it plausibly belongs to a process this user may not
// inspect.
func readProcSockets(root string, order binary.ByteOrder, euid int) ([]lib.SSStats, error) {
	var sockets []procSocket
	for _, t := range procTables {
		f, err := os.Open(filepath.Join(root, t.file))
		if err != nil {
			if t.optional && errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, procTableError(t.file, err)
		}
		got, err := parseProcNet(f, t.protocol, t.family, order)
		_ = f.Close()
		if err != nil {
			return nil, fmt.Errorf("could not read /proc/%s: %w", t.file, err)
		}
		sockets = append(sockets, got...)
	}

	wanted := make(map[uint64]bool, len(sockets))
	for _, s := range sockets {
		if s.inode != 0 {
			wanted[s.inode] = true
		}
	}
	owners, denied := procOwners(root, wanted)

	names := map[uint32]string{}
	out := make([]lib.SSStats, 0, len(sockets))
	for _, s := range sockets {
		if s.inode == 0 {
			// No process holds it (TIME-WAIT, or a socket the kernel keeps
			// for itself), and the user ID the table shows, 0, is not root's.
			out = append(out, s.stat)
			continue
		}
		uid := s.uid
		s.stat.UID = &uid
		name, seen := names[uid]
		if !seen {
			name = procUserName(uid)
			names[uid] = name
		}
		s.stat.User = name
		switch procs := owners[s.inode]; {
		case len(procs) > 0:
			s.stat.Processes = procs
		case euid != 0 && (denied || int64(uid) != int64(euid)):
			s.stat.OwnerHidden = true
		}
		out = append(out, s.stat)
	}
	return out, nil
}

// procTableError says why a socket table could not be opened, with the
// likely reason where there is one.
func procTableError(file string, err error) error {
	switch {
	case errors.Is(err, fs.ErrPermission) && runtime.GOOS == "android":
		return fmt.Errorf("could not read /proc/%s: permission denied (apps built for Android 10 and later may not read the socket tables)", file)
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("could not read /proc/%s: it does not exist (is /proc mounted?)", file)
	}
	return fmt.Errorf("could not read /proc/%s: %w", file, unwrapPath(err))
}

// unwrapPath drops the "open <path>:" a *fs.PathError adds, when the message
// already names the file.
func unwrapPath(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}

// parseProcNet reads one /proc/net/{tcp,tcp6,udp,udp6} table. Its format has
// been the same for decades: a header line, then one line per socket whose
// first ten fields are a slot number, the local and remote address:port in
// hex, the state in hex, "tx_queue:rx_queue" in hex, two timer fields, the
// user ID, a timeout and the inode. A line that does not fit is an error
// rather than a socket shown wrong.
func parseProcNet(r io.Reader, protocol, family string, order binary.ByteOrder) ([]procSocket, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 512), 4096)
	var out []procSocket
	seen := map[string]bool{}
	for line := 0; sc.Scan(); line++ {
		if line == 0 {
			if !strings.HasPrefix(strings.TrimSpace(sc.Text()), "sl") {
				return nil, errors.New("the first line is not the table's header")
			}
			continue
		}
		f := strings.Fields(sc.Text())
		if len(f) == 0 {
			continue
		}
		s, err := procLine(f, protocol, family, order)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line+1, err)
		}
		// A table larger than a page is read in pieces, and a socket that
		// moves meanwhile can be read twice.
		key := strings.Join(f[1:4], " ") + " " + f[9]
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, s)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func procLine(f []string, protocol, family string, order binary.ByteOrder) (procSocket, error) {
	if len(f) < 10 {
		return procSocket{}, fmt.Errorf("%d fields, want at least 10", len(f))
	}
	local, err := procAddrPort(f[1], family, order)
	if err != nil {
		return procSocket{}, fmt.Errorf("local address %q: %w", f[1], err)
	}
	remote, err := procAddrPort(f[2], family, order)
	if err != nil {
		return procSocket{}, fmt.Errorf("remote address %q: %w", f[2], err)
	}
	st, err := strconv.ParseUint(f[3], 16, 8)
	if err != nil || len(f[3]) != 2 {
		return procSocket{}, fmt.Errorf("state %q is not a hex byte", f[3])
	}
	tx, rx, ok := strings.Cut(f[4], ":")
	sendQ, err1 := strconv.ParseUint(tx, 16, 32)
	recvQ, err2 := strconv.ParseUint(rx, 16, 32)
	if !ok || err1 != nil || err2 != nil {
		return procSocket{}, fmt.Errorf("queues %q are not tx:rx in hex", f[4])
	}
	uid, err := strconv.ParseUint(f[7], 10, 32)
	if err != nil {
		return procSocket{}, fmt.Errorf("user ID %q is not a number", f[7])
	}
	inode, err := strconv.ParseUint(f[9], 10, 64)
	if err != nil {
		return procSocket{}, fmt.Errorf("inode %q is not a number", f[9])
	}

	state := linuxTCPStates[st]
	if protocol == "udp" {
		// UDP uses two of the TCP numbers: 7 (close) for a socket with no
		// peer, 1 (established) for a connected one.
		state = map[uint64]string{1: "ESTABLISHED", 7: "UNCONN"}[st]
	}
	if state == "" {
		state = fmt.Sprintf("UNKNOWN-%d", st)
	}
	sq, rq := uint32(sendQ), uint32(recvQ)
	s := procSocket{
		stat: lib.SSStats{
			Protocol: protocol, Family: family, State: state,
			LocalAddress: local.Addr().String(), LocalPort: int(local.Port()),
			RecvQ: &rq, SendQ: &sq,
		},
		inode: inode,
		uid:   uint32(uid),
	}
	setRemote(&s.stat, remote)
	return s, nil
}

// procAddrPort decodes "0100007F:0016": the address is the kernel's in-memory
// bytes printed as 32-bit words in the kernel's byte order (one word for IPv4,
// four for IPv6), and the port is a plain hex number.
func procAddrPort(s, family string, order binary.ByteOrder) (netip.AddrPort, error) {
	hexAddr, hexPort, ok := strings.Cut(s, ":")
	want := 8
	if family == "ipv6" {
		want = 32
	}
	if !ok || len(hexAddr) != want || len(hexPort) != 4 {
		return netip.AddrPort{}, errors.New("not address:port in hex")
	}
	port, err := strconv.ParseUint(hexPort, 16, 16)
	if err != nil {
		return netip.AddrPort{}, errors.New("the port is not hex")
	}
	b := make([]byte, want/2)
	for i := 0; i < want/8; i++ {
		w, err := strconv.ParseUint(hexAddr[i*8:i*8+8], 16, 32)
		if err != nil {
			return netip.AddrPort{}, errors.New("the address is not hex")
		}
		order.PutUint32(b[i*4:], uint32(w))
	}
	addr, _ := netip.AddrFromSlice(b) // 4 or 16 bytes: always valid
	return netip.AddrPortFrom(addr, uint16(port)), nil
}

// setRemote records a socket's peer, if it has one: an unspecified address
// with port 0 is "no peer", and a listening socket has none whatever the
// table holds (Windows leaves garbage in a listener's remote port).
func setRemote(s *lib.SSStats, remote netip.AddrPort) {
	if s.State == "LISTEN" || (remote.Addr().IsUnspecified() && remote.Port() == 0) {
		return
	}
	s.RemoteAddress, s.RemotePort = remote.Addr().String(), int(remote.Port())
}

// procOwners finds which processes hold the sockets wanted, by inode: every
// root/<pid>/fd/<n> that is a link to "socket:[inode]". denied is whether the
// descriptors of some process could not be read - another user's, without
// root - so a socket with no process found may be one of theirs.
func procOwners(root string, wanted map[uint64]bool) (owners map[uint64][]lib.SSProcess, denied bool) {
	owners = map[uint64][]lib.SSProcess{}
	entries, err := os.ReadDir(root)
	if err != nil {
		return owners, true
	}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 {
			continue
		}
		fdDir := filepath.Join(root, e.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			if errors.Is(err, fs.ErrPermission) {
				denied = true
			}
			continue // or the process has exited meanwhile
		}
		name, named := "", false
		for _, fd := range fds {
			n, err := strconv.Atoi(fd.Name())
			if err != nil {
				continue
			}
			link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil {
				continue
			}
			inner, ok := strings.CutPrefix(link, "socket:[")
			inner, ok2 := strings.CutSuffix(inner, "]")
			if !ok || !ok2 {
				continue
			}
			inode, err := strconv.ParseUint(inner, 10, 64)
			if err != nil || !wanted[inode] {
				continue
			}
			if !named {
				name, named = procName(root, e.Name()), true
			}
			fdNum := n
			owners[inode] = append(owners[inode], lib.SSProcess{PID: pid, Name: name, FD: &fdNum})
		}
	}
	for _, procs := range owners {
		slices.SortFunc(procs, func(a, b lib.SSProcess) int {
			if a.PID != b.PID {
				return a.PID - b.PID
			}
			return *a.FD - *b.FD
		})
	}
	return owners, denied
}

// procName is a process's short name from root/<pid>/comm (at most 15
// characters, set by the process itself, so printed escaped), or "" if it
// has gone.
func procName(root, pid string) string {
	f, err := os.Open(filepath.Join(root, pid, "comm"))
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	b, _ := io.ReadAll(io.LimitReader(f, 64))
	return strings.TrimSuffix(string(b), "\n")
}
