package main

import (
	"context"
	"fmt"
	"github.com/libp2p/go-libp2p-core/peer"
	record_pb "github.com/libp2p/go-libp2p-record/pb"

	"github.com/multiformats/go-multihash"

	"github.com/libp2p/go-libp2p-core/routing"
	pb "github.com/libp2p/go-libp2p-kad-dht/pb"
)

// getValueOrPeers queries a particular peer p for the value for
// key. It returns either the value or a list of closer peers.
// NOTE: It will update the dht's peerstore with any new addresses
// it finds for the given peer.
func (dht *IpfsDHT) getValueOrPeers(ctx context.Context, p peer.ID, key string) (*record_pb.Record, []*peer.AddrInfo, error) {
	pmes, err := dht.getValueSingle(ctx, p, key)
	if err != nil {
		return nil, nil, err
	}

	// Perhaps we were given closer peers
	peers := pb.PBPeersToPeerInfos(pmes.GetCloserPeers())

	if record := pmes.GetRecord(); record != nil {
		return record, peers, err
	}

	if len(peers) > 0 {
		return nil, peers, nil
	}

	return nil, nil, routing.ErrNotFound
}

// getValueSingle simply performs the get value RPC with the given parameters
func (dht *IpfsDHT) getValueSingle(ctx context.Context, p peer.ID, key string) (*pb.Message, error) {
	pmes := pb.NewMessage(pb.Message_GET_VALUE, []byte(key), 0)
	return dht.sendRequest(ctx, p, pmes)
}

func (dht *IpfsDHT) fp(ctx context.Context, p peer.ID, key multihash.Multihash) ([]*peer.AddrInfo, error) {
	// For DHT query command
	routing.PublishQueryEvent(ctx, &routing.QueryEvent{
		Type: routing.SendingQuery,
		ID:   p,
	})

	pmes, err := dht.findProvidersSingle(ctx, p, key)
	if err != nil {
		return nil, err
	}

	provs := pb.PBPeersToPeerInfos(pmes.GetProviderPeers())
	_ = provs

	// Give closer peers back to the query to be queried
	closer := pmes.GetCloserPeers()
	peers := pb.PBPeersToPeerInfos(closer)

	return peers, nil
}

// findPeerSingle asks peer 'p' if they know where the peer with id 'id' is
func (dht *IpfsDHT) findPeerSingle(ctx context.Context, p peer.ID, id peer.ID) ([]*peer.AddrInfo, error) {
	pmes := pb.NewMessage(pb.Message_FIND_NODE, []byte(id), 0)
	res, err := dht.sendRequest(ctx, p, pmes)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, fmt.Errorf("null result returned")
	}
	return pb.PBPeersToPeerInfos(res.CloserPeers), nil
}

func (dht *IpfsDHT) findProvidersSingle(ctx context.Context, p peer.ID, key multihash.Multihash) (*pb.Message, error) {
	pmes := pb.NewMessage(pb.Message_GET_PROVIDERS, key, 0)
	return dht.sendRequest(ctx, p, pmes)
}

func (dht *IpfsDHT) makeProvRecord(key []byte) (*pb.Message, error) {
	pi := peer.AddrInfo{
		ID:    dht.host.ID(),
		Addrs: dht.host.Addrs(),
	}

	// // only share WAN-friendly addresses ??
	// pi.Addrs = addrutil.WANShareableAddrs(pi.Addrs)
	if len(pi.Addrs) < 1 {
		return nil, fmt.Errorf("no known addresses for self, cannot put provider")
	}

	pmes := pb.NewMessage(pb.Message_ADD_PROVIDER, key, 0)
	pmes.ProviderPeers = pb.RawPeerInfosToPBPeers([]peer.AddrInfo{pi})
	return pmes, nil
}