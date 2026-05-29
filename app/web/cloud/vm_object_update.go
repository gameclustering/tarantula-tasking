package main

import (
	"bytes"
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

	keyFile, err := os.CreateTemp("", "id_ed25519_*")
	if err != nil {
		return fmt.Errorf("create temp key file: %w", err)
	}
	defer os.Remove(keyFile.Name())
	if _, err := keyFile.WriteString(gitKey.Git.Key); err != nil {
		keyFile.Close()
		return fmt.Errorf("write git key: %w", err)
	}
	keyFile.Close()

	for i := uint32(1); i <= vm.NumberOfInstances; i++ {
		name := fmt.Sprintf("%s-%02d", gcpKey.Gcp.Prefix, i)
		if err := v.setupInstance(gcp, gcpKey.Gcp.Ssh, gcpKey.Gcp.User, name, keyFile.Name()); err != nil {
			core.AppLog.Warn().Msgf("setup instance %s: %s", name, err.Error())
		}
	}
	return v.insert(t.Meta)
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

	if err := v.installDocker(ssh, user, name); err != nil {
		return fmt.Errorf("install docker: %w", err)
	}

	if err := v.uploadGitKey(ssh, user, keyFile, name); err != nil {
		return fmt.Errorf("upload git key: %w", err)
	}
	return nil
}

func (v *VMObjectUpdate) installDocker(ssh util.SshClient, user string, name string) error {
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
		"sudo apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin",
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

func (v *VMObjectUpdate) uploadGitKey(ssh util.SshClient, user string, keyFile string, name string) error {
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

func (v *VMObjectUpdate) confirm(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("update confirm %v", t.Meta)
	return v.insert(t.Meta)
}

func (v *VMObjectUpdate) cancel(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("update cancel %v", t.Meta)
	return v.insert(t.Meta)
}
