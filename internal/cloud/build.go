package cloud

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"gameclustering.com/internal/bootstrap"
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewBuild(mgr *bootstrap.AppManager, store *Store, vaultKey string, factory PlatformFactory) *protocol.TccTransationListener {
	h := &buildHandler{mgr: mgr, store: store, vaultKey: vaultKey, factory: factory}
	tcc := protocol.TccTransationListener{}
	tcc.Reserve = h.reserve
	tcc.Confirm = h.confirm
	tcc.Cancel = h.cancel
	return &tcc
}

type buildHandler struct {
	mgr      *bootstrap.AppManager
	store    *Store
	vaultKey string
	factory  PlatformFactory
}

func (h *buildHandler) reserve(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("build reserve %v", t.Meta)
	var plan protocol.PlanObject
	if err := anypb.UnmarshalTo(t.Message, &plan, proto.UnmarshalOptions{}); err != nil {
		return err
	}
	gitKey, err := h.mgr.Cluster().AuthKey("git")
	if err != nil {
		return fmt.Errorf("git auth key: %w", err)
	}
	cfg, err := LoadDeployConfig(plan.DeployRepo, plan.Platform, plan.Name, gitKey)
	if err != nil {
		return fmt.Errorf("deploy config: %w", err)
	}
	buildPhase := cfg.Resolve(plan.Env, "build")
	deployPhase := cfg.Resolve(plan.Env, "deploy")

	platformKey, err := h.mgr.Cluster().AuthKey(h.vaultKey)
	if err != nil {
		return fmt.Errorf("%s auth key: %w", h.vaultKey, err)
	}
	dockerKey, err := h.mgr.Cluster().AuthKey("docker")
	if err != nil {
		return fmt.Errorf("docker auth key: %w", err)
	}

	var buildIP, sshKey, sshUser string
	if buildPhase.BuildHost != "" {
		buildIP = buildPhase.BuildHost
		platform, err := h.factory(buildPhase, platformKey)
		if err != nil {
			return fmt.Errorf("platform init: %w", err)
		}
		defer platform.Close()
		sshKey = platform.SSHKey()
		sshUser = buildPhase.SshUser
		if sshUser == "" {
			sshUser = platform.SSHUser()
		}
	} else {
		platform, err := h.factory(buildPhase, platformKey)
		if err != nil {
			return fmt.Errorf("platform init: %w", err)
		}
		defer platform.Close()
		name := fmt.Sprintf("%s-%02d", buildPhase.Prefix, 1)
		ip, err := platform.IP(name)
		if err != nil {
			return fmt.Errorf("get build instance IP: %w", err)
		}
		buildIP = ip
		sshKey = platform.SSHKey()
		sshUser = buildPhase.SshUser
		if sshUser == "" {
			sshUser = platform.SSHUser()
		}
	}

	if err := h.buildOnHost(buildIP, sshKey, sshUser, gitKey, &plan, dockerKey.Docker, deployPhase.Services); err != nil {
		core.AppLog.Warn().Msgf("build on host %s: %s", buildIP, err.Error())
	}
	return h.store.Insert(t.Meta)
}

func (h *buildHandler) buildOnHost(host, sshKey, sshUser string, gitKey *protocol.AuthKey, plan *protocol.PlanObject, docker *protocol.DockerAccess, services []core.GcpServiceConfig) error {
	ssh := util.SshClient{Host: host, User: sshUser, PrivateKey: sshKey, KHFile: "../.ssh/known_hosts"}
	const maxWait = 5 * time.Minute
	deadline := time.Now().Add(maxWait)
	for {
		if err := ssh.WithKey(); err == nil {
			break
		} else if time.Now().After(deadline) {
			return fmt.Errorf("ssh connect: timed out: %w", err)
		}
		core.AppLog.Debug().Msgf("build [%s]: waiting for SSH...", host)
		time.Sleep(10 * time.Second)
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
		return fmt.Errorf("docker login: %w — %s", err, out.String())
	}

	// Service task: build from deploy repo using docker_application_build
	if plan.AppRepo == nil || plan.AppRepo.Name == "" {
		return h.buildService(ssh, host, gitKey, plan, docker, services, &out)
	}

	// App task: clone app repo and build
	return h.buildApp(ssh, host, gitKey.Git.Org, plan.AppRepo, docker, services, &out)
}

