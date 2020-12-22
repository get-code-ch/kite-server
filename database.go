package main

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"time"
)

func (ks *KiteServer) connectDatabase() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	uri := fmt.Sprintf("mongodb+srv://%s:%s@%s/%s?retryWrites=true&w=majority",
		ks.conf.DatabaseUsername,
		ks.conf.DatabasePassword,
		ks.conf.DatabaseServer,
		ks.conf.DatabaseName)
	if client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri)); err == nil {
		ks.dbClient = client
	} else {
		log.Printf("Error connecting database --> %s", err)
	}

}
