package config

import (
	"database/sql"
	"fmt"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"backend/utils"
	"os"
	"github.com/joho/godotenv"
	"log"
)

var DB *sql.DB
var RDB *redis.Client

func Init() {
	err := godotenv.Load("../.env")
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
	initLogger()
	initPostgres()
	initRedis()
}

func initLogger() {
	utils.Logger.SetFormatter(&logrus.JSONFormatter{})
	utils.Logger.SetLevel(logrus.InfoLevel)
}

func initPostgres() {
	const (
		host = "localhost"
		port = 5432
		user = "postgres"
		dbname = "zocket"
	)
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		utils.Logger.Fatalf("DB_PASSWORD environment variable is not set")
	}

	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	var err error
	DB, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		utils.Logger.Fatalf("Error opening database connection: %v", err)
	}

	err = DB.Ping()
	if err != nil {
		utils.Logger.Fatalf("Error connecting to the database: %v", err)
	}
	utils.Logger.Info("Connected to the PostgreSQL database")
}

func initRedis() {
	RDB = redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // Update with your Redis server address
		DB:   0,
	})
	utils.Logger.Info("Connected to Redis")
}
