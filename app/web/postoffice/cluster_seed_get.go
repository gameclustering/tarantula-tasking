package main

import (
	"encoding/json"
	"net"
	"net/http"

	"gameclustering.com/internal/core"
)

type ClusterSeedGet struct {
	*PostofficeService
}

func (s *ClusterSeedGet) AccessControl() int32 {
	return core.PUBLIC_ACCESS_CONTROL
}

func (s *ClusterSeedGet) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	members := s.mm.Members()
	seeds := make([]string, 0, len(members))
	for _, m := range members {
		host, _, err := net.SplitHostPort(m.Address())
		if err != nil {
			continue
		}
		seeds = append(seeds, host)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(seeds)
}
