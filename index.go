package main

import (
	"context"
	"encoding/json"
	"fmt"
	"go-notes-app-api/models"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/golang-jwt/jwt/v5"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Config struct for config.json
type Config struct {
	ConnectionString string `json:"connectionString"`
}

// Global Mongo client
var Client *mongo.Client

func loadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

func connectMongo(connectionString string) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(connectionString))
	if err != nil {
		return nil, err
	}

	return client, nil
}

func main() {
	// Load .env file (optional)
	if err := godotenv.Load(); err != nil {
		log.Println(".env not found, continuing...")
	}

	// Load config.json
	config, err := loadConfig("config.json")
	if err != nil {
		log.Fatal("Error loading config.json:", err)
	}

	// Connect to MongoDB
	Client, err = connectMongo(config.ConnectionString)
	if err != nil {
		log.Fatal("Error connecting to MongoDB:", err)
	}

	fmt.Println("MongoDB connected successfully!")

	app := fiber.New()

	// Cors middleware
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "*",
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	// Create Account
	app.Post("/create-account", func(c *fiber.Ctx) error {
		type Request struct {
			FullName string `json:"fullName"`
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		var body Request
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid request body"})
		}

		// Validate inputs
		if body.FullName == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Please enter a name."})
		}
		if body.Email == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Please enter an email."})
		}
		if body.Password == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Please enter a password."})
		}

		// MongoDB user collection
		userCollection := Client.Database("go-notes-app").Collection("users")

		// Check if user already exists
		var existingUser models.User
		err := userCollection.FindOne(context.Background(), bson.M{"email": body.Email}).Decode(&existingUser)
		if err == nil {
			// User found
			return c.JSON(fiber.Map{"error": true, "message": "User already exists."})
		}

		// Hash password
		// hashedPassword, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
		// if err != nil {
		// 	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Error hashing password."})
		// }

		// Create user struct
		user := models.User{
			FullName:  body.FullName,
			Email:     body.Email,
			Password:  body.Password,
			CreatedOn: time.Now(),
		}

		// Insert into MongoDB
		insertResult, err := userCollection.InsertOne(context.Background(), user)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Error saving user."})
		}
		// Set the generated ID
		user.ID = insertResult.InsertedID.(primitive.ObjectID)

		// Generate JWT token
		secret := []byte(os.Getenv("ACCESS_TOKEN_SECRET"))
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id": user.ID.Hex(),
			"exp":     time.Now().Add(time.Hour * 36000).Unix(),
		})

		tokenString, err := token.SignedString(secret)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Error generating token."})
		}

		// Return response
		return c.JSON(fiber.Map{
			"error":       false,
			"user":        user,
			"accessToken": tokenString,
			"message":     "Registration Successful.",
		})
	})

	// Login Account
	app.Post("/login", func(c *fiber.Ctx) error {
		type LoginRequest struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		var body LoginRequest
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid request"})
		}

		// Validation
		if body.Email == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Please enter an email."})
		}
		if body.Password == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Please enter a password."})
		}

		// MongoDB collection
		userCollection := Client.Database("go-notes-app").Collection("users")

		// Find user by email
		var user models.User
		err := userCollection.FindOne(context.Background(), bson.M{"email": body.Email}).Decode(&user)
		if err != nil {
			return c.JSON(fiber.Map{"error": true, "message": "User not found."})
		}

		// Compare password using bcrypt
		// err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(body.Password))
		// 	if err != nil {
		// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Invalid credentials."})
		// 	}

		// Generate JWT token
		secret := []byte(os.Getenv("ACCESS_TOKEN_SECRET"))
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id": user.ID.Hex(),
			"exp":     time.Now().Add(time.Hour * 36000).Unix(),
		})

		tokenString, err := token.SignedString(secret)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Error generating token."})
		}

		// Return response (without password)
		user.Password = "" // hide password
		return c.JSON(fiber.Map{
			"error":       false,
			"message":     "Login Successful.",
			"email":       user.Email,
			"accessToken": tokenString,
		})
	})

	// Get User
	app.Get("/get-user", authenticationToken, func(c *fiber.Ctx) error {
		// Get user id from middleware
		userID := c.Locals("user_id")

		if userID == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Unauthorized",
			})
		}

		objectID, err := primitive.ObjectIDFromHex(userID.(string))
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Invalid user ID",
			})
		}

		// MongoDB collection
		userCollection := Client.Database("go-notes-app").Collection("users")

		var isUser models.User

		err = userCollection.FindOne(context.Background(), bson.M{
			"_id": objectID,
		}).Decode(&isUser)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "User not found",
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"user": fiber.Map{
				"fullName":  isUser.FullName,
				"email":     isUser.Email,
				"_id":       isUser.ID,
				"createdOn": isUser.CreatedOn,
			},
			"message": "",
		})
	})

	// Add Note
	app.Post("/add-note", authenticationToken, func(c *fiber.Ctx) error {
		type AddNoteRequest struct {
			Title   string   `json:"title"`
			Content string   `json:"content"`
			Tags    []string `json:"tags"`
		}

		// Get user ID from middleware
		userID := c.Locals("user_id")
		if userID == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Unauthorized",
			})
		}

		var body AddNoteRequest
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid request"})
		}

		// Validation
		if body.Title == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Please enter a title."})
		}

		if body.Content == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Please enter a content."})
		}

		// Convert string ID to ObjectID
		objectID := userID.(string)

		// Create Note Struct
		note := models.Note{
			Title:     body.Title,
			Content:   body.Content,
			Tags:      body.Tags,
			UserID:    objectID,
			CreatedOn: time.Now(),
		}

		// MongoDB collection
		noteCollection := Client.Database("go-notes-app").Collection("notes")

		// Insert into MongoDB
		insertResult, err := noteCollection.InsertOne(context.Background(), note)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   true,
				"message": "Internal Server Error",
			})
		}

		// Set the generated ID in note
		note.ID = insertResult.InsertedID.(primitive.ObjectID)

		// Return response
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"error":   false,
			"note":    note,
			"message": "Note added successfully.",
		})
	})

	// Edit Note
	app.Put("/edit-note/:noteId", authenticationToken, func(c *fiber.Ctx) error {
		type UpdateNoteRequest struct {
			Title    string   `json:"title"`
			Content  string   `json:"content"`
			Tags     []string `json:"tags"`
			IsPinned *bool    `json:"isPinned"`
		}

		// Get noteId from URL params
		noteID := c.Params("noteId")
		if noteID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Missing note ID",
			})
		}

		// Get user ID from middleware
		userID := c.Locals("user_id")
		if userID == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Unauthorized",
			})
		}

		// Parse body
		var body UpdateNoteRequest
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Invalid request",
			})
		}

		// Validation
		if body.Title == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Please enter a title."})
		}

		if body.Content == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Please enter a content."})
		}

		// Convert noteID to ObjectID
		objID, err := primitive.ObjectIDFromHex(noteID)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Invalid note ID",
			})
		}

		// MongoDB collection
		noteCollection := Client.Database("go-notes-app").Collection("notes")

		// Find note by _id and userId
		var note models.Note
		err = noteCollection.FindOne(context.Background(), bson.M{
			"_id":    objID,
			"userId": userID.(string),
		}).Decode(&note)
		if err != nil {
			return c.Status(fiber.StatusFound).JSON(fiber.Map{
				"message": "Note not found",
			})
		}

		// Update fields if provided
		update := bson.M{}
		if body.Title != "" {
			update["title"] = body.Title
		}
		if body.Content != "" {
			update["content"] = body.Content
		}
		if body.Tags != nil {
			update["tags"] = body.Tags
		}
		if body.IsPinned != nil {
			update["isPinned"] = *body.IsPinned
		}

		// Update in MongoDB
		_, err = noteCollection.UpdateOne(context.Background(), bson.M{"_id": objID, "userId": userID.(string)}, bson.M{"$set": update})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   true,
				"message": "Internal Server Error",
			})
		}

		// Return updated one
		// You can optionally re-fetch the updated note, or just merge fields locally
		for k, v := range update {
			switch k {
			case "title":
				note.Title = v.(string)
			case "content":
				note.Content = v.(string)
			case "tags":
				note.Tags = v.([]string)
			case "isPinned":
				note.IsPinned = v.(bool)
			}
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"error":   false,
			"note":    note,
			"message": "Note updated successfully",
		})
	})

	app.Get("/get-all-notes", authenticationToken, func(c *fiber.Ctx) error {
		// Get user ID from middleware
		userID := c.Locals("user_id")
		if userID == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Unauthorized",
			})
		}

		// MongoDB collection
		noteCollection := Client.Database("go-notes-app").Collection("notes")

		// Filter: notes for this user
		filter := bson.M{"userId": userID.(string)}

		// Sort: pinned notes first
		findOptions := options.Find()
		findOptions.SetSort(bson.D{{Key: "isPinned", Value: -1}})

		// Query MongoDB
		cursor, err := noteCollection.Find(context.Background(), filter, findOptions)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   true,
				"message": "Internal Server Error",
			})
		}
		defer cursor.Close(context.Background())

		// Decode all notes
		var notes []models.Note
		if err := cursor.All(context.Background(), &notes); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   true,
				"message": "Internal Server Error",
			})
		}

		// Return response
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"error":   false,
			"notes":   notes,
			"message": "Notes fetched successfully",
		})
	})

	// Delete Note
	app.Delete("/delete-note/:noteId", authenticationToken, func(c *fiber.Ctx) error {
		// Get noteId from URL params
		noteID := c.Params("noteId")
		if noteID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Missing note ID",
			})
		}

		// Get user ID from middleware
		userID := c.Locals("user_id")
		if userID == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Unauthorized",
			})
		}

		// Convert noteID to ObjectID
		objID, err := primitive.ObjectIDFromHex(noteID)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Invalid note ID",
			})
		}

		// MongoDB collection
		noteCollection := Client.Database("go-notes-app").Collection("notes")

		// Check if note exists for this user
		var note models.Note
		err = noteCollection.FindOne(context.Background(), bson.M{
			"_id":    objID,
			"userId": userID.(string),
		}).Decode(&note)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"message": "Note not found",
			})
		}

		// Delete the note
		_, err = noteCollection.DeleteOne(context.Background(), bson.M{
			"_id":    objID,
			"userId": userID.(string),
		})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   true,
				"message": "Internal Server Error",
			})
		}

		// Return success
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"error":   false,
			"message": "Note deleted successfully",
		})
	})

	app.Put("/update-note-pinned/:noteId", authenticationToken, func(c *fiber.Ctx) error {
		// Get noteId from URL params
		noteID := c.Params("noteId")
		if noteID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Missing note ID",
			})
		}

		// Get user ID from middleware
		userID := c.Locals("user_id")
		if userID == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Unauthorized",
			})
		}

		// Parse request body
		type UpdatePinnedRequest struct {
			IsPinned bool `json:"isPinned"`
		}

		var body UpdatePinnedRequest
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Invalid request",
			})
		}

		// Convert noteID to ObjectID
		objID, err := primitive.ObjectIDFromHex(noteID)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Invalid note ID",
			})
		}

		// MongoDB collection
		noteCollection := Client.Database("go-notes-app").Collection("notes")

		// Find the note for this user
		var note models.Note
		err = noteCollection.FindOne(context.Background(), bson.M{
			"_id":    objID,
			"userId": userID.(string),
		}).Decode(&note)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"message": "Note not found",
			})
		}

		// Update isPinned value
		update := bson.M{"isPinned": body.IsPinned}
		_, err = noteCollection.UpdateOne(
			context.Background(),
			bson.M{"_id": objID, "userId": userID.(string)},
			bson.M{"$set": update},
		)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   true,
				"message": "Internal Server Error",
			})
		}

		// Update the note struct locally to return updated value
		note.IsPinned = body.IsPinned

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"error":   false,
			"note":    note,
			"message": "Note updated successfully",
		})
	})

	app.Get("/search-notes/", authenticationToken, func(c *fiber.Ctx) error {
		// Get user ID from middleware
		userID := c.Locals("user_id")
		if userID == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Unauthorized",
			})
		}

		// Get query parameter
		query := c.Query("query")
		if query == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": "Search query is required.",
			})
		}

		// MongoDB collection
		noteCollection := Client.Database("go-notes-app").Collection("notes")

		// Search filter: match title or content (case-insensitive)
		filter := bson.M{
			"userId": userID.(string),
			"$or": []bson.M{
				{"title": bson.M{"$regex": query, "$options": "i"}},
				{"content": bson.M{"$regex": query, "$options": "i"}},
			},
		}

		// Find matching notes
		cursor, err := noteCollection.Find(context.Background(), filter)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   true,
				"message": "Internal Server Error",
			})
		}
		defer cursor.Close(context.Background())

		notes := []models.Note{}
		if err := cursor.All(context.Background(), &notes); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   true,
				"message": "Internal Server Error",
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"error":   false,
			"notes":   notes,
			"message": "Notes matching the search query retrieved successfully.",
		})
	})

	log.Fatal(app.Listen(":8000"))
}
