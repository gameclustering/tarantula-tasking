package main

import (
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
)

type gcpPlatform struct {
	api    util.GcpApi
	gcpKey *protocol.GcpAccess
	phase  core.PhaseConfig
}

func newGcpPlatform(phase core.PhaseConfig, key *protocol.AuthKey) (*gcpPlatform, error) {
	zone := phase.Settings["zone"]
	if zone == "" {
		zone = phase.Settings["Zone"] // case-insensitive fallback
	}
	gcp := util.GcpApi{
		ServiceAccount: key.Gcp.Iam,
		ProjectId:      key.Gcp.ProjectId,
		Zone:           zone,
	}
	if err := gcp.Auth(); err != nil {
		return nil, err
	}
	return &gcpPlatform{api: gcp, gcpKey: key.Gcp, phase: phase}, nil
}

func (p *gcpPlatform) Provision(name string) error {
	if _, err := p.api.Get(name); err == nil {
		return nil // already exists
	}
	return p.api.Insert(name, p.phase.Settings["machineType"], p.phase.Settings["imageType"])
}

func (p *gcpPlatform) IP(name string) (string, error) {
	ins, err := p.api.Get(name)
	if err != nil {
		return "", err
	}
	return ins.GetNetworkInterfaces()[0].AccessConfigs[0].GetNatIP(), nil
}

func (p *gcpPlatform) Remove(name string) error { return p.api.Delete(name) }
func (p *gcpPlatform) Close()                    { p.api.Close() }
func (p *gcpPlatform) SSHKey() string            { return p.gcpKey.Ssh }
func (p *gcpPlatform) SSHUser() string           { return p.gcpKey.User }
