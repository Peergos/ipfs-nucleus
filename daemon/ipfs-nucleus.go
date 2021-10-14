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
	s3ds "github.com/ipfs/go-ds-s3"
	manet "github.com/multiformats/go-multiaddr/net"
	ipfsnucleus "github.com/peergos/ipfs-nucleus"
	api "github.com/peergos/ipfs-nucleus/api"
	config "github.com/peergos/ipfs-nucleus/config"
	p2phttp "github.com/peergos/ipfs-nucleus/p2phttp"
	ldbopts "github.com/syndtr/goleveldb/leveldb/opt"
)

func buildDatastore(ipfsDir string, config config.DataStoreConfig) ds.Batching {
	if config.Type == "flatfs" {
		shardFun, err := flatfs.ParseShardFunc(config.Params["shardFunc"].(string))
		if err != nil {
			panic(err)
		}
		rawds, err := flatfs.CreateOrOpen(ipfsDir+config.Path, shardFun, config.Params["sync"].(bool))
		if err != nil {
			panic(err)
		}
		mount := dsmount.Mount{Prefix: ds.NewKey(config.MountPoint), Datastore: rawds}
		mds := dsmount.New([]dsmount.Mount{mount})
		return mds
	}
	if config.Type == "levelds" {
		leveldb, err := levelds.NewDatastore(ipfsDir+config.Path, &levelds.Options{
			Compression: ldbopts.NoCompression,
		})
		if err != nil {
			panic(err)
		}
		return leveldb
	}
	if config.Type == "s3ds" {
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
		s3,err := s3ds.NewS3Datastore(cfg)
                if err != nil {
			panic(err)
		}
		mount := dsmount.Mount{Prefix: ds.NewKey(config.MountPoint), Datastore: s3}
		mds := dsmount.New([]dsmount.Mount{mount})
		return mds
	}
	panic("Unknown datastore type: " + config.Type)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config := config.ParseOrGenerateConfig(".ipfs/config")

	blockstore := buildDatastore(".ipfs/", config.Blockstore)
	rootstore := buildDatastore(".ipfs/", config.Rootstore)

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

	nucleus, err := ipfsnucleus.New(ctx, blockstore, h, dht, nil)
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
