package main

import (
	"errors"
	"fmt"
	kite "github.com/get-code-ch/kite-common"
	"io/ioutil"
	"os"
	"path/filepath"
)

// function setupServer get setting from client (browser or cli tools)
func (ks *KiteServer) setupServer(msg kite.Message, this *AddressObs) error {

	data := kite.SetupMessage{}
	data = data.SetFromInterface(msg.Data)

	// we accept only setting up if Apikey is correctly configured
	if data.ApiKey != ks.conf.ApiKey {
		ks.address.Notify(kite.Event{Data: fmt.Sprintf("Sorry, you are not authorized to setup server...")}, this, msg.Sender)
		return errors.New("invalid ApiKey")
	}

	// Importing and saving configuration files
	for _, file := range data.SetupFiles {
		folder := filepath.Dir(file.Path)
		if _, err := os.Stat(folder); err != nil {
			if os.IsNotExist(err) {
				os.Mkdir(folder, os.ModeDir)
			}
		}
		ioutil.WriteFile(file.Path, file.Content, 0744)
	}

	// Sending restart notification to all clients
	ks.address.Notify(kite.Event{Data: fmt.Sprintf("Server is provisioned and is restarting...")}, this, kite.Address{Domain: "*", Type: "*", Host: "*", Address: "*", Id: "*"})
	ks.address.Close(kite.Event{Data: "Setup done"})
	ks.srv.Shutdown(ks.ctx)

	// Reloading configuration
	ks.conf = *loadConfig("")
	ks.connectDatabase()
	ks.configureTelegram()


	ks.wg.Add(1)
	ks.startServer()
	ks.sendToTelegram("Server is provisioned and is restarting...")

	return nil
}
