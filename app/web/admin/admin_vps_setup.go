package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/util"
	"golang.org/x/crypto/ssh"
)

type vpsSetupRequest struct {
	IP       string `json:"ip"`
	Password string `json:"password"`
	Vendor   string `json:"vendor"`
}

type AdminVpsSetup struct {
	*AdminService
}

func (s *AdminVpsSetup) AccessControl() int32 {
	return core.SUDO_ACCESS_CONTROL
}

func (s *AdminVpsSetup) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var req vpsSetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "invalid request body"}))
		return
	}
	if req.Vendor == "" || req.IP == "" || req.Password == "" {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "vendor, ip and password are required"}))
		return
	}
	if req.Vendor != "vultr" {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "vendor not supported: " + req.Vendor}))
		return
	}

	vpsKey, err := s.Cluster().AuthKey(req.Vendor)
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "failed to load vps key: " + err.Error()}))
		return
	}

	signer, err := ssh.ParsePrivateKey([]byte(vpsKey.Vps.Ssh))
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "invalid vps ssh key: " + err.Error()}))
		return
	}
	pubKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))

	sc := &util.SshClient{Host: req.IP, User: "root", Password: req.Password}
	if err := sc.WithPassword(); err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "ssh connect failed: " + err.Error()}))
		return
	}
	defer sc.Close()

	username := vpsKey.Vps.User
	sudoPassword := vpsKey.Vps.Password
	cmds := []string{
		// prerequisites
		"export DEBIAN_FRONTEND=noninteractive && apt-get update -y && apt-get install -y ca-certificates curl git",
		// Docker official GPG key
		"curl -fsSL https://download.docker.com/linux/$(. /etc/os-release && echo $ID)/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg",
		// Docker apt repo
		"echo \"deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/$(. /etc/os-release && echo $ID) $(. /etc/os-release && echo $VERSION_CODENAME) stable\" > /etc/apt/sources.list.d/docker.list",
		// install latest Docker Engine
		"export DEBIAN_FRONTEND=noninteractive && apt-get update -y && apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin",
		"systemctl enable docker && systemctl start docker",
		// create user and grant docker access
		fmt.Sprintf("id -u %s > /dev/null 2>&1 || useradd -m -s /bin/bash %s", username, username),
		fmt.Sprintf("usermod -aG docker %s", username),
		// install vault SSH public key
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
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := sc.Run(ctx, cmd, &buf); err != nil {
			w.Write(util.ToJson(core.OnSession{
				Successful: false,
				Message:    fmt.Sprintf("setup failed: %s — %s", err.Error(), strings.TrimSpace(buf.String())),
			}))
			return
		}
		core.AppLog.Info().Msgf("vps setup [%s]: %s", req.IP, strings.TrimSpace(buf.String()))
	}

	w.Write(util.ToJson(core.OnSession{Successful: true, Message: "VPS setup complete"}))
}
