//go:build linux

package daemon

import (
	"os"
	"testing"
	"time"
)

// TestProcessInfoReadsRealProcStat runs only on CI (ubuntu-latest) since
// this repo's local dev machine is macOS — a GOOS=linux cross-compile
// only proves this builds, not that the /proc field indices or the
// USER_HZ/btime time conversion are actually correct at runtime.
func TestProcessInfoReadsRealProcStat(t *testing.T) {
	identity, parentPID, ok := processInfo(int32(os.Getpid()))
	if !ok {
		t.Fatal("processInfo(self) = ok false, want true")
	}
	if parentPID != int32(os.Getppid()) {
		t.Fatalf("parentPID = %d, want %d (os.Getppid())", parentPID, os.Getppid())
	}
	if identity.Command == "" {
		t.Fatal("identity.Command is empty")
	}
	bootTime, ok := linuxBootTime()
	if !ok {
		t.Fatal("linuxBootTime() = ok false, want true")
	}
	now := time.Now().Unix()
	if identity.StartedAt < bootTime || identity.StartedAt > now {
		t.Fatalf("identity.StartedAt = %d, want between boot time %d and now %d", identity.StartedAt, bootTime, now)
	}
}

func TestParseProcStatHandlesCommWithParensAndSpaces(t *testing.T) {
	// A real comm this weird is rare but not impossible (kernel truncates
	// to 15 chars, but doesn't forbid spaces or parens within that). The
	// bug this guards against: naively splitting the whole line on
	// whitespace, or splitting on the FIRST ')' instead of the last.
	line := "1234 (weird (proc) name) S 5678 1234 1234 0 -1 4194304 100 0 0 0 10 5 0 0 20 0 1 0 999999999 0 0 0 0 0 0 0 0 0 0 0 0 0 0 17 1 0 0 0 0 0 0 0 0 0 0 0 0 0"
	identity, parentPID, ok := parseProcStat(1234, line, 1000000000, true)
	if !ok {
		t.Fatal("parseProcStat() = ok false, want true")
	}
	if identity.Command != "weird (proc) name" {
		t.Fatalf("Command = %q, want %q", identity.Command, "weird (proc) name")
	}
	if identity.PID != 1234 {
		t.Fatalf("PID = %d, want 1234", identity.PID)
	}
	if parentPID != 5678 {
		t.Fatalf("parentPID = %d, want 5678", parentPID)
	}
	if identity.StartedAt != 1000000000+999999999/linuxClockTicksPerSecond {
		t.Fatalf("StartedAt = %d, want %d", identity.StartedAt, 1000000000+999999999/linuxClockTicksPerSecond)
	}
}

func TestParseProcStatRejectsMalformedLine(t *testing.T) {
	if _, _, ok := parseProcStat(1, "no parens here at all", 0, true); ok {
		t.Fatal("parseProcStat() = ok true for a line with no parens, want false")
	}
	if _, _, ok := parseProcStat(1, "1 (short) S", 0, true); ok {
		t.Fatal("parseProcStat() = ok true for a line with too few fields, want false")
	}
}
