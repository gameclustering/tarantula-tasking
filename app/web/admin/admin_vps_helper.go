package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/util"
)

// runSetupCmds executes the VPS provisioning command sequence on an already-connected SshClient.
// Installs Docker, Git, creates the tarantula user, and copies the vault SSH public key.
func runSetupCmds(sc *util.SshClient, pubKey, username, sudoPassword string) error {
	cmds := []string{
		"export DEBIAN_FRONTEND=noninteractive && apt-get update -y && apt-get install -y ca-certificates curl git",
		"curl -fsSL https://download.docker.com/linux/$(. /etc/os-release && echo $ID)/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg",
		"echo \"deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/$(. /etc/os-release && echo $ID) $(. /etc/os-release && echo $VERSION_CODENAME) stable\" > /etc/apt/sources.list.d/docker.list",
		"export DEBIAN_FRONTEND=noninteractive && apt-get update -y && apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin",
		"systemctl enable docker && systemctl start docker",
		fmt.Sprintf("id -u %s > /dev/null 2>&1 || useradd -m -s /bin/bash %s", username, username),
		fmt.Sprintf("usermod -aG docker %s", username),
		fmt.Sprintf("mkdir -p /home/%s/.ssh && chmod 700 /home/%s/.ssh", username, username),
		fmt.Sprintf("grep -qxF '%s' /home/%s/.ssh/authorized_keys 2>/dev/null || echo '%s' >> /home/%s/.ssh/authorized_keys",
			pubKey, username, pubKey, username),
		fmt.Sprintf("chmod 600 /home/%s/.ssh/authorized_keys && chown -R %s:%s /home/%s/.ssh",
			username, username, username, username),
	}
	if sudoPassword != "" {
		cmds = append(cmds,
			fmt.Sprintf("echo '%s:%s' | chpasswd", username, sudoPassword),
			fmt.Sprintf("echo '%s ALL=(ALL) ALL' > /etc/sudoers.d/%s && chmod 440 /etc/sudoers.d/%s", username, username, username),
		)
	}

	var buf bytes.Buffer
	for _, cmd := range cmds {
		buf.Reset()
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		err := sc.Run(ctx, cmd, &buf)
		cancel()
		if err != nil {
			return fmt.Errorf("setup failed: %s — %s", err.Error(), strings.TrimSpace(buf.String()))
		}
		core.AppLog.Info().Msgf("vps setup [%s]: %s", sc.Host, strings.TrimSpace(buf.String()))
	}
	return nil
}
