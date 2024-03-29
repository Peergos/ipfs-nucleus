package ipfsnucleus

import (
	"context"
	"sync"
	"time"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	ds "github.com/ipfs/go-datastore"
	provider "github.com/ipfs/go-ipfs-provider"
	"github.com/ipfs/go-ipfs-provider/queue"
	"github.com/ipfs/go-ipfs-provider/simple"
	cbor "github.com/ipfs/go-ipld-cbor"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipfs/go-merkledag"
	host "github.com/libp2p/go-libp2p-core/host"
	peer "github.com/libp2p/go-libp2p-core/peer"
	routing "github.com/libp2p/go-libp2p-core/routing"
	"github.com/peergos/go-bitswap-auth"
	"github.com/peergos/go-bitswap-auth/auth"
	"github.com/peergos/go-bitswap-auth/network"
	"github.com/peergos/ipfs-nucleus/blockservice"
	bstore "github.com/peergos/go-ipfs-blockstore"
	p2p "github.com/peergos/ipfs-nucleus/p2p"
)

func init() {
	ipld.Register(cid.DagProtobuf, merkledag.DecodeProtobufBlock)
	ipld.Register(cid.Raw, merkledag.DecodeRawBlock)
	ipld.Register(cid.DagCBOR, cbor.DecodeBlock) // need to decode CBOR
}

var logger = logging.Logger("ipfsnucleus")

var (
	defaultReprovideInterval = 12 * time.Hour
)

// Config wraps configuration options for the Peer.
type Config struct {
	// ReprovideInterval sets how often to reprovide records to the DHT
	ReprovideInterval time.Duration
}

func (cfg *Config) setDefaults() {
	if cfg.ReprovideInterval == 0 {
		cfg.ReprovideInterval = defaultReprovideInterval
	}
}

type Peer struct {
	ctx context.Context

	cfg *Config

	Host  host.Host
	dht   routing.Routing
	store datastore.Batching

	P2P          p2p.P2P
	bstore       bstore.Blockstore
	authedbstore auth.AuthBlockstore
	bserv        blockservice.BlockService
	reprovider   provider.System
	allow        func(cid.Cid, peer.ID, string) bool
}

// New creates an IPFS-Nucleus Peer. It uses the given datastore, libp2p Host and
// Routing (DHT). Peer implements the ipld.DAGService interface.
func New(
	ctx context.Context,
	blockstore bstore.Blockstore,
	authedblockstore auth.AuthBlockstore,
	rootstore ds.Batching,
	host host.Host,
	dht routing.Routing,
	cfg *Config,
) (*Peer, error) {

	if cfg == nil {
		cfg = &Config{}
	}

	cfg.setDefaults()
	proxy := p2p.New(host.ID(), host, host.Peerstore())

	p := &Peer{
		ctx:          ctx,
		cfg:          cfg,
		Host:         host,
		dht:          dht,
		bstore:       blockstore,
		authedbstore: authedblockstore,
		store:        rootstore,
		P2P:          *proxy,
	}

	err := p.setupBlockService()
	if err != nil {
		return nil, err
	}
	err = p.setupReprovider()
	if err != nil {
		p.bserv.Close()
		return nil, err
	}

	go p.autoclose()

	return p, nil
}

func (p *Peer) setupBlockService() error {
	bswapnet := network.NewFromIpfsHost(p.Host, p.dht)
	bswap := bitswap.New(p.ctx, bswapnet, p.authedbstore)
	p.bserv = blockservice.New(p.authedbstore, bswap, p.Host.ID())
	return nil
}

func (p *Peer) setupReprovider() error {
	if p.cfg.ReprovideInterval < 0 {
		p.reprovider = provider.NewOfflineProvider()
		return nil
	}

	queue, err := queue.NewQueue(p.ctx, "repro", p.store)
	if err != nil {
		return err
	}

	prov := simple.NewProvider(
		p.ctx,
		queue,
		p.dht,
	)

	reprov := simple.NewReprovider(
		p.ctx,
		p.cfg.ReprovideInterval,
		p.dht,
		func(ctx context.Context) (<-chan cid.Cid, error) {
			return p.bstore.AllKeysChan(ctx)
		},
	)

	p.reprovider = provider.NewSystem(prov, reprov)
	p.reprovider.Run()
	return nil
}

func (p *Peer) autoclose() {
	<-p.ctx.Done()
	p.reprovider.Close()
	p.bserv.Close()
}

// Bootstrap is an optional helper to connect to the given peers and bootstrap
// the Peer DHT (and Bitswap). This is a best-effort function. Errors are only
// logged and a warning is printed when less than half of the given peers
// could be contacted. It is fine to pass a list where some peers will not be
// reachable.
func (p *Peer) Bootstrap(peers []peer.AddrInfo) {
	connected := make(chan struct{})

	var wg sync.WaitGroup
	for _, pinfo := range peers {
		//h.Peerstore().AddAddrs(pinfo.ID, pinfo.Addrs, peerstore.PermanentAddrTTL)
		wg.Add(1)
		go func(pinfo peer.AddrInfo) {
			defer wg.Done()
			err := p.Host.Connect(p.ctx, pinfo)
			if err != nil {
				logger.Warn(err)
				return
			}
			logger.Info("Connected to", pinfo.ID)
			connected <- struct{}{}
		}(pinfo)
	}

	go func() {
		wg.Wait()
		close(connected)
	}()

	i := 0
	for range connected {
		i++
	}
	if nPeers := len(peers); i < nPeers/2 {
		logger.Warnf("only connected to %d bootstrap peers out of %d", i, nPeers)
	}

	err := p.dht.Bootstrap(p.ctx)
	if err != nil {
		logger.Error(err)
		return
	}
}

// BlockStore offers access to the blockstore underlying the Peer's DAGService.
func (p *Peer) BlockStore() bstore.Blockstore {
	return p.bstore
}

// HasBlock returns whether a given block is available locally. It is
// a shorthand for .Blockstore().Has().
func (p *Peer) HasBlock(c cid.Cid) (bool, error) {
	return p.BlockStore().Has(c)
}

func (p *Peer) GetBlock(w auth.Want) (auth.AuthBlock, error) {
	local, _ := p.HasBlock(w.Cid)
	if local {
		block, err := p.BlockStore().Get(w.Cid)
		if err != nil {
			panic(err)
		}
		return auth.NewBlock(block, w.Auth), nil
	}
	return p.bserv.GetBlock(p.ctx, w)
}

func (p *Peer) PutBlock(b blocks.Block) error {
	return p.BlockStore().Put(b)
}

func (p *Peer) RmBlock(c cid.Cid) error {
	local, _ := p.HasBlock(c)
	if local {
		return p.BlockStore().DeleteBlock(c)
	}
	return nil
}

func (p *Peer) BloomAdd(c cid.Cid) error {
     return p.BlockStore().BloomAdd(c)
}

func (p *Peer) GetRefs() (<-chan cid.Cid, error) {
	return p.BlockStore().AllKeysChan(p.ctx)
}
