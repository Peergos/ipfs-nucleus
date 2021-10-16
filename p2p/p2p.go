package p2p

import (
	"fmt"
	"strconv"

	logging "github.com/ipfs/go-log"
	p2phost "github.com/libp2p/go-libp2p-core/host"
	peer "github.com/libp2p/go-libp2p-core/peer"
	pstore "github.com/libp2p/go-libp2p-core/peerstore"
	ma "github.com/multiformats/go-multiaddr"
)

var log = logging.Logger("p2p-mount")

// P2P structure holds information on currently running streams/Listeners
type P2P struct {
	ListenersLocal *Listeners
	ListenersP2P   *Listeners
	Streams        *StreamRegistry

	identity  peer.ID
	peerHost  p2phost.Host
	peerstore pstore.Peerstore
}

// New creates new P2P struct
func New(identity peer.ID, peerHost p2phost.Host, peerstore pstore.Peerstore) *P2P {
	return &P2P{
		identity:  identity,
		peerHost:  peerHost,
		peerstore: peerstore,

		ListenersLocal: newListenersLocal(),
		ListenersP2P:   newListenersP2P(peerHost),

		Streams: &StreamRegistry{
			Streams:     map[uint64]*Stream{},
			ConnManager: peerHost.ConnManager(),
			conns:       map[peer.ID]int{},
		},
	}
}

// CheckProtoExists checks whether a proto handler is registered to
// mux handler
func (p2p *P2P) CheckProtoExists(proto string) bool {
	protos := p2p.peerHost.Mux().Protocols()

	for _, p := range protos {
		if p != proto {
			continue
		}
		return true
	}
	return false
}

// checkPort checks whether target multiaddr contains tcp or udp protocol
// and whether the port is equal to 0
func CheckPort(target ma.Multiaddr) error {
	// get tcp or udp port from multiaddr
	getPort := func() (string, error) {
		sport, _ := target.ValueForProtocol(ma.P_TCP)
		if sport != "" {
			return sport, nil
		}

		sport, _ = target.ValueForProtocol(ma.P_UDP)
		if sport != "" {
			return sport, nil
		}
		return "", fmt.Errorf("address does not contain tcp or udp protocol")
	}

	sport, err := getPort()
	if err != nil {
		return err
	}

	port, err := strconv.Atoi(sport)
	if err != nil {
		return err
	}

	if port == 0 {
		return fmt.Errorf("port can not be 0")
	}

	return nil
}
