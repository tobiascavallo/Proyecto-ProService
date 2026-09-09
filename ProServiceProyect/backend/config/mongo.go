package config

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// mongoConnectTimeout acota cuánto se espera a que Mongo responda al conectar.
// Preferible fallar rápido en el arranque a levantar el servidor y recién
// enterarse del problema con la primera petición de un usuario real.
const mongoConnectTimeout = 10 * time.Second

// ConnectMongo abre el cliente de Mongo contra uri y verifica con un ping que
// la base esté realmente disponible antes de devolverlo.
func ConnectMongo(ctx context.Context, uri string) (*mongo.Client, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, mongoConnectTimeout)
	defer cancel()

	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	return client, nil
}
