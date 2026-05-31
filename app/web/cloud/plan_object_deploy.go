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

func NewPlanObjectDeploy(s *CloudService) *protocol.TccTransationListener {
	p := PlanObjectDeploy{s}
	tcc := protocol.TccTransationListener{}
	tcc.Reserve = p.reserve
	tcc.Confirm = p.confirm
	tcc.Cancel = p.cancel
	return &tcc
}

type PlanObjectDeploy struct {
	*CloudService
}

func (v *PlanObjectDeploy) reserve(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("deploy reserve %v", t.Meta)
	var plan protocol.PlanObject
	if err := anypb.UnmarshalTo(t.Message, &plan, proto.UnmarshalOptions{}); err != nil {
		return err
	}
	gitKey, err := v.Cluster().AuthKey("git")
	if err != nil {
		return fmt.Errorf("git auth key: %w", err)
	}
	cfg, err := loadGcpDeployConfig(plan.DeployRepo, gitKey)
	if err != nil {
		return fmt.Errorf("deploy config: %w", err)
	}
	phase := cfg.Resolve(plan.Env, "deploy")

	gcpKey, err := v.Cluster().AuthKey("gcp")
	if err != nil {
		return fmt.Errorf("gcp auth key: %w", err)
	}
	gcp := util.GcpApi{ServiceAccount: gcpKey.Gcp.Iam, ProjectId: gcpKey.Gcp.ProjectId, Zone: phase.Zone}
	if err := gcp.Auth(); err != nil {
		return fmt.Errorf("gcp auth: %w", err)
	}
	defer gcp.Close()

	for i := 1; i <= phase.InstanceNumber; i++ {
		name := fmt.Sprintf("%s-%02d", phase.Prefix, i)
		if err := v.deployOnInstance(gcp, gcpKey.Gcp.Ssh, gcpKey.Gcp.User, name, plan.AppRepo); err != nil {
			core.AppLog.Warn().Msgf("deploy on instance %s: %s", name, err.Error())
		}
	}
	return v.insert(t.Meta)
}

func (v *PlanObjectDeploy) deployOnInstance(gcp util.GcpApi, sshKey string, user string, name string, repo *protocol.RepoObject) error {
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

func (v *PlanObjectDeploy) confirm(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("deploy confirm %v", t.Meta)
	return v.insert(t.Meta)
}

func (v *PlanObjectDeploy) cancel(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("deploy cancel %v", t.Meta)
	return v.insert(t.Meta)
}
