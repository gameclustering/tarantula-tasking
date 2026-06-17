package cloud

import (
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

// InstancePlatform abstracts compute instance lifecycle across cloud providers.
type InstancePlatform interface {
	Provision(name string) error
	IP(name string) (string, error)
	Remove(name string) error
	Close()
	SSHKey() string
	SSHUser() string
}

// PlatformFactory creates an InstancePlatform for a given phase and auth key.
type PlatformFactory func(phase core.PhaseConfig, key *protocol.AuthKey) (InstancePlatform, error)
