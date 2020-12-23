package main

import (
	"context"
	"fmt"
	kite "github.com/get-code-ch/kite-common"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"net/url"
	"time"
)



func (ks *KiteServer) connectDatabase() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	uri := fmt.Sprintf("mongodb+srv://%s:%s@%s/%s?retryWrites=true&w=majority",
		ks.conf.DatabaseUsername,
		url.QueryEscape(ks.conf.DatabasePassword),
		ks.conf.DatabaseServer,
		ks.conf.DatabaseName)
	if client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri)); err == nil {
		ks.dbClient = client
	} else {
		log.Printf("Error connecting database --> %s", err)
	}
}

func (ks *KiteServer) writeLog(message string, endpoint kite.Endpoint) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logMessageCollection := ks.dbClient.Database(ks.conf.DatabaseName).Collection("messages")
	logMessage := kite.LogMessage{Endpoint: endpoint.String(), Message: message, Time: time.Now()}

	if _, err := logMessageCollection.InsertOne(ctx, logMessage); err != nil {
		log.Printf("Error logging message to database --> %s", err)
	}
}

func (ks *KiteServer) readLog(filter string) []kite.LogMessage {
	var messages []kite.LogMessage
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := bson.D{
		{"$or", []interface{}{
			bson.D{{"endpoint", bson.D{{"$regex", filter}}}},
			bson.D{{"message", bson.D{{"$regex", filter}}}},
		}},
	}
	logMessageCollection := ks.dbClient.Database(ks.conf.DatabaseName).Collection("messages")
	if cursor, err := logMessageCollection.Find(ctx, query); err == nil {
		defer cursor.Close(ctx)
		if err := cursor.All(ctx, &messages); err == nil {
			return messages
		}
	}
	return nil
}