package blockstore

import (
	"context"
	"errors"

	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	blockstore "github.com/peergos/go-ipfs-blockstore"
)

var ErrInvalidBlockType = errors.New("Invalid block type")

type TypeLimitedBlockstore interface {
	BloomAdd(cid.Cid) error
	DeleteBlock(cid.Cid) error
	Has(cid.Cid) (bool, error)
	Get(cid.Cid) (blocks.Block, error)

	// GetSize returns the CIDs mapped BlockSize
	GetSize(cid.Cid) (int, error)

	// Put puts a given block to the underlying datastore
	Put(blocks.Block) error

	// PutMany puts a slice of blocks at the same time using batching
	// capabilities of the underlying datastore whenever possible.
	PutMany([]blocks.Block) error

	// AllKeysChan returns a channel from which
	// the CIDs in the Blockstore can be read. It should respect
	// the given context, closing the channel if it becomes Done.
	AllKeysChan(ctx context.Context) (<-chan cid.Cid, error)

	// HashOnRead specifies if every read block should be
	// rehashed to make sure it matches its CID.
	HashOnRead(enabled bool)
}

type PeergosBlockstore struct {
	source blockstore.Blockstore
	TypeLimitedBlockstore
}

func NewPeergosBlockstore(bstore blockstore.Blockstore) TypeLimitedBlockstore {
	return &PeergosBlockstore{source: bstore}
}

func (bs *PeergosBlockstore) Get(c cid.Cid) (blocks.Block, error) {
	codec := c.Type()
	// we only allow these block types
	if codec != cid.Raw && codec != cid.DagCBOR {
		return nil, blockstore.ErrNotFound
	}
	return bs.source.Get(c)
}

func (bs *PeergosBlockstore) DeleteBlock(c cid.Cid) error {
	return bs.source.DeleteBlock(c)
}

func (bs *PeergosBlockstore) BloomAdd(c cid.Cid) error {
	return bs.source.BloomAdd(c)
}

func (bs *PeergosBlockstore) Has(c cid.Cid) (bool, error) {
	codec := c.Type()
	// we only allow these block types
	if codec != cid.Raw && codec != cid.DagCBOR {
		return false, nil
	}
	return bs.source.Has(c)
}

func (bs *PeergosBlockstore) GetSize(c cid.Cid) (int, error) {
	codec := c.Type()
	// we only allow these block types
	if codec != cid.Raw && codec != cid.DagCBOR {
		return -1, blockstore.ErrNotFound
	}
	return bs.source.GetSize(c)
}

func (bs *PeergosBlockstore) Put(b blocks.Block) error {
	codec := b.Cid().Type()
	// we only allow these block types
	if codec != cid.Raw && codec != cid.DagCBOR {
		return ErrInvalidBlockType
	}
	return bs.source.Put(b)
}

func (bs *PeergosBlockstore) PutMany(blocks []blocks.Block) error {
	return bs.source.PutMany(blocks)
}

func (bs *PeergosBlockstore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	return bs.source.AllKeysChan(ctx)
}

func (bs *PeergosBlockstore) HashOnRead(enabled bool) {
	bs.source.HashOnRead(enabled)
}
