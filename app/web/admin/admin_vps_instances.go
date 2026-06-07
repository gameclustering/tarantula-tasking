package main

import (
	"net/http"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/util"
)

type AdminVpsInstances struct {
	*AdminService
}

func (s *AdminVpsInstances) AccessControl() int32 {
	return core.ADMIN_ACCESS_CONTROL
}

func (s *AdminVpsInstances) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	vendor := r.URL.Query().Get("vendor")
	if vendor == "" {
		vendor = "vultr"
	}
	if vendor != "vultr" {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "vendor not supported: " + vendor}))
		return
	}

	vpsKey, err := s.Cluster().AuthKey(vendor)
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "failed to load vps key: " + err.Error()}))
		return
	}

	va := util.VultrApi{ApiKey: vpsKey.Vps.ApiKey}
	instances, err := va.ListInstances()
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "failed to list instances: " + err.Error()}))
		return
	}
	w.Write(util.ToJson(instances))
}
