package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/atharvamhaske/flowtel/pkg/model"
)

const (
	EnvHarness       = "FLOWTEL_HARNESS"
	EnvProfile       = "FLOWTEL_ATTRIBUTE_PROFILE"
	EnvOTLPEndpoint  = "FLOWTEL_OTLP_ENDPOINT"
	EnvExportTimeout = "FLOWTEL_EXPORT_TIMEOUT_SECONDS"
)

var ErrInvalid = errors.New("config: invalid configuration")

type Config struct {
	Harness       string
	Profile       model.Profile
	OTLPEndpoint  string
	ExportTimeout int
}

func FromEnv() (Config, error) {
	config := Config{
		Harness:      os.Getenv(EnvHarness),
		Profile:      model.Profile(os.Getenv(EnvProfile)),
		OTLPEndpoint: os.Getenv(EnvOTLPEndpoint),
	}
	if raw := os.Getenv(EnvExportTimeout); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("parse %s: %w", EnvExportTimeout, err)
		}
		config.ExportTimeout = seconds
	}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (c Config) Validate() error {
	if c.Harness == "" || !c.Profile.Valid() {
		return ErrInvalid
	}
	if c.ExportTimeout < 0 {
		return fmt.Errorf("%w: export timeout cannot be negative", ErrInvalid)
	}
	return nil
}
