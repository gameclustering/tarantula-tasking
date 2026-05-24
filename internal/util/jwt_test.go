package util

import (
	"crypto/rand"
	"errors"
	"fmt"
	"testing"
	"time"

	"gameclustering.com/internal/core"
)

func onToken(h *core.JwtHeader, p *core.JwtPayload) error {
	h.Kid = "kid"
	p.Aud = "player"
	exp := time.Now().Add(time.Hour * 24).UTC()
	//fmt.Println(exp)
	p.Exp = exp.UnixMilli()
	return nil
}

func onVerify(h *core.JwtHeader, p *core.JwtPayload) error {
	//fmt.Println(h.Kid)
	//fmt.Println(p.Aud)
	t := time.UnixMilli(p.Exp).UTC()
	if t.Before(time.Now().UTC()) {
		return errors.New("token expired")
	}
	//fmt.Println(t)
	return nil
}

func TestSum(t *testing.T) {
	jwt := JwtHMac{Alg: "SHS256"}
	key := make([]byte, 32)
	rand.Read(key)
	jwt.HMacFromKey(key)
	token, _ := jwt.Token(onToken)
	fmt.Printf("Len : %d\n", len(token))
	time.Sleep(1000 * time.Millisecond)
	err := jwt.Verify(token, onVerify)
	if err != nil {
		t.Errorf("failed %s\n", err.Error())
	}
}
