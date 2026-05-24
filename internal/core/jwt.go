package core

type JwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
	Kid string `json:"kid"`
}

const (
	JWT_KEY_NAME string = "jwtkey"
	JWT_KEY_SIZE int16  = 32
	JWT_ALG      string = "SHS256"
)

type JwtPayload struct {
	Aud string `json:"aud"`
	Exp int64  `json:"exp"`
}

type JwtProcess func(*JwtHeader, *JwtPayload) error

type Jwt interface {
	Token(jp JwtProcess) (string, error)
	Verify(token string, jp JwtProcess) error
}
