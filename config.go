package main

import (
	"encoding/json"
	"fmt"
	kite "github.com/get-code-ch/kite-common"
	"io/ioutil"
	"log"
	"os"
)

type ServerConf struct {
	ApiKey              string               `json:"api_key"`
	Server              string               `json:"server"`
	Port                string               `json:"port"`
	CheckOrigin         bool                 `json:"check_origin"`
	Ssl                 bool                 `json:"ssl"`
	Cert                ConfCertificate      `json:"cert,omitempty"`
	TelegramConf        string               `json:"telegram_conf"`
	AuthorizedEndpoints []AuthorizedEndpoint `json:"authorized_endpoints"`
	Endpoint            kite.Endpoint        `json:"endpoint"`
}

type AuthorizedEndpoint struct {
	ApiKey   string        `json:"api_key"`
	Name     string        `json:"name"`
	MacAddr  string        `json:"mac_addr"`
	Enabled  bool          `json:"enabled"`
	Endpoint kite.Endpoint `json:"endpoint"`
}

type ConfCertificate struct {
	SslKey  string `json:"ssl_key"`
	SslCert string `json:"ssl_cert,"`
}

const defaultConfigFile = "./config/default.json"

func loadConfig(configFile string) *ServerConf {

	// New config creation
	c := new(ServerConf)

	// If no config file is provided we use "hardcoded" default filepath
	if configFile == "" {
		configFile = defaultConfigFile
	}

	// Testing if config file exist if not, return a fatal error
	_, err := os.Stat(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Panic(fmt.Sprintf("Config file %s not exist\n", configFile))
		} else {
			log.Panic(fmt.Sprintf("Something wrong with config file %s -> %v\n", configFile, err))
		}
	}

	// Reading and parsing configuration file
	if buffer, err := ioutil.ReadFile(configFile); err != nil {
		log.Printf(fmt.Sprintf("Error reading config file --> %v", err))
		return nil
	} else {
		if err := json.Unmarshal(buffer, c); err != nil {
			log.Printf(fmt.Sprintf("Error parsing config file --> %v", err))
			return nil
		}
		return c
	}
}

// TODO Create Stringer interface to return human readable config content
func (c *ServerConf) String() string {
	if jsonConf, err := json.Marshal(c); err == nil {
		return fmt.Sprintf("Server configuration -> %s", jsonConf)
	} else {
		return fmt.Sprintf("Error jsonify configuration -> %v", err)
	}
}
