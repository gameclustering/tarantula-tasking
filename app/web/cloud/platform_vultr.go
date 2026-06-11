package main

import (
	"fmt"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
)

type vultrPlatform struct {
	api    util.VultrApi
	vpsKey *protocol.VpsAccess
	phase  core.PhaseConfig
}

func newVultrPlatform(phase core.PhaseConfig, key *protocol.AuthKey) (*vultrPlatform, error) {
	if key.Vps == nil {
		return nil, fmt.Errorf("vps auth key not configured")
	}
	return &vultrPlatform{
		api:    util.VultrApi{ApiKey: key.Vps.ApiKey},
		vpsKey: key.Vps,
		phase:  phase,
	}, nil
}

// Provision creates the Vultr VPS instance if it does not already exist.
// Settings required: "region", "plan", "osId".
// The SSH key ID is resolved automatically from the Vault SSH private key
// (vps.Ssh) so it never needs to be hardcoded in the deploy config.
func (p *vultrPlatform) Provision(name string) error {
	if _, err := p.api.GetInstanceByLabel(name); err == nil {
		return nil // already exists
	}
	region := p.phase.Settings["region"]
	plan := p.phase.Settings["plan"]
	osId := util.OsIdFromSettings(p.phase.Settings)
	if region == "" || plan == "" || osId == 0 {
		return fmt.Errorf("vultr provision %q: region/plan/osId required in settings", name)
	}
	// Look up the registered Vultr SSH key that matches our Vault private key.
	// An empty result is fine — instance boots without a pre-attached key.
	var sshKeys []string
	if p.vpsKey.Ssh != "" {
		keyId, err := p.api.FindSshKeyId(p.vpsKey.Ssh)
		if err != nil {
			return fmt.Errorf("find ssh key id: %w", err)
		}
		if keyId != "" {
			sshKeys = []string{keyId}
		}
	}
	_, err := p.api.CreateInstance(name, region, plan, osId, sshKeys)
	return err
}

func (p *vultrPlatform) IP(name string) (string, error) {
	ins, err := p.api.GetInstanceByLabel(name)
	if err != nil {
		return "", err
	}
	if ins.MainIP == "" || ins.MainIP == "0.0.0.0" {
		return "", fmt.Errorf("instance %q not yet assigned an IP", name)
	}
	return ins.MainIP, nil
}

func (p *vultrPlatform) Remove(name string) error {
	ins, err := p.api.GetInstanceByLabel(name)
	if err != nil {
		return nil // already gone
	}
	return p.api.DeleteInstance(ins.Id)
}

func (p *vultrPlatform) Close()         {}
func (p *vultrPlatform) SSHKey() string  { return p.vpsKey.Ssh }
func (p *vultrPlatform) SSHUser() string { return p.vpsKey.User }
