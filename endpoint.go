package main

import (
	"encoding/json"
	"errors"
	kite "github.com/get-code-ch/kite-common"
	"github.com/gorilla/websocket"
	"log"
	"sync"
	"time"
)

type (
	EndpointObs struct {
		kite.Observer
		endpoint kite.Endpoint
		conn     *websocket.Conn
		sync     sync.Mutex
	}
)

// NewEndpointObs function check and configure new endpoint
func NewEndpointObs(conn *websocket.Conn, ks *KiteServer) (*EndpointObs, error) {
	var msg kite.Message

	o := &EndpointObs{}
	o.conn = conn

	// Setting max delay to receive a new registration message
	_ = o.conn.SetReadDeadline(time.Now().Add(1 * time.Minute))
	defer o.conn.SetReadDeadline(time.Time{})

	o.sync.Lock()
	defer o.sync.Unlock()

	// Get endpoint registration
	if err := o.conn.ReadJSON(&msg); err == nil {
		// at this point client is not registered, we accept only register action message
		if msg.Action != kite.REGISTER {
			data := make(map[string]string)
			data["Message"] = "invalid action, must be register"
			if closeMessage, err := json.Marshal(kite.Message{Sender: ks.conf.Endpoint, Receiver: o.endpoint, Action: kite.REJECTED, Data: data}); err != nil {
				_ = o.conn.WriteControl(websocket.CloseMessage, closeMessage, time.Now().Add(10*time.Second))
			} else {
				_ = o.conn.Close()
			}
			return nil, errors.New("endpoint registration invalid message")
		} else {
			// Configuring endpoint information
			o.endpoint.Id = msg.Sender.Id
			o.endpoint.Address = msg.Sender.Address
			o.endpoint.Host = msg.Sender.Host
			o.endpoint.Type = msg.Sender.Type
			o.endpoint.Domain = msg.Sender.Domain
			if o.endpoint.Id == "" {
				o.endpoint.Id = "*"
			}
			if o.endpoint.Address == "" {
				o.endpoint.Address = "*"
			}
			if o.endpoint.Host == "" {
				o.endpoint.Host = "*"
			}
			if o.endpoint.Type == "" {
				o.endpoint.Type = "*"
			}
			if o.endpoint.Domain == "" {
				o.endpoint.Domain = "*"
			}
			// Checking if endpoint is authorized (api key and enabled)
			if !ks.conf.SetupMode {
				authorized := false
				for _, e := range ks.conf.AuthorizedEndpoints {
					if o.endpoint.String() == e.Endpoint.String() && e.Enabled && msg.Data == e.ApiKey {
						authorized = true
						break
					}
				}
				if !authorized {
					data := make(map[string]string)
					data["Message"] = "unauthorized endpoint connection"
					if closeMessage, err := json.Marshal(kite.Message{Sender: ks.conf.Endpoint, Receiver: o.endpoint, Action: kite.REJECTED, Data: data}); err != nil {
						_ = o.conn.WriteControl(websocket.CloseMessage, closeMessage, time.Now().Add(10*time.Second))
					} else {
						_ = o.conn.Close()
					}
					return nil, errors.New(data["Message"])
				}
			}
		}
		// If everything is Ok, sending accept message
		data := make(map[string]string)
		if ks.conf.SetupMode {
			data["Message"] = "setup mode"
		} else {
			data["Message"] = "welcome " + o.endpoint.String()
		}
		if err := o.conn.WriteJSON(kite.Message{Sender: ks.conf.Endpoint, Receiver: o.endpoint, Action: kite.ACCEPTED, Data: data}); err != nil {
			err = errors.New("error accepting client " + err.Error())
			_ = o.conn.Close()
			return nil, err
		}

		return o, nil
	} else {
		_ = o.conn.Close()
		return nil, err
	}
}

func (o *EndpointObs) OnNotify(e kite.Event, sender kite.Observer, receiver kite.Endpoint) {

	if o.endpoint.Match(receiver) {
		msg := kite.Message{Data: e.Data, Sender: sender.(*EndpointObs).endpoint, Receiver: receiver}

		o.sync.Lock()
		defer o.sync.Unlock()
		if err := o.conn.WriteJSON(msg); err != nil {
			log.Printf("Error sending message to %s", receiver)
		}
	}
}

//goland:noinspection GoUnusedParameter
func (o *EndpointObs) OnClose(e kite.Event) {
	if err := o.conn.WriteControl(websocket.CloseMessage, []byte{}, time.Now().Add(1*time.Second)); err != nil {
		log.Printf("Error closing connection --> %v",err)
	}
}

func (o *EndpointObs) OnBroadcast(e kite.Event) {
	log.Printf("OnBroadcast not yet implemented, message to send --> %v", e.Data)
}

func (o *EndpointObs) Key() interface{} {
	return o.endpoint
}
