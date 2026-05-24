package bootstrap

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/util"
)

const (
	TOKEN_TIME_OUT_HOURS    int = 24
	TICKET_TIME_OUT_MINUTES int = 60
)

type AuthManager struct {
	Kid    string
	Tkn    core.Jwt
	Cipher *util.Aes
}

func (s *AuthManager) HashPassword(password string) (string, error) {
	return util.HashPassword(password)
}
func (s *AuthManager) ValidatePassword(password string, hash string) error {
	return util.ValidatePassword(password, hash)
}
func (s *AuthManager) CreateToken(systemId int64, stub int32, accessControl int32) (string, error) {
	return s.Tkn.Token(func(h *core.JwtHeader, p *core.JwtPayload) error {
		h.Kid = s.Kid
		exp := time.Now().Add(time.Hour * time.Duration(TOKEN_TIME_OUT_HOURS)).UTC()
		p.Exp = exp.UnixMilli()
		aud := fmt.Sprintf("%d.%d.%d.%d", systemId, stub, accessControl, p.Exp)
		p.Aud = s.Cipher.Encrypt(aud)
		return nil
	})
}
func (s *AuthManager) ValidateToken(token string) (core.OnSession, error) {
	session := core.OnSession{Successful: false}
	err := s.Tkn.Verify(token, func(jh *core.JwtHeader, jp *core.JwtPayload) error {
		aud, err := s.Cipher.Decrypt(jp.Aud)
		if err != nil {
			return err
		}
		parts := strings.Split(aud, ".")
		sysId, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return err
		}
		stub, err := strconv.ParseInt(parts[1], 10, 32)
		if err != nil {
			return err
		}
		acc, err := strconv.ParseInt(parts[2], 10, 32)
		if err != nil {
			return err
		}
		exp, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			return err
		}
		if exp != jp.Exp {
			return errors.New("invalid timestamp")
		}
		tm := time.UnixMilli(exp)
		if tm.UTC().Before(time.Now().UTC()) {
			return errors.New("token timeout")
		}
		session.SystemId = sysId
		session.Stub = int32(stub)
		session.AccessControl = int32(acc)
		session.Successful = true
		return nil
	})
	if err != nil {
		return session, err
	}
	return session, nil
}

func (s *AuthManager) CreateTicket(systemId int64, stub int32, accessControl int32, durationMinutes int) (string, error) {
	exp := time.Now().Add(time.Minute * time.Duration(durationMinutes)).UTC()
	aud := fmt.Sprintf("%d.%d.%d.%d", systemId, stub, accessControl, exp.UnixMilli())
	ticket := s.Cipher.Encrypt(aud)
	return ticket, nil
}
func (s *AuthManager) ValidateTicket(ticket string) (core.OnSession, error) {
	data, err := s.Cipher.Decrypt(ticket)
	session := core.OnSession{}
	if err != nil {
		return session, err
	}
	parts := strings.Split(data, ".")
	if len(parts) != 4 {
		return session, fmt.Errorf("wrong ticket format %s", data)
	}
	sysId, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return session, err
	}
	session.SystemId = sysId
	stub, err := strconv.ParseInt(parts[1], 10, 32)
	if err != nil {
		return session, err
	}
	session.Stub = int32(stub)
	acc, err := strconv.ParseInt(parts[2], 10, 32)
	if err != nil {
		return session, err
	}
	session.AccessControl = int32(acc)
	exp, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return session, err
	}
	tm := time.UnixMilli(exp)
	if tm.UTC().Before(time.Now().UTC()) {
		return session, errors.New("token timeout")
	}
	return session, nil
}
