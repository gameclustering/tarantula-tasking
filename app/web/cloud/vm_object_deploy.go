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

func NewVMObjectDeploy(s *CloudService) *protocol.TccTransationListener {
	d := VMObjectDeploy{s}
	tcc := protocol.TccTransationListener{}
	tcc.Reserve = d.reserve
	tcc.Confirm = d.confirm
	tcc.Cancel = d.cancel
	return &tcc
}

type VMObjectDeploy struct {
	*CloudService
}

func (v *VMObjectDeploy) reserve(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("deploy reserve %v", t.Meta)
	var plan protocol.PlanObject
	if err := anypb.UnmarshalTo(t.Message, &plan, proto.UnmarshalOptions{}); err != nil {
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

	name := fmt.Sprintf("%s-%02d", gcpKey.Gcp.Prefix, 1)
	if err := v.deployOnInstance(gcp, gcpKey.Gcp.Ssh, gcpKey.Gcp.User, name, plan.AppRepo); err != nil {
		core.AppLog.Warn().Msgf("deploy on instance %s: %s", name, err.Error())
	}
	return v.insert(t.Meta)
}

func (v *VMObjectDeploy) deployOnInstance(gcp util.GcpApi, sshKey string, user string, name string, repo *protocol.RepoObject) error {
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

	ref := repo.Tag
	if ref == "" {
		ref = repo.Branch
	}
	var out bytes.Buffer
	cmds := []string{
		fmt.Sprintf("docker stop %s 2>/dev/null || true && docker rm %s 2>/dev/null || true", repo.Name, repo.Name),
		fmt.Sprintf("docker run -d --name %s -P %s:%s", repo.Name, repo.Name, ref),
	}
	for _, cmd := range cmds {
		out.Reset()
		if err := ssh.Run(cmd, &out); err != nil {
			return fmt.Errorf("cmd %q: %w — %s", cmd, err, out.String())
		}
		core.AppLog.Debug().Msgf("deploy [%s]: %s", name, out.String())
	}
	return nil
}

func (v *VMObjectDeploy) confirm(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("deploy confirm %v", t.Meta)
	return v.insert(t.Meta)
}

func (v *VMObjectDeploy) cancel(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("deploy cancel %v", t.Meta)
	return v.insert(t.Meta)
}
