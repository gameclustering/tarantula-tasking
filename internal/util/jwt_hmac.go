package util

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"hash"
	"strings"
	"sync"

	"gameclustering.com/internal/core"
)

type JwtHMac struct {
	Mac  hash.Hash
	Alg  string
	Ksz  int16
	lock *sync.Mutex
}

func (j *JwtHMac) HMac() {
	key := make([]byte, j.Ksz)
	rand.Read(key)
	j.Mac = hmac.New(sha256.New, key)
	j.lock = &sync.Mutex{}
}

func (j *JwtHMac) HMacFromKey(key []byte) {
	j.Mac = hmac.New(sha256.New, key)
	j.lock = &sync.Mutex{}
}

func (j *JwtHMac) Token(jp core.JwtProcess) (string, error) {
	j.lock.Lock()
	defer j.lock.Unlock()
	h := core.JwtHeader{Alg: j.Alg, Typ: "JWT"}
	p := core.JwtPayload{}
	err := jp(&h, &p)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	hd, _ := json.Marshal(h)
	sb.WriteString(base64.URLEncoding.EncodeToString(hd))
	sb.WriteString(".")
	pd, _ := json.Marshal(p)
	sb.WriteString(base64.URLEncoding.EncodeToString(pd))
	j.Mac.Reset()
	j.Mac.Write([]byte(sb.String()))
	sum := j.Mac.Sum(nil)
	sb.WriteString(".")
	sb.WriteString(base64.URLEncoding.EncodeToString(sum))
	return sb.String(), nil
}

func (j *JwtHMac) Verify(token string, jp core.JwtProcess) error {
	j.lock.Lock()
	defer j.lock.Unlock()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return errors.New("bad token format")
	}
	var sb strings.Builder
	sb.WriteString(parts[0])
	sb.WriteString(".")
	sb.WriteString(parts[1])
	j.Mac.Reset()
	j.Mac.Write([]byte(sb.String()))
	sum := j.Mac.Sum(nil)
	if base64.URLEncoding.EncodeToString(sum) != parts[2] {
		return errors.New("bad signature")
	}

	h, _ := base64.URLEncoding.DecodeString(parts[0])
	p, _ := base64.URLEncoding.DecodeString(parts[1])

	th := core.JwtHeader{}
	json.Unmarshal(h, &th)
	tp := core.JwtPayload{}
	json.Unmarshal(p, &tp)
	err := jp(&th, &tp)
	if err != nil {
		return err
	}
	return nil
}
