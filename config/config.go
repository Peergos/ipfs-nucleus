package config

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"os"

	crypto "github.com/libp2p/go-libp2p-core/crypto"
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
	Swarm           []multiaddr.Multiaddr
	HostKey         crypto.PrivKey
	Api             multiaddr.Multiaddr
	Gateway         multiaddr.Multiaddr
	BloomFilterSize int
	Blockstore      DataStoreConfig
	Rootstore       DataStoreConfig
}

func defaultConfig() Config {
	priv, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	if err != nil {
		panic(err)
	}
	return buildConfig(priv,
		[]string{"/ip4/0.0.0.0/tcp/4001", "/ip6/::/tcp/4001"},
		"/ip4/127.0.0.1/tcp/5001",
		"/ip4/127.0.0.1/tcp/8080",
		0,
		DataStoreConfig{Type: "flatfs", MountPoint: "/blocks", Path: "blocks",
			Params: map[string]interface{}{"sync": true, "shardFunc": "/repo/flatfs/shard/v1/next-to-last/2"}},
		DataStoreConfig{Type: "levelds", MountPoint: "/", Path: "datastore", Params: map[string]interface{}{"compression": "none"}})
}

func buildConfig(priv crypto.PrivKey,
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
		Swarm:           swarm,
		HostKey:         priv,
		Api:             api,
		Gateway:         gateway,
		BloomFilterSize: BloomFilterSize,
		Blockstore:      blockstore,
		Rootstore:       rootStore,
	}
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
	var result = map[string]interface{}{
		"Addresses": map[string]interface{}{
			"API":     config.Api,
			"Gateway": config.Gateway,
			"Swarm":   config.Swarm,
		},
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
	bytes, err := json.MarshalIndent(&result, "", "    ")
	if err != nil {
		panic(err)
	}
	ioutil.WriteFile(filePath, bytes, 0600)
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

func ParseOrGenerateConfig(filePath string) Config {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		config := defaultConfig()
		saveConfig(config, filePath)
		return config
	}
	jsonFile, err := os.Open(filePath)

	if err != nil {
		panic(err)
	}
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	var result map[string]interface{}
	json.Unmarshal([]byte(byteValue), &result)

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

	ds := result["Datastore"].(map[string]interface{})
	bloomFilterSize := int(ds["BloomFilterSize"].(float64))
	dsmounts := ds["Spec"].(map[string]interface{})["mounts"].([]interface{})
	blockstore := parseDataStoreConfig(dsmounts[0].(map[string]interface{}))
	rootstore := parseDataStoreConfig(dsmounts[1].(map[string]interface{}))
	return buildConfig(priv,
		swarm,
		addresses["API"].(string),
		addresses["Gateway"].(string),
		bloomFilterSize, blockstore, rootstore)
}
