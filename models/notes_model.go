// Package models contains MongoDB data models for the Go Notes app.
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Note represents a MongoDB note document
type Note struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	Title     string             `bson:"title" json:"title"`
	Content   string             `bson:"content" json:"content"`
	Tags      []string           `bson:"tags" json:"tags"`
	IsPinned  bool               `bson:"isPinned" json:"isPinned"`
	UserID    string             `bson:"userId" json:"userId"`
	CreatedOn time.Time          `bson:"createdOn" json:"createdOn"`
}
