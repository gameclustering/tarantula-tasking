package bootstrap

import (
	"net/http"

	"gameclustering.com/internal/core"
	"github.com/rs/zerolog"
)

const (
	STANDALONE_APP     int    = 200000
	STANDALONE_APP_MSG string = "publish skipped"
	DB_OP_ERR_CODE     int    = 500100

	WRONG_PASS_CODE int    = 400100
	WRONG_PASS_MSG  string = "wrong user/password"

	BAD_REQUEST_CODE    int = 400100
	ILLEGAL_TOKEN_CODE  int = 400101
	INVALID_TOKEN_CODE  int = 400102
	ILLEGAL_ACCESS_CODE int = 400103
	INVALID_TICKET_CODE int = 400104
	INVALID_JSON_CODE   int = 400105

	INVALID_TOKEN_MSG  string = "invalid token"
	ILLEGAL_ACCESS_MSG string = "illegal access"
	ILLEGAL_TOKEN_MSG  string = "bad token"
	BAD_REQUEST_MSG    string = "bad request"
)

type LogForwarder interface {
	Forward(level zerolog.Level, log []byte)
}

type TarantulaContext interface {
	Config() string
	Start(f core.Env) error
	Shutdown()
	Context() string
	Service() TarantulaService
}

type TarantulaService interface {
	
	Authenticator() core.Authenticator
	Sequence() core.Sequence
	Cluster() core.ClusterService
	RegisterLogForwarder(threshold zerolog.Level, logf LogForwarder)
}

type TarantulaApp interface {
	TarantulaService
	AccessControl() int32
	NodeId() string
	Context() string
	ClusterMember() bool
	Request(sesion core.OnSession, w http.ResponseWriter, r *http.Request)
}
