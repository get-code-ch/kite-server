package main

import (
	"encoding/json"
	"errors"
	"fmt"
	kite "github.com/get-code-ch/kite-common"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"sync"
	"time"
)

type (
	AddressObs struct {
		kite.Observer
		address kite.Address
		conn    *websocket.Conn
		sync    sync.Mutex
	}
)

// NewAddressObs function check and configure new address
func NewAddressObs(conn *websocket.Conn, ks *KiteServer) (*AddressObs, error) {
	var msg kite.Message

	o := &AddressObs{}
	o.conn = conn

	// Setting max delay to receive a new registration message
	_ = o.conn.SetReadDeadline(time.Now().Add(1 * time.Minute))
	defer o.conn.SetReadDeadline(time.Time{})

	o.sync.Lock()
	defer o.sync.Unlock()

	// Get address registration
	if err := o.conn.ReadJSON(&msg); err == nil {
		// at this point client is not registered, we accept only register action message
		if msg.Action != kite.A_REGISTER {
			data := make(map[string]string)
			data["Message"] = "invalid action, must be register"
			if closeMessage, err := json.Marshal(kite.Message{Sender: ks.conf.Address, Receiver: o.address, Action: kite.A_REJECTED, Data: data}); err != nil {
				_ = o.conn.WriteControl(websocket.CloseMessage, closeMessage, time.Now().Add(10*time.Second))
			} else {
				_ = o.conn.Close()
			}
			return nil, errors.New("address registration invalid message")
		} else {
			if msg.Sender.Address == ks.conf.Address.Domain {
				return nil, errors.New("missing or wrong domain in address registration message")
			}
			// Configuring address information
			o.address.Id = msg.Sender.Id
			o.address.Address = msg.Sender.Address
			o.address.Host = msg.Sender.Host
			o.address.Type = msg.Sender.Type
			o.address.Domain = msg.Sender.Domain
			if o.address.Id == "" {
				o.address.Id = "*"
			}
			if o.address.Address == "" {
				o.address.Address = "*"
			}
			if o.address.Host == "" {
				o.address.Host = "*"
			}
			if o.address.Type == "" {
				o.address.Type = "*"
			}

			// Checking if address is authorized (api key and enabled)
			if !ks.conf.SetupMode {
				authorized := false

				if addressAuth, err := ks.findAddressAuth(o.address.String()); err == nil {
					authorized = msg.Data.(string) == addressAuth.ApiKey && addressAuth.Enabled
				} else {
					if err == mongo.ErrNoDocuments && len(msg.Data.(string)) > 10 {
						addressAuth = kite.AddressAuth{}
						addressAuth.Enabled = false

						// If host type is endpoint we creating authorization for host only
						if o.address.Type == kite.H_ENDPOINT {
							o.address.Address = "*"
							o.address.Id = "*"
						}

						addressAuth.Name = o.address.String()
						addressAuth.ApiKey = msg.Data.(string)
						addressAuth.ActivationCode = kite.RandomString(6)
						if err := ks.upsertAddressAuth(addressAuth); err == nil {
							data := make(map[string]string)
							data["Message"] = fmt.Sprintf("new address %s try to connect server, activation code %s", addressAuth.Name, addressAuth.ActivationCode)
							ks.sendToTelegram(data["Message"])
							ks.address.Notify(kite.Event{Data: data["Message"]}, new(AddressObs), kite.Address{Domain: ks.conf.Address.Domain, Type: "*", Host: "*", Address: "*", Id: "*"})
							return nil, errors.New(data["Message"])
						}
					}
				}

				if !authorized {
					data := make(map[string]string)
					data["Message"] = "unauthorized address connection"
					if closeMessage, err := json.Marshal(kite.Message{Sender: ks.conf.Address, Receiver: o.address, Action: kite.A_REJECTED, Data: data}); err != nil {
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
			data["Message"] = "welcome " + o.address.String()
		}
		if err := o.conn.WriteJSON(kite.Message{Sender: ks.conf.Address, Receiver: o.address, Action: kite.A_ACCEPTED, Data: data}); err != nil {
			err = errors.New("error accepting client " + err.Error())
			_ = o.conn.Close()
			return nil, err
		}
		// Everything is Ok, we return observable object
		return o, nil
	} else {
		_ = o.conn.Close()
		return nil, err
	}
}

func (o *AddressObs) OnNotify(e kite.Event, sender kite.Observer, receiver kite.Address) {
	if o.address.Match(receiver) {
		msg := kite.Message{Data: e.Data, Action: e.Action, Sender: sender.(*AddressObs).address, Receiver: receiver}

		o.sync.Lock()
		defer o.sync.Unlock()
		if err := o.conn.WriteJSON(msg); err != nil {
			log.Printf("Error sending message to %s", receiver)
		}
	}
}

//goland:noinspection GoUnusedParameter
func (o *AddressObs) OnClose(e kite.Event) {
	o.sync.Lock()
	defer o.sync.Unlock()
	if err := o.conn.WriteControl(websocket.CloseMessage, []byte{}, time.Now().Add(1*time.Second)); err != nil {
		log.Printf("Error closing connection --> %v", err)
	}
}

//func (o *AddressObs) OnBroadcast(e kite.Event) {
//	log.Printf("OnBroadcast not yet implemented, message to send --> %v", e.Data)
//}

func (o *AddressObs) Key() interface{} {
	return o.address
}
