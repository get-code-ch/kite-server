package main

import (
	"flag"
	"fmt"
	kite "github.com/get-code-ch/kite-common"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"
)

type KiteServer struct {
	upgrader websocket.Upgrader
	conn     *websocket.Conn
	endpoint kite.EventNotifier
	conf     ServerConf
}

func main() {
	sentinel := make(chan bool)
	ks := new(KiteServer)

	ks.endpoint = kite.EventNotifier{
		Observers: map[kite.Observer]struct{}{},
	}

	// Loading configuration from configuration file
	configFile := ""
	if len(os.Args) >= 2 {
		configFile = os.Args[1]
	}
	conf := loadConfig(configFile)
	ks.conf = *conf
	fmt.Printf("%v\n", conf)

	// Starting to listen and waiting connection
	go func(sentinel chan bool) {
		// Configuring listening URL
		addr := flag.String("addr", fmt.Sprintf("%s:%s", conf.Server, conf.Port), "https service address")
		flag.Parse()

		// Configuring websocket upgrader and handler function
		ks.upgrader = websocket.Upgrader{ReadBufferSize: 2048, WriteBufferSize: 2048, CheckOrigin: func(r *http.Request) bool { return false }}
		http.HandleFunc("/ws", ks.wsHandler)

		// For all http(s) web request we display a minimal text message
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "<h1>kite server is running...</h1>")
		})

		// Starting server (normally https in production mode)
		if conf.Ssl {
			// If something wrong with certificate files
			err := http.ListenAndServeTLS(*addr, conf.Cert.SslCert, conf.Cert.SslKey, nil)
			if err != nil {
				log.Printf("ERROR starting server -> %v", err)
			}
		} else {
			http.ListenAndServe(*addr, nil)
		}
		close(sentinel)
	}(sentinel)

	// Waiting end condition
	log.Printf("kite server %s listening on port %s\n", conf.Server, conf.Port)
	<-sentinel
}

func (ks *KiteServer) wsHandler(w http.ResponseWriter, r *http.Request) {
	sentinel := make(chan bool)

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
		close(sentinel)
		return
	}

	ks.endpoint.Register(this)
	ks.endpoint.Broadcast(kite.Event{Data: fmt.Sprintf("Host %s connected", r.RemoteAddr)})

	// Sending a keep alive ping every minute
	ticker := time.NewTicker(1 * time.Minute).C
	go func(sentinel chan bool) {
		for {
			select {
			case <-ticker:
				if err := this.conn.WriteControl(websocket.PingMessage, []byte(fmt.Sprint(ks.conf.Endpoint)), time.Now().Add(1*time.Second)); err != nil {
					log.Printf("Error pinging peer --> %v", err)
					ks.endpoint.Deregister(this)
					ks.endpoint.Broadcast(kite.Event{Data: fmt.Sprintf("Host %s disconnected", r.RemoteAddr)})
					this.conn.Close()
					close(sentinel)
					return
				}
				break
			case <-sentinel:
				return
			}
		}
	}(sentinel)

	// Waiting message/request
	go func(sentinel chan bool) {

		for {
			msg := kite.Message{}
			if err := this.conn.ReadJSON(&msg); err == nil {

				switch msg.Action {
				case kite.NOTIFY:
					//log.Printf("NOTIFY received -> %s", msg.Data.(string))
					ks.endpoint.Notify(kite.Event{Data: msg.Data.(string)}, this, msg.Receiver)
				default:
					log.Printf("Messsage received with unhandle action --> %s", msg.Action)
				}
				ks.endpoint.Broadcast(kite.Event{Data: msg})
			} else {
				log.Printf("Error receiving message --> %v", err)
				ks.endpoint.Deregister(this)
				ks.endpoint.Broadcast(kite.Event{Data: fmt.Sprintf("Host %s disconnected", r.RemoteAddr)})
				this.conn.Close()
				close(sentinel)
				return
			}
		}
	}(sentinel)

	<-sentinel
}
