package main

import (
	"encoding/json"
	"net/http"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
)

type SudoAddLogin struct {
	*AdminService
}

func (s *SudoAddLogin) AccessControl() int32 {
	return core.SUDO_ACCESS_CONTROL
}
func (s *SudoAddLogin) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var login protocol.LoginObject
	json.NewDecoder(r.Body).Decode(&login)
	if login.AccessControl > uint32(rs.AccessControl) {
		session := core.OnSession{Successful: false, Message: "over permission"}
		w.Write(util.ToJson(session))
		return
	}
	if login.AccessControl <= 0 {
		login.AccessControl = uint32(core.PROTECTED_ACCESS_CONTROL)
	}
	hash, err := s.Authenticator().HashPassword(login.Password)
	if err != nil {
		session := core.OnSession{Successful: false, Message: err.Error()}
		w.Write(util.ToJson(session))
		return
	}
	w.WriteHeader(http.StatusOK)
	login.Password = hash
	err = s.SaveLogin(&login)
	if err != nil {
		session := core.OnSession{Successful: false, Message: err.Error()}
		w.Write(util.ToJson(session))
		return
	}
	session := core.OnSession{Successful: true, Message: "new login added"}
	w.Write(util.ToJson(session))
}
