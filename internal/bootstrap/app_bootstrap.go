package bootstrap

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/event"
	"gameclustering.com/internal/persistence"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func Register(name string, fac func() core.QueryFactory) {
	core.QueryFactoryRegistry[name] = fac
}

func AppBootstrap(tcx TarantulaContext) {
	Register(event.MESSAGE_TOPIC_NAME, func() core.QueryFactory { return event.NewMessageEventFactory() })
	Register(event.REGISTER_TOPIC_NAME, func() core.QueryFactory { return event.NewRegisterEventFactory() })
	Register(event.LOG_TOPIC_NAME, func() core.QueryFactory { return event.NewLogEventFactory() })
	Register(event.LOGIN_TOPIC_NAME, func() core.QueryFactory { return event.NewLoginEventFactory() })
	Register(event.REQUEST_TOPIC_NAME, func() core.QueryFactory { return event.NewRequestEventFactory() })
	Register(event.TASK_TOPIC_NAME, func() core.QueryFactory { return event.NewTaskEventFactory() })
	Register(event.TRANSACTION_TOPIC_NAME, func() core.QueryFactory { return event.NewTransactionEventFactory() })
	Register(persistence.LOGIN_OBJECT_FACTORY_NAME, func() core.QueryFactory { return persistence.NewLoginObjectFactory() })
	Register(persistence.VM_OBJECT_FACTORY_NAME, func() core.QueryFactory { return persistence.NewVMObjectFactory() })
	Register(persistence.REPOSITORY_OBJECT_FACTORY_NAME, func() core.QueryFactory { return persistence.NewRepositoryObjectFactory() })

	f := core.Env{}
	err := f.Load(tcx.Config())
	if err != nil {
		panic(err)
	}
	mountDir := fmt.Sprintf("%s/%s", f.HomeDir, f.GroupName)
	err = os.MkdirAll(mountDir, 0755)
	if err != nil {
		panic(err)
	}
	f.LogDir = mountDir
	go func() {
		err := tcx.Start(f)
		if err != nil {
			panic(err)
		}
		http.Handle("/"+tcx.Context()+"/metrics", metricsHandler(tcx.Service().Authenticator(), promhttp.Handler()))
		http.Handle("/", http.HandlerFunc(badRequest))
		core.AppLog.Fatal().Err(http.ListenAndServe(f.HttpBinding, nil))

	}()
	core.AppLog.Info().Msg("Wating for signal to exit ...")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	core.AppLog.Info().Msg("Signal to exit")
	tcx.Shutdown()
	os.Remove(core.CERT_NAME)
	os.Remove(core.KEY_NAME)
	signal.Stop(sigs)
	close(sigs)
}

func badRequest(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	session := core.OnSession{Successful: false, Message: "bad request [" + r.URL.Path + "]", ErrorCode: BAD_REQUEST_CODE}
	w.Write(util.ToJson(session))
}

func invalidToken(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	session := core.OnSession{Successful: false, Message: INVALID_TOKEN_MSG, ErrorCode: INVALID_TOKEN_CODE}
	w.Write(util.ToJson(session))
}
func illegalAccess(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	session := core.OnSession{Successful: false, Message: ILLEGAL_ACCESS_MSG, ErrorCode: ILLEGAL_ACCESS_CODE}
	w.Write(util.ToJson(session))
}
func preflight(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	w.WriteHeader(http.StatusNoContent)
}

func metricsHandler(auth core.Authenticator, h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tkn := r.Header.Get("Authorization")
		parts := strings.Split(tkn, " ")
		if len(parts) != 2 {
			invalidToken(w, r)
			return
		}
		_, err := auth.ValidateTicket(parts[1])
		if err != nil {
			core.AppLog.Warn().Msgf("metrics validation failed %s\n", err.Error())
			invalidToken(w, r)
			return
		}
		h.ServeHTTP(w, r)
	}
}

func Logging(s TarantulaApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		var code int32 = 0
		defer func() {
			if s.ClusterMember() {
				return
			}
			dur := time.Since(start)
			re := protocol.RequestEvent{Path: r.URL.Path, Method: r.Method, Duration: uint64(dur.Milliseconds()), Code: uint32(code)}
			re.DateTime = timestamppb.Now()
			re.Source = r.RemoteAddr
			rf := event.RequestEventFactory{}
			t, err := rf.FromRequestEvent(&re)
			if err != nil {
				core.AppLog.Warn().Msgf("request event error %s\n", err.Error())
				return
			}
			t.NodeId = s.NodeId()
			t.Tag = s.Context()
			t.Event.Key.Array = core.ToBytes(s.Sequence())
			s.Cluster().Publish(t)
		}()
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Allow-Methods", "*")
		if r.Method == "OPTIONS" {
			preflight(w, r)
			return
		}
		if s.AccessControl() == core.PUBLIC_ACCESS_CONTROL {
			s.Request(core.OnSession{}, w, r)
			return
		}
		tkn := r.Header.Get("Authorization")
		parts := strings.Split(tkn, " ")
		if len(parts) != 2 {
			code = int32(ILLEGAL_TOKEN_CODE)
			invalidToken(w, r)
			return
		}
		session, err := s.Authenticator().ValidateToken(parts[1])
		if err != nil {
			code = int32(INVALID_TOKEN_CODE)
			invalidToken(w, r)
			return
		}
		if session.AccessControl < s.AccessControl() {

			code = int32(ILLEGAL_ACCESS_CODE)
			illegalAccess(w, r)
			return
		}

		s.Request(session, w, r)
	}
}
