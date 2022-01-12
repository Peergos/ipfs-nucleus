package api

import (
	"fmt"
	"io/ioutil"
	"net/http"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
	"github.com/peergos/go-bitswap-auth/auth"
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
	authz := req.URL.Query()["auth"][0]
	block, err := a.Nucleus.GetBlock(auth.NewWant(cid, authz))
	if err != nil {
        fmt.Println("Ipfs error := ", err)
                handleError(rw, err.Error(), err, 400)
                return
	}
	rw.Write(block.GetAuthedData())
}

func (a *HttpApi) BlockPut(rw http.ResponseWriter, req *http.Request) {
	if !a.isPost(req) {
		rw.WriteHeader(http.StatusForbidden)
		return
	}
	format := req.URL.Query()["format"][0]
	reader, e := req.MultipartReader()
	if e != nil {
                handleError(rw, e.Error(), e, 400)
                return
	}
	part, e := reader.NextPart()
	if e != nil {
                handleError(rw, e.Error(), e, 400)
                return
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
                handleError(rw, err.Error(), err, 400)
                return
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
                handleError(rw, err.Error(), err, 400)
                return
	}
}

func (a *HttpApi) BlockStat(rw http.ResponseWriter, req *http.Request) {
	if !a.isPost(req) {
		rw.WriteHeader(http.StatusForbidden)
		return
	}
	arg := req.URL.Query()["arg"][0]
	cid, _ := cid.Decode(arg)
	authz := req.URL.Query()["auth"][0]
	block, err := a.Nucleus.GetBlock(auth.NewWant(cid, authz))
	if err != nil {
                handleError(rw, err.Error(), err, 400)
                return
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.Write([]byte(fmt.Sprintf("{\"Size\":%d}", len(block.GetAuthedData()))))
}

func (a *HttpApi) LocalRefs(rw http.ResponseWriter, req *http.Request) {
	if !a.isPost(req) {
		rw.WriteHeader(http.StatusForbidden)
		return
	}
	refs, err := a.Nucleus.GetRefs()
	if err != nil {
		handleError(rw, err.Error(), err, 400)
                return
	}
	rw.Header().Set("Content-Type", "application/json")
	for ref := range refs {
		rw.Write([]byte(fmt.Sprintf("{\"Ref\":\"%s\"}", ref)))
	}
}

func (a *HttpApi) isPost(req *http.Request) bool {
	return req.Method == "POST"
}

func handleError(w http.ResponseWriter, msg string, err error, code int) {
     fmt.Println("IPFS returning HTTP error", code, msg, err)
	http.Error(w, fmt.Sprintf("%s: %s", msg, err), code)
}