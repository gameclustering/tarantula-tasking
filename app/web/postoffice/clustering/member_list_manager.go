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
	NODE_EVENT_BUFFER_SIZE int = 256
	NODE_WEIGHT            int = 7 //virtual nodes per ip node
	REPLICA_MAX            int = 7
	RETRY_MAX              int = 3
)

type MemberlistManager struct {
	Seed []string
	MemberListListener
	dsp           *DataServiceProvider
	StoreDir      string
	Ctx           string
	Binding       string
	AdvertiseAddr string // Vultr private IP advertised to cluster peers for inter-node RPC
	LocalHost     string // Docker bridge IP (e.g. 172.17.0.1) added as TLS SAN for admin/cloud connectivity
}

func (m *MemberlistManager) Start(meta []byte, auth core.Authenticator, seq core.Sequence, vt *util.VaultClient) error {
	memberHashRing := MemberHashRing{weight: NODE_WEIGHT, hLock: &sync.Mutex{}, auth: auth}
	memberHashRing.nodes = make([]core.Node, 0)
	cfg := memberlist.DefaultWANConfig()
	cfg.Name = m.Binding
	if m.AdvertiseAddr != "" {
		cfg.AdvertiseAddr = m.AdvertiseAddr
	}
	ch := make(chan memberlist.NodeEvent, NODE_EVENT_BUFFER_SIZE) //HAVE TO BUFFER
	cl := memberlist.ChannelEventDelegate{Ch: ch}
	m.mEvent = ch
	m.mMerge = make(chan []core.Node, NODE_EVENT_BUFFER_SIZE)
	m.mAlive = make(chan core.Node, NODE_EVENT_BUFFER_SIZE)
	m.mPing = make(chan core.Node, NODE_EVENT_BUFFER_SIZE)
	m.mConflict = make(chan []core.Node, NODE_EVENT_BUFFER_SIZE)

	cfg.Events = &cl
	cfg.Delegate = m
	cfg.Ping = m
	cfg.Merge = m
	cfg.Alive = m
	cfg.Conflict = m
	cfg.LogOutput = core.AppLog
	list, err := memberlist.Create(cfg)
	if err != nil {
		return err
	}
	m.meta = meta
	m.Memberlist = list

	localIP := m.LocalNode().Addr.String()
	// advertiseIP is the IP peers use to reach this node's RPC server across the cluster
	advertiseIP := localIP
	if m.AdvertiseAddr != "" {
		advertiseIP = m.AdvertiseAddr
	}
	// cert SANs: advertiseIP for inter-node RPC; 127.0.0.1 so co-located admin/cloud
	// containers connecting via POST_OFFICE_HOST=127.0.0.1 pass TLS verification;
	// LocalHost for Docker bridge scenarios.
	sans := []string{advertiseIP, "127.0.0.1"}
	if m.LocalHost != "" && m.LocalHost != advertiseIP && m.LocalHost != "127.0.0.1" {
		sans = append(sans, m.LocalHost)
	}
	ak, err := vt.Load(m.Ctx, "auth")
	if err != nil {
		return fmt.Errorf("load auth for tls: %w", err)
	}
	tlsCert, err := util.CASignedTLS([]byte(ak.Cert), []byte(ak.Key), sans, 365*24*time.Hour)
	if err != nil {
		return fmt.Errorf("generate tls cert: %w", err)
	}
	memberHashRing.caCert = []byte(ak.Cert)
	// Start Listen after caCert is set: memberlist.Create fires NodeJoin for the local node
	// into the buffered channel; starting Listen before caCert is set means OnAdd runs with
	// nil CACert, producing an RpcConnPool that fails TLS handshakes.
	DSP := DataServiceProvider{seq: seq, vault: vt, auth: auth}
	DSP.MemberHashRing = &memberHashRing
	DSP.TLSCert = tlsCert
	DSP.CACert = []byte(ak.Cert)
	DSP.rpcEndpoint = fmt.Sprintf("%s:%d", advertiseIP, core.RPC_PORT)
	DSP.Memberlist = list
	m.dsp = &DSP //assign for shuwdown hook run
	mll := MemberHashRingListener{&DSP}

	m.memberListChangeListener = &mll
	DSP.DWait.Add(1)
	go DSP.Start(m.StoreDir, m.Ctx)
	DSP.DWait.Wait()
	go m.Listen()
	time.Sleep(3 * time.Second)
	go mll.RingUpdated()
	joined, err := list.Join(m.Seed)
	list.UpdateNode(time.Second * 5)
	if err != nil {
		return err
	}
	core.AppLog.Info().Msgf("total nodes have joined %d on local node  %s", joined, DSP.rpcEndpoint)
	//go DSP.recoverTasks() might be good to be called on request intead of auto-recover
	return nil
}

func (m *MemberlistManager) ShutdownHook() {
	core.AppLog.Info().Msg("running shut down hook ...")
	m.Leave(3 * time.Second)
	time.Sleep(3 * time.Second)
	m.Shutdown()
	core.AppLog.Info().Msg("closing resouces ...")

	time.Sleep(3 * time.Second)
	close(m.mEvent)
	close(m.mAlive)
	close(m.mPing)
	close(m.mMerge)
	close(m.mConflict)
	m.dsp.shuwdownHook()
	core.AppLog.Info().Msg("shut down has done successfully.")
}

//member list delegate hooks

// delegate
func (m *MemberListListener) NodeMeta(limit int) []byte {
	//limit 512
	return m.meta
}

func (m *MemberListListener) NotifyMsg(msg []byte) {
	//callback from send
}

func (m *MemberListListener) GetBroadcasts(overhead, limit int) [][]byte {
	//overhead 3 limit 1350
	return nil
}

func (m *MemberListListener) LocalState(join bool) []byte {
	return nil
}
func (m *MemberListListener) MergeRemoteState(buf []byte, join bool) {

}

// ping delegate
func (m *MemberListListener) AckPayload() []byte {
	return nil
}

func (m *MemberListListener) NotifyPingComplete(other *memberlist.Node, rtt time.Duration, payload []byte) {
	m.mPing <- m.toNode(other)
}

// merge delegate
func (m *MemberListListener) NotifyMerge(peers []*memberlist.Node) error {
	nodes := make([]core.Node, 0, len(peers))
	for _, n := range peers {
		nodes = append(nodes, m.toNode(n))
	}
	m.mMerge <- nodes
	return nil
}

// alive delegate
func (m *MemberListListener) NotifyAlive(peer *memberlist.Node) error {
	m.mAlive <- m.toNode(peer)
	return nil
}

// conflict delegate
func (m *MemberListListener) NotifyConflict(existing, other *memberlist.Node) {
	m.mConflict <- []core.Node{m.toNode(existing), m.toNode(other)}
}
