//go:build linux

package daemon

import (
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// linuxClockTicksPerSecond is USER_HZ, the unit /proc/[pid]/stat reports
// process start time in. It has been 100 on every mainstream Linux distro
// and kernel for decades; x/sys exposes no sysconf(_SC_CLK_TCK) helper,
// so this is the same fixed assumption most /proc-reading tools make.
const linuxClockTicksPerSecond = 100

func peerPID(fd int) (int32, error) {
	cred, err := unix.GetsockoptUcred(fd, unix.SOL_SOCKET, unix.SO_PEERCRED)
	if err != nil {
		return 0, err
	}
	return cred.Pid, nil
}

func processInfo(pid int32) (ProcessIdentity, int32, bool) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(int(pid)) + "/stat")
	if err != nil {
		return ProcessIdentity{}, 0, false
	}
	bootTime, bootTimeOK := linuxBootTime()
	return parseProcStat(pid, string(data), bootTime, bootTimeOK)
}

// parseProcStat parses one /proc/[pid]/stat line: "pid (comm) state ppid
// ... starttime ...". comm can itself contain spaces and parens, so this
// splits on the LAST ')' rather than naively splitting the whole line on
// whitespace. Pure (no I/O) so the parsing logic is testable without a
// real /proc filesystem.
func parseProcStat(pid int32, line string, bootTime int64, bootTimeOK bool) (ProcessIdentity, int32, bool) {
	openParen := strings.IndexByte(line, '(')
	closeParen := strings.LastIndexByte(line, ')')
	if openParen == -1 || closeParen == -1 || openParen > closeParen {
		return ProcessIdentity{}, 0, false
	}
	comm := line[openParen+1 : closeParen]
	fields := strings.Fields(line[closeParen+1:])
	// fields[0]=state, fields[1]=ppid, ..., fields[19]=starttime (ticks
	// since boot), matching proc(5)'s field numbering minus the pid/comm
	// pair already consumed above.
	if len(fields) < 20 {
		return ProcessIdentity{}, 0, false
	}
	ppid, err := strconv.ParseInt(fields[1], 10, 32)
	if err != nil {
		return ProcessIdentity{}, 0, false
	}
	identity := ProcessIdentity{PID: pid, Command: comm}
	if startTicks, err := strconv.ParseInt(fields[19], 10, 64); err == nil && bootTimeOK {
		identity.StartedAt = bootTime + startTicks/linuxClockTicksPerSecond
	}
	return identity, int32(ppid), true
}

func linuxBootTime() (int64, bool) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, false
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if after, ok := strings.CutPrefix(line, "btime "); ok {
			seconds, err := strconv.ParseInt(strings.TrimSpace(after), 10, 64)
			if err != nil {
				return 0, false
			}
			return seconds, true
		}
	}
	return 0, false
}
