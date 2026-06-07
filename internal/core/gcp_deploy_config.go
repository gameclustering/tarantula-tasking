package core

import (
	"encoding/json"
	"io"
	"os"
)

type GcpServiceConfig struct {
	Name        string `json:"name"`
	Network     string `json:"network"`
	HttpBinding string `json:"httpBinding"`
}

type GcpPhaseConfig struct {
	Zone           string             `json:"zone"`
	Prefix         string             `json:"prefix"`
	MachineType    string             `json:"machineType"`
	ImageType      string             `json:"imageType"`
	InstanceNumber int                `json:"instanceNumber"`
	Description    string             `json:"description"`
	Services       []GcpServiceConfig `json:"services"`
}

type GcpEnvConfig struct {
	Build  GcpPhaseConfig `json:"build"`
	Deploy GcpPhaseConfig `json:"deploy"`
	Test   GcpPhaseConfig `json:"test"`
}

type GcpDeployConfig struct {
	raw map[string]GcpEnvConfig
}

func LoadGcpDeployConfig(path string) (*GcpDeployConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	var raw map[string]GcpEnvConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return &GcpDeployConfig{raw: raw}, nil
}

// Resolve returns the phase config for the given env, falling back field-by-field to default.
func (c *GcpDeployConfig) Resolve(env, phase string) GcpPhaseConfig {
	def := c.phaseOf("default", phase)
	if env == "default" {
		return def
	}
	return mergeGcpPhase(c.phaseOf(env, phase), def)
}

func (c *GcpDeployConfig) phaseOf(env, phase string) GcpPhaseConfig {
	envCfg, ok := c.raw[env]
	if !ok {
		return GcpPhaseConfig{}
	}
	switch phase {
	case "build":
		return envCfg.Build
	case "deploy":
		return envCfg.Deploy
	case "test":
		return envCfg.Test
	default:
		return GcpPhaseConfig{}
	}
}

func mergeGcpPhase(primary, fallback GcpPhaseConfig) GcpPhaseConfig {
	if primary.Zone == "" {
		primary.Zone = fallback.Zone
	}
	if primary.Prefix == "" {
		primary.Prefix = fallback.Prefix
	}
	if primary.MachineType == "" {
		primary.MachineType = fallback.MachineType
	}
	if primary.ImageType == "" {
		primary.ImageType = fallback.ImageType
	}
	if primary.InstanceNumber == 0 {
		primary.InstanceNumber = fallback.InstanceNumber
	}
	if primary.Description == "" {
		primary.Description = fallback.Description
	}
	if len(primary.Services) == 0 {
		primary.Services = fallback.Services
	}
	return primary
}
