package main

import (
	host "github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"sync"
)

type IpfsDHT struct {
	host host.Host
	strmap map[peer.ID]*messageSender
	smlk   sync.Mutex

	protocols []protocol.ID
}
