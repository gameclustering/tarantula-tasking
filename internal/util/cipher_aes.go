package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"

	"encoding/hex"
)

type Aes struct {
	Gcm cipher.AEAD
	Ksz int16
}

func (g *Aes) AesGcm() error {
	key := make([]byte, g.Ksz)
	rand.Read(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	g.Gcm = gcm
	return nil
}

func (g *Aes) AesGcmFromKey(key []byte) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	g.Gcm = gcm
	return nil
}

func (g *Aes) Encrypt(clearText string) string {
	nonce := make([]byte, g.Gcm.NonceSize())
	rand.Read(nonce)
	cipherText := g.Gcm.Seal(nonce, nonce, []byte(clearText), nil)
	return hex.EncodeToString(cipherText)
}

func (g *Aes) Decrypt(cipherText string) (string, error) {
	pendingText, err := hex.DecodeString(cipherText)
	if err != nil {
		return cipherText, err
	}
	clearText, err := g.Gcm.Open(nil, pendingText[:g.Gcm.NonceSize()], pendingText[g.Gcm.NonceSize():], nil)
	if err != nil {
		return cipherText, err
	}
	return string(clearText), nil
}
