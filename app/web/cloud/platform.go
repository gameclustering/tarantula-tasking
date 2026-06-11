package main

import (
	"fmt"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

// InstancePlatform abstracts compute instance lifecycle across cloud providers.
// All SSH operations use SSHKey/SSHUser to connect once an IP is known.
type InstancePlatform interface {
	// Provision creates the named instance; idempotent if it already exists.
	Provision(name string) error
	// IP returns the public IP of the named instance.
	IP(name string) (string, error)
	// Remove deletes the named instance.
	Remove(name string) error
	// Close releases any held resources (API clients, credentials, etc.).
	Close()
	// SSHKey returns the PEM private key for SSH connections to instances.
	SSHKey() string
	// SSHUser returns the default SSH username; callers may override with
	// phase.SshUser when the phase requires a different user (e.g. root during setup).
	SSHUser() string
}

// newPlatform constructs the right InstancePlatform for the given vendor.
// phase provides the config for the current operation (deploy or build).
// key is the vault AuthKey for the vendor (AuthKey("gcp") or AuthKey("vps")).
func newPlatform(vendor string, phase core.PhaseConfig, key *protocol.AuthKey) (InstancePlatform, error) {
	switch vendor {
	case "gcp":
		return newGcpPlatform(phase, key)
	case "vultr":
		return newVultrPlatform(phase, key)
	default:
		return nil, fmt.Errorf("unknown vendor %q", vendor)
	}
}

// platformVaultKey returns the vault key name for a given vendor.
func platformVaultKey(vendor string) string {
	switch vendor {
	case "vultr":
		return "vps"
	default:
		return vendor // "gcp" → "gcp"
	}
}
