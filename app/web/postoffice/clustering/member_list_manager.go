package clustering

import (
	"fmt"
	"sync"
	"time"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/util"
	"github.com/hashicorp/memberlist"
)

const (
	NODE_EVENT_BUFFER_SIZE int = 16
	NODE_WEIGHT            int = 7 //virtual nodes per ip node
	REPLICA_MAX            int = 7
	RETRY_MAX              int = 3
)

type MemberlistManager struct {
	Seed []string
	MemberListListener

	StoreDir string
	Ctx      string
	Binding  string
}

func (m *MemberlistManager) Start(meta []byte, auth core.Authenticator, seq core.Sequence, vt *util.VaultClient) error {
	m.MemberHashRing = &MemberHashRing{weight: NODE_WEIGHT, hLock: &sync.Mutex{}, auth: auth}
	m.nodes = make([]core.Node, 0)
	cfg := memberlist.DefaultLANConfig()
	cfg.Name = m.Binding
	ch := make(chan memberlist.NodeEvent, NODE_EVENT_BUFFER_SIZE) //HAVE TO BUFFER
	cl := memberlist.ChannelEventDelegate{Ch: ch}
	m.MEvent = ch
	m.MMerge = make(chan []core.Node, NODE_EVENT_BUFFER_SIZE)
	m.MAlive = make(chan core.Node, NODE_EVENT_BUFFER_SIZE)
	m.MPing = make(chan core.Node, NODE_EVENT_BUFFER_SIZE)
	m.MConflict = make(chan []core.Node, NODE_EVENT_BUFFER_SIZE)
	m.MRequest = make(chan core.RingRequest, NODE_EVENT_BUFFER_SIZE)
	rwNode := make(chan RingUpdate, NODE_EVENT_BUFFER_SIZE)
	rwSync := make(chan []byte, NODE_EVENT_BUFFER_SIZE)
	m.WNode = rwNode
	m.MSync = rwSync
	cfg.Events = &cl
	cfg.Delegate = m
	cfg.Ping = m
	cfg.Merge = m
	cfg.Alive = m
	cfg.Conflict = m
	cfg.LogOutput = core.AppLog
	list, err := memberlist.Create(cfg)
	if err != nil {
		panic(err)
	}
	m.meta = meta
	m.Memberlist = list

	localIP := m.LocalNode().Addr.String()
	ak, err := vt.Load(m.Ctx, "auth")
	if err != nil {
		return fmt.Errorf("load auth for tls: %w", err)
	}
	tlsCert, err := util.CASignedTLS([]byte(ak.Cert), []byte(ak.Key), []string{localIP}, 365*24*time.Hour)
	if err != nil {
		return fmt.Errorf("generate tls cert: %w", err)
	}
	m.MemberHashRing.caCert = []byte(ak.Cert)
	// Start Listen after caCert is set: memberlist.Create fires NodeJoin for the local node
	// into the buffered channel; starting Listen before caCert is set means OnAdd runs with
	// nil CACert, producing an RpcConnPool that fails TLS handshakes.
	go m.Listen()
	m.DataServiceProvider = &DataServiceProvider{RNode: rwNode, RSync: rwSync, seq: seq, vault: vt, auth: auth}
	m.DataServiceProvider.TLSCert = tlsCert
	m.DataServiceProvider.CACert = []byte(ak.Cert)
	m.DataServiceProvider.rpcEndpoint = fmt.Sprintf("%s:%d", localIP, core.RPC_PORT)
	m.Mll = &m.MemberListListener
	m.Mll.DWait.Add(1)
	go m.DataServiceProvider.Start(m.StoreDir, m.Ctx)
	m.Mll.DWait.Wait()
	time.Sleep(3 * time.Second)
	go m.RingUpdated()
	joined, err := list.Join(m.Seed)
	list.UpdateNode(time.Second * 5)
	if err != nil {
		panic(err)
	}
	core.AppLog.Info().Msgf("total nodes have joined %d on local node  %s", joined, m.DataServiceProvider.rpcEndpoint)
	return nil
}

func (m *MemberlistManager) ShutdownHook() {
	core.AppLog.Info().Msg("running shut down hook ...")
	m.running = false
	m.Leave(3 * time.Second)
	time.Sleep(3 * time.Second)
	m.Shutdown()
	core.AppLog.Info().Msg("closing resouces ...")
	m.MRequest <- core.RingRequest{Opt: CLOSE_RING_OPT}
	m.WNode <- RingUpdate{State: NODE_STATE_SHUTDOWN}
	time.Sleep(3 * time.Second)
	close(m.MEvent)
	close(m.MAlive)
	close(m.MPing)
	close(m.MMerge)
	close(m.MConflict)
	close(m.MRequest)
	close(m.WNode)
	close(m.MSync)
	core.AppLog.Info().Msg("shut down has done successfully.")
}
