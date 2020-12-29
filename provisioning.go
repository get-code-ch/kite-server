package main

import (
	kite "github.com/get-code-ch/kite-common"
	"log"
)

// iotProvisioning function sent endpoints configuration to newly connected iot
func (ks *KiteServer) iotProvisioning(this *AddressObs) {
	if endpoints, err := ks.findEndpoint(this.address); err == nil {
		//log.Printf("%v", endpoints)
		ks.sync.Lock()
		defer ks.sync.Unlock()
		if err := this.conn.WriteJSON(kite.Message{Sender: ks.conf.Address, Receiver: this.address, Action: kite.A_PROVISION, Data: endpoints}); err != nil {
			log.Printf("Error provisioning iot --> %v", err)
		}
	}

}
