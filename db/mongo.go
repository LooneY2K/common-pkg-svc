package db

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

func GetDBConn(ctx context.Context, log *zap.SugaredLogger, dbURI, dbName string) (*mongo.Database, error) {
	if dbURI == "" {
		return nil, errors.New("Set MongoDB URI in your env file")
	}
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(dbURI))
	if err != nil {
		return nil, errors.New("Error occured connecting to the Database")
	}
	database := client.Database(dbName)
	return database, nil
}
