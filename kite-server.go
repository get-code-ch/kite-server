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
	endpoint kite.EventNotifier
	conf     ServerConf
	tme      TmeConf
	srv      http.Server
	mux      *http.ServeMux
	wg       sync.WaitGroup
	lock     sync.Mutex
}

func (ks *KiteServer) sendPing(this *EndpointObs) {
	defer ks.wg.Done()
	sentinel := make(chan bool)

	ticker := time.NewTicker(1 * time.Minute).C
	for {
		select {
		case <-ticker:
			if err := this.conn.WriteControl(websocket.PingMessage, []byte(fmt.Sprint(ks.conf.Endpoint)), time.Now().Add(1*time.Second)); err != nil {
				log.Printf("Error pinging peer --> %v", err)
				ks.endpoint.Deregister(this)
				this.conn.Close()
				return
			}
			break
		case <-sentinel:
			return
		}
	}
}

func (ks *KiteServer) waitMessage(this *EndpointObs) {

	defer ks.wg.Done()

	for {
		msg := kite.Message{}
		if err := this.conn.ReadJSON(&msg); err == nil {
			if ks.conf.SetupMode {
				if msg.Action == kite.A_SETUP {
					if err := ks.setupServer(msg, this); err != nil {
						ks.endpoint.Notify(kite.Event{Data: fmt.Sprintf("Error provisioning setup -> %s", err)}, this, msg.Sender)
						log.Printf("Error provisioning setup from %s -> %s", msg.Sender, err)
					} else {
						ks.endpoint.Notify(kite.Event{Data: "Server setup successfully provisioned"}, this, msg.Sender)
						log.Printf("Server setup successfully provisioned from %s", msg.Sender)
					}
				} else {
					ks.endpoint.Notify(kite.Event{Data: fmt.Sprintf("%s action rejected in setup mode", msg.Action)}, this, msg.Sender)
					log.Printf("%s action ignored in setup mode", msg.Action)
				}
			} else {
				switch msg.Action {
				case kite.A_LOG:
					log.Printf("Log message from %s : %s", msg.Sender, msg.Data.(string))
					ks.writeLog(msg.Data.(string), this.endpoint)
					break
				case kite.A_READLOG:
					if logs := ks.readLog(msg.Data.(string)); logs != nil {
						ks.endpoint.Notify(kite.Event{Data: logs, Action: kite.A_LOG}, this, msg.Sender)
					}
					break
				case kite.A_SETUP:
					if err := ks.setupServer(msg, this); err != nil {
						ks.endpoint.Notify(kite.Event{Data: fmt.Sprintf("Error provisioning setup -> %s", err)}, this, msg.Sender)
						log.Printf("Error provisioning setup from %s -> %s", msg.Sender, err)
					} else {
						ks.endpoint.Notify(kite.Event{Data: "Server setup successfully provisioned"}, this, msg.Sender)
						log.Printf("Server setup successfully provisioned from %s", msg.Sender)
					}
					break
				default:
					ks.endpoint.Notify(kite.Event{Data: msg.Data.(string)}, this, msg.Receiver)
					if ks.conf.Endpoint.Match(msg.Receiver) {
						log.Printf("%s Action received -> %s from %s to %s\n", msg.Action, msg.Data.(string), msg.Sender, msg.Receiver)
					}
				}
			}

		} else {
			log.Printf("Error receiving message --> %v", err)
			ks.endpoint.Deregister(this)
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
		re := regexp.MustCompile(`(?i)(?:[http|ws][s]?:\/\/)([^/]*)`)
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

	// Adding new endpoint
	this, err := NewEndpointObs(conn, ks)
	if err != nil {
		log.Printf("Endpoint creation error --> %v", err)
		return
	}

	conn.SetCloseHandler(func(code int, text string) error {
		ks.endpoint.Deregister(this)
		return nil
	})

	ks.endpoint.Register(this)

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
	var cancel context.CancelFunc
	ks := new(KiteServer)

	ks.endpoint = kite.EventNotifier{
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

	ks.ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ks.configureTelegram()

	if !ks.conf.SetupMode {
		ks.connectDatabase()
	}

	// Starting to listen and waiting connection
	ks.wg.Add(1)
	go ks.startServer()

	// Waiting end condition
	log.Printf("kite server %s listening on port %s\n", conf.Server, conf.Port)
	ks.sendToTelegram(fmt.Sprintf("Server %s is listening on port %s...", ks.conf.Endpoint, ks.conf.Port))
	ks.wg.Wait()
	ks.sendToTelegram(fmt.Sprintf("Server %s is stopped...", ks.conf.Endpoint))
}
