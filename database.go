package main

import (
	"context"
	"fmt"
	kite "github.com/get-code-ch/kite-common"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
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
		ks.db = client.Database(ks.conf.DatabaseName)
	} else {
		log.Printf("Error connecting database --> %s", err)
	}
}

func (ks *KiteServer) writeLog(message string, endpoint kite.Endpoint) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logMessageCollection := ks.db.Collection(string(kite.C_LOG))
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
	logMessageCollection := ks.db.Collection(string(kite.C_LOG))
	if cursor, err := logMessageCollection.Find(ctx, query); err == nil {
		defer cursor.Close(ctx)
		if err := cursor.All(ctx, &messages); err == nil {
			return messages
		}
	}
	return nil
}

func (ks *KiteServer) upsertEndpointAuth(endpoint kite.EndpointAuth) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	endpointAuthCollection := ks.db.Collection(string(kite.C_ENDPOINTAUTH))

	update := bson.M{"$set": endpoint}
	opts := options.Update().SetUpsert(true)

	if _, err := endpointAuthCollection.UpdateOne(ctx, bson.D{{"name", endpoint.Name}}, update, opts); err != nil {
		return err
	}
	return nil
}

func (ks *KiteServer) findEndpointAuth(endpoint string) (kite.EndpointAuth, error) {
	var endpointAuth kite.EndpointAuth

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	endpointAuthCollection := ks.db.Collection(string(kite.C_ENDPOINTAUTH))

	query := bson.M{"name": endpoint}

	if err := endpointAuthCollection.FindOne(ctx, query).Decode(&endpointAuth); err != nil {
		return kite.EndpointAuth{Name: "", ApiKey: "", Enabled: false}, err
	}
	return endpointAuth, nil
}

func (ks *KiteServer) activateEndpoint(activationCode string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	endpointAuthCollection := ks.db.Collection(string(kite.C_ENDPOINTAUTH))
	query := bson.M{"activation_code": activationCode}
	update := bson.M{"$set": bson.M{
		"enable": true,
		"activation_code": bsontype.Null,
	}}

	if result := endpointAuthCollection.FindOneAndUpdate(ctx, query, update); result.Err() != nil {
		return result.Err()
	}

	return nil
}