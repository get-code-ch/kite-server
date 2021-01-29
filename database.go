package main

import (
	"context"
	"encoding/json"
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
		ks.db = client.Database(ks.conf.DatabaseName)
	} else {
		log.Printf("Error connecting database --> %s", err)
	}
	log.Printf("Database %s connected...", ks.conf.DatabaseName)

}

func (ks *KiteServer) writeLog(message string, address kite.Address) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logMessageCollection := ks.db.Collection(string(kite.C_LOG))
	logMessage := kite.LogMessage{Address: address.String(), Message: message, Time: time.Now()}

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
			bson.D{{"address", bson.D{{"$regex", filter}}}},
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

func (ks *KiteServer) upsertAddressAuth(address kite.AddressAuth) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	addressAuthCollection := ks.db.Collection(string(kite.C_ADDRESSAUTH))

	update := bson.M{"$set": address}
	opts := options.Update().SetUpsert(true)

	if _, err := addressAuthCollection.UpdateOne(ctx, bson.D{{"name", address.Name}}, update, opts); err != nil {
		return err
	}
	return nil
}

func (ks *KiteServer) findAddressAuth(address string) (kite.AddressAuth, error) {
	var addressAuth kite.AddressAuth

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	addressAuthCollection := ks.db.Collection(string(kite.C_ADDRESSAUTH))

	a := new(kite.Address)
	a.StringToAddress(address)

	regexAddress := `^`
	regexAddress += a.Domain + `\.`
	regexAddress += a.Type.String() + `\.`
	regexAddress += a.Host + `\.`
	if a.Address == "*" {
		a.Address = `\*`
	}
	regexAddress += `(?:\*|` + a.Address + `)\.\*`
	regexAddress += `$`

	query := bson.D{{"name", bson.D{{"$regex", regexAddress}}}}

	if err := addressAuthCollection.FindOne(ctx, query).Decode(&addressAuth); err != nil {
		return kite.AddressAuth{Name: "", ApiKey: "", Enabled: false}, err
	}
	return addressAuth, nil
}

func (ks *KiteServer) activateAddress(activationCode string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	addressAuthCollection := ks.db.Collection(string(kite.C_ADDRESSAUTH))
	query := bson.M{"activation_code": activationCode}
	update := bson.M{"$set": bson.M{
		"enabled":         true,
		"activation_code": "",
	}}

	if result := addressAuthCollection.FindOneAndUpdate(ctx, query, update); result.Err() != nil {
		return result.Err()
	}

	return nil
}

func (ks *KiteServer) findEndpoint(address kite.Address) ([]kite.Endpoint, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	endpointCollection := ks.db.Collection(string(kite.C_ENDPOINT))

	regexAddress := `^`
	regexAddress += address.Domain + `\.`
	regexAddress += kite.H_ENDPOINT.String() + `\.`
	regexAddress += address.Host + `\..*$`

	query := bson.D{{"name", bson.D{{"$regex", regexAddress}}}}

	if cursor, err := endpointCollection.Find(ctx, query); err != nil {
		return nil, err
	} else {
		var endpoints []kite.Endpoint
		for cursor.Next(ctx) {
			endpoint := kite.Endpoint{}
			if err := cursor.Decode(&endpoint); err == nil {
				endpoints = append(endpoints, endpoint)
			}
		}
		return endpoints, nil
	}
}

func (ks *KiteServer) discover() []kite.Endpoint {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	endpointCollection := ks.db.Collection(string(kite.C_ENDPOINT))

	regexAddress := `^`
	regexAddress += ks.conf.Address.Domain + `\.`
	regexAddress += kite.H_ENDPOINT.String() + `\..*$`

	query := bson.D{{"name", bson.D{{"$regex", regexAddress}}}}

	if cursor, err := endpointCollection.Find(ctx, query); err != nil {
		log.Printf("Error endpoint discover --> %v", err)
		return nil
	} else {
		var endpoints []kite.Endpoint
		for cursor.Next(ctx) {
			endpoint := kite.Endpoint{}
			if err := cursor.Decode(&endpoint); err == nil {
				endpoints = append(endpoints, endpoint)
			}
		}
		return endpoints
	}
}

func (ks *KiteServer) exportDB() map[string]interface{} {
	export := make(map[string]interface{})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if collections, err := ks.db.ListCollectionNames(ctx, bson.M{}); err != nil {
		log.Printf("Error listing DB Collections --> %v", err)
		return nil
	} else {
		for _, name := range collections {
			collection := ks.db.Collection(name)
			// when exported,
			opts := options.Find().SetProjection(bson.D{{"_id", 0}})
			if cursor, err := collection.Find(ctx, bson.M{}, opts); err == nil {
				switch name {
				// to ignore a collection in export
				case "name_of_to_ignore_collection":
					break
				default:
					var c []bson.M
					cursor.All(ctx, &c)
					export[name] = c
					break
				}
			}
		}
		log.Printf("%v", export)
	}
	return export
}

func (ks *KiteServer) importDB(data string) []error {
	var result []error
	ok := true

	collections := make(map[string][]interface{})

	if err := json.Unmarshal([]byte(data), &collections); err != nil {
		log.Printf("importDB error -> %v", err)
		result = append(result, err)
		return result
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for name, collection := range collections {
		// Testing if data to import are valid, if yes we drop collection and import new data
		dbCollection := ks.db.Collection(name + "__test_import_")
		if _, err := dbCollection.InsertMany(ctx, collection); err != nil {
			log.Printf("importDB: error testing import %s_test --> %v", name, err)
			result = append(result, err)
			ok = false
		} else {
			_ = dbCollection.Drop(ctx)
			dbCollection = ks.db.Collection(name)
			if err := dbCollection.Drop(ctx); err == nil {
				if _, err := dbCollection.InsertMany(ctx, collection); err != nil {
					log.Printf("importDB: error importing %s --> %v", name, err)
					result = append(result, err)
					ok = false
				}
			} else {
				log.Printf("importDB: error importing %s --> %v", name, err)
				result = append(result, err)
				ok = false
			}
		}
	}
	if ok {
		return nil
	} else {
		return result
	}
}
