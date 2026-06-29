package main

import (
	"encoding/json"
	"net/http"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/util"
)

type vpsCreateRequest struct {
	Label  string `json:"label"`
	Region string `json:"region"`
	Plan   string `json:"plan"`
	OsId   int    `json:"osId"`
	Vendor string `json:"vendor"`
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

	// Match vault SSH private key to a registered Vultr SSH key so it is
	// attached to the instance at creation time, enabling immediate key login.
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

	w.Write(util.ToJson(map[string]any{
		"successful": true,
		"instance":   instance,
	}))
}
