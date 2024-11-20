package config

import (
	"database/sql"
	"fmt"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"backend/utils"
)

var DB *sql.DB
var RDB *redis.Client

func Init() {
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
		host     = "localhost"
		port     = 5432
		user     = "postgres"
		password = "DB_PASSWORD"
		dbname   = "zocket"
	)
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
