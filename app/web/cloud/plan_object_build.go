package main

import (
	"bytes"
	"fmt"
	"strings"
	"time"

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
	// build phase gives us the instance (always #1); deploy phase has the services list
	buildPhase := cfg.Resolve(plan.Env, "build")
	deployPhase := cfg.Resolve(plan.Env, "deploy")

	gcpKey, err := v.Cluster().AuthKey("gcp")
	if err != nil {
		return fmt.Errorf("gcp auth key: %w", err)
	}
	dockerKey, err := v.Cluster().AuthKey("docker")
	if err != nil {
		return fmt.Errorf("docker auth key: %w", err)
	}
	gcp := util.GcpApi{ServiceAccount: gcpKey.Gcp.Iam, ProjectId: gcpKey.Gcp.ProjectId, Zone: buildPhase.Zone}
	if err := gcp.Auth(); err != nil {
		return fmt.Errorf("gcp auth: %w", err)
	}
	defer gcp.Close()

	name := fmt.Sprintf("%s-%02d", buildPhase.Prefix, 1)
	if err := v.buildOnInstance(gcp, gcpKey.Gcp.Ssh, gcpKey.Gcp.User, gitKey.Git.Org, name, plan.AppRepo, dockerKey.Docker, deployPhase.Services); err != nil {
		core.AppLog.Warn().Msgf("build on instance %s: %s", name, err.Error())
	}
	return v.insert(t.Meta)
}

func (v *PlanObjectBuild) buildOnInstance(gcp util.GcpApi, sshKey string, user string, org string, name string, repo *protocol.RepoObject, docker *protocol.DockerAccess, services []core.GcpServiceConfig) error {
	if repo == nil || repo.Name == "" {
		return nil
	}
	ins, err := gcp.Get(name)
	if err != nil {
		return fmt.Errorf("get instance: %w", err)
	}
	natIP := ins.GetNetworkInterfaces()[0].AccessConfigs[0].GetNatIP()
	ssh := util.SshClient{Host: natIP, User: user, PrivateKey: sshKey, KHFile: "../.ssh/known_hosts"}
	const maxWait = 5 * time.Minute
	deadline := time.Now().Add(maxWait)
	for {
		if err := ssh.WithKey(); err == nil {
			break
		} else if time.Now().After(deadline) {
			return fmt.Errorf("ssh connect: timed out: %w", err)
		}
		core.AppLog.Debug().Msgf("build [%s]: waiting for SSH...", name)
		time.Sleep(10 * time.Second)
	}
	defer ssh.Close()

	ref := repo.Tag
	if ref == "" {
		ref = repo.Branch
	}
	if ref == "" {
		ref = "latest"
	}
	cred := docker.Token
	if cred == "" {
		cred = docker.Password
	}

	var out bytes.Buffer
	setupCmds := []string{
		fmt.Sprintf("rm -rf %s", repo.Name),
		fmt.Sprintf("git clone git@github.com:%s/%s.git", org, repo.Name),
		fmt.Sprintf("cd %s && git checkout %s", repo.Name, ref),
		fmt.Sprintf("printf '%%s' '%s' | docker login %s -u %s --password-stdin", cred, docker.Server, docker.Username),
	}
	for _, cmd := range setupCmds {
		out.Reset()
		if err := ssh.Run(cmd, &out); err != nil {
			return fmt.Errorf("cmd %q: %w — %s", cmd, err, out.String())
		}
		core.AppLog.Debug().Msgf("build [%s]: %s", name, strings.TrimSpace(out.String()))
	}

	if len(services) == 0 {
		// single-image fallback: build with root Dockerfile
		image := fmt.Sprintf("%s/%s:%s", docker.Username, repo.Name, ref)
		cmds := []string{
			fmt.Sprintf("cd %s && docker build -t %s .", repo.Name, image),
			fmt.Sprintf("docker push %s", image),
		}
		for _, cmd := range cmds {
			out.Reset()
			if err := ssh.Run(cmd, &out); err != nil {
				return fmt.Errorf("cmd %q: %w — %s", cmd, err, out.String())
			}
			core.AppLog.Debug().Msgf("build [%s]: %s", name, strings.TrimSpace(out.String()))
		}
		return nil
	}

	// build and push each service image using docker_application_build
	for _, svc := range services {
		parts := strings.SplitN(svc.Name, ".", 2)
		appName := svc.Name
		if len(parts) == 2 {
			appName = parts[1]
		}
		image := fmt.Sprintf("%s/%s:%s", docker.Username, svc.Name, ref)
		cmds := []string{
			fmt.Sprintf("cd %s && docker build -f docker_application_build --build-arg app=%s -t %s .", repo.Name, appName, image),
			fmt.Sprintf("docker push %s", image),
		}
		for _, cmd := range cmds {
			out.Reset()
			if err := ssh.Run(cmd, &out); err != nil {
				return fmt.Errorf("build [%s/%s] %q: %w — %s", name, svc.Name, cmd, err, out.String())
			}
			core.AppLog.Debug().Msgf("build [%s/%s]: %s", name, svc.Name, strings.TrimSpace(out.String()))
		}
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
