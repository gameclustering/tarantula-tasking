package core

import (
	"encoding/json"
	"io"
	"os"
)

// PhaseConfig is a platform-agnostic deployment phase.
// Platform-specific values (zone, region, machineType, etc.) go in Settings.
type PhaseConfig struct {
	Prefix         string            `json:"prefix"`
	InstanceNumber int               `json:"instanceNumber"`
	SshUser        string            `json:"sshUser"`
	BuildHost      string            `json:"buildHost"` // pre-existing build server; empty means provision one
	Description    string            `json:"description"`
	Services       []GcpServiceConfig `json:"services"` // fields are platform-agnostic
	Settings       map[string]string  `json:"settings"` // platform-specific key-value pairs
}

type DeployEnvConfig struct {
	Build  PhaseConfig `json:"build"`
	Deploy PhaseConfig `json:"deploy"`
	Test   PhaseConfig `json:"test"`
}

type DeployConfig struct {
	raw map[string]DeployEnvConfig
}

func LoadDeployConfig(path string) (*DeployConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	var raw map[string]DeployEnvConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return &DeployConfig{raw: raw}, nil
}

// Resolve returns the phase config for the given env, merging missing fields
// from the "default" env entry.
func (c *DeployConfig) Resolve(env, phase string) PhaseConfig {
	def := c.phaseOf("default", phase)
	if env == "default" {
		return def
	}
	return mergePhase(c.phaseOf(env, phase), def)
}

func (c *DeployConfig) phaseOf(env, phase string) PhaseConfig {
	envCfg, ok := c.raw[env]
	if !ok {
		return PhaseConfig{}
	}
	switch phase {
	case "build":
		return envCfg.Build
	case "deploy":
		return envCfg.Deploy
	case "test":
		return envCfg.Test
	default:
		return PhaseConfig{}
	}
}

func mergePhase(primary, fallback PhaseConfig) PhaseConfig {
	if primary.Prefix == "" {
		primary.Prefix = fallback.Prefix
	}
	if primary.InstanceNumber == 0 {
		primary.InstanceNumber = fallback.InstanceNumber
	}
	if primary.SshUser == "" {
		primary.SshUser = fallback.SshUser
	}
	if primary.BuildHost == "" {
		primary.BuildHost = fallback.BuildHost
	}
	if primary.Description == "" {
		primary.Description = fallback.Description
	}
	if len(primary.Services) == 0 {
		primary.Services = fallback.Services
	}
	if primary.Settings == nil {
		primary.Settings = make(map[string]string)
		for k, v := range fallback.Settings {
			primary.Settings[k] = v
		}
	} else {
		for k, v := range fallback.Settings {
			if _, exists := primary.Settings[k]; !exists {
				primary.Settings[k] = v
			}
		}
	}
	return primary
}
