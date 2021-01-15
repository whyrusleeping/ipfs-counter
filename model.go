package main

import (
	"net"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
)

// Node is a top-level record of the state of a remote peer
type Node struct {
	Observed time.Time
	ID       peer.ID               `bigquery:"peer_id"`
	Addrs    []multiaddr.Multiaddr `bigquery:"-"`
	// string interpretation of Addrs used when saving to bigquery.
	Addresses                  []string `json:"-"`
	UserAgent, ProtocolVersion string
	RTPeers                    map[peer.ID]struct{} `bigquery:"-"`
	// list of peers for bigquery
	RT  []peer.ID `bigquery:"rt" json:"-"`
	Err string
}

// Save formats node instance for bigquery
func (n *Node) Save() (map[string]bigquery.Value, string, error) {
	pl := make([]peer.ID, 0, len(n.RTPeers))
	for p := range n.RTPeers {
		pl = append(pl, p)
	}
	n.RT = pl
	textAddrs := make([]string, 0, len(n.Addrs))
	for _, a := range n.Addrs {
		textAddrs = append(textAddrs, a.String())
	}
	n.Addresses = textAddrs
	return map[string]bigquery.Value{
		"Observed":        n.Observed,
		"peer_id":         n.ID,
		"Addresses":       textAddrs,
		"UserAgent":       n.UserAgent,
		"ProtocolVersion": n.ProtocolVersion,
		"rt":              pl,
		"Err":             n.Err,
	}, "", nil
}

// Trial defines a row of input / connection attempts to a node.
type Trial struct {
	Observed time.Time
	peer.ID
	multiaddr.Multiaddr
	Address    net.IP
	Retries    uint // TODO: This can be obtained from length of results array?
	Results    []Result
	Blocked    bool
	FailSanity bool
	RTT        time.Duration
}

// TrialSchema is a bigquery schema for saving Trials
type TrialSchema struct {
	Observed     time.Time `bigquery:"observed"`
	peer.ID      `bigquery:"peer_id"`
	MultiAddress string            `bigquery:"multi_address"`
	Address      string            `bigquery:"address"`
	Retries      uint32            `bigquery:"retries"`
	Results      []Result          `bigquery:"results"`
	Blocked      bool              `bigquery:"blocked"`
	FailSanity   bool              `bigquery:"fail_sanity"`
	RTT          bigquery.NullTime `bigquery:"rtt"`
}

// Save formats a trial for bigquery insertion
func (t *Trial) Save() (map[string]bigquery.Value, string, error) {
	return map[string]bigquery.Value{
		"observed":      t.Observed,
		"peer_id":       t.ID,
		"multi_address": t.Multiaddr.String(),
		"address":       t.Address.String(),
		"retries":       t.Retries,
		"results":       t.Results,
		"blocked":       t.Blocked,
		"fail_sanity":   t.FailSanity,
		"rtt":           t.RTT,
	}, "", nil
}

// Result is a single connection to a node.
// There are many of these in a given Trial
type Result struct {
	Success   bool // Reply matches template
	Error     bigquery.NullString
	StartTime time.Time
	EndTime   bigquery.NullDateTime
}
