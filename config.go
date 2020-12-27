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
	ApiKey           string          `json:"api_key"`
	Server           string          `json:"server"`
	Port             string          `json:"port"`
	CheckOrigin      bool            `json:"check_origin"`
	Ssl              bool            `json:"ssl"`
	Cert             ConfCertificate `json:"cert,omitempty"`
	TelegramConf     string          `json:"telegram_conf"`
	Address          kite.Address    `json:"address"`
	SetupMode        bool            `json:"setup_mode"`
	DatabaseServer   string          `json:"database_server"`
	DatabaseName     string          `json:"database_name"`
	DatabaseUsername string          `json:"database_username"`
	DatabasePassword string          `json:"database_password"`
}

type ConfCertificate struct {
	SslKey  string `json:"ssl_key"`
	SslCert string `json:"ssl_cert,"`
}

const defaultConfigFile = "./config/default.json"
const setupConfigFile = "./config/setup.json"

func loadConfig(configFile string) *ServerConf {

	// New config creation
	c := new(ServerConf)

	// If no config file is provided we use "hardcoded" default filepath
	if len(os.Args) >= 2 && configFile == "" {
		configFile = os.Args[1]
	}

	if configFile == "" {
		configFile = defaultConfigFile
	}

	// Testing if config file exist if not, loading setup file and if not exist, return a fatal error
	if _, err := os.Stat(configFile); err != nil {
		if os.IsNotExist(err) {
			if _, err := os.Stat(setupConfigFile); err != nil {
				if os.IsNotExist(err) {
					log.Panic(fmt.Sprintf("config and setup files not exists\n"))
				}
			} else {
				configFile = setupConfigFile
			}
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
