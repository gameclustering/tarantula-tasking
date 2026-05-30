package main

import (
	"bytes"
	"fmt"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewVMObjectBuild(s *CloudService) *protocol.TccTransationListener {
	b := VMObjectBuild{s}
	tcc := protocol.TccTransationListener{}
	tcc.Reserve = b.reserve
	tcc.Confirm = b.confirm
	tcc.Cancel = b.cancel
	return &tcc
}

type VMObjectBuild struct {
	*CloudService
}

func (v *VMObjectBuild) reserve(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("build reserve %v", t.Meta)
	var vm protocol.VMObject
	if err := anypb.UnmarshalTo(t.Message, &vm, proto.UnmarshalOptions{}); err != nil {
		return err
	}
	gcpKey, err := v.Cluster().AuthKey("gcp")
	if err != nil {
		return fmt.Errorf("gcp auth key: %w", err)
	}
	gcp := util.GcpApi{ServiceAccount: gcpKey.Gcp.Iam, ProjectId: gcpKey.Gcp.ProjectId, Zone: gcpKey.Gcp.Zone}
	if err := gcp.Auth(); err != nil {
		return fmt.Errorf("gcp auth: %w", err)
	}
	defer gcp.Close()

	gitKey, err := v.Cluster().AuthKey("git")
	if err != nil {
		return fmt.Errorf("git auth key: %w", err)
	}

	for i := uint32(1); i <= vm.NumberOfInstances; i++ {
		name := fmt.Sprintf("%s-%02d", gcpKey.Gcp.Prefix, i)
		if err := v.buildOnInstance(gcp, gcpKey.Gcp.Ssh, gcpKey.Gcp.User, gitKey.Git.Org, name, &vm); err != nil {
			core.AppLog.Warn().Msgf("build on instance %s: %s", name, err.Error())
		}
	}
	return v.insert(t.Meta)
}

func (v *VMObjectBuild) buildOnInstance(gcp util.GcpApi, sshKey string, user string, org string, name string, vm *protocol.VMObject) error {
	ins, err := gcp.Get(name)
	if err != nil {
		return fmt.Errorf("get instance: %w", err)
	}
	natIP := ins.GetNetworkInterfaces()[0].AccessConfigs[0].GetNatIP()
	ssh := util.SshClient{Host: natIP, User: user, PrivateKey: sshKey, KHFile: "../.ssh/known_hosts"}
	if err := ssh.WithKey(); err != nil {
		return fmt.Errorf("ssh connect: %w", err)
	}
	defer ssh.Close()

	ref := vm.Tag
	if ref == "" {
		ref = vm.Branch
	}
	var out bytes.Buffer
	cmds := []string{
		fmt.Sprintf("rm -rf %s", vm.Repository),
		fmt.Sprintf("git clone git@github.com:%s/%s.git", org, vm.Repository),
		fmt.Sprintf("cd %s && git checkout %s", vm.Repository, ref),
		fmt.Sprintf("cd %s && docker build -t %s:%s .", vm.Repository, vm.Repository, ref),
	}
	for _, cmd := range cmds {
		out.Reset()
		if err := ssh.Run(cmd, &out); err != nil {
			return fmt.Errorf("cmd %q: %w — %s", cmd, err, out.String())
		}
		core.AppLog.Debug().Msgf("build [%s]: %s", name, out.String())
	}
	return nil
}

func (v *VMObjectBuild) confirm(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("build confirm %v", t.Meta)
	return v.insert(t.Meta)
}

func (v *VMObjectBuild) cancel(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("build cancel %v", t.Meta)
	return v.insert(t.Meta)
}
