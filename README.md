# IPFS-Nucleus

IPFS-Nucleus is a minimal block daemon for IPLD based services. You could call it an **IPLDaemon**. It includes optional auth for retrieving blocks over bitswap.

It implements the following http api calls from IPFS:
* id
* block.get
* block.put
* block.stat
* block.rm (only needed by GC)
* refs.local (only needed by GC)

As well as implementing the p2p http proxy (incoming and outgoing). 

## New config options
* Addresses.ProxyTarget - The incoming http proxy target
* Addresses.AllowTarget - The api for allow calls when blocks are requested over bitswap - allow(cid, block data, source peer id, auth string) => boolean

It is designed as a drop in replacment for IPFS with the minimal functionality that [Peergos](https://github.com/peergos/peergos) needs to operate, including running an external GC. It includes support for leveldb datastore and flatfs and S3 based blockstores including bloomfilter based wrapping.

It will read its config from a prior existing ipfs config file if present, or create one with the relevant parameters. It uses a v0 blockstore (cids) rather than a v1 (multihashes).

The CLI supports the following sub-commands similar to ipfs:
* init
* config

## Building
> go build daemon/ipfs-nucleus.go

## License

AGPL
