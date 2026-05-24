package bootstrap

import (
	"crypto/rand"
	"testing"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/util"
)

func TestAuthManager(t *testing.T) {

	tkn := util.JwtHMac{Alg: core.JWT_ALG}
	key := make([]byte, tkn.Ksz)
	rand.Read(key)
	tkn.HMacFromKey(key)

	ci := util.Aes{Ksz: 32}
	ckey := make([]byte, ci.Ksz)
	rand.Read(ckey)
	err := ci.AesGcmFromKey(ckey)
	if err != nil {
		t.Errorf("Error%s", err.Error())
	}
	auth := AuthManager{Tkn: &tkn, Cipher: &ci, Kid: "presence"}
	tk, err := auth.CreateToken(100, 120, 1)
	if err != nil {
		t.Errorf("Error%s", err.Error())
	}
	session, err := auth.ValidateToken(tk)
	if err != nil {
		t.Errorf("Error%s", err.Error())
	}
	if session.SystemId != 100 {
		t.Errorf("Error SystemId%d", session.SystemId)
	}

	if session.AccessControl != 1 {
		t.Errorf("Error Access%d", session.AccessControl)
	}

	if session.Stub != 120 {
		t.Errorf("Error Stub%d", session.SystemId)
	}
}
