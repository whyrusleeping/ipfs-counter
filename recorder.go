package main

import (
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
	"github.com/urfave/cli/v2"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
)

// Recorder holds the collected output of a crawl
type Recorder struct {
	records map[peer.ID]Node
	dials   sync.Map //map Multiaddr->Trial
	host    host.Host
	log     logging.StandardLogger

	Client      *bigquery.Client
	nodeStream  chan *Node
	trialStream chan *Trial
	done        chan bool
	wg          sync.WaitGroup
}

// NewRecorder creates a recorder in a given context
func NewRecorder(c *cli.Context) (*Recorder, error) {
	ll := "info"
	if c.Bool("debug") {
		ll = "debug"
	}

	l := logging.Logger("crawlapp")
	logging.SetLogLevel("crawlapp", ll)

	rec := &Recorder{
		log:     l,
		dials:   sync.Map{},
		records: make(map[peer.ID]Node),
	}

	if c.IsSet("dataset") || c.IsSet("table") {
		if err := rec.Connect(c.Context, c.String("dataset"), c.String("table")); err != nil {
			return nil, err
		}
		if err := rec.setupBigquery(c.Context, c.String("dataset"), c.String("table"), c.Bool("create-tables")); err != nil {
			return nil, err
		}
	}
	return rec, nil
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

func (r *Recorder) onPeerConnectednessEvent(sub event.Subscription) error {
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
	return nil
}

func (r *Recorder) onPeerSuccess(p peer.ID, rtPeers []*peer.AddrInfo) {
	if _, ok := r.records[p]; ok {
		panic("should not hit this twice")
	}
	rtPeerSet := make(map[peer.ID]struct{}, len(rtPeers))
	for _, ai := range rtPeers {
		rtPeerSet[ai.ID] = struct{}{}
	}

	ua, err := r.host.Peerstore().Get(p, "AgentVersion")
	if err != nil {
		ua = ""
	}
	pv, err := r.host.Peerstore().Get(p, "ProtocolVersion")
	if err != nil {
		pv = ""
	}

	n := Node{
		Observed:        time.Now(),
		ID:              p,
		Addrs:           r.host.Peerstore().Addrs(p),
		UserAgent:       ua.(string),
		ProtocolVersion: pv.(string),
		RTPeers:         rtPeerSet,
	}
	r.records[p] = n
	if r.nodeStream != nil {
		r.nodeStream <- &n
	}

	r.log.Debugf("%s crawl successful", p)
}

func (r *Recorder) onPeerFailure(p peer.ID, err error) {
	if _, ok := r.records[p]; ok {
		panic("should not hit this twice")
	}

	n := Node{
		Observed: time.Now(),
		ID:       p,
		Addrs:    r.host.Peerstore().Addrs(p),
		Err:      err.Error(),
	}
	r.records[p] = n
	if r.nodeStream != nil {
		r.nodeStream <- &n
	}

	r.log.Debugf("%s crawl failed", p)
}
