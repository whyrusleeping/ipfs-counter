package main

import (
	"context"
	"time"

	logging "github.com/ipfs/go-log"
	"github.com/urfave/cli/v2"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/libp2p/go-libp2p-kad-dht/crawler"
	"github.com/multiformats/go-multiaddr"
)

var crawlFlags = []cli.Flag{
	&cli.StringFlag{
		Name:        "output",
		TakesFile:   true,
		Required:    true,
		Usage:       "Output file location",
		DefaultText: "crawl-output.json",
	},
	&cli.StringFlag{
		Name:      "seed",
		TakesFile: true,
		Usage:     "Output from a previous execution",
	},
	&cli.IntFlag{
		Name:        "parallelism",
		Usage:       "How many connections to open at once",
		DefaultText: "1000",
	},
	&cli.BoolFlag{
		Name:  "debug",
		Usage: "Print debugging messages",
	},
}

func must(m multiaddr.Multiaddr, e error) multiaddr.Multiaddr {
	if e != nil {
		panic(e)
	}
	return m
}

var bootstrapAddrs = []multiaddr.Multiaddr{
	must(multiaddr.NewMultiaddr("/ip4/139.178.89.189/tcp/4001/p2p/QmZa1sAxajnQjVM8WjWXoMbmPd7NsWhfKsPkErzpm9wGkp")),
	must(multiaddr.NewMultiaddr("/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ")),
	must(multiaddr.NewMultiaddr("/ip4/207.148.19.196/tcp/20074/p2p/12D3KooWGXBbSZ3ko3UvoekdnnSrdmuFic3XHuNKvGcZyrH1mVxr")),
	must(multiaddr.NewMultiaddr("/ip4/18.185.241.99/tcp/20001/p2p/12D3KooWA4NVc1GytssyhxGqaT22kJ9XwdhCpS2VwNPPMw59Ctf4")),
	must(multiaddr.NewMultiaddr("/ip4/64.225.116.25/tcp/30017/p2p/12D3KooWHHVPRYiXuWsVmATm8nduX7dXXpw3kC5Co1QSUYVLNXZN")),
}

func makeHost(c *cli.Context) (host.Host, *Recorder, error) {
	r := NewRecorder(c)

	h, err := libp2p.New(c.Context, libp2p.ConnectionGater(r))
	r.host = h
	if err != nil {
		return nil, nil, err
	}
	return h, r, nil
}

func crawl(c *cli.Context) error {
	ll := "info"
	if c.Bool("debug") {
		ll = "debug"
	}
	logger := logging.Logger("dht-crawler")
	logging.SetLogLevel("dht-crawler", ll)

	ctx := c.Context

	pending := make(map[peer.ID]Node, 0)

	// TODO: Load previous nodes from seed.

	for _, ma := range bootstrapAddrs {
		pi, err := peer.AddrInfoFromP2pAddr(ma)
		if err != nil {
			logger.Warnf("Unable to parse address %s: %w", ma, err)
			continue
		}
		if _, ok := pending[pi.ID]; !ok {
			pending[pi.ID] = Node{
				ID:    pi.ID,
				Addrs: []multiaddr.Multiaddr{ma},
			}
		}
	}

	host, r, err := makeHost(c)
	if err != nil {
		return err
	}

	// populate host info
	peers := make([]*peer.AddrInfo, len(pending))
	for _, p := range pending {
		pis, err := peer.AddrInfosFromP2pAddrs(p.Addrs...)
		if err != nil {
			logger.Warnf("Failed to parse addresses for %s: %w", p.ID, err)
			continue
		}
		for _, pi := range pis {
			peers = append(peers, &pi)
		}
		host.Peerstore().AddAddrs(p.ID, p.Addrs, time.Hour)
	}

	crawl, err := crawler.New(host, crawler.WithParallelism(c.Int("parallelism")))
	if err != nil {
		panic(err)
	}

	// TODO: configure timeout.
	short, c2 := context.WithTimeout(ctx, time.Hour*20)
	defer c2()
	crawl.Run(short, peers,
		r.onPeerSuccess,
		r.onPeerFailure)

	Output(c.String("output"), r)
	return nil
}
