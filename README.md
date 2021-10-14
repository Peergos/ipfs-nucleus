# IPFS-Nucleus

IPFS-Nucleus is a minimal block daemon for IPLD based services. You could call it an IPLDaemon.

It implements the following http api calls from IPFS:
* id
* block.get
* block.put
* block.stat
* refs
* block.rm (only needed by GC)
* refs.local (only needed by GC)

As well as implementing the p2p http proxy.

It is designed as a drop in replacment for IPFS with the minimal functionality that [Peergos](https://github.com/peergos/peergos) needs to operate, including running an external GC. It includes support for flatfs, leveldb and S3 based blockstores.

It will read its config from a prior existing ipfs config file if present, or create one with the relevant parameters.

## Building
> go build daemon/ipfs-nucleus.go

## License

AGPL
