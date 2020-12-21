package main

import (
	r "gopkg.in/rethinkdb/rethinkdb-go.v6"
	"log"
)

func (ks *KiteServer) connectDatabase() {

	opts := r.ConnectOpts{Address: ks.conf.DatabaseServer, Database: ks.conf.DatabaseName}

	if session, err := r.Connect(opts); err == nil {
		ks.session = session
	} else {
		ks.session = nil
		log.Printf("Error connecting rethink database --> %v", err)
	}
}
