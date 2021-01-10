package main

import (
	"time"

	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"

	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/control"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/urfave/cli/v2"
)

// Recorder holds the collected output of a crawl
type Recorder struct {
	records map[peer.ID]Node
	dials   map[ma.Multiaddr]*Trial
	host    host.Host
	log     logging.StandardLogger
}

// NewRecorder creates a recorder in a given context
func NewRecorder(c *cli.Context) *Recorder {
	ll := "info"
	if c.Bool("debug") {
		ll = "debug"
	}

	l := logging.Logger("crawlapp")
	logging.SetLogLevel("crawlapp", ll)

	return &Recorder{
		log:     l,
		dials:   make(map[ma.Multiaddr]*Trial),
		records: make(map[peer.ID]Node),
	}
}

// InterceptPeerDial is part of the ConnectionGater interface
func (r *Recorder) InterceptPeerDial(p peer.ID) (allow bool) {
	return true
}

// InterceptAddrDial is part of the ConnectionGater interface
func (r *Recorder) InterceptAddrDial(id peer.ID, addr ma.Multiaddr) (allow bool) {
	if _, ok := r.dials[addr]; !ok {
		ip, err := manet.ToIP(addr)
		r.dials[addr] = &Trial{
			ID:         id,
			Address:    ip,
			FailSanity: (err != nil),
		}
	}
	t := r.dials[addr]
	t.Results = append(r.dials[addr].Results, Result{
		StartTime: time.Now(),
	})
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
		ID:              p,
		Addrs:           r.host.Peerstore().Addrs(p),
		UserAgent:       ua.(string),
		ProtocolVersion: pv.(string),
		RTPeers:         rtPeerSet,
	}
	r.records[p] = n

	r.log.Debugf("%s crawl successful", p)
}

func (r *Recorder) onPeerFailure(p peer.ID, err error) {
	if _, ok := r.records[p]; ok {
		panic("should not hit this twice")
	}

	n := Node{
		ID:    p,
		Addrs: r.host.Peerstore().Addrs(p),
		Err:   err.Error(),
	}
	r.records[p] = n

	r.log.Debugf("%s crawl failed", p)
}
