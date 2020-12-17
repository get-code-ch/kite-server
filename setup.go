package main

import (
	"context"
	"errors"
	"fmt"
	kite "github.com/get-code-ch/kite-common"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

func (ks *KiteServer) setupServer(msg kite.Message, this *EndpointObs) error {
	data := kite.SetupMessage{}
	data = data.SetFromInterface(msg.Data)

	log.Printf("Data %v\n", data.ApiKey)
	if data.ApiKey != ks.conf.ApiKey {
		return errors.New("invalid ApiKey")
	}

	for _, file := range data.SetupFiles {
		folder := filepath.Dir(file.Path)
		if _, err := os.Stat(folder); err != nil {
			if os.IsNotExist(err) {
				os.Mkdir(folder, os.ModeDir)
			}
		}
		ioutil.WriteFile(file.Path, file.Content, 0744)
	}
	ks.endpoint.Notify(kite.Event{Data: fmt.Sprintf("Server is provisionned restarting")}, this, msg.Sender)

	ks.endpoint.Close(kite.Event{Data: "Setup done"})
	ks.srv.Shutdown(context.Background())

	ks.conf = *loadConfig("")
	ks.loadTelegramConf()

	ks.wg.Add(1)
	ks.startServer()

	return nil
}
