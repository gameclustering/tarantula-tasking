package cloud

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"strings"

	"gameclustering.com/internal/bootstrap"
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewDeploy(mgr *bootstrap.AppManager, store *Store, vaultKey string, factory PlatformFactory) *protocol.TccTransationListener {
	h := &deployHandler{mgr: mgr, store: store, vaultKey: vaultKey, factory: factory}
	tcc := protocol.TccTransationListener{}
	tcc.Reserve = h.reserve
	tcc.Confirm = h.confirm
	tcc.Cancel = h.cancel
	return &tcc
}

type deployHandler struct {
	mgr      *bootstrap.AppManager
	store    *Store
	vaultKey string
	factory  PlatformFactory
}

func (h *deployHandler) reserve(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("deploy reserve %v", t.Meta)
	var plan protocol.PlanObject
	if err := anypb.UnmarshalTo(t.Message, &plan, proto.UnmarshalOptions{}); err != nil {
		return err
	}
	gitKey, err := h.mgr.Cluster().AuthKey("git")
	if err != nil {
		return fmt.Errorf("git auth key: %w", err)
	}
	planName := plan.Name
	if planName == "" && plan.AppRepo != nil {
		planName = plan.AppRepo.Name
	}
	cfg, err := LoadDeployConfig(plan.DeployRepo, plan.Platform, planName, gitKey)
	if err != nil {
		return fmt.Errorf("deploy config: %w", err)
	}
	deployPhase := cfg.Resolve(plan.Env, "deploy")

	platformKey, err := h.mgr.Cluster().AuthKey(h.vaultKey)
	if err != nil {
		return fmt.Errorf("%s auth key: %w", h.vaultKey, err)
	}
	dockerKey, err := h.mgr.Cluster().AuthKey("docker")
	if err != nil {
		return fmt.Errorf("docker auth key: %w", err)
	}
	platform, err := h.factory(deployPhase, platformKey)
	if err != nil {
		return fmt.Errorf("platform init: %w", err)
	}
	defer platform.Close()

	// App task ref: from appRepo tag/branch. Service task ref: env name.
	ref := plan.Env
	if plan.AppRepo != nil {
		if plan.AppRepo.Tag != "" {
			ref = plan.AppRepo.Tag
		} else if plan.AppRepo.Branch != "" {
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

	vaultHost := h.mgr.F.Vlt.Host
	if deployPhase.VaultHost != "" {
		vaultHost = deployPhase.VaultHost
	}
	vaultToken := h.mgr.F.Vlt.Token

	if deployPhase.Credentials != nil {
		if err := h.seedCredentials(deployPhase.Credentials); err != nil {
			return fmt.Errorf("seed credentials: %w", err)
		}
	}

	var firstNodeIP string
	for i := 1; i <= deployPhase.InstanceNumber; i++ {
		name := fmt.Sprintf("%s-%02d", deployPhase.Prefix, i)
		clusterBootstrap := ""
		if i > 1 && firstNodeIP != "" {
			clusterBootstrap = fmt.Sprintf("http://%s:8080", firstNodeIP)
		}
		ip, err := h.deployOnInstance(platform, name, sshUser, i, ref, deployPhase.Services, plan.AppRepo, dockerKey.Docker, vaultHost, vaultToken, clusterBootstrap)
		if err != nil {
			core.AppLog.Warn().Msgf("deploy on instance %s: %s", name, err.Error())
		} else if firstNodeIP == "" {
			firstNodeIP = ip
		}
	}
	return h.store.Insert(t.Meta)
}

func (h *deployHandler) deployOnInstance(platform InstancePlatform, name, sshUser string, seq int, ref string, services []core.GcpServiceConfig, repo *protocol.RepoObject, docker *protocol.DockerAccess, vaultHost, vaultToken, clusterBootstrap string) (string, error) {
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
		if err := h.runContainer(ssh, name, repo.Name, ref, "", "", docker, vaultHost, vaultToken, "", seq, &out); err != nil {
			return "", err
		}
		return ip, nil
	}

	for _, svc := range services {
		bootstrap := ""
		if strings.Contains(svc.Name, "postoffice") {
			bootstrap = clusterBootstrap
		}
		if err := h.runContainer(ssh, name, svc.Name, ref, svc.Network, svc.HttpBinding, docker, vaultHost, vaultToken, bootstrap, seq, &out); err != nil {
			return "", err
		}
	}
	return ip, nil
}

func (h *deployHandler) runContainer(ssh util.SshClient, instanceName, svcName, ref, network, httpBinding string, docker *protocol.DockerAccess, vaultHost, vaultToken, clusterBootstrap string, seq int, out *bytes.Buffer) error {
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
		fmt.Sprintf("docker run -d --restart unless-stopped --name %s %s %s", svcName, runArgs, image),
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

func (h *deployHandler) seedCredentials(spec *core.CredentialSpec) error {
	vc := &util.VaultClient{Host: h.mgr.F.Vlt.Host, Token: h.mgr.F.Vlt.Token}
	if err := vc.Auth(); err != nil {
		return fmt.Errorf("vault auth: %w", err)
	}
	existing, _ := vc.GetSecret(spec.VaultMount, spec.VaultPath)
	if existing != nil && len(existing.Data) > 0 {
		core.AppLog.Info().Msgf("credentials already at %s/%s, skipping seed", spec.VaultMount, spec.VaultPath)
		return nil
	}
	data := make(map[string]any, len(spec.Fields))
	for key, field := range spec.Fields {
		if field.Generate {
			pass, err := randomPassword(24)
			if err != nil {
				return fmt.Errorf("generate %s: %w", key, err)
			}
			data[key] = pass
		} else {
			data[key] = field.Value
		}
	}
	if err := vc.PutSecretMap(spec.VaultMount, spec.VaultPath, data); err != nil {
		return fmt.Errorf("put secret %s/%s: %w", spec.VaultMount, spec.VaultPath, err)
	}
	core.AppLog.Info().Msgf("credentials seeded at %s/%s", spec.VaultMount, spec.VaultPath)
	return nil
}

func randomPassword(length int) (string, error) {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b), nil
}

func (h *deployHandler) confirm(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("deploy confirm %v", t.Meta)
	return h.store.Insert(t.Meta)
}

func (h *deployHandler) cancel(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("deploy cancel %v", t.Meta)
	return h.store.Insert(t.Meta)
}
