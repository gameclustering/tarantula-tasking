package core



type Token interface {
	CreateToken(systemId int64, stub int32, accessControl int32) (string, error)
	ValidateToken(token string) (OnSession, error)
}
