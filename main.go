package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"

	kbucket "github.com/libp2p/go-libp2p-kbucket"
	"github.com/multiformats/go-multiaddr"
)

func main(){
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := flag.String("o", "rt.dot", "output file location")
	flag.Parse()

	peerMap := make(map[peer.ID]map[peer.ID]struct{})
	allPeers := make([]*peer.AddrInfo, 0, 1000)

	h, err := libp2p.New(ctx)
	if err != nil {
		panic(err)
	}

	dhtmock := &IpfsDHT{
		host:      h,
		strmap:     make(map[peer.ID]*messageSender),
		protocols: []protocol.ID{"/ipfs/kad/1.0.0"},
	}

	//"/dnsaddr/sjc-2.bootstrap.libp2p.io/p2p/QmZa1sAxajnQjVM8WjWXoMbmPd7NsWhfKsPkErzpm9wGkp"
	v2Addr := "/ip4/139.178.89.189/tcp/4001/p2p/QmZa1sAxajnQjVM8WjWXoMbmPd7NsWhfKsPkErzpm9wGkp"
	//v1Addr := "/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ"
	startingPeer, err := multiaddr.NewMultiaddr(v2Addr)
	if err != nil {
		panic(err)
	}

	startingPeerInfo, err := peer.AddrInfoFromP2pAddr(startingPeer)
	if err != nil {
		panic(err)
	}
	dhtmock.host.Peerstore().AddAddrs(startingPeerInfo.ID, startingPeerInfo.Addrs, time.Hour)

	allPeers = append(allPeers, startingPeerInfo)
	lastQuery := 0

	for lastQuery < len(allPeers) {
		lastQuery++
		nextPeerAddr := allPeers[lastQuery - 1]
		nextPeer := nextPeerAddr.ID

		tmpRT, err := kbucket.NewRoutingTable(20, kbucket.ConvertPeerID(nextPeer), time.Hour, h.Peerstore(), time.Hour)
		if err != nil {
			fmt.Printf("error creating rt for peer %v : %v", nextPeer, err)
			continue
		}

		localPeers := make(map[peer.ID]struct{})

		for i := 0; i < 12; i++ {
			generatePeer, err := tmpRT.GenRandPeerID(uint(i))
			if err != nil {
				panic(err)
			}
			peers, err := dhtmock.findPeerSingle(ctx, nextPeer, generatePeer)
			if err != nil {
				fmt.Printf("error finding data on peer %v with cpl %d : %v", nextPeer, i, err)
				break
			}
			for _, ai := range peers {
				if _, ok := localPeers[ai.ID]; !ok {
					localPeers[ai.ID] = struct{}{}
					if _, ok := peerMap[ai.ID]; !ok {
						allPeers = append(allPeers, ai)
						dhtmock.host.Peerstore().AddAddrs(ai.ID, ai.Addrs, time.Hour)
					}
				}
			}
		}
		peerMap[nextPeer] = localPeers
		fmt.Printf("peer %v had %d peers", nextPeer, len(localPeers))
	}

	f, err := os.Create(*out)
	if err != nil {
		panic(err)
	}
	f.WriteString("digraph D { \n")
	for p, rtPeers := range peerMap {
		for rtp := range rtPeers {
			f.WriteString(fmt.Sprintf("Z%v -> Z%v;\n", p, rtp))
		}
	}
	f.WriteString("\n}")
}
