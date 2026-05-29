package util

import (
	"encoding/base64"
	"os"
	"testing"
)

func vaultClient(t *testing.T) *VaultClient {
	host := os.Getenv("VAULT_HOST")
	token := os.Getenv("VAULT_TOKEN")
	if host == "" || token == "" {
		t.Skip("VAULT_HOST and VAULT_TOKEN env vars not set")
	}
	return &VaultClient{Host: host, Token: token}
}

func TestPassword(t *testing.T) {
	h, err := HashPassword("password")
	if err != nil {
		t.Errorf("failed %s\n", err.Error())
	}
	er := ValidatePassword("password", h)
	if er != nil {
		t.Errorf("failed %s\n", er.Error())
	}
}

func TestPartition(t *testing.T) {
	p := Partition([]byte("hellp"), 5)
	if p > 5 {
		t.Errorf("falied partition %d\n", p)
	}
}

func TestKey(t *testing.T) {
	skey := KeyToBase64(Key(32))
	bkey, err := KeyFromBase64(skey)
	if err != nil {
		t.Errorf("bad format key %s\n", err.Error())
	}
	ckey := base64.StdEncoding.EncodeToString(bkey)
	if skey != ckey {
		t.Errorf("key not same %s %s \n", skey, ckey)
	}
}
