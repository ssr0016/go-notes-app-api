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

	log.Fatal(app.Listen(":3000"))
}
