//go:build darwin

package daemon

import (
	"strings"

	"golang.org/x/sys/unix"
)

func peerPID(fd int) (int32, error) {
	pid, err := unix.GetsockoptInt(fd, unix.SOL_LOCAL, unix.LOCAL_PEERPID)
	if err != nil {
		return 0, err
	}
	return int32(pid), nil
}

func processInfo(pid int32) (ProcessIdentity, int32, bool) {
	info, err := unix.SysctlKinfoProc("kern.proc.pid", int(pid))
	if err != nil {
		return ProcessIdentity{}, 0, false
	}
	comm := string(info.Proc.P_comm[:])
	if nul := strings.IndexByte(comm, 0); nul != -1 {
		comm = comm[:nul]
	}
	identity := ProcessIdentity{
		PID:       pid,
		Command:   comm,
		StartedAt: int64(info.Proc.P_starttime.Sec),
	}
	return identity, info.Eproc.Ppid, true
}
