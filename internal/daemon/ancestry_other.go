//go:build !linux && !darwin

package daemon

import "errors"

func peerPID(int) (int32, error) {
	return 0, errors.New("process ancestry capture is not supported on this platform")
}

func processInfo(int32) (ProcessIdentity, int32, bool) {
	return ProcessIdentity{}, 0, false
}
