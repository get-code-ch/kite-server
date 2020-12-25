package main

import (
	"encoding/json"
	"errors"
	"fmt"
	kite "github.com/get-code-ch/kite-common"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

// function setupServer get setting from client (browser or cli tools)
func (ks *KiteServer) setupServer(msg kite.Message, this *EndpointObs) error {

	var endpoints []kite.EndpointAuth

	data := kite.SetupMessage{}
	data = data.SetFromInterface(msg.Data)

	// we accept only setting up if Apikey is correctly configured
	if data.ApiKey != ks.conf.ApiKey {
		ks.endpoint.Notify(kite.Event{Data: fmt.Sprintf("Sorry, you are not authorized to setup server...")}, this, msg.Sender)
		return errors.New("invalid ApiKey")
	}

	// Importing and saving configuration files
	for _, file := range data.SetupFiles {
		// Special processing for endpoints
		if file.Path == "++endpoints++" {
			if err := json.Unmarshal(file.Content, &endpoints); err != nil {
				log.Printf("Error parsing endpoints --> %s", err)
			}
			continue
		}
		folder := filepath.Dir(file.Path)
		if _, err := os.Stat(folder); err != nil {
			if os.IsNotExist(err) {
				os.Mkdir(folder, os.ModeDir)
			}
		}
		ioutil.WriteFile(file.Path, file.Content, 0744)
	}

	// Sending restart notification to all clients
	ks.endpoint.Notify(kite.Event{Data: fmt.Sprintf("Server is provisioned and is restarting...")}, this, kite.Endpoint{Domain: "*", Type: "*", Host: "*", Address: "*", Id: "*"})
	ks.endpoint.Close(kite.Event{Data: "Setup done"})
	ks.srv.Shutdown(ks.ctx)

	// Reloading configuration
	ks.conf = *loadConfig("")
	ks.connectDatabase()
	ks.configureTelegram()

	// loading endpoint in database
	for _, endpoint := range endpoints {
		ks.upsertEndpointAuth(endpoint)
	}

	ks.wg.Add(1)
	ks.startServer()
	ks.sendToTelegram("Server is provisioned and is restarting...")

	return nil
}
