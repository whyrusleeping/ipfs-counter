package main

import (
	"net"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
)

// Node is a top-level record of the state of a remote peer
type Node struct {
	ID                         peer.ID
	Addrs                      []multiaddr.Multiaddr
	UserAgent, ProtocolVersion string
	RTPeers                    map[peer.ID]struct{}
	Err                        string
}

// Trial defines a row of input / connection attempts to a node.
type Trial struct {
	peer.ID
	multiaddr.Multiaddr
	Address    net.IP
	Retries    uint // TODO: This can be obtained from length of results array?
	Results    []Result
	Blocked    bool
	FailSanity bool
	RTT        time.Duration
}

// Result is a single connection to a node.
// There are many of these in a given Trial
type Result struct {
	Success   bool   // Reply matches template
	Error     string `json:"Error,omitempty"`
	StartTime time.Time
	EndTime   time.Time
}
