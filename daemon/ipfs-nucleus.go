package main

// This launches an IPFS-Nucleus peer

import (
	"context"
	"fmt"
	"net/http"
	"os"

	ds "github.com/ipfs/go-datastore"
	dsmount "github.com/ipfs/go-datastore/mount"
	flatfs "github.com/ipfs/go-ds-flatfs"
	levelds "github.com/ipfs/go-ds-leveldb"
	manet "github.com/multiformats/go-multiaddr/net"
	ipfsnucleus "github.com/peergos/ipfs-nucleus"
	api "github.com/peergos/ipfs-nucleus/api"
	config "github.com/peergos/ipfs-nucleus/config"
	p2phttp "github.com/peergos/ipfs-nucleus/p2phttp"
	ldbopts "github.com/syndtr/goleveldb/leveldb/opt"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shardFun, err := flatfs.ParseShardFunc("/repo/flatfs/shard/v1/next-to-last/2")
	if err != nil {
		panic(err)
	}
	rawds, err := flatfs.CreateOrOpen(".ipfs/blocks", shardFun, true)
	if err != nil {
		panic(err)
	}
	leveldb, err := levelds.NewDatastore(".ipfs/datastore", &levelds.Options{
		Compression: ldbopts.NoCompression,
	})
	if err != nil {
		panic(err)
	}
	mount := dsmount.Mount{Prefix: ds.NewKey("blocks"), Datastore: rawds}
	mds := dsmount.New([]dsmount.Mount{mount})

	config := config.ParseOrGenerateConfig(".ipfs/config")

	h, dht, err := ipfsnucleus.SetupLibp2p(
		ctx,
		config.HostKey,
		config.Swarm,
		leveldb,
		ipfsnucleus.Libp2pOptionsExtra...,
	)
	if err != nil {
		panic(err)
	}

	nucleus, err := ipfsnucleus.New(ctx, mds, h, dht, nil)
	if err != nil {
		panic(err)
	}

	api := &api.HttpApi{nucleus}

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/v0/id", api.Id)
	apiMux.HandleFunc("/api/v0/block/get", api.BlockGet)
	apiMux.HandleFunc("/api/v0/block/put", api.BlockPut)
	apiMux.HandleFunc("/api/v0/block/rm", api.BlockRm)
	apiMux.HandleFunc("/api/v0/block/stat", api.BlockStat)
	apiMux.HandleFunc("/api/v0/refs", api.Refs)
	apiMux.HandleFunc("/api/v0/refs/local", api.LocalRefs)

	nucleus.Bootstrap(ipfsnucleus.DefaultBootstrapPeers())

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
