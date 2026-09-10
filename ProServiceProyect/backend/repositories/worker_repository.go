package repositories

import (
	"context"
	"time"

	"backend/models"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// workerQueryTimeout acota cada acceso a Mongo sobre la colección workers.
const workerQueryTimeout = 5 * time.Second

// WorkerRepository es la única capa autorizada a ejecutar consultas Mongo sobre
// la colección workers.
type WorkerRepository struct {
	collection *mongo.Collection
}

// NewWorkerRepository crea el repositorio sobre la base ya conectada.
func NewWorkerRepository(db *mongo.Database) *WorkerRepository {
	return &WorkerRepository{collection: db.Collection("workers")}
}

// Create inserta un perfil nuevo y le completa el ID asignado por Mongo.
func (r *WorkerRepository) Create(ctx context.Context, worker *models.Worker) error {
	ctx, cancel := context.WithTimeout(ctx, workerQueryTimeout)
	defer cancel()

	result, err := r.collection.InsertOne(ctx, worker)
	if err != nil {
		return err
	}

	if id, ok := result.InsertedID.(bson.ObjectID); ok {
		worker.ID = id
	}
	return nil
}

// FindByID busca un perfil por su ObjectID. Devuelve mongo.ErrNoDocuments si no
// existe.
func (r *WorkerRepository) FindByID(ctx context.Context, id bson.ObjectID) (*models.Worker, error) {
	ctx, cancel := context.WithTimeout(ctx, workerQueryTimeout)
	defer cancel()

	var worker models.Worker
	if err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&worker); err != nil {
		return nil, err
	}
	return &worker, nil
}

// FindByUserID busca el perfil de un usuario. Es la consulta con la que el
// service resuelve la propiedad: siempre por el user_id del token, nunca por un
// id del body.
func (r *WorkerRepository) FindByUserID(ctx context.Context, userID bson.ObjectID) (*models.Worker, error) {
	ctx, cancel := context.WithTimeout(ctx, workerQueryTimeout)
	defer cancel()

	var worker models.Worker
	if err := r.collection.FindOne(ctx, bson.M{"user_id": userID}).Decode(&worker); err != nil {
		return nil, err
	}
	return &worker, nil
}

// Update persiste los campos editables del perfil. user_id, average_rating,
// review_count, active y created_at no se tocan por acá.
func (r *WorkerRepository) Update(ctx context.Context, worker *models.Worker) error {
	ctx, cancel := context.WithTimeout(ctx, workerQueryTimeout)
	defer cancel()

	_, err := r.collection.UpdateOne(ctx,
		bson.M{"_id": worker.ID},
		bson.M{"$set": bson.M{
			"bio":                   worker.Bio,
			"specialty_ids":         worker.SpecialtyIDs,
			"phone":                 worker.Phone,
			"contact_email":         worker.ContactEmail,
			"social_links":          worker.SocialLinks,
			"availability_status":   worker.AvailabilityStatus,
			"availability_schedule": worker.AvailabilitySchedule,
			"photos":                worker.Photos,
		}},
	)
	return err
}

// SetActive marca el perfil como activo o inactivo. La baja es lógica: el
// documento nunca se borra por moderación ni por baja propia (regla 8).
func (r *WorkerRepository) SetActive(ctx context.Context, id bson.ObjectID, active bool) error {
	ctx, cancel := context.WithTimeout(ctx, workerQueryTimeout)
	defer cancel()

	_, err := r.collection.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"active": active}},
	)
	return err
}

// Delete borra físicamente un perfil. Se usa solo como compensación: si la
// promoción del rol del usuario falla justo después de crear el perfil, hay que
// deshacerlo para no dejar un worker cuyo dueño sigue siendo client.
func (r *WorkerRepository) Delete(ctx context.Context, id bson.ObjectID) error {
	ctx, cancel := context.WithTimeout(ctx, workerQueryTimeout)
	defer cancel()

	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

// Search devuelve la página de perfiles activos que matchean, ordenados por
// calificación descendente. Usa el promedio desnormalizado en vez de una
// agregación porque es el endpoint más consultado y tiene que responder en
// menos de 3 segundos; se apoya en el índice (specialty_ids, average_rating).
func (r *WorkerRepository) Search(ctx context.Context, specialtyID *bson.ObjectID, page, pageSize int) ([]models.Worker, error) {
	ctx, cancel := context.WithTimeout(ctx, workerQueryTimeout)
	defer cancel()

	filter := bson.M{"active": true}
	if specialtyID != nil {
		filter["specialty_ids"] = *specialtyID
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "average_rating", Value: -1}}).
		SetSkip(int64((page - 1) * pageSize)).
		SetLimit(int64(pageSize))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}

	var workers []models.Worker
	if err := cursor.All(ctx, &workers); err != nil {
		return nil, err
	}
	return workers, nil
}
