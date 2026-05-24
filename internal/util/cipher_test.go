package util

import (
	"testing"
)

func TestCipher(t *testing.T) {
	cipher := Aes{Ksz: 32}
	err := cipher.AesGcm()
	if err != nil {
		t.Errorf("failed %s\n", err.Error())
	}
	cipherText := cipher.Encrypt("hello work")
	txt, err := cipher.Decrypt(cipherText)
	if err != nil{
		t.Errorf("failed %s\n", err.Error())
	}
	if txt != "hello work"{
		t.Errorf("failed %s\n", err.Error())
	}

	cipherText1 := cipher.Encrypt("hello work 123")
	txt1, err := cipher.Decrypt(cipherText1)
	if err != nil{
		t.Errorf("failed %s\n", err.Error())
	}
	if txt1 != "hello work 123"{
		t.Errorf("failed %s\n", err.Error())
	}
}