func (h *buildHandler) buildService(ssh util.SshClient, host string, gitKey *protocol.AuthKey, plan *protocol.PlanObject, docker *protocol.DockerAccess, services []core.GcpServiceConfig, out *bytes.Buffer) error {
	ref := plan.Env
	if ref == "" {
		ref = "latest"
	}
	repoName := plan.DeployRepo.Name
	setupCmds := []string{
		fmt.Sprintf("rm -rf %s", repoName),
		fmt.Sprintf("git clone git@github.com:%s/%s.git", gitKey.Git.Org, repoName),
	}
	if plan.DeployRepo.Tag != "" || plan.DeployRepo.Branch != "" {
		checkoutRef := plan.DeployRepo.Tag
		if checkoutRef == "" {
			checkoutRef = plan.DeployRepo.Branch
		}
		setupCmds = append(setupCmds, fmt.Sprintf("cd %s && git checkout %s", repoName, checkoutRef))
	}
	for _, cmd := range setupCmds {
		out.Reset()
		if err := ssh.Run(cmd, out); err != nil {
			return fmt.Errorf("cmd %q: %w — %s", cmd, err, out.String())
		}
		core.AppLog.Debug().Msgf("build [%s]: %s", host, strings.TrimSpace(out.String()))
	}
	for _, svc := range services {
		parts := strings.SplitN(svc.Name, ".", 2)
		appName := svc.Name
		if len(parts) == 2 {
			appName = parts[1]
		}
		image := fmt.Sprintf("%s/%s:%s", docker.Username, svc.Name, ref)
		cmds := []string{
			fmt.Sprintf("cd %s && docker build -f docker_application_build --build-arg app=%s -t %s .", repoName, appName, image),
			fmt.Sprintf("docker push %s", image),
		}
		for _, cmd := range cmds {
			out.Reset()
			if err := ssh.Run(cmd, out); err != nil {
				return fmt.Errorf("build [%s/%s] %q: %w — %s", host, svc.Name, cmd, err, out.String())
			}
			core.AppLog.Debug().Msgf("build [%s/%s]: %s", host, svc.Name, strings.TrimSpace(out.String()))
		}
	}
	return nil
}

func (h *buildHandler) buildApp(ssh util.SshClient, host, org string, repo *protocol.RepoObject, docker *protocol.DockerAccess, services []core.GcpServiceConfig, out *bytes.Buffer) error {
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
	setupCmds := []string{
		fmt.Sprintf("rm -rf %s", repo.Name),
		fmt.Sprintf("git clone git@github.com:%s/%s.git", org, repo.Name),
		fmt.Sprintf("cd %s && git checkout %s", repo.Name, ref),
	}
	for _, cmd := range setupCmds {
		out.Reset()
		if err := ssh.Run(cmd, out); err != nil {
			return fmt.Errorf("cmd %q: %w — %s", cmd, err, out.String())
		}
		core.AppLog.Debug().Msgf("build [%s]: %s", host, strings.TrimSpace(out.String()))
	}

	if len(services) == 0 {
		image := fmt.Sprintf("%s/%s:%s", docker.Username, repo.Name, ref)
		for _, cmd := range []string{
			fmt.Sprintf("cd %s && docker build -t %s .", repo.Name, image),
			fmt.Sprintf("docker push %s", image),
		} {
			out.Reset()
			if err := ssh.Run(cmd, out); err != nil {
				return fmt.Errorf("cmd %q: %w — %s", cmd, err, out.String())
			}
			core.AppLog.Debug().Msgf("build [%s]: %s", host, strings.TrimSpace(out.String()))
		}
		return nil
	}

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
			if err := ssh.Run(cmd, out); err != nil {
				return fmt.Errorf("build [%s/%s] %q: %w — %s", host, svc.Name, cmd, err, out.String())
			}
			core.AppLog.Debug().Msgf("build [%s/%s]: %s", host, svc.Name, strings.TrimSpace(out.String()))
		}
	}
	return nil
}

func (h *buildHandler) confirm(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("build confirm %v", t.Meta)
	return h.store.Insert(t.Meta)
}

func (h *buildHandler) cancel(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("build cancel %v", t.Meta)
	return h.store.Insert(t.Meta)
}
