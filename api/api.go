package api

import (
	"fmt"
	"io/ioutil"
	"net/http"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
	ipfsnucleus "github.com/peergos/ipfs-nucleus"
)

type HttpApi struct {
	Nucleus *ipfsnucleus.Peer
}

func (a *HttpApi) Id(rw http.ResponseWriter, req *http.Request) {
	if !a.isPost(req) {
		rw.WriteHeader(http.StatusForbidden)
		return
	}
	rw.Write([]byte(fmt.Sprintf("{\"ID\":\"%s\"}", a.Nucleus.Host.ID())))
}

func (a *HttpApi) BlockGet(rw http.ResponseWriter, req *http.Request) {
	if !a.isPost(req) {
		rw.WriteHeader(http.StatusForbidden)
		return
	}
	arg := req.URL.Query()["arg"][0]
	cid, _ := cid.Decode(arg)
	block, err := a.Nucleus.GetBlock(cid)
	if err != nil {
		panic(err)
	}
	rw.Write(block.RawData())
}

func (a *HttpApi) BlockPut(rw http.ResponseWriter, req *http.Request) {
	if !a.isPost(req) {
		rw.WriteHeader(http.StatusForbidden)
		return
	}
	format := req.URL.Query()["format"][0]
	reader, e := req.MultipartReader()
	if e != nil {
		panic(e)
	}
	part, e := reader.NextPart()
	if e != nil {
		panic(e)
	}
	data, _ := ioutil.ReadAll(part)
	hash, _ := mh.Sum(data, mh.SHA2_256, -1)

	var c cid.Cid
	if format == "raw" {
		c = cid.NewCidV1(cid.Raw, hash)
	} else if format == "cbor" {
		c = cid.NewCidV1(cid.DagCBOR, hash)
	} else {
		return
	}

	b, _ := blocks.NewBlockWithCid(data, c)
	err := a.Nucleus.PutBlock(b)
	if err != nil {
		panic(err)
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.Write([]byte(fmt.Sprintf("{\"Hash\":\"%s\"}", c)))
}

func (a *HttpApi) BlockRm(rw http.ResponseWriter, req *http.Request) {
	if !a.isPost(req) {
		rw.WriteHeader(http.StatusForbidden)
		return
	}
	arg := req.URL.Query()["arg"][0]
	cid, _ := cid.Decode(arg)
	err := a.Nucleus.RmBlock(cid)
	if err != nil {
		panic(err)
	}
}

func (a *HttpApi) BlockStat(rw http.ResponseWriter, req *http.Request) {
	if !a.isPost(req) {
		rw.WriteHeader(http.StatusForbidden)
		return
	}
	arg := req.URL.Query()["arg"][0]
	cid, _ := cid.Decode(arg)
	block, err := a.Nucleus.GetBlock(cid)
	if err != nil {
		panic(err)
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.Write([]byte(fmt.Sprintf("{\"Size\":%d}", len(block.RawData()))))
}

func (a *HttpApi) Refs(rw http.ResponseWriter, req *http.Request) {
	if !a.isPost(req) {
		rw.WriteHeader(http.StatusForbidden)
		return
	}
	arg := req.URL.Query()["arg"][0]
	cid, _ := cid.Decode(arg)
	links, err := a.Nucleus.GetLinks(cid)
	if err != nil {
		panic(err)
	}
	rw.Header().Set("Content-Type", "application/json")
	for _, link := range links {
		rw.Write([]byte(fmt.Sprintf("{\"Ref\":\"%s\"}", link.Cid)))
	}
}

func (a *HttpApi) LocalRefs(rw http.ResponseWriter, req *http.Request) {
	if !a.isPost(req) {
		rw.WriteHeader(http.StatusForbidden)
		return
	}
	refs, err := a.Nucleus.GetRefs()
	if err != nil {
		panic(err)
	}
	rw.Header().Set("Content-Type", "application/json")
	for ref := range refs {
		rw.Write([]byte(fmt.Sprintf("{\"Ref\":\"%s\"}", ref)))
	}
}

func (a *HttpApi) isPost(req *http.Request) bool {
	return req.Method == "POST"
}
