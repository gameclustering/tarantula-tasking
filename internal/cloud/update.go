package cloud

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"gameclustering.com/internal/bootstrap"
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewUpdate(mgr *bootstrap.AppManager, store *Store, vaultKey string, factory PlatformFactory) *protocol.TccTransationListener {
	h := &updateHandler{mgr: mgr, store: store, vaultKey: vaultKey, factory: factory}
	tcc := protocol.TccTransationListener{}
	tcc.Reserve = h.reserve
	tcc.Confirm = h.confirm
	tcc.Cancel = h.cancel
	return &tcc
}

type updateHandler struct {
	mgr      *bootstrap.AppManager
	store    *Store
	vaultKey string
	factory  PlatformFactory
}

func (h *updateHandler) reserve(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("update reserve %v", t.Meta)
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
	deployPhase := cfg.Resolve(plan.Env, "deploy")

	platformKey, err := h.mgr.Cluster().AuthKey(h.vaultKey)
	if err != nil {
		return fmt.Errorf("%s auth key: %w", h.vaultKey, err)
	}
	platform, err := h.factory(deployPhase, platformKey)
	if err != nil {
		return fmt.Errorf("platform init: %w", err)
	}
	defer platform.Close()

	keyFile, err := os.CreateTemp("", "id_ed25519_*")
	if err != nil {
		return fmt.Errorf("create temp key file: %w", err)
	}
	defer os.Remove(keyFile.Name())
	if _, err := keyFile.WriteString(util.NormalizePemKey(gitKey.Git.Key)); err != nil {
		keyFile.Close()
		return fmt.Errorf("write git key: %w", err)
	}
	keyFile.Close()

	sshUser := deployPhase.SshUser
	if sshUser == "" {
		sshUser = platform.SSHUser()
	}

	for i := 1; i <= deployPhase.InstanceNumber; i++ {
		name := fmt.Sprintf("%s-%02d", deployPhase.Prefix, i)
		if err := h.setupInstance(platform, name, sshUser, keyFile.Name()); err != nil {
			core.AppLog.Warn().Msgf("setup instance %s: %s", name, err.Error())
		}
	}
	return h.store.Insert(t.Meta)
}

func (h *updateHandler) setupInstance(platform InstancePlatform, name, sshUser, keyFile string) error {
	ip, err := platform.IP(name)
	if err != nil {
		return fmt.Errorf("get IP: %w", err)
	}
	ssh := util.SshClient{Host: ip, User: sshUser, PrivateKey: platform.SSHKey(), KHFile: "../.ssh/known_hosts"}

	const maxWait = 5 * time.Minute
	deadline := time.Now().Add(maxWait)
	for {
		if err := ssh.WithKey(); err == nil {
			break
		} else if time.Now().After(deadline) {
			return fmt.Errorf("ssh connect: timed out: %w", err)
		}
		core.AppLog.Debug().Msgf("setup [%s]: waiting for SSH...", name)
		time.Sleep(10 * time.Second)
	}
	defer ssh.Close()

	if err := h.installDocker(ssh, sshUser, name); err != nil {
		return fmt.Errorf("install docker: %w", err)
	}
	if err := h.uploadGitKey(ssh, sshUser, keyFile, name); err != nil {
		return fmt.Errorf("upload git key: %w", err)
	}
	return nil
}

func (h *updateHandler) installDocker(ssh util.SshClient, user string, name string) error {
	var out bytes.Buffer
	cmds := []string{
		"mkdir -p ~/.ssh && chmod 700 ~/.ssh",
		"sudo apt-get update -qq",
		"sudo apt-get install -y -qq ca-certificates curl",
		"sudo install -m 0755 -d /etc/apt/keyrings",
		"sudo curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc",
		"sudo chmod a+r /etc/apt/keyrings/docker.asc",
		`echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null`,
		"sudo apt-get update -qq",
		"sudo apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin git",
		fmt.Sprintf("sudo usermod -aG docker %s", user),
		"ssh-keyscan github.com >> ~/.ssh/known_hosts",
	}
	for _, cmd := range cmds {
		out.Reset()
		if err := ssh.Run(cmd, &out); err != nil {
			return fmt.Errorf("cmd %q: %w — %s", cmd, err, out.String())
		}
		core.AppLog.Debug().Msgf("setup [%s]: %s", name, out.String())
	}
	return nil
}

func (h *updateHandler) uploadGitKey(ssh util.SshClient, user string, keyFile string, name string) error {
	f, err := os.Open(keyFile)
	if err != nil {
		return fmt.Errorf("open key file: %w", err)
	}
	defer f.Close()
	remotePath := fmt.Sprintf("/home/%s/.ssh/id_ed25519", user)
	if err := ssh.Upload(f, remotePath, "0600"); err != nil {
		return fmt.Errorf("upload: %w", err)
	}
	core.AppLog.Info().Msgf("git key uploaded to %s on %s", remotePath, name)
	return nil
}

func (h *updateHandler) confirm(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("update confirm %v", t.Meta)
	return h.store.Insert(t.Meta)
}

func (h *updateHandler) cancel(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("update cancel %v", t.Meta)
	return h.store.Insert(t.Meta)
}
