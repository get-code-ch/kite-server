package main

import (
	"context"
	"fmt"
	kite "github.com/get-code-ch/kite-common"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"net/http"
	"regexp"
	"sync"
	"time"
)

type KiteServer struct {
	upgrader websocket.Upgrader
	conn     *websocket.Conn
	ctx      context.Context
	db       *mongo.Database
	address  kite.EventNotifier
	conf     ServerConf
	tme      TmeConf
	srv      http.Server
	mux      *http.ServeMux
	wg       sync.WaitGroup
	sync     sync.Mutex
}

func (ks *KiteServer) sendPing(this *AddressObs) {
	defer ks.wg.Done()
	sentinel := make(chan bool)

	ticker := time.NewTicker(1 * time.Minute).C
	for {
		select {
		case <-ticker:
			ks.sync.Lock()
			if err := this.conn.WriteControl(websocket.PingMessage, []byte(fmt.Sprint(ks.conf.Address)), time.Now().Add(1*time.Second)); err != nil {
				log.Printf("Error pinging peer --> %v", err)
				ks.sync.Unlock()
				ks.address.Deregister(this)
				this.conn.Close()
				return
			}
			ks.sync.Unlock()
			break
		case <-sentinel:
			return
		}
	}
}

func (ks *KiteServer) waitMessage(this *AddressObs) {

	defer ks.wg.Done()

	for {
		message := kite.Message{}
		if err := this.conn.ReadJSON(&message); err == nil {
			if ks.conf.SetupMode {
				if message.Action == kite.A_SETUP {
					if err := ks.setupServer(message, this); err != nil {
						ks.address.Notify(kite.Event{Data: fmt.Sprintf("Error provisioning setup -> %s", err)}, this, message.Sender)
						log.Printf("Error provisioning setup from %s -> %s", message.Sender, err)
					} else {
						ks.address.Notify(kite.Event{Data: "Server setup successfully provisioned"}, this, message.Sender)
						log.Printf("Server setup successfully provisioned from %s", message.Sender)
					}
				} else {
					ks.address.Notify(kite.Event{Data: fmt.Sprintf("%s action rejected in setup mode", message.Action)}, this, message.Sender)
					log.Printf("%s action ignored in setup mode", message.Action)
				}
			} else {
				switch message.Action {
				case kite.A_LOG:
					log.Printf("Log message from %s : %s", message.Sender, message.Data.(string))
					ks.writeLog(message.Data.(string), message.Sender)
					break
				case kite.A_VALUE:
					ks.address.Notify(kite.Event{Data: message.Data, Action: message.Action}, this, message.Receiver)
					if ks.conf.Address.Match(message.Receiver) {
						log.Printf("%s Action received -> %v from %s to %s\n", message.Action, message.Data, message.Sender, message.Receiver)
					}
				case kite.A_READLOG:
					if logs := ks.readLog(message.Data.(string)); logs != nil {
						ks.address.Notify(kite.Event{Data: logs, Action: kite.A_LOG}, this, message.Sender)
					}
					break
				case kite.A_DISCOVER:
					if endpoints := ks.discover(); endpoints != nil {
						ks.address.Notify(kite.Event{Data: endpoints, Action: kite.A_INFORM}, this, message.Sender)
					}
				case kite.A_SETUP:
					if err := ks.setupServer(message, this); err != nil {
						ks.address.Notify(kite.Event{Data: fmt.Sprintf("Error provisioning setup -> %s", err)}, this, message.Sender)
						log.Printf("Error provisioning setup from %s -> %s", message.Sender, err)
					} else {
						ks.address.Notify(kite.Event{Data: "Server setup successfully provisioned"}, this, message.Sender)
						log.Printf("Server setup successfully provisioned from %s", message.Sender)
					}
					break
				case kite.A_ACTIVATE:
					if err := ks.activateAddress(message.Data.(string)); err == nil {
						log.Printf("New address activated")
					}
					break
				case kite.A_IMPORT:
					if err := ks.importDB(message.Data.(string)); err == nil {
						log.Printf("Configuration imported")
					}
					break
				case kite.A_EXPORT:
					if export := ks.exportDB(); export != nil {
						ks.address.Notify(kite.Event{Data: export, Action: kite.A_EXPORT}, this, message.Sender)
					}
					break
				default:
					if message.Receiver.Domain == "telegram" {
						ks.sendToTelegram(message.Data.(string))
					} else {
						ks.address.Notify(kite.Event{Data: message.Data.(string), Action: message.Action}, this, message.Receiver)
						if ks.conf.Address.Match(message.Receiver) {
							log.Printf("%s Action received -> %s from %s to %s\n", message.Action, message.Data.(string), message.Sender, message.Receiver)
						}
					}
				}
			}

		} else {
			log.Printf("Error receiving message --> %v", err)
			ks.address.Deregister(this)
			this.conn.Close()
			return
		}
	}

}

