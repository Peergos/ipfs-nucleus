package main

// This launches an IPFS-Nucleus peer

import (
	"context"
	"fmt"
	"net/http"
	"os"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-cid"
	peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/peergos/go-bitswap-auth/auth"
	dsmount "github.com/ipfs/go-datastore/mount"
	flatfs "github.com/ipfs/go-ds-flatfs"
	levelds "github.com/ipfs/go-ds-leveldb"
	s3ds "github.com/peergos/go-ds-s3"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	protocol "github.com/libp2p/go-libp2p-core/protocol"
	manet "github.com/multiformats/go-multiaddr/net"
	ipfsnucleus "github.com/peergos/ipfs-nucleus"
	api "github.com/peergos/ipfs-nucleus/api"
	config "github.com/peergos/ipfs-nucleus/config"
        pbs "github.com/peergos/ipfs-nucleus/blockstore"
	p2p "github.com/peergos/ipfs-nucleus/p2p"
	p2phttp "github.com/peergos/ipfs-nucleus/p2phttp"
	ldbopts "github.com/syndtr/goleveldb/leveldb/opt"
)

func buildBlockstore(ipfsDir string, config config.DataStoreConfig, bloomFilterSize int, allow func(cid.Cid, peer.ID, string)bool, ctx context.Context) auth.AuthBlockstore {
	var rawds ds.Batching
	var err error
	if config.Type == "flatfs" {
		shardFun, err := flatfs.ParseShardFunc(config.Params["shardFunc"].(string))
		if err != nil {
			panic(err)
		}
		rawds, err = flatfs.CreateOrOpen(ipfsDir+config.Path, shardFun, config.Params["sync"].(bool))
		if err != nil {
			panic(err)
		}
	} else if config.Type == "s3ds" {
		accessKey := config.Params["accessKey"].(string)
		bucket := config.Params["bucket"].(string)
		region := config.Params["region"].(string)
		regionEndpoint := config.Params["regionEndpoint"].(string)
		secretKey := config.Params["secretKey"].(string)
		cfg := s3ds.Config{
			Region:              region,
			Bucket:              bucket,
			AccessKey:           accessKey,
			SecretKey:           secretKey,
			SessionToken:        "",
			RootDirectory:       "",
			Workers:             0,
			RegionEndpoint:      regionEndpoint,
			CredentialsEndpoint: "",
		}
		rawds, err = s3ds.NewS3Datastore(cfg)
		if err != nil {
			panic(err)
		}
	} else {
		panic("Unknown blockstore type: " + config.Type)
	}
	mount := dsmount.Mount{Prefix: ds.NewKey(config.MountPoint), Datastore: rawds}
	mds := dsmount.New([]dsmount.Mount{mount})
	bs := blockstore.NewBlockstore(mds)
	bs = blockstore.NewIdStore(bs)
	cacheOpts := blockstore.DefaultCacheOpts()
	cacheOpts.HasBloomFilterSize = bloomFilterSize
	cached, err := blockstore.CachedBlockstore(ctx, bs, cacheOpts)
	if err != nil {
		panic(err)
	}
	return auth.NewAuthBlockstore(cached, allow)
}

func buildDatastore(ipfsDir string, config config.DataStoreConfig) ds.Batching {
	if config.Type == "levelds" {
		leveldb, err := levelds.NewDatastore(ipfsDir+config.Path, &levelds.Options{
			Compression: ldbopts.NoCompression,
		})
		if err != nil {
			panic(err)
		}
		return leveldb
	}
	panic("Unknown datastore type: " + config.Type)
}

func allowAll(cid.Cid, peer.ID, string) bool {
     return true
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ipfsDir := os.Getenv("IPFS_PATH")
	nArgs := len(os.Args[1:])
	if nArgs > 0 && os.Args[1] == "config" {
		// update a field in the config file
		isJSON := nArgs > 1 && os.Args[2] == "--json"
		var fieldName string
		var value string
		if isJSON {
			fieldName = os.Args[3]
			value = os.Args[4]
		} else {
			fieldName = os.Args[2]
			value = os.Args[3]
		}
		config.SetField(ipfsDir+"/config", fieldName, value, isJSON)
		return
	}

	config := config.ParseOrGenerateConfig(ipfsDir + "/config")
	if nArgs > 0 && os.Args[1] == "init" {
		return
	}

	blockstore := pbs.NewPeergosBlockstore(buildBlockstore(ipfsDir+"/", config.Blockstore, config.BloomFilterSize, allowAll, ctx))
	rootstore := buildDatastore(ipfsDir+"/", config.Rootstore)

	h, dht, err := ipfsnucleus.SetupLibp2p(
		ctx,
		config.HostKey,
		config.Swarm,
		rootstore,
		ipfsnucleus.Libp2pOptionsExtra...,
	)
	if err != nil {
		panic(err)
	}

	nucleus, err := ipfsnucleus.New(ctx, blockstore, rootstore, h, dht, nil)
	if err != nil {
		panic(err)
	}

	// setup incoming http proxy
	proto := protocol.ID("/http")
	// port can't be 0
	if err := p2p.CheckPort(config.ProxyTarget); err != nil {
		panic(err)
	}
	_, err = nucleus.P2P.ForwardRemote(ctx, proto, config.ProxyTarget, false)

	api := &api.HttpApi{nucleus}

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/v0/id", api.Id)
	apiMux.HandleFunc("/api/v0/block/get", api.BlockGet)
	apiMux.HandleFunc("/api/v0/block/put", api.BlockPut)
	apiMux.HandleFunc("/api/v0/block/rm", api.BlockRm)
	apiMux.HandleFunc("/api/v0/block/stat", api.BlockStat)
	apiMux.HandleFunc("/api/v0/refs/local", api.LocalRefs)

	nucleus.Bootstrap(config.Bootstrap)

	apiAddr, err := manet.ToNetAddr(config.Api)
	go serve(apiAddr.String(), apiMux)

	p2p := &p2phttp.P2PProxy{nucleus}
	gatewayMux := http.NewServeMux()
	gatewayMux.HandleFunc("/p2p/", p2p.Proxy)
	gatewayAddr, err := manet.ToNetAddr(config.Gateway)
	serve(gatewayAddr.String(), gatewayMux)
}

func serve(addr string, muxer http.Handler) {
	err := http.ListenAndServe(addr, muxer)
	if err != nil {
		fmt.Println("Fatal error ", err.Error())
		os.Exit(1)
	}
}
