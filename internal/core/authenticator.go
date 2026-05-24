package core

const (
	PUBLIC_ACCESS_CONTROL    int32 = 0
	PROTECTED_ACCESS_CONTROL int32 = 1
	ADMIN_ACCESS_CONTROL     int32 = 30
	SUDO_ACCESS_CONTROL      int32 = 100
)

type Authenticator interface {
	Password
	Token
	Ticket
}

type OnSession struct {
	Successful    bool   `json:"successful"`
	ErrorCode     int    `json:"errorCode"`
	Message       string `json:"message,omitempty"`
	SystemId      int64  `json:"systemId,omitempty"`
	Stub          int32  `json:"-"`
	Token         string `json:"token,omitempty"`
	Ticket        string `json:"ticket,omitempty"`
	Home          string `json:"home,omitempty"`
	AccessControl int32  `json:"-"`
}
