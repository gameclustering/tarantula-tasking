package main

import (
	"bytes"
	"fmt"
	"strings"

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
	cfg, err := loadDeployConfig(plan.DeployRepo, plan.Platform, plan.Name, gitKey)
	if err != nil {
		return fmt.Errorf("deploy config: %w", err)
	}
	deployPhase := cfg.Resolve(plan.Env, "deploy")

	platformKey, err := v.Cluster().AuthKey(platformVaultKey(plan.Platform))
	if err != nil {
		return fmt.Errorf("%s auth key: %w", plan.Platform, err)
	}
	dockerKey, err := v.Cluster().AuthKey("docker")
	if err != nil {
		return fmt.Errorf("docker auth key: %w", err)
	}
	platform, err := newPlatform(plan.Platform, deployPhase, platformKey)
	if err != nil {
		return fmt.Errorf("platform init: %w", err)
	}
	defer platform.Close()

	ref := ""
	if plan.AppRepo != nil {
		ref = plan.AppRepo.Tag
		if ref == "" {
			ref = plan.AppRepo.Branch
		}
	}
	if ref == "" {
		ref = "latest"
	}

	sshUser := deployPhase.SshUser
	if sshUser == "" {
		sshUser = platform.SSHUser()
	}

	vaultHost := v.F.Vlt.Host
	if deployPhase.VaultHost != "" {
		vaultHost = deployPhase.VaultHost
	}
	var firstNodeIP string
	for i := 1; i <= deployPhase.InstanceNumber; i++ {
		name := fmt.Sprintf("%s-%02d", deployPhase.Prefix, i)
		clusterBootstrap := ""
		if i > 1 && firstNodeIP != "" {
			clusterBootstrap = fmt.Sprintf("http://%s:8080", firstNodeIP)
		}
		ip, err := v.deployOnInstance(platform, name, sshUser, i, ref, deployPhase.Services, plan.AppRepo, dockerKey.Docker, vaultHost, v.F.Vlt.Token, clusterBootstrap)
		if err != nil {
			core.AppLog.Warn().Msgf("deploy on instance %s: %s", name, err.Error())
		} else if firstNodeIP == "" {
			firstNodeIP = ip
		}
	}
	return v.insert(t.Meta)
}

func (v *PlanObjectDeploy) deployOnInstance(platform InstancePlatform, name, sshUser string, seq int, ref string, services []core.GcpServiceConfig, repo *protocol.RepoObject, docker *protocol.DockerAccess, vaultHost, vaultToken, clusterBootstrap string) (string, error) {
	ip, err := platform.IP(name)
	if err != nil {
		return "", fmt.Errorf("get IP: %w", err)
	}
	ssh := util.SshClient{Host: ip, User: sshUser, PrivateKey: platform.SSHKey(), KHFile: "../.ssh/known_hosts"}
	if err := ssh.WithKey(); err != nil {
		return "", fmt.Errorf("ssh connect: %w", err)
	}
	defer ssh.Close()

	cred := docker.Token
	if cred == "" {
		cred = docker.Password
	}

	var out bytes.Buffer
	loginCmd := fmt.Sprintf("printf '%%s' '%s' | docker login %s -u %s --password-stdin", cred, docker.Server, docker.Username)
	out.Reset()
	if err := ssh.Run(loginCmd, &out); err != nil {
		return "", fmt.Errorf("docker login: %w — %s", err, out.String())
	}
	core.AppLog.Debug().Msgf("deploy [%s]: docker login OK", name)

	if len(services) == 0 {
		if repo == nil || repo.Name == "" {
			return ip, nil
		}
		if err := v.runContainer(ssh, name, repo.Name, ref, "", "", docker, vaultHost, vaultToken, "", seq, &out); err != nil {
			return "", err
		}
		return ip, nil
	}

	for _, svc := range services {
		bootstrap := ""
		if strings.Contains(svc.Name, "postoffice") {
			bootstrap = clusterBootstrap
		}
		if err := v.runContainer(ssh, name, svc.Name, ref, svc.Network, svc.HttpBinding, docker, vaultHost, vaultToken, bootstrap, seq, &out); err != nil {
			return "", err
		}
	}
	return ip, nil
}

func (v *PlanObjectDeploy) runContainer(ssh util.SshClient, instanceName, svcName, ref, network, httpBinding string, docker *protocol.DockerAccess, vaultHost, vaultToken, clusterBootstrap string, seq int, out *bytes.Buffer) error {
	image := fmt.Sprintf("%s/%s:%s", docker.Username, svcName, ref)

	var flags []string
	if network != "" {
		flags = append(flags, fmt.Sprintf("--network %s", network))
	}
	flags = append(flags, fmt.Sprintf("-e VAULT_HOST='%s'", vaultHost))
	flags = append(flags, fmt.Sprintf("-e VAULT_TOKEN='%s'", vaultToken))
	flags = append(flags, fmt.Sprintf("-e SEQ=%d", seq))
	if httpBinding != "" {
		flags = append(flags, fmt.Sprintf("-e HTTP_BINDING='%s'", httpBinding))
	}
	if strings.Contains(svcName, "postoffice") {
		flags = append(flags, fmt.Sprintf("-e CLUSTER_BOOTSTRAP='%s'", clusterBootstrap))
	} else {
		flags = append(flags, "-e POST_OFFICE_HOST=127.0.0.1")
	}

	runArgs := strings.Join(flags, " ")
	cmds := []string{
		fmt.Sprintf("docker pull %s", image),
		fmt.Sprintf("docker stop %s 2>/dev/null || true && docker rm %s 2>/dev/null || true", svcName, svcName),
		fmt.Sprintf("docker run -d --name %s %s %s", svcName, runArgs, image),
	}
	for _, cmd := range cmds {
		out.Reset()
		if err := ssh.Run(cmd, out); err != nil {
			return fmt.Errorf("deploy [%s/%s] %q: %w — %s", instanceName, svcName, cmd, err, out.String())
		}
		core.AppLog.Debug().Msgf("deploy [%s/%s]: %s", instanceName, svcName, strings.TrimSpace(out.String()))
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
