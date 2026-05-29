package main

import (
	"fmt"
	"os"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewVMObjectUpdate(s *CloudService) *protocol.TccTransationListener {
	vm := VMObjectUpdate{s}
	tcc := protocol.TccTransationListener{}
	tcc.Reserve = vm.reserve
	tcc.Confirm = vm.confirm
	tcc.Cancel = vm.cancel
	return &tcc
}

type VMObjectUpdate struct {
	*CloudService
}

func (v *VMObjectUpdate) reserve(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("update reserve %v", t.Meta)
	var vm protocol.VMObject
	if err := anypb.UnmarshalTo(t.Message, &vm, proto.UnmarshalOptions{}); err != nil {
		return err
	}
	core.AppLog.Debug().Msgf("vm object %v", &vm)
	switch vm.Vendor {
	case "gcp":
		return v.updateGcp(&vm, t.Meta)
	default:
		return fmt.Errorf("unsupported vendor: %s", vm.Vendor)
	}
}

func (v *VMObjectUpdate) updateGcp(vm *protocol.VMObject, meta *protocol.Meta) error {
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
	keyFile := "id_ed25519"
	if err := os.WriteFile(keyFile, []byte(gitKey.Git.Key), 0700); err != nil {
		return fmt.Errorf("write git key: %w", err)
	}
	defer os.Remove(keyFile)

	for i := uint32(1); i <= vm.NumberOfInstances; i++ {
		name := fmt.Sprintf("%s-%02d", gcpKey.Gcp.Prefix, i)
		if err := v.setupInstance(gcp, gcpKey.Gcp.Ssh, gcpKey.Gcp.User, name, keyFile); err != nil {
			core.AppLog.Warn().Msgf("setup instance %s: %s", name, err.Error())
		}
	}
	return v.insert(meta)
}

func (v *VMObjectUpdate) setupInstance(gcp util.GcpApi, sshKey string, user string, name string, keyFile string) error {
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

	f, err := os.Open(keyFile)
	if err != nil {
		return fmt.Errorf("open key file: %w", err)
	}
	defer f.Close()

	if err := ssh.Upload(f, "/home/yinghu_lu/.ssh/id_ed25519", "0700"); err != nil {
		return fmt.Errorf("upload git key: %w", err)
	}
	return nil
}

func (v *VMObjectUpdate) confirm(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("update confirm %v", t.Meta)
	return v.insert(t.Meta)
}

func (v *VMObjectUpdate) cancel(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("update cancel %v", t.Meta)
	return v.insert(t.Meta)
}
