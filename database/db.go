package database

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Username  string            `bson:"username"`
	Email     string            `bson:"email"`
	Password  string            `bson:"password"`
	CreatedAt time.Time         `bson:"created_at"`
}

type ChatHistory struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserID    primitive.ObjectID `bson:"user_id"`
	Messages  []string          `bson:"messages"`
	CreatedAt time.Time         `bson:"created_at"`
}

var client *mongo.Client
var userCollection *mongo.Collection
var chatCollection *mongo.Collection

func InitDB() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use the Windows host's MongoDB instance from WSL
	// For WSL, we need to use the Windows host IP
	clientOptions := options.Client().ApplyURI("mongodb://127.0.0.1:27017")
	
	var err error
	client, err = mongo.Connect(ctx, clientOptions)
	if err != nil {
		return err
	}

	// Check the connection
	err = client.Ping(ctx, nil)
	if err != nil {
		return err
	}

	// Initialize collections
	userCollection = client.Database("askgpt").Collection("users")
	chatCollection = client.Database("askgpt").Collection("chats")

	return nil
}

func CreateUser(username, email, password string) (*User, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := User{
		Username:  username,
		Email:     email,
		Password:  string(hashedPassword),
		CreatedAt: time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := userCollection.InsertOne(ctx, user)
	if err != nil {
		return nil, err
	}

	user.ID = result.InsertedID.(primitive.ObjectID)
	return &user, nil
}

func AuthenticateUser(email, password string) (*User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user User
	err := userCollection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func SaveChatHistory(userID primitive.ObjectID, messages []string) error {
	chat := ChatHistory{
		UserID:    userID,
		Messages:  messages,
		CreatedAt: time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := chatCollection.InsertOne(ctx, chat)
	return err
}

func GetChatHistory(userID primitive.ObjectID) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var chat ChatHistory
	err := chatCollection.FindOne(ctx, bson.M{"user_id": userID}).Decode(&chat)
	if err == mongo.ErrNoDocuments {
		return []string{}, nil
	} else if err != nil {
		return nil, err
	}

	return chat.Messages, nil
}

func ClearChatHistory(userID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := chatCollection.DeleteOne(ctx, bson.M{"user_id": userID})
	return err
}

func GetUserByID(id primitive.ObjectID) (*User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user User
	err := userCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func CloseDB() {
	if client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		client.Disconnect(ctx)
	}
} 