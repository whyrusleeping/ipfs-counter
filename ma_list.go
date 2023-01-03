package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
)

// maList is a list of peers by id
type maList map[peer.ID]Node

// NewMAList makes a new multiaddr list
func newMAList() maList {
	return make(map[peer.ID]Node)
}

func (m maList) Add(ma multiaddr.Multiaddr) error {
	pi, err := peer.AddrInfoFromP2pAddr(ma)
	if err != nil {
		return err
	}
	if _, ok := m[pi.ID]; !ok {
		m[pi.ID] = Node{
			ID:    pi.ID,
			Addrs: []multiaddr.Multiaddr{ma},
		}
	}
	return nil
}

func (m maList) AddString(ma string) error {
	parsed, err := multiaddr.NewMultiaddr(ma)
	if err != nil {
		return err
	}
	return m.Add(parsed)
}

func (m maList) AddStrings(lines []string) (bool, error) {
	var err error
	for _, maddr := range lines {
		if len(maddr) > 0 {
			if e2 := m.AddString(maddr); e2 != nil {
				err = fmt.Errorf("%v; %w", err, e2)
				continue
			}
		}
	}
	return true, err
}

func (m maList) AddFile(f string) (bool, error) {
	lines, err := os.ReadFile(f)
	if err != nil {
		return false, err
	}
	return m.AddStrings(strings.Split(string(lines), "\n"))
}
