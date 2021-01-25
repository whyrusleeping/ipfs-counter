package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"

	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/control"
	"github.com/libp2p/go-libp2p-core/event"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
	"github.com/urfave/cli/v2"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
)

// Recorder holds the collected output of a crawl
type Recorder struct {
	records map[peer.ID]*Node
	dials   sync.Map //map Multiaddr->Trial
	host    host.Host
	pinger  *ping.PingService
	log     logging.StandardLogger
	ctx     context.Context
	cancel  context.CancelFunc

	Client      *bigquery.Client
	nodeStream  chan *Node
	trialStream chan []*Trial
	wg          sync.WaitGroup
}

// NewRecorder creates a recorder in a given context
func NewRecorder(c *cli.Context) (*Recorder, error) {
	ll := "info"
	if c.Bool("debug") {
		ll = "debug"
	}

	l := logging.Logger("crawlapp")
	if err := logging.SetLogLevel("crawlapp", ll); err != nil {
		return nil, err
	}

	rec := &Recorder{
		log:     l,
		dials:   sync.Map{},
		records: make(map[peer.ID]*Node),
	}
	rec.ctx, rec.cancel = context.WithCancel(c.Context)

	if c.IsSet("dataset") || c.IsSet("table") {
		if err := rec.Connect(rec.ctx, c.String("dataset"), c.String("table")); err != nil {
			return nil, err
		}
		if err := rec.setupBigquery(rec.ctx, c.String("dataset"), c.String("table"), c.Bool("create-tables")); err != nil {
			return nil, err
		}
	}
	return rec, nil
}

func (r *Recorder) setHost(h host.Host) error {
	r.host = h
	r.pinger = ping.NewPingService(h)

	h.Network().Notify(r)

	sub, err := h.EventBus().Subscribe(new(event.EvtPeerConnectednessChanged))
	if err != nil {
		return err
	}
	go r.onPeerConnectednessEvent(sub)

	return nil
}

// InterceptPeerDial is part of the ConnectionGater interface
func (r *Recorder) InterceptPeerDial(p peer.ID) (allow bool) {
	return true
}

// InterceptAddrDial is part of the ConnectionGater interface
func (r *Recorder) InterceptAddrDial(id peer.ID, addr ma.Multiaddr) (allow bool) {
	val, ok := r.dials.Load(addr)
	if !ok {
		ip, err := manet.ToIP(addr)
		val = &Trial{
			Observed:   time.Now(),
			ID:         id,
			Multiaddr:  addr,
			Address:    ip,
			FailSanity: (err != nil),
		}
	}
	t := val.(*Trial)
	t.Results = append(t.Results, Result{
		StartTime: time.Now(),
	})
	r.dials.Store(addr, t)
	return true
}

// InterceptAccept is part of the ConnectionGater interface
func (r *Recorder) InterceptAccept(_ network.ConnMultiaddrs) (allow bool) {
	return true
}

// InterceptSecured is part of the ConnectionGater interface
func (r *Recorder) InterceptSecured(network.Direction, peer.ID, network.ConnMultiaddrs) (allow bool) {
	return true
}

// InterceptUpgraded is part of the ConnectionGater interface
func (r *Recorder) InterceptUpgraded(network.Conn) (allow bool, reason control.DisconnectReason) {
	return true, 0
}

// Listen is part of the network.Notifee interface
func (r *Recorder) Listen(network.Network, ma.Multiaddr) {

}

// ListenClose is part of the network.Notifee interface
func (r *Recorder) ListenClose(network.Network, ma.Multiaddr) {

}

// Connected is part of the network.Notifee interface
func (r *Recorder) Connected(_ network.Network, c network.Conn) {
	addr := c.RemoteMultiaddr()
	if t, ok := r.dials.Load(addr); ok {
		trials := t.(*Trial)
		if !trials.Results[len(trials.Results)-1].EndTime.Valid {
			trials.Results[len(trials.Results)-1].EndTime.DateTime = civil.DateTimeOf(time.Now())
			trials.Results[len(trials.Results)-1].EndTime.Valid = true
		}
	}

	pr := r.pinger.Ping(r.ctx, c.RemotePeer())
	startTime := time.Now()
	go func() {
		res, ok := <-pr
		if !ok {
			// failed.
			return
		}
		if t, ok := r.dials.Load(addr); ok {
			trials := t.(*Trial)
			trials.Results = append(trials.Results, Result{
				Success:   true,
				Error:     bigquery.NullString{Valid: (res.Error != nil), StringVal: fmt.Sprintf("%v", res.Error)},
				StartTime: startTime,
				EndTime:   bigquery.NullDateTime{Valid: true, DateTime: civil.DateTimeOf(time.Now())},
			})
		}
	}()
}

// Disconnected is part of the network.Notifee interface
func (r *Recorder) Disconnected(_ network.Network, c network.Conn) {
	addr := c.RemoteMultiaddr()
	durr := r.host.Peerstore().LatencyEWMA(c.RemotePeer())
	if t, ok := r.dials.Load(addr); ok {
		trial := t.(*Trial)
		trial.RTT = durr
	}
}

// OpenedStream is part of the network.Notifee interface
func (r *Recorder) OpenedStream(network.Network, network.Stream) {

}

// ClosedStream is part of the network.Notifee interface
func (r *Recorder) ClosedStream(network.Network, network.Stream) {

}

func (r *Recorder) onPeerConnectednessEvent(sub event.Subscription) {
	for e := range sub.Out() {
		ev := e.(event.EvtPeerConnectednessChanged)
		if ev.Connectedness == network.Connected {
			// figure out how we're connected
			c2p := r.host.Network().ConnsToPeer(ev.Peer)
			for _, conn := range c2p {
				addr := conn.RemoteMultiaddr()
				// see if a pending dial for the peer
				if t, ok := r.dials.Load(addr); ok {
					trials := t.(*Trial)
					if !trials.Results[len(trials.Results)-1].EndTime.Valid {
						trials.Results[len(trials.Results)-1].EndTime.DateTime = civil.DateTimeOf(time.Now())
						trials.Results[len(trials.Results)-1].EndTime.Valid = true
					}
				}
			}
		}
	}
}

func mkNode(p peer.ID, ps peerstore.Peerstore) *Node {
	ua, err := ps.Get(p, "AgentVersion")
	if err != nil {
		ua = ""
	}
	pv, err := ps.Get(p, "ProtocolVersion")
	if err != nil {
		pv = ""
	}

	protos, err := ps.GetProtocols(p)
	if err != nil {
		protos = []string{}
	}

	return &Node{
		Observed:        time.Now(),
		ID:              p,
		Addrs:           ps.Addrs(p),
		Protocols:       protos,
		UserAgent:       ua.(string),
		ProtocolVersion: pv.(string),
	}
}

func (r *Recorder) onPeerSuccess(p peer.ID, rtPeers []*peer.AddrInfo) {
	if _, ok := r.records[p]; ok {
		panic("should not hit this twice")
	}
	rtPeerSet := make(map[peer.ID]struct{}, len(rtPeers))
	for _, ai := range rtPeers {
		rtPeerSet[ai.ID] = struct{}{}
	}

	n := mkNode(p, r.host.Peerstore())
	n.RTPeers = rtPeerSet
	r.records[p] = n
	if r.nodeStream != nil {
		r.nodeStream <- n
	}

	r.log.Debugf("%s crawl successful", p)
}

func (r *Recorder) onPeerFailure(p peer.ID, err error) {
	if _, ok := r.records[p]; ok {
		panic("should not hit this twice")
	}

	n := mkNode(p, r.host.Peerstore())
	n.Err = err.Error()
	r.records[p] = n
	if r.nodeStream != nil {
		r.nodeStream <- n
	}

	r.log.Debugf("%s crawl failed", p)
}
