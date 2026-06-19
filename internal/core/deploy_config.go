package core

import (
	"encoding/json"
	"io"
	"os"
)

// PromotionSpec describes how to mark a successful test run in the app repo.
type PromotionSpec struct {
	Repo       string `json:"repo"`       // repo name to tag (e.g., "tarantula-sample-app")
	TagPattern string `json:"tagPattern"` // sprintf pattern with env, e.g., "%s-ready"
}

// CredentialField is a single field in a credential spec: either a static value or a generated password.
type CredentialField struct {
	Value    string `json:"value"`
	Generate bool   `json:"generate"`
}

// CredentialSpec describes credentials to create in Vault before deploying the first instance.
// If the secret already exists in Vault the seed step is skipped (idempotent).
type CredentialSpec struct {
	VaultMount string                     `json:"vaultMount"`
	VaultPath  string                     `json:"vaultPath"`
	Fields     map[string]CredentialField `json:"fields"`
}

// PhaseConfig is a platform-agnostic deployment phase.
// Platform-specific values (zone, region, machineType, etc.) go in Settings.
type PhaseConfig struct {
	Prefix         string            `json:"prefix"`
	InstanceNumber int               `json:"instanceNumber"`
	SshUser        string            `json:"sshUser"`
	BuildHost      string            `json:"buildHost"` // pre-existing build server; empty means provision one
	VaultHost      string            `json:"vaultHost"` // public vault URL for deployed containers; overrides worker's VAULT_HOST
	Description    string            `json:"description"`
	Services       []GcpServiceConfig `json:"services"`   // fields are platform-agnostic
	Ports          []string           `json:"ports"`      // host:container port mappings for single-repo deploys
	Settings       map[string]string  `json:"settings"`   // platform-specific key-value pairs
	Credentials    *CredentialSpec    `json:"credentials"` // auto-seed service credentials into Vault on first deploy
	TestRepo       string             `json:"testRepo"`   // test phase: repo containing k6 scripts
	AppPrefix      string             `json:"appPrefix"`  // test phase: prefix of app instances to test against
	Promotion      *PromotionSpec     `json:"promotion"`  // test phase: git tag to push on test pass
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
	if primary.VaultHost == "" {
		primary.VaultHost = fallback.VaultHost
	}
	if primary.Description == "" {
		primary.Description = fallback.Description
	}
	if len(primary.Services) == 0 {
		primary.Services = fallback.Services
	}
	if len(primary.Ports) == 0 {
		primary.Ports = fallback.Ports
	}
	if primary.Credentials == nil {
		primary.Credentials = fallback.Credentials
	}
	if primary.TestRepo == "" {
		primary.TestRepo = fallback.TestRepo
	}
	if primary.AppPrefix == "" {
		primary.AppPrefix = fallback.AppPrefix
	}
	if primary.Promotion == nil {
		primary.Promotion = fallback.Promotion
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
