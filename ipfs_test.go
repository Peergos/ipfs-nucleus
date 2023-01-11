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
	crypto "github.com/libp2p/go-libp2p-core/crypto"
	peer "github.com/libp2p/go-libp2p-core/peer"
	multiaddr "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
	"github.com/peergos/go-bitswap-auth/auth"
	blockstore "github.com/peergos/go-ipfs-blockstore"
)

func NewInMemoryDatastore() datastore.Batching {
	return dssync.MutexWrap(datastore.NewMapDatastore())
}

func allowAll(cid.Cid, []byte, peer.ID, string) bool {
	return true
}

func buildBlockstore(useBloom bool, ctx context.Context) (ds datastore.Batching, bs blockstore.Blockstore, abs auth.AuthBlockstore) {
	ds = NewInMemoryDatastore()
	bs = blockstore.NewBlockstore(ds)
	cacheOpts := blockstore.DefaultCacheOpts()
	if useBloom {
		cacheOpts.HasBloomFilterSize = 268435456
	}
	cached, err := blockstore.CachedBlockstore(ctx, bs, cacheOpts)
	if err != nil {
		panic(err)
	}
	abs = auth.NewAuthBlockstore(bs, allowAll)
	return ds, cached, abs
}

func setupPeers(t *testing.T, useBloom bool) (p1, p2 *Peer, closer func(t *testing.T)) {
	ctx, cancel := context.WithCancel(context.Background())

	ds1, bs1, abs1 := buildBlockstore(useBloom, ctx)
	ds2, bs2, abs2 := buildBlockstore(useBloom, ctx)

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
	p1, p2, closer := setupPeers(t, false)
	defer closer(t)

	content := []byte("hola")
	hash, _ := mh.Sum(content, mh.SHA2_256, -1)
	c := cid.NewCidV1(cid.Raw, hash)
	block, _ := blocks.NewBlockWithCid(content, c)
	err := p1.PutBlock(block)
	if err != nil {
		t.Fatal(err)
	}

	res, err := p2.GetBlock(auth.NewWant(c, "1234567890abcdef"))
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

func TestBlockBloom(t *testing.T) {
	p1, p2, closer := setupPeers(t, true)
	defer closer(t)

	content := []byte("hola")
	hash, _ := mh.Sum(content, mh.SHA2_256, -1)
	c := cid.NewCidV1(cid.Raw, hash)
	block, _ := blocks.NewBlockWithCid(content, c)
	err := p1.PutBlock(block)
	if err != nil {
		t.Fatal(err)
	}

	res, err := p2.GetBlock(auth.NewWant(c, "1234567890abcdef"))
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
