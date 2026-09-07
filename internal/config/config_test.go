package config_test

import (
	"errors"
	"testing"

	"github.com/atharvamhaske/flowtel/internal/config"
	"github.com/atharvamhaske/flowtel/pkg/model"
)

func TestFromEnv(t *testing.T) {
	t.Setenv(config.EnvHarness, "pi")
	t.Setenv(config.EnvProfile, string(model.ProfileBoth))
	t.Setenv(config.EnvOTLPEndpoint, "https://collector.example.invalid")
	t.Setenv(config.EnvExportTimeout, "30")

	got, err := config.FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() error = %v", err)
	}
	if got.Harness != "pi" || got.Profile != model.ProfileBoth || got.ExportTimeout != 30 {
		t.Fatalf("FromEnv() = %+v", got)
	}
}

func TestFromEnvRejectsInvalidProfile(t *testing.T) {
	t.Setenv(config.EnvHarness, "pi")
	t.Setenv(config.EnvProfile, "unsupported")

	_, err := config.FromEnv()
	if !errors.Is(err, config.ErrInvalid) {
		t.Fatalf("FromEnv() error = %v, want %v", err, config.ErrInvalid)
	}
}
