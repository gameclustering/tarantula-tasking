package core

const (
	CIPHER_KEY_NAME string = "cipherkey"
	CIPHER_KEY_SIZE int16  = 32
)

type Cipher interface {
	Encrypt(clearText string) string
	Decrypt(cipherText string) (string, error)
}
