package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"

	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	kbucket "github.com/libp2p/go-libp2p-kbucket"
	"github.com/multiformats/go-multiaddr"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := flag.String("o", "rt.dot", "output file location")
	outJson := flag.String("ojson", "rt.json", "output json file location")
	flag.Parse()

	peerMap := make(map[peer.ID]map[peer.ID]struct{})
	allPeers := make([]*peer.AddrInfo, 0, 1000)
	allPeersSet := make(map[peer.ID]struct{})

	h, err := libp2p.New(ctx)
	if err != nil {
		panic(err)
	}

	dhtRPC := kaddht.NewProtocolMessenger(h, []protocol.ID{"/ipfs/kad/1.0.0"}, nil)

	//"/dnsaddr/sjc-2.bootstrap.libp2p.io/p2p/QmZa1sAxajnQjVM8WjWXoMbmPd7NsWhfKsPkErzpm9wGkp"
	v2Addr := "/ip4/139.178.89.189/tcp/4001/p2p/QmZa1sAxajnQjVM8WjWXoMbmPd7NsWhfKsPkErzpm9wGkp"
	v1Addr := "/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ"
	boosterAddr := "/ip4/207.148.19.196/tcp/20074/p2p/12D3KooWGXBbSZ3ko3UvoekdnnSrdmuFic3XHuNKvGcZyrH1mVxr"
	booster2Addr := "/ip4/18.185.241.99/tcp/20001/p2p/12D3KooWA4NVc1GytssyhxGqaT22kJ9XwdhCpS2VwNPPMw59Ctf4"
	hydra1 := "/ip4/64.225.116.25/tcp/30017/p2p/12D3KooWHHVPRYiXuWsVmATm8nduX7dXXpw3kC5Co1QSUYVLNXZN"

	allStarting := []string{v1Addr, v2Addr, boosterAddr, booster2Addr, hydra1, booster2Addr}
	_ = allStarting

	startingPeer, err := multiaddr.NewMultiaddr(v1Addr)
	if err != nil {
		panic(err)
	}

	startingPeerInfo, err := peer.AddrInfoFromP2pAddr(startingPeer)
	if err != nil {
		panic(err)
	}
	h.Peerstore().AddAddrs(startingPeerInfo.ID, startingPeerInfo.Addrs, time.Hour)

	queryQueue := make(chan peer.ID, 1)
	queryResQueue := make(chan *queryResult, 1)

	maxQueries := 10000
	for i := 0; i < maxQueries; i++ {
		go func() {
			for {
				select {
				case p := <-queryQueue:
					res := queryPeer(ctx, h, dhtRPC, p)
					select {
					case queryResQueue <- res:
					case <-ctx.Done():
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	allPeers = append(allPeers, startingPeerInfo)
	allPeersSet[startingPeerInfo.ID] = struct{}{}

	numPeersTotal := 1
	numPeersQueried := 0
	numOutstandingQueries := 0

	var nextPeer peer.ID
	var sendNextPeer chan peer.ID

	for numPeersQueried < numPeersTotal || numOutstandingQueries != 0 {
		if nextPeer == "" && numPeersQueried < numPeersTotal {
			nextPeer = allPeers[numPeersQueried].ID
		}

		if nextPeer == "" {
			sendNextPeer = nil
		} else {
			sendNextPeer = queryQueue
		}

		select {
		case res := <-queryResQueue:
			if res.err == nil {
				fmt.Printf("peer %v had %d peers\n", res.peer, len(res.data))
				if _, ok := peerMap[res.peer]; !ok {
					rtPeers := make(map[peer.ID]struct{}, len(res.data))
					for p := range res.data {
						rtPeers[p] = struct{}{}
					}
					peerMap[res.peer] = rtPeers
				} else {
					panic("how?")
				}

				for p, ai := range res.data {
					h.Peerstore().AddAddrs(p, ai.Addrs, time.Hour)
					if _, ok := allPeersSet[p]; !ok {
						allPeersSet[p] = struct{}{}
						allPeers = append(allPeers, ai)
						numPeersTotal++
					}
				}
			}
			numOutstandingQueries--
		case sendNextPeer <- nextPeer:
			numOutstandingQueries++
			numPeersQueried++
			nextPeer = ""
			fmt.Printf("starting %d out of %d\n", numPeersQueried, len(allPeers))
		case <-ctx.Done():
			return
		}
	}

	OutputData(*out, *outJson, peerMap)
}

func OutputData(filePath, jsonFile string, peerMap map[peer.ID]map[peer.ID]struct{}) {
	f, err := os.Create(filePath)
	defer f.Close()

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

type queryResult struct {
	peer peer.ID
	data map[peer.ID]*peer.AddrInfo
	err  error
}

func queryPeer(ctx context.Context, host host.Host, dhtRPC *kaddht.ProtocolMessenger, nextPeer peer.ID) *queryResult {
	tmpRT, err := kbucket.NewRoutingTable(20, kbucket.ConvertPeerID(nextPeer), time.Hour, host.Peerstore(), time.Hour)
	if err != nil {
		fmt.Printf("error creating rt for peer %v : %v\n", nextPeer, err)
		return &queryResult{nextPeer, nil, err}
	}

	localPeers := make(map[peer.ID]*peer.AddrInfo)
	for i := 0; i < 12; i++ {
		generatePeer, err := tmpRT.GenRandPeerID(uint(i))
		if err != nil {
			panic(err)
		}
		peers, err := dhtRPC.GetClosestPeers(ctx, nextPeer, generatePeer)
		if err != nil {
			fmt.Printf("error finding data on peer %v with cpl %d : %v\n", nextPeer, i, err)
			break
		}
		for _, ai := range peers {
			if _, ok := localPeers[ai.ID]; !ok {
				localPeers[ai.ID] = ai
			}
		}
	}
	return &queryResult{nextPeer, localPeers, nil}
}
