package repositories

import (
	"context"
	"time"

	"backend/models"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// specialtyQueryTimeout acota cada acceso a Mongo sobre la colección
// specialties.
const specialtyQueryTimeout = 5 * time.Second

// SpecialtyRepository es la única capa autorizada a ejecutar consultas Mongo
// sobre la colección specialties.
type SpecialtyRepository struct {
	collection *mongo.Collection
}

// NewSpecialtyRepository crea el repositorio sobre la base ya conectada.
func NewSpecialtyRepository(db *mongo.Database) *SpecialtyRepository {
	return &SpecialtyRepository{collection: db.Collection("specialties")}
}

// List devuelve el catálogo completo ordenado por nombre. Es un catálogo chico
// y cerrado, así que se trae entero sin paginar.
func (r *SpecialtyRepository) List(ctx context.Context) ([]models.Specialty, error) {
	ctx, cancel := context.WithTimeout(ctx, specialtyQueryTimeout)
	defer cancel()

	cursor, err := r.collection.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return nil, err
	}

	var specialties []models.Specialty
	if err := cursor.All(ctx, &specialties); err != nil {
		return nil, err
	}

	return specialties, nil
}

// AllExist indica si todos los ids dados existen en la colección. Cuenta cuántos
// documentos matchean y compara contra la cantidad de ids únicos pedidos: si el
// caller manda repetidos, este método no los delata (esa validación es del
// service), solo confirma existencia.
func (r *SpecialtyRepository) AllExist(ctx context.Context, ids []bson.ObjectID) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, specialtyQueryTimeout)
	defer cancel()

	unique := make(map[bson.ObjectID]struct{}, len(ids))
	for _, id := range ids {
		unique[id] = struct{}{}
	}
	if len(unique) == 0 {
		return true, nil
	}

	count, err := r.collection.CountDocuments(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return false, err
	}

	return count == int64(len(unique)), nil
}
