package core

type Password interface {
	HashPassword(password string) (string, error)
	ValidatePassword(password string, hash string) error
}
