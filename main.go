package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	logging "github.com/ipfs/go-log"
	"os"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/libp2p/go-libp2p-kad-dht/crawler"
	"github.com/multiformats/go-multiaddr"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := flag.String("o", "rt.dot", "output file location")
	outJson := flag.String("ojson", "rt.json", "output json file location")
	flag.Parse()

	peerMap := make(map[peer.ID]map[peer.ID]struct{})
	failedPeers := make(map[peer.ID]error)

	pd := make(map[peer.ID]*PeerData)

	h, err := libp2p.New(ctx)
	if err != nil {
		panic(err)
	}

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

	c, err := crawler.New(h, crawler.WithParallelism(1000))
	if err != nil {
		panic(err)
	}

	logging.SetLogLevel("dht-crawler", "debug")
	l := logging.Logger("crawlapp")
	logging.SetLogLevel("crawlapp", "debug")

	short, c2 := context.WithTimeout(ctx, time.Hour*20)
	defer c2()
	c.Run(short, []*peer.AddrInfo{startingPeerInfo},
		func(p peer.ID, rtPeers []*peer.AddrInfo) {
			if _, ok := peerMap[p]; ok {
				panic("should not hit this twice")
			}
			rtPeerSet := make(map[peer.ID]struct{}, len(rtPeers))
			for _, ai := range rtPeers {
				rtPeerSet[ai.ID] = struct{}{}
			}
			peerMap[p] = rtPeerSet

			ua, err := h.Peerstore().Get(p, "AgentVersion")
			if err != nil {
				ua = ""
			}
			pv, err := h.Peerstore().Get(p, "ProtocolVersion")
			if err != nil {
				pv = ""
			}
			if _, ok := pd[p]; ok {
				panic("nooo")
			}
			pd[p] = &PeerData{
				Peer: p,
				Success:         true,
				Addrs:           h.Peerstore().Addrs(p),
				UserAgent:       ua.(string),
				ProtocolVersion: pv.(string),
				RTPeers:         rtPeerSet,
			}
			l.Debugf("%d out of %d successful", len(peerMap), len(pd))
		},
		func(p peer.ID, err error) {
			if _, ok := failedPeers[p]; ok {
				panic("we hit this twice?")
			}
			failedPeers[p] = err
			if _, ok := pd[p]; ok {
				panic("nooo")
			}
			pd[p] = &PeerData{
				Peer: p,
				Success: false,
				Addrs : h.Peerstore().Addrs(p),
				Err : err.Error(),
			}

			l.Debugf("%d out of %d successful", len(peerMap), len(pd))
		},
	)

	OutputData(*out, *outJson, peerMap, pd)
}

type PeerData struct {
	Peer                       peer.ID
	Success                    bool
	Addrs                      []multiaddr.Multiaddr
	UserAgent, ProtocolVersion string
	RTPeers                    map[peer.ID]struct{}
	Err                        string
}

func OutputData(filePath, jsonFile string, peerMap map[peer.ID]map[peer.ID]struct{}, pd map[peer.ID]*PeerData) {
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


	f2, err := os.Create(jsonFile)
	defer f2.Close()

	if err != nil {
		panic(err)
	}

	for p := range peerMap {
		if v, ok := pd[p]; !ok {
			panic("?")
		} else {
			if !v.Success {
				panic("hoowww")
			}
		}
	}

	for _, d := range pd {
		b, err := json.Marshal(d)
		if err != nil {
			panic(err)
		}
		f2.Write(b)
		f2.WriteString("\n")
	}
}
