package main

import (
	"net/http"

	"gameclustering.com/internal/core"
)

type ClusterHealthCheck struct {
	*PostofficeService
}

func (s *ClusterHealthCheck) AccessControl() int32 {
	return core.PUBLIC_ACCESS_CONTROL
}

func (s *ClusterHealthCheck) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("{}"))
}
