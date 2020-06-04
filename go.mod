module github.com/aschmahmann/dht-graph

go 1.14

require (
	github.com/ipfs/go-log v1.0.4
	github.com/libp2p/go-libp2p v0.8.2
	github.com/libp2p/go-libp2p-core v0.5.4
	github.com/libp2p/go-libp2p-kad-dht v0.8.2-0.20200601201404-b918f4221db7
	github.com/multiformats/go-multiaddr v0.2.1
	go.uber.org/zap v1.14.1
)

replace github.com/libp2p/go-libp2p-kad-dht => ../../libp2p/go-libp2p-kad-dht