func (ks *KiteServer) wsHandler(w http.ResponseWriter, r *http.Request) {

	ks.wg.Add(1)
	defer ks.wg.Done()

	// Configuring check CORS (in production mode CORS should be on)
	ks.upgrader.CheckOrigin = func(r *http.Request) bool {
		re := regexp.MustCompile(`(?i)(?:(?:http|ws)[s]?://)([^/]*)`)
		rHost := re.FindStringSubmatch(r.Header.Get("origin"))
		if len(rHost) != 2 {
			return false
		}
		if ks.conf.CheckOrigin {
			if ks.conf.Server+":"+ks.conf.Port == rHost[1] {
				return true
			} else {
				return false
			}
		} else {
			return true
		}
	}

	// Starting websocket connection
	header := http.Header{}
	conn, err := ks.upgrader.Upgrade(w, r, header)
	if err != nil {
		log.Printf("ERROR handle serveWs --> %v", err)
		return
	}

	// Adding new address
	this, err := NewAddressObs(conn, ks)
	if err != nil {
		log.Printf("address creation error --> %v", err)
		conn.WriteControl(websocket.CloseMessage, []byte(""), time.Now().Add(10*time.Second))
		conn.Close()
		return
	}

	conn.SetCloseHandler(func(code int, text string) error {
		ks.address.Deregister(this)
		return nil
	})

	ks.sync.Lock()
	ks.address.Register(this)
	ks.sync.Unlock()

	// If client is of type Iot we provisioning configuration of it
	if this.address.Type == kite.H_IOT {
		ks.iotProvisioning(this)
	}

	// Sending a keep alive ping every minute
	ks.wg.Add(1)
	go ks.sendPing(this)
	// Waiting message/request
	ks.wg.Add(1)
	go ks.waitMessage(this)

}

func (ks *KiteServer) startServer() {

	defer ks.wg.Done()

	// Configuring websocket upgrader and handler function
	ks.srv = http.Server{Addr: fmt.Sprintf("%s:%s", ks.conf.Server, ks.conf.Port), Handler: ks.mux}

	ks.upgrader = websocket.Upgrader{ReadBufferSize: 2048, WriteBufferSize: 2048, CheckOrigin: func(r *http.Request) bool { return false }}

	// Starting server (normally https in production mode)
	if ks.conf.Ssl {
		if err := ks.srv.ListenAndServeTLS(ks.conf.Cert.SslCert, ks.conf.Cert.SslKey); err != nil {
			log.Printf("Ending listening server -> %v", err)
		}
	} else {
		if err := ks.srv.ListenAndServe(); err != nil {
			log.Printf("Ending listening server -> %v", err)
		}
	}
}

func main() {
	ks := new(KiteServer)

	ks.address = kite.EventNotifier{
		Observers: map[kite.Observer]struct{}{},
	}

	// Loading configuration from configuration file
	configFile := ""
	conf := loadConfig(configFile)
	ks.conf = *conf

	// Initializing http server
	ks.mux = http.NewServeMux()

	ks.mux.HandleFunc("/ws", ks.wsHandler)
	ks.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "<h1>kite server is running...</h1>")
	})

	ks.ctx = context.Background()

	if !ks.conf.SetupMode {
		ks.configureTelegram()
		ks.connectDatabase()
	}

	// Starting to listen and waiting connection
	ks.wg.Add(1)
	go ks.startServer()

	// Waiting end condition
	log.Printf("kite server %s listening on port %s\n", conf.Server, conf.Port)
	ks.sendToTelegram(fmt.Sprintf("Server %s is listening on port %s...", ks.conf.Address, ks.conf.Port))
	ks.wg.Wait()
	ks.sendToTelegram(fmt.Sprintf("Server %s is stopped...", ks.conf.Address))
}
