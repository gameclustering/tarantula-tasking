package main

import (
	"encoding/json"
	"net/http"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
)

type AdminLogin struct {
	*AdminService
}

func (s *AdminLogin) AccessControl() int32 {
	return core.PUBLIC_ACCESS_CONTROL
}
func (s *AdminLogin) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var login protocol.LoginObject
	err := json.NewDecoder(r.Body).Decode(&login)
	if err != nil {
		session := core.OnSession{Successful: false, Message: err.Error()}
		w.Write(util.ToJson(session))
		return
	}
	pwd := login.Password
	err = s.LoadLogin(&login)
	w.WriteHeader(http.StatusOK)
	if err != nil {
		session := core.OnSession{Successful: false, Message: err.Error()}
		w.Write(util.ToJson(session))
		return
	}
	err = s.Authenticator().ValidatePassword(pwd, login.Password)
	if err != nil {
		session := core.OnSession{Successful: false, Message: err.Error()}
		w.Write(util.ToJson(session))
		return
	}
	tk, err := s.Authenticator().CreateToken(int64(login.SystemId), int32(login.Id), int32(login.AccessControl))
	if err != nil {
		session := core.OnSession{Successful: false, Message: err.Error()}
		w.Write(util.ToJson(session))
		return
	}
	session := core.OnSession{Successful: true, SystemId: int64(login.SystemId), Stub: int32(login.Id), Token: tk, Home: ""}
	w.Write(util.ToJson(session))
}
