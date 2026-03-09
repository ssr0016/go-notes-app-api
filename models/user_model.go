// Package models contains MongoDB data models for the Go Notes app.
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// User represents a MongoDB user document
type User struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	FullName  string             `bson:"fullName" json:"fullName"`   // full name of the user
	Email     string             `bson:"email" json:"email"`         // email
	Password  string             `bson:"password" json:"-"`          // hide password from JSON output
	CreatedOn time.Time          `bson:"createdOn" json:"createdOn"` // creation timestamp
}
