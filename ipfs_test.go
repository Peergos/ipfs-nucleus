package ipfsnucleus

import (
	"bytes"
	"context"
	"io"
	"testing"

	blockstore "github.com/ipfs/go-ipfs-blockstore"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
	datastore "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	cbor "github.com/ipfs/go-ipld-cbor"
	crypto "github.com/libp2p/go-libp2p-core/crypto"
	peer "github.com/libp2p/go-libp2p-core/peer"
	multiaddr "github.com/multiformats/go-multiaddr"
	multihash "github.com/multiformats/go-multihash"
)

func NewInMemoryDatastore() datastore.Batching {
	return dssync.MutexWrap(datastore.NewMapDatastore())
}

func setupPeers(t *testing.T) (p1, p2 *Peer, closer func(t *testing.T)) {
	ctx, cancel := context.WithCancel(context.Background())

	ds1 := NewInMemoryDatastore()
	ds2 := NewInMemoryDatastore()
        bs1 := blockstore.NewBlockstore(ds1)
        bs2 := blockstore.NewBlockstore(ds2)
        
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
	p1, err = New(ctx, bs1, ds1, h1, dht1, nil)
	if err != nil {
		closer(t)
		t.Fatal(err)
	}
	p2, err = New(ctx, bs2, ds2, h2, dht2, nil)
	if err != nil {
		closer(t)
		t.Fatal(err)
	}

	p1.Bootstrap([]peer.AddrInfo{pinfo2})
	p2.Bootstrap([]peer.AddrInfo{pinfo1})

	return
}

func TestDAG(t *testing.T) {
	ctx := context.Background()
	p1, p2, closer := setupPeers(t)
	defer closer(t)

	m := map[string]string{
		"akey": "avalue",
	}

	codec := uint64(multihash.SHA2_256)
	node, err := cbor.WrapObject(m, codec, multihash.DefaultLengths[codec])
	if err != nil {
		t.Fatal(err)
	}

	t.Log("created node: ", node.Cid())
	err = p1.Add(ctx, node)
	if err != nil {
		t.Fatal(err)
	}

	_, err = p2.Get(ctx, node.Cid())
	if err != nil {
		t.Error(err)
	}

	err = p1.Remove(ctx, node.Cid())
	if err != nil {
		t.Error(err)
	}

	err = p2.Remove(ctx, node.Cid())
	if err != nil {
		t.Error(err)
	}

	if ok, err := p1.BlockStore().Has(node.Cid()); ok || err != nil {
		t.Error("block should have been deleted")
	}

	if ok, err := p2.BlockStore().Has(node.Cid()); ok || err != nil {
		t.Error("block should have been deleted")
	}
}

func TestSession(t *testing.T) {
	ctx := context.Background()
	p1, p2, closer := setupPeers(t)
	defer closer(t)

	m := map[string]string{
		"akey": "avalue",
	}

	codec := uint64(multihash.SHA2_256)
	node, err := cbor.WrapObject(m, codec, multihash.DefaultLengths[codec])
	if err != nil {
		t.Fatal(err)
	}

	t.Log("created node: ", node.Cid())
	err = p1.Add(ctx, node)
	if err != nil {
		t.Fatal(err)
	}

	sesGetter := p2.Session(ctx)
	_, err = sesGetter.Get(ctx, node.Cid())
	if err != nil {
		t.Fatal(err)
	}
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

	res, err := p2.GetBlock(c)
	if err != nil {
		t.Fatal(err)
	}

	content2 := res.RawData()

	if !bytes.Equal(content, content2) {
		t.Error(string(content))
		t.Error(string(content2))
		t.Error("different content put and retrieved")
	}
}
