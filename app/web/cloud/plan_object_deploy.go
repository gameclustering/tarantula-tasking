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
	cfg, err := loadGcpDeployConfig(plan.DeployRepo, gitKey)
	if err != nil {
		return fmt.Errorf("deploy config: %w", err)
	}
	phase := cfg.Resolve(plan.Env, "deploy")

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

	var firstNodeIP string
	for i := 1; i <= phase.InstanceNumber; i++ {
		name := fmt.Sprintf("%s-%02d", phase.Prefix, i)
		clusterBootstrap := ""
		if i > 1 && firstNodeIP != "" {
			clusterBootstrap = fmt.Sprintf("http://%s:8080", firstNodeIP)
		}
		ip, err := v.deployOnInstance(gcp, gcpKey.Gcp.Ssh, gcpKey.Gcp.User, name, i, ref, phase.Services, plan.AppRepo, dockerKey.Docker, v.F.Vlt.Host, v.F.Vlt.Token, clusterBootstrap)
		if err != nil {
			core.AppLog.Warn().Msgf("deploy on instance %s: %s", name, err.Error())
		} else if firstNodeIP == "" {
			firstNodeIP = ip
		}
	}
	return v.insert(t.Meta)
}

func (v *PlanObjectDeploy) deployOnInstance(gcp util.GcpApi, sshKey, user, name string, seq int, ref string, services []core.GcpServiceConfig, repo *protocol.RepoObject, docker *protocol.DockerAccess, vaultHost, vaultToken, clusterBootstrap string) (string, error) {
	ins, err := gcp.Get(name)
	if err != nil {
		return "", fmt.Errorf("get instance: %w", err)
	}
	natIP := ins.GetNetworkInterfaces()[0].AccessConfigs[0].GetNatIP()
	ssh := util.SshClient{Host: natIP, User: user, PrivateKey: sshKey, KHFile: "../.ssh/known_hosts"}
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
			return natIP, nil
		}
		if err := v.runContainer(ssh, name, repo.Name, ref, "", "", docker, vaultHost, vaultToken, "", seq, &out); err != nil {
			return "", err
		}
		return natIP, nil
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
	return natIP, nil
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
		// always set CLUSTER_BOOTSTRAP (even empty for node 1) to override the baked-in conf value
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
