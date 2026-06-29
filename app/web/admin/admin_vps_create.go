package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/util"
	"golang.org/x/crypto/ssh"
)

type vpsCreateRequest struct {
	Label  string `json:"label"`
	Region string `json:"region"`
	Plan   string `json:"plan"`
	OsId   int    `json:"osId"`
	Vendor string `json:"vendor"`
	Setup  bool   `json:"setup"`
}

type AdminVpsCreate struct {
	*AdminService
}

func (s *AdminVpsCreate) AccessControl() int32 {
	return core.SUDO_ACCESS_CONTROL
}

func (s *AdminVpsCreate) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var req vpsCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "invalid request body"}))
		return
	}
	if req.Vendor == "" {
		req.Vendor = "vultr"
	}
	if req.Vendor != "vultr" {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "vendor not supported: " + req.Vendor}))
		return
	}
	if req.Label == "" || req.Region == "" || req.Plan == "" || req.OsId == 0 {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "label, region, plan and osId are required"}))
		return
	}

	vpsKey, err := s.Cluster().AuthKey(req.Vendor)
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "failed to load vps key: " + err.Error()}))
		return
	}

	va := util.VultrApi{ApiKey: vpsKey.Vps.ApiKey}

	sshKeyIds := []string{}
	if vpsKey.Vps.Ssh != "" {
		id, err := va.FindSshKeyId(vpsKey.Vps.Ssh)
		if err != nil {
			core.AppLog.Warn().Msgf("FindSshKeyId: %s", err.Error())
		} else if id != "" {
			sshKeyIds = append(sshKeyIds, id)
		}
	}

	instance, err := va.CreateInstance(req.Label, req.Region, req.Plan, req.OsId, sshKeyIds)
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "create instance failed: " + err.Error()}))
		return
	}

	if !req.Setup {
		w.Write(util.ToJson(map[string]any{
			"successful": true,
			"instance":   instance,
		}))
		return
	}

	// Poll until active (up to 5 minutes).
	core.AppLog.Info().Msgf("waiting for instance %s to become active", instance.Id)
	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		time.Sleep(15 * time.Second)
		inst, err := va.GetInstance(instance.Id)
		if err != nil {
			core.AppLog.Warn().Msgf("poll instance %s: %s", instance.Id, err.Error())
			continue
		}
		instance = inst
		core.AppLog.Info().Msgf("instance %s status=%s ip=%s", instance.Id, instance.Status, instance.MainIP)
		if instance.Status == "active" && instance.MainIP != "" && instance.MainIP != "0.0.0.0" {
			break
		}
	}

	if instance.Status != "active" {
		w.Write(util.ToJson(map[string]any{
			"successful": false,
			"message":    "instance did not become active within 5 minutes",
			"instance":   instance,
		}))
		return
	}

	// Extra boot wait — cloud-init and sshd may not be ready immediately.
	time.Sleep(20 * time.Second)

	signer, err := ssh.ParsePrivateKey([]byte(vpsKey.Vps.Ssh))
	if err != nil {
		w.Write(util.ToJson(map[string]any{
			"successful": false,
			"message":    "invalid vps ssh key: " + err.Error(),
			"instance":   instance,
		}))
		return
	}
	pubKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))

	sc := &util.SshClient{Host: instance.MainIP, User: "root", PrivateKey: vpsKey.Vps.Ssh}
	if err := sc.WithKeyInsecure(); err != nil {
		w.Write(util.ToJson(map[string]any{
			"successful": false,
			"message":    "ssh connect failed: " + err.Error(),
			"instance":   instance,
		}))
		return
	}
	defer sc.Close()

	if err := runSetupCmds(sc, pubKey, vpsKey.Vps.User, vpsKey.Vps.Password); err != nil {
		w.Write(util.ToJson(map[string]any{
			"successful": false,
			"message":    "instance created but setup failed: " + err.Error(),
			"instance":   instance,
		}))
		return
	}

	w.Write(util.ToJson(map[string]any{
		"successful": true,
		"message":    "instance created and setup complete",
		"instance":   instance,
	}))
}
