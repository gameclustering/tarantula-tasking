package main

import (
	"net/http"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/util"
)

type AdminSubscriptionTaskEndpoint struct {
	*AdminService
}

func (s *AdminSubscriptionTaskEndpoint) AccessControl() int32 {
	return core.ADMIN_ACCESS_CONTROL
}
func (s *AdminSubscriptionTaskEndpoint) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	resp, err := s.Cluster().TaskList()
	if err != nil {
		w.Write(util.ToJson("[]"))
		return
	}
	w.Write(util.ToJson(resp.Subscriptions))
}
