package main

import (
	"io"
	"net/http"
	"os"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/util"
)

type AdminWebProtected struct {
	*AdminService
}

func (s *AdminWebProtected) AccessControl() int32 {
	return core.ADMIN_ACCESS_CONTROL
}
func (s *AdminWebProtected) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	fn := r.PathValue("name")
	dest, err := os.OpenFile(s.contentDir+"/"+fn, os.O_RDONLY, 0644)
	if err != nil {
		session := core.OnSession{Successful: false, Message: err.Error()}
		w.Write(util.ToJson(session))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	defer dest.Close()
	io.Copy(w, dest)
}
