package main

import (
	"net/http"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/util"
)

type AdminKeyRingEndpoint struct {
	*AdminService
}

func (s *AdminKeyRingEndpoint) AccessControl() int32 {
	return core.ADMIN_ACCESS_CONTROL
}

func (s *AdminKeyRingEndpoint) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	key := r.PathValue("key")
	resp, err := s.Cluster().KeyRing(core.RingRequest{Token: s.Cluster().RingToken([]byte(key))})
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	ring := make([]core.Node, 0)
	for _, data := range resp.Nodes {
		ring = append(ring, core.Node{Name: data.Name, RingToken: data.Hash, RpcEndpoint: data.Endpoint, IP: data.Address})
	}
	w.Write(util.ToJson(ring))
}
