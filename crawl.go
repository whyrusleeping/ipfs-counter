package main

import (
	"context"
	"time"

	logging "github.com/ipfs/go-log"
	"github.com/urfave/cli/v2"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	noise "github.com/libp2p/go-libp2p-noise"
	quic "github.com/libp2p/go-libp2p-quic-transport"
	secio "github.com/libp2p/go-libp2p-secio"
	tls "github.com/libp2p/go-libp2p-tls"

	"github.com/libp2p/go-libp2p-kad-dht/crawler"
	"github.com/multiformats/go-multiaddr"
)

var crawlFlags = []cli.Flag{
	&cli.StringFlag{
		Name:      "output",
		TakesFile: true,
		Usage:     "Output file location",
		Value:     "crawl-output",
	},
	&cli.StringFlag{
		Name:  "dataset",
		Usage: "Google biquery dataset ID for insertion",
	},
	&cli.StringFlag{
		Name:  "table",
		Usage: "Google bigquery table prefix for insertion",
	},
	&cli.BoolFlag{
		Name:  "create-tables",
		Usage: "To create bigquery tables if they do not exist",
	},
	&cli.StringFlag{
		Name:      "seed",
		TakesFile: true,
		Usage:     "Output from a previous execution",
	},
	&cli.IntFlag{
		Name:  "parallelism",
		Usage: "How many connections to open at once",
		Value: 1000,
	},
	&cli.DurationFlag{
		Name:  "crawltime",
		Usage: "How long to crawl for",
		Value: 20 * time.Hour,
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

	must(multiaddr.NewMultiaddr("/ip4/104.131.131.82/tcp/4001/ipfs/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ")),  // mars.i.ipfs.io
	must(multiaddr.NewMultiaddr("/ip4/104.236.179.241/tcp/4001/ipfs/QmSoLPppuBtQSGwKDZT2M73ULpjvfd3aZ6ha4oFGL1KrGM")), // pluto.i.ipfs.io
	must(multiaddr.NewMultiaddr("/ip4/128.199.219.111/tcp/4001/ipfs/QmSoLSafTMBsPKadTEgaXctDQVcqN88CNLHXMkTNwMKPnu")), // saturn.i.ipfs.io
	must(multiaddr.NewMultiaddr("/ip4/104.236.76.40/tcp/4001/ipfs/QmSoLV4Bbm51jM9C4gDYZQ9Cy3U6aXMJDAbzgu2fzaDs64")),   // venus.i.ipfs.io
	must(multiaddr.NewMultiaddr("/ip4/178.62.158.247/tcp/4001/ipfs/QmSoLer265NRgSp2LA3dPaeykiS1J6DifTC88f5uVQKNAd")),  // earth.i.ipfs.io
	must(multiaddr.NewMultiaddr("/ip4/104.236.151.122/tcp/4001/ipfs/QmSoLju6m7xTh3DuokvT3886QRYqxAzb1kShaanJgW36yx")),
	must(multiaddr.NewMultiaddr("/ip4/188.40.114.11/tcp/4001/ipfs/QmZY7MtK8ZbG1suwrxc7xEYZ2hQLf1dAWPRHhjxC8rjq8E")),
	must(multiaddr.NewMultiaddr("/ip4/5.9.59.34/tcp/4001/ipfs/QmRv1GNseNP1krEwHDjaQMeQVJy41879QcDwpJVhY8SWve")),
}

func makeHost(c *cli.Context, r *Recorder) (host.Host, error) {
	crypto.MinRsaKeyBits = 512

	h, err := libp2p.New(c.Context,
		libp2p.ConnectionGater(r),
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/4001"),
		libp2p.Transport(quic.NewTransport),
		libp2p.DefaultTransports,
		//		libp2p.Transport(tcp.NewTCPTransport),
		//		libp2p.Transport(ws.New),
		libp2p.Security(tls.ID, tls.New),
		libp2p.Security(noise.ID, noise.New),
		libp2p.Security(secio.ID, secio.New),
	)
	if err != nil {
		return nil, err
	}
	if err := r.setHost(h); err != nil {
		return nil, err
	}

	return h, nil
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

	r, err := NewRecorder(c)
	if err != nil {
		return err
	}

	host, err := makeHost(c, r)
	if err != nil {
		return err
	}

	// populate host info
	peers := make([]*peer.AddrInfo, 0, len(pending))
	for _, p := range pending {
		pis, err := peer.AddrInfosFromP2pAddrs(p.Addrs...)
		if err != nil {
			logger.Warnf("Failed to parse addresses for %s: %w", p.ID, err)
			continue
		}
		for _, pi := range pis {
			peers = append(peers, &pi)
		}

		nonIDAddrs := make([]multiaddr.Multiaddr, 0, len(p.Addrs))
		// Remove the /p2p/<id> portion of the addresses.
		for _, a := range p.Addrs {
			na, _ := multiaddr.SplitFunc(a, func(c multiaddr.Component) bool {
				return c.Protocol().Code == multiaddr.P_P2P
			})
			nonIDAddrs = append(nonIDAddrs, na)
		}
		host.Peerstore().AddAddrs(p.ID, nonIDAddrs, time.Hour)
	}

	crawl, err := crawler.New(host, crawler.WithParallelism(c.Int("parallelism")))
	if err != nil {
		panic(err)
	}

	// TODO: configure timeout.
	short, c2 := context.WithTimeout(ctx, c.Duration("crawltime"))
	defer c2()
	crawl.Run(short, peers,
		r.onPeerSuccess,
		r.onPeerFailure)

	logger.Info("Crawl complete. Collecting Output...")
	Output(c.String("output"), r)
	return nil
}
