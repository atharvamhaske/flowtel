package daemon

import "net"

// ProcessIdentity is one hop in a captured process ancestry chain: enough
// to identify a process instance for an audit trail (which shell/process
// actually spawned this connection), deliberately excluding command-line
// arguments and environment variables.
type ProcessIdentity struct {
	PID       int32  `json:"pid"`
	Command   string `json:"command,omitempty"`
	StartedAt int64  `json:"started_at,omitempty"` // unix seconds, best-effort
}

const maxAncestryDepth = 64

// captureAncestry returns the process ancestry of whatever connected on
// the other end of connection, closest process first, or nil if it can't
// be determined: not a real Unix socket (e.g. net.Pipe in tests),
// permission denied on the peer-credential syscall, or a lookup failure
// partway up the chain. This is best-effort audit enrichment, never an
// error — callers must not fail the connection or drop an event over it.
func captureAncestry(connection net.Conn) []ProcessIdentity {
	unixConn, ok := connection.(*net.UnixConn)
	if !ok {
		return nil
	}
	rawConn, err := unixConn.SyscallConn()
	if err != nil {
		return nil
	}
	var pid int32
	var peerErr error
	if controlErr := rawConn.Control(func(fd uintptr) {
		pid, peerErr = peerPID(int(fd))
	}); controlErr != nil || peerErr != nil {
		return nil
	}

	chain := make([]ProcessIdentity, 0, maxAncestryDepth)
	seen := make(map[int32]struct{}, maxAncestryDepth)
	for pid != 0 && len(chain) < maxAncestryDepth {
		if _, cycle := seen[pid]; cycle {
			break
		}
		seen[pid] = struct{}{}
		identity, parentPID, ok := processInfo(pid)
		if !ok {
			break
		}
		chain = append(chain, identity)
		pid = parentPID
	}
	if len(chain) == 0 {
		return nil
	}
	return chain
}
