package config

import (
	"encoding/json"
	"encoding/base64"
	"io/ioutil"
	"os"

	crypto "github.com/libp2p/go-libp2p-core/crypto"
	mh "github.com/multiformats/go-multihash"
	"github.com/multiformats/go-multiaddr"
)

type Config struct {
	Swarm   []multiaddr.Multiaddr
	HostKey crypto.PrivKey
	Api     multiaddr.Multiaddr
	Gateway multiaddr.Multiaddr
}

func defaultConfig() Config {
	priv, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	if err != nil {
		panic(err)
	}
	return buildConfig(priv,
		[]string{"/ip4/0.0.0.0/tcp/4001", "/ip6/::/tcp/4001"},
		"/ip4/127.0.0.1/tcp/5001",
		"/ip4/127.0.0.1/tcp/8080")
}

func buildConfig(priv crypto.PrivKey, swarmAddrs []string, apiAddr string, gatewayAddr string) Config {
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
		Swarm:   swarm,
		HostKey: priv,
		Api:     api,
		Gateway: gateway,
	}
}

func saveConfig(config Config, filePath string) {
        pubBytes,_ := crypto.MarshalPublicKey(config.HostKey.GetPublic())
        mhash, _ := mh.Sum(pubBytes, mh.IDENTITY, -1)
	privBytes,_ := crypto.MarshalPrivateKey(config.HostKey)
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
	}
	bytes, err := json.MarshalIndent(&result, "", "    ")
	if err != nil {
		panic(err)
	}
	ioutil.WriteFile(filePath, bytes, 0600)
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
        privBytes,err := base64.StdEncoding.DecodeString(privString)
        if err != nil {
		panic(err)
	}
        priv,err := crypto.UnmarshalPrivateKey(privBytes)
	if err != nil {
		panic(err)
	}
	addresses := result["Addresses"].(map[string]interface{})
	swarmJson := addresses["Swarm"].([]interface{})
	swarm := make([]string, len(swarmJson))
	for i, v := range swarmJson {
		swarm[i] = v.(string)
	}

	return buildConfig(priv,
		swarm,
		addresses["API"].(string),
		addresses["Gateway"].(string))
}
