package ipfsnucleus

import (
	"bytes"
	"context"
	"io"
	"testing"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	datastore "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/peergos/go-ipfs-blockstore"
	crypto "github.com/libp2p/go-libp2p-core/crypto"
	peer "github.com/libp2p/go-libp2p-core/peer"
	multiaddr "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
	"github.com/peergos/go-bitswap-auth/auth"
)

func NewInMemoryDatastore() datastore.Batching {
	return dssync.MutexWrap(datastore.NewMapDatastore())
}

func allowAll(cid.Cid, []byte, peer.ID, string) bool {
	return true
}

func setupPeers(t *testing.T) (p1, p2 *Peer, closer func(t *testing.T)) {
	ctx, cancel := context.WithCancel(context.Background())

	ds1 := NewInMemoryDatastore()
	ds2 := NewInMemoryDatastore()
        bs1 := blockstore.NewBlockstore(ds1)
	abs1 := auth.NewAuthBlockstore(bs1, allowAll)
	bs2 := blockstore.NewBlockstore(ds2)
	abs2 := auth.NewAuthBlockstore(bs2, allowAll)

	priv1, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	if err != nil {
		t.Fatal(err)
	}
	priv2, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	if err != nil {
		t.Fatal(err)
	}

	listen, _ := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/0")
	h1, dht1, err := SetupLibp2p(
		ctx,
		priv1,
		[]multiaddr.Multiaddr{listen},
		nil,
		Libp2pOptionsExtra...,
	)
	if err != nil {
		t.Fatal(err)
	}

	pinfo1 := peer.AddrInfo{
		ID:    h1.ID(),
		Addrs: h1.Addrs(),
	}

	h2, dht2, err := SetupLibp2p(
		ctx,
		priv2,
		[]multiaddr.Multiaddr{listen},
		nil,
		Libp2pOptionsExtra...,
	)
	if err != nil {
		t.Fatal(err)
	}

	pinfo2 := peer.AddrInfo{
		ID:    h2.ID(),
		Addrs: h2.Addrs(),
	}

	closer = func(t *testing.T) {
		cancel()
		for _, cl := range []io.Closer{dht1, dht2, h1, h2} {
			err := cl.Close()
			if err != nil {
				t.Error(err)
			}
		}
	}
	p1, err = New(ctx, bs1, abs1, ds1, h1, dht1, nil)
	if err != nil {
		closer(t)
		t.Fatal(err)
	}
	p2, err = New(ctx, bs2, abs2, ds2, h2, dht2, nil)
	if err != nil {
		closer(t)
		t.Fatal(err)
	}

	p1.Bootstrap([]peer.AddrInfo{pinfo2})
	p2.Bootstrap([]peer.AddrInfo{pinfo1})

	return
}

func TestBlock(t *testing.T) {
	p1, p2, closer := setupPeers(t)
	defer closer(t)

	content := []byte("hola")
	hash, _ := mh.Sum(content, mh.SHA2_256, -1)
	c := cid.NewCidV1(cid.Raw, hash)
	block, _ := blocks.NewBlockWithCid(content, c)
	err := p1.PutBlock(block)
	if err != nil {
		t.Fatal(err)
	}

	res, err := p2.GetBlock(auth.NewWant(c, "auth"))
	if err != nil {
		t.Fatal(err)
	}

	content2 := res.GetAuthedData()

	if !bytes.Equal(content, content2) {
		t.Error(string(content))
		t.Error(string(content2))
		t.Error("different content put and retrieved")
	}
}
