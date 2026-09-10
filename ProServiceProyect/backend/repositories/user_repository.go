package repositories

import (
	"context"
	"time"

	"backend/models"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// userQueryTimeout acota cada acceso a Mongo. Sin esto, un problema de red deja
// la petición colgada indefinidamente.
const userQueryTimeout = 5 * time.Second

// UserRepository es la única capa autorizada a ejecutar consultas Mongo sobre
// la colección users. Implementa services.UserRepository.
type UserRepository struct {
	collection *mongo.Collection
}

// NewUserRepository crea el repositorio de usuarios sobre la base ya conectada.
func NewUserRepository(db *mongo.Database) *UserRepository {
	return &UserRepository{collection: db.Collection("users")}
}

// Create inserta un usuario nuevo y le completa el ID asignado por Mongo.
func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	ctx, cancel := context.WithTimeout(ctx, userQueryTimeout)
	defer cancel()

	result, err := r.collection.InsertOne(ctx, user)
	if err != nil {
		return err
	}

	if id, ok := result.InsertedID.(bson.ObjectID); ok {
		user.ID = id
	}
	return nil
}

// FindByID busca un usuario por su ObjectID. Devuelve mongo.ErrNoDocuments si
// no existe, para que el service lo traduzca a su error de dominio.
func (r *UserRepository) FindByID(ctx context.Context, id bson.ObjectID) (*models.User, error) {
	ctx, cancel := context.WithTimeout(ctx, userQueryTimeout)
	defer cancel()

	var user models.User
	if err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&user); err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByGoogleID busca un usuario por su google_id. Es la consulta central del
// flujo de login: determina si el usuario ya existe o hay que crearlo.
func (r *UserRepository) FindByGoogleID(ctx context.Context, googleID string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(ctx, userQueryTimeout)
	defer cancel()

	var user models.User
	if err := r.collection.FindOne(ctx, bson.M{"google_id": googleID}).Decode(&user); err != nil {
		return nil, err
	}
	return &user, nil
}

// Update persiste los campos editables de un usuario existente (nombre, foto
// de perfil y rol). google_id y created_at no se tocan una vez creados.
func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	ctx, cancel := context.WithTimeout(ctx, userQueryTimeout)
	defer cancel()

	_, err := r.collection.UpdateOne(ctx,
		bson.M{"_id": user.ID},
		bson.M{"$set": bson.M{
			"name":        user.Name,
			"picture_url": user.PictureURL,
			"role":        user.Role,
		}},
	)
	return err
}

// SetRole cambia solo el rol de un usuario. Se usa cuando un usuario crea su
// perfil profesional y pasa a worker: no hace falta traer y reescribir el
// documento entero para tocar un campo.
func (r *UserRepository) SetRole(ctx context.Context, id bson.ObjectID, role models.Role) error {
	ctx, cancel := context.WithTimeout(ctx, userQueryTimeout)
	defer cancel()

	_, err := r.collection.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"role": role}},
	)
	return err
}

// Delete elimina definitivamente un usuario. La colección users no tiene baja
// lógica en el modelo de datos (a diferencia de workers).
func (r *UserRepository) Delete(ctx context.Context, id bson.ObjectID) error {
	ctx, cancel := context.WithTimeout(ctx, userQueryTimeout)
	defer cancel()

	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}
