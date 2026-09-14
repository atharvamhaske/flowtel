package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/atharvamhaske/flowtel/internal/config"
)

func TestCheckEnvVars(t *testing.T) {
	t.Run("all set correctly", func(t *testing.T) {
		t.Setenv(config.EnvHarness, "pi")
		t.Setenv(config.EnvProfile, "both")
		t.Setenv(config.EnvExportTimeout, "30")
		result := checkEnvVars()
		if result.status != statusPass {
			t.Fatalf("status = %v, message = %q, want statusPass", result.status, result.message)
		}
	})

	t.Run("missing harness fails", func(t *testing.T) {
		t.Setenv(config.EnvHarness, "")
		t.Setenv(config.EnvProfile, "")
		t.Setenv(config.EnvExportTimeout, "")
		result := checkEnvVars()
		if result.status != statusFail {
			t.Fatalf("status = %v, want statusFail", result.status)
		}
	})

	t.Run("invalid profile fails", func(t *testing.T) {
		t.Setenv(config.EnvHarness, "pi")
		t.Setenv(config.EnvProfile, "not-a-real-profile")
		t.Setenv(config.EnvExportTimeout, "")
		result := checkEnvVars()
		if result.status != statusFail {
			t.Fatalf("status = %v, want statusFail", result.status)
		}
	})

	t.Run("negative export timeout fails", func(t *testing.T) {
		t.Setenv(config.EnvHarness, "pi")
		t.Setenv(config.EnvProfile, "")
		t.Setenv(config.EnvExportTimeout, "-5")
		result := checkEnvVars()
		if result.status != statusFail {
			t.Fatalf("status = %v, want statusFail", result.status)
		}
	})
}

func TestCheckPiBinary(t *testing.T) {
	t.Run("override to missing path fails", func(t *testing.T) {
		t.Setenv("FLOWTEL_PI", filepath.Join(t.TempDir(), "does-not-exist"))
		result := checkPiBinary()
		if result.status != statusFail {
			t.Fatalf("status = %v, want statusFail", result.status)
		}
	})

	t.Run("override to existing path passes", func(t *testing.T) {
		bin := filepath.Join(t.TempDir(), "fake-pi")
		if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatalf("write fake binary: %v", err)
		}
		t.Setenv("FLOWTEL_PI", bin)
		result := checkPiBinary()
		if result.status != statusPass {
			t.Fatalf("status = %v, want statusPass", result.status)
		}
	})
}

func TestCheckCollectorEndpoint(t *testing.T) {
	t.Run("unset endpoint warns", func(t *testing.T) {
		t.Setenv(config.EnvOTLPEndpoint, "")
		result := checkCollectorEndpoint()
		if result.status != statusWarn {
			t.Fatalf("status = %v, want statusWarn", result.status)
		}
	})

	t.Run("invalid url fails", func(t *testing.T) {
		t.Setenv(config.EnvOTLPEndpoint, "::not a url::")
		result := checkCollectorEndpoint()
		if result.status != statusFail {
			t.Fatalf("status = %v, want statusFail", result.status)
		}
	})
}
