package config

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"

	ipfsconfig "github.com/ipfs/go-ipfs-config"
	crypto "github.com/libp2p/go-libp2p-core/crypto"
	peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
)

type DataStoreConfig struct {
	Type       string
	MountPoint string
	Path       string
	Params     map[string]interface{}
}

type Config struct {
	Bootstrap       []peer.AddrInfo
	Swarm           []multiaddr.Multiaddr
	HostKey         crypto.PrivKey
	Api             multiaddr.Multiaddr
	Gateway         multiaddr.Multiaddr
	BloomFilterSize int
	Blockstore      DataStoreConfig
	Rootstore       DataStoreConfig
}

func defaultBootstrapPeers() []peer.AddrInfo {
	defaults, _ := ipfsconfig.DefaultBootstrapPeers()
	return defaults
}

func defaultConfig() Config {
	priv, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	if err != nil {
		panic(err)
	}
	return buildConfig(priv,
		defaultBootstrapPeers(),
		[]string{"/ip4/0.0.0.0/tcp/4001", "/ip6/::/tcp/4001"},
		"/ip4/127.0.0.1/tcp/5001",
		"/ip4/127.0.0.1/tcp/8080",
		0,
		DataStoreConfig{Type: "flatfs", MountPoint: "/blocks", Path: "blocks",
			Params: map[string]interface{}{"sync": true, "shardFunc": "/repo/flatfs/shard/v1/next-to-last/2"}},
		DataStoreConfig{Type: "levelds", MountPoint: "/", Path: "datastore", Params: map[string]interface{}{"compression": "none"}})
}

func buildConfig(priv crypto.PrivKey,
	bootstrap []peer.AddrInfo,
	swarmAddrs []string,
	apiAddr string,
	gatewayAddr string,
	BloomFilterSize int,
	blockstore DataStoreConfig,
	rootStore DataStoreConfig) Config {
	swarm := make([]multiaddr.Multiaddr, len(swarmAddrs))
	var err error
	for i, addr := range swarmAddrs {
		swarm[i], err = multiaddr.NewMultiaddr(addr)
		if err != nil {
			panic(err)
		}
	}
	api, _ := multiaddr.NewMultiaddr(apiAddr)
	gateway, _ := multiaddr.NewMultiaddr(gatewayAddr)
	return Config{
		Bootstrap:       bootstrap,
		Swarm:           swarm,
		HostKey:         priv,
		Api:             api,
		Gateway:         gateway,
		BloomFilterSize: BloomFilterSize,
		Blockstore:      blockstore,
		Rootstore:       rootStore,
	}
}

func writeConfigJSON(jsonRoot map[string]interface{}, filePath string) {
	bytes, err := json.MarshalIndent(jsonRoot, "", "    ")
	if err != nil {
		panic(err)
	}
	ioutil.WriteFile(filePath, bytes, 0600)
}

func saveConfig(config Config, filePath string) {
	pubBytes, _ := crypto.MarshalPublicKey(config.HostKey.GetPublic())
	mhash, _ := mh.Sum(pubBytes, mh.IDENTITY, -1)
	privBytes, _ := crypto.MarshalPrivateKey(config.HostKey)
	dstores := make([]interface{}, 2)
	for i, ds := range []DataStoreConfig{config.Blockstore, config.Rootstore} {
		params := ds.Params
		params["path"] = ds.Path
		params["type"] = ds.Type
		dstores[i] = map[string]interface{}{
			"mountpoint": ds.MountPoint,
			"prefix":     ds.Type + ".datastore",
			"type":       "measure",
			"child":      params,
		}
	}
	bootstrap := make([]multiaddr.Multiaddr, len(config.Bootstrap))
	for i, target := range config.Bootstrap {
		mas, err := peer.AddrInfoToP2pAddrs(&target)
		if err != nil {
			panic(err)
		}
		bootstrap[i] = mas[0]
	}
	var result = map[string]interface{}{
		"Addresses": map[string]interface{}{
			"API":     config.Api,
			"Gateway": config.Gateway,
			"Swarm":   config.Swarm,
		},
		"Bootstrap": bootstrap,
		"Identity": map[string]interface{}{
			"PeerID":  mhash.B58String(),
			"PrivKey": base64.StdEncoding.EncodeToString(privBytes),
		},
		"Datastore": map[string]interface{}{
			"BloomFilterSize": config.BloomFilterSize,
			"Spec": map[string]interface{}{
				"mounts": dstores,
				"type":   "mount",
			},
		},
	}
	writeConfigJSON(result, filePath)
}

func parseDataStoreConfig(json map[string]interface{}) DataStoreConfig {
	child := json["child"].(map[string]interface{})
	return DataStoreConfig{
		MountPoint: json["mountpoint"].(string),
		Type:       child["type"].(string),
		Path:       child["path"].(string),
		Params:     child,
	}
}

func parseFile(filePath string) map[string]interface{} {
	jsonFile, err := os.Open(filePath)

	if err != nil {
		panic(err)
	}
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	var result map[string]interface{}
	json.Unmarshal([]byte(byteValue), &result)
	return result
}

func ParseOrGenerateConfig(filePath string) Config {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		config := defaultConfig()
		saveConfig(config, filePath)
		return config
	}

	result := parseFile(filePath)

	privString := result["Identity"].(map[string]interface{})["PrivKey"].(string)
	privBytes, err := base64.StdEncoding.DecodeString(privString)
	if err != nil {
		panic(err)
	}
	priv, err := crypto.UnmarshalPrivateKey(privBytes)
	if err != nil {
		panic(err)
	}
	addresses := result["Addresses"].(map[string]interface{})
	swarmJson := addresses["Swarm"].([]interface{})
	swarm := make([]string, len(swarmJson))
	for i, v := range swarmJson {
		swarm[i] = v.(string)
	}

	bootstrapMas := result["Bootstrap"].([]interface{})
	bootstrap := make([]peer.AddrInfo, len(bootstrapMas))
	for i, addr := range bootstrapMas {
		targetMa, err := multiaddr.NewMultiaddr(addr.(string))
		if err != nil {
			panic(err)
		}
		target, err := peer.AddrInfoFromP2pAddr(targetMa)
		if err != nil {
			panic(err)
		}
		bootstrap[i] = *target
	}

	ds := result["Datastore"].(map[string]interface{})
	bloomFilterSize := int(ds["BloomFilterSize"].(float64))
	dsmounts := ds["Spec"].(map[string]interface{})["mounts"].([]interface{})
	blockstore := parseDataStoreConfig(dsmounts[0].(map[string]interface{}))
	rootstore := parseDataStoreConfig(dsmounts[1].(map[string]interface{}))
	return buildConfig(priv,
		bootstrap,
		swarm,
		addresses["API"].(string),
		addresses["Gateway"].(string),
		bloomFilterSize, blockstore, rootstore)
}

func setConfigField(field []string, value interface{}, json map[string]interface{}) {
	if len(field) == 1 {
		json[field[0]] = value
		return
	}
	subTree := json[field[0]]
	if subTree == nil {
		subTree = map[string]interface{}{}
		json[field[0]] = subTree
	}
	setConfigField(field[1:], value, json[field[0]].(map[string]interface{}))
}

func SetField(filePath string, fieldPath string, fieldVal string, isJSON bool) {
	var newVal interface{}
	if isJSON {
		json.Unmarshal([]byte(fieldVal), &newVal)
	} else {
		newVal = fieldVal
	}

	json := parseFile(filePath)
	path := strings.Split(fieldPath, ".")
	setConfigField(path, newVal, json)
	writeConfigJSON(json, filePath)
}
