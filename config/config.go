package config

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"os"
	"runtime"
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

type ConnectionManager struct {
	Type        string
	LowWater    int
	HighWater   int
	GracePeriod string
}

type Metrics struct {
	Enabled bool
	Address string
	Port    int
}

type Config struct {
	Bootstrap       []peer.AddrInfo
	Swarm           []multiaddr.Multiaddr
	HostKey         crypto.PrivKey
	Api             multiaddr.Multiaddr
	Gateway         multiaddr.Multiaddr
	ProxyTarget     multiaddr.Multiaddr
	BloomFilterSize int
	Blockstore      DataStoreConfig
	Rootstore       DataStoreConfig
	AllowTarget     string
	ConnectionMgr   ConnectionManager
	CpuCount        int
	Metrics         Metrics
}

func defaultBootstrapPeers() []peer.AddrInfo {
	defaults, _ := ipfsconfig.DefaultBootstrapPeers()
	return defaults
}

func defaultConnectionManager() ConnectionManager {
	return ConnectionManager{Type: "basic", LowWater: 10, HighWater: 20, GracePeriod: "30s"}
}

func defaultConfig() Config {
	priv, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	if err != nil {
		panic(err)
	}

	cpuCount := defaultCpuCount()
	return buildConfig(priv,
		defaultBootstrapPeers(),
		[]string{"/ip4/0.0.0.0/tcp/4001", "/ip6/::/tcp/4001"},
		"/ip4/127.0.0.1/tcp/5001",
		"/ip4/127.0.0.1/tcp/8080",
		"/ip4/127.0.0.1/tcp/8000",
		0,
		DataStoreConfig{Type: "flatfs", MountPoint: "/blocks", Path: "blocks",
			Params: map[string]interface{}{"sync": true, "shardFunc": "/repo/flatfs/shard/v1/next-to-last/2"}},
		DataStoreConfig{Type: "levelds", MountPoint: "/", Path: "datastore", Params: map[string]interface{}{"compression": "none"}},
		"http://localhost:8000",
		cpuCount,
		Metrics{Enabled: false},
		defaultConnectionManager())
}

func defaultCpuCount() int {
	// we want to leave cpu time for peergos to run:
	// (host cpus, ipfs-nucelus cpus) in
	// (1, 1), (2, 1), (3, 1), (4, 1), (5, 2), (6, 2), (7, 2), (8, 2)
	if runtime.NumCPU() < 5 {
		return 1
	} else if runtime.NumCPU() < 9 {
		return 2
	} else if runtime.NumCPU() < 13 {
		return 3
	} else {
		return 4
	}
}

func buildConfig(priv crypto.PrivKey,
	bootstrap []peer.AddrInfo,
	swarmAddrs []string,
	apiAddr string,
	gatewayAddr string,
	proxyTargetAddr string,
	BloomFilterSize int,
	blockstore DataStoreConfig,
	rootStore DataStoreConfig,
	allowTarget string,
	cpuCount int,
	metrics Metrics,
	connMgr ConnectionManager) Config {
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
	proxyTarget, _ := multiaddr.NewMultiaddr(proxyTargetAddr)
	return Config{
		Bootstrap:       bootstrap,
		Swarm:           swarm,
		HostKey:         priv,
		Api:             api,
		Gateway:         gateway,
		ProxyTarget:     proxyTarget,
		BloomFilterSize: BloomFilterSize,
		Blockstore:      blockstore,
		Rootstore:       rootStore,
		AllowTarget:     allowTarget,
		CpuCount:        cpuCount,
		ConnectionMgr:   connMgr,
		Metrics:         metrics,
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
			"API":         config.Api,
			"Gateway":     config.Gateway,
			"Swarm":       config.Swarm,
			"ProxyTarget": config.ProxyTarget,
			"AllowTarget": config.AllowTarget,
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
		"CpuCount": config.CpuCount,
		"Swarm": map[string]interface{}{
			"ConnMgr": map[string]interface{}{
				"Type":        config.ConnectionMgr.Type,
				"LowWater":    config.ConnectionMgr.LowWater,
				"HighWater":   config.ConnectionMgr.HighWater,
				"GracePeriod": config.ConnectionMgr.GracePeriod,
			},
		},
		"Metrics": map[string]interface{}{
			"Enabled": config.Metrics.Enabled,
			"Address": config.Metrics.Address,
			"Port":    config.Metrics.Port,
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
	var cpuCount int
	if ds["CpuCount"] != nil {
		cpuCount = int(ds["CpuCount"].(float64))
	} else {
		cpuCount = defaultCpuCount()
	}
	dsmounts := ds["Spec"].(map[string]interface{})["mounts"].([]interface{})
	blockstore := parseDataStoreConfig(dsmounts[0].(map[string]interface{}))
	rootstore := parseDataStoreConfig(dsmounts[1].(map[string]interface{}))
	connsJson := result["Swarm"]
	var conns map[string]interface{}
	if connsJson != nil {
		conns = connsJson.(map[string]interface{})
	} else {
		conns = nil
	}
	var cm map[string]interface{}
	if conns != nil {
		cm = conns["ConnMgr"].(map[string]interface{})
	} else {
		cm = nil
	}
	var connMgr ConnectionManager
	if cm != nil {
		connMgr = ConnectionManager{Type: cm["Type"].(string), LowWater: int(cm["LowWater"].(float64)),
			HighWater: int(cm["HighWater"].(float64)), GracePeriod: cm["GracePeriod"].(string)}
	} else {
		connMgr = defaultConnectionManager()
	}
	var metricsJson map[string]interface{}
	if result["Metrics"] != nil {
		metricsJson = result["Metrics"].(map[string]interface{})
	} else {
		metricsJson = nil
	}
	var metrics Metrics
	if metricsJson != nil {
		enabled := metricsJson["Enabled"].(bool)
		port := int(metricsJson["Port"].(float64))
		metrics = Metrics{Enabled: enabled, Address: metricsJson["Address"].(string), Port: port}
	}
	return buildConfig(priv,
		bootstrap,
		swarm,
		addresses["API"].(string),
		addresses["Gateway"].(string),
		addresses["ProxyTarget"].(string),
		bloomFilterSize,
		blockstore,
		rootstore,
		addresses["AllowTarget"].(string),
		cpuCount,
		metrics,
		connMgr)
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
