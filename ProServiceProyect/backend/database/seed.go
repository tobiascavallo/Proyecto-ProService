package database

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// seedTimeout acota la siembra del catálogo en el arranque.
const seedTimeout = 30 * time.Second

// specialtyCatalog es la lista fija de especialidades de la V1. Agregar una es
// sumar una línea acá y reiniciar. Sacar una de acá NO la borra de la base: el
// seed nunca elimina, porque puede haber trabajadores asociados.
var specialtyCatalog = []struct {
	name string
	slug string
}{
	{"Plomero", "plomero"},
	{"Electricista", "electricista"},
	{"Gasista matriculado", "gasista-matriculado"},
	{"Albañil", "albanil"},
	{"Pintor", "pintor"},
	{"Carpintero", "carpintero"},
	{"Herrero", "herrero"},
	{"Techista", "techista"},
	{"Cerrajero", "cerrajero"},
	{"Vidriero", "vidriero"},
	{"Aire acondicionado y refrigeración", "aire-acondicionado"},
	{"Service de electrodomésticos", "electrodomesticos"},
	{"Durlock", "durlock"},
	{"Colocador de pisos", "colocador-de-pisos"},
	{"Jardinería y parquización", "jardineria"},
	{"Fletes y mudanzas", "fletes-y-mudanzas"},
	{"Limpieza", "limpieza"},
	{"Reparación de PC y redes", "reparacion-pc"},
	{"Mecánico", "mecanico"},
	{"Costura y arreglos de ropa", "costura"},
}

// SeedSpecialties inserta el catálogo de forma idempotente: upsert por slug, así
// reiniciar N veces deja el mismo resultado. Actualiza el name (por si se
// corrige una tilde) pero nunca borra documentos existentes. Depende del índice
// único en slug para que dos arranques concurrentes no dupliquen.
func SeedSpecialties(ctx context.Context, db *mongo.Database) error {
	ctx, cancel := context.WithTimeout(ctx, seedTimeout)
	defer cancel()

	collection := db.Collection("specialties")
	now := time.Now().UTC()

	for _, item := range specialtyCatalog {
		filter := bson.M{"slug": item.slug}
		update := bson.M{
			"$set":         bson.M{"name": item.name},
			"$setOnInsert": bson.M{"created_at": now},
		}

		if _, err := collection.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true)); err != nil {
			return fmt.Errorf("seeding specialty %q: %w", item.slug, err)
		}
	}

	return nil
}
