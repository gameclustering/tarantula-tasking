package main

import (
	"encoding/json"
	"net/http"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
)

type AdminChangePwd struct {
	*AdminService
}

type ChangePassword struct {
	Login  string `json:"login"`
	OldPwd string `json:"oldPassword"`
	NewPwd string `json:"newPassword"`
}

func (s *AdminChangePwd) AccessControl() int32 {
	return core.ADMIN_ACCESS_CONTROL
}
func (s *AdminChangePwd) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var cp ChangePassword
	json.NewDecoder(r.Body).Decode(&cp)
	login := protocol.LoginObject{Name: cp.Login}
	err := s.LoadLogin(&login)
	w.WriteHeader(http.StatusOK)
	if err != nil {
		session := core.OnSession{Successful: false, Message: err.Error()}
		w.Write(util.ToJson(session))
		return
	}
	hash, err := s.Authenticator().HashPassword(cp.NewPwd)
	if err != nil {
		session := core.OnSession{Successful: false, Message: err.Error()}
		w.Write(util.ToJson(session))
		return
	}
	login.Password = hash
	err = s.UpdatePassword(&login)
	if err != nil {
		session := core.OnSession{Successful: false, Message: err.Error()}
		w.Write(util.ToJson(session))
		return
	}
	session := core.OnSession{Successful: true, Message: "password changed"}
	w.Write(util.ToJson(session))
}
