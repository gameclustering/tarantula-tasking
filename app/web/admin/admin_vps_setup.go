package main

import (
	"encoding/json"
	"net/http"
	"strings"

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

	if err := runSetupCmds(sc, pubKey, vpsKey.Vps.User, vpsKey.Vps.Password); err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}

	w.Write(util.ToJson(core.OnSession{Successful: true, Message: "VPS setup complete"}))
}
