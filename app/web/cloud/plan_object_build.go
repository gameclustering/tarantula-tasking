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

func NewPlanObjectBuild(s *CloudService) *protocol.TccTransationListener {
	p := PlanObjectBuild{s}
	tcc := protocol.TccTransationListener{}
	tcc.Reserve = p.reserve
	tcc.Confirm = p.confirm
	tcc.Cancel = p.cancel
	return &tcc
}

type PlanObjectBuild struct {
	*CloudService
}

func (v *PlanObjectBuild) reserve(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("build reserve %v", t.Meta)
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
	phase := cfg.Resolve(plan.Env, "build")

	gcpKey, err := v.Cluster().AuthKey("gcp")
	if err != nil {
		return fmt.Errorf("gcp auth key: %w", err)
	}
	dockerKey, err := v.Cluster().AuthKey("docker")
	if err != nil {
		return fmt.Errorf("docker auth key: %w", err)
	}
	gcp := util.GcpApi{ServiceAccount: gcpKey.Gcp.Iam, ProjectId: gcpKey.Gcp.ProjectId, Zone: phase.Zone}
	if err := gcp.Auth(); err != nil {
		return fmt.Errorf("gcp auth: %w", err)
	}
	defer gcp.Close()

	name := fmt.Sprintf("%s-%02d", phase.Prefix, 1)
	if err := v.buildOnInstance(gcp, gcpKey.Gcp.Ssh, gcpKey.Gcp.User, gitKey.Git.Org, name, plan.AppRepo, dockerKey.Docker); err != nil {
		core.AppLog.Warn().Msgf("build on instance %s: %s", name, err.Error())
	}
	return v.insert(t.Meta)
}

func (v *PlanObjectBuild) buildOnInstance(gcp util.GcpApi, sshKey string, user string, org string, name string, repo *protocol.RepoObject, docker *protocol.DockerAccess) error {
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
	cred := docker.Token
	if cred == "" {
		cred = docker.Password
	}
	image := fmt.Sprintf("%s/%s:%s", docker.Username, repo.Name, ref)
	var out bytes.Buffer
	cmds := []string{
		fmt.Sprintf("rm -rf %s", repo.Name),
		fmt.Sprintf("git clone git@github.com:%s/%s.git", org, repo.Name),
		fmt.Sprintf("cd %s && git checkout %s", repo.Name, ref),
		fmt.Sprintf("cd %s && docker build -t %s .", repo.Name, image),
		fmt.Sprintf("printf '%%s' '%s' | docker login %s -u %s --password-stdin", cred, docker.Server, docker.Username),
		fmt.Sprintf("docker push %s", image),
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

func (v *PlanObjectBuild) confirm(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("build confirm %v", t.Meta)
	return v.insert(t.Meta)
}

func (v *PlanObjectBuild) cancel(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("build cancel %v", t.Meta)
	return v.insert(t.Meta)
}
