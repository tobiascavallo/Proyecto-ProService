package database

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// indexTimeout acota la creación de índices en el arranque.
const indexTimeout = 30 * time.Second

// EnsureIndexes crea todos los índices del modelo de datos. Es idempotente:
// crear un índice que ya existe con la misma definición no hace nada. Se llama
// al arrancar, antes de levantar el servidor; si falla, el proceso corta.
func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	ctx, cancel := context.WithTimeout(ctx, indexTimeout)
	defer cancel()

	indexes := []struct {
		collection string
		model      mongo.IndexModel
	}{
		{"users", mongo.IndexModel{
			Keys:    bson.D{{Key: "google_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		}},
		{"workers", mongo.IndexModel{
			Keys:    bson.D{{Key: "user_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		}},
		{"workers", mongo.IndexModel{
			// Soporta la búsqueda pública: filtro por especialidad + orden por
			// calificación descendente, sin agregación.
			Keys: bson.D{{Key: "specialty_ids", Value: 1}, {Key: "average_rating", Value: -1}},
		}},
		{"specialties", mongo.IndexModel{
			// Único: evita duplicados y es lo que hace idempotente el seed.
			Keys:    bson.D{{Key: "slug", Value: 1}},
			Options: options.Index().SetUnique(true),
		}},
		{"reviews", mongo.IndexModel{
			Keys: bson.D{{Key: "worker_id", Value: 1}},
		}},
		{"reviews", mongo.IndexModel{
			// Único: un cliente deja una sola reseña por trabajador.
			Keys:    bson.D{{Key: "worker_id", Value: 1}, {Key: "client_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		}},
		{"favorites", mongo.IndexModel{
			Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "worker_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		}},
	}

	for _, idx := range indexes {
		if _, err := db.Collection(idx.collection).Indexes().CreateOne(ctx, idx.model); err != nil {
			return fmt.Errorf("creating index on %s: %w", idx.collection, err)
		}
	}

	return nil
}
