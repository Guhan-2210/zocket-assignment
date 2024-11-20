package main

import (
	"net/http"
	"backend/config"
	"backend/handlers"
	"backend/middleware"
	"backend/utils"
)

func main() {
	// Initialize configuration
	config.Init()

	// Set up routes
	http.HandleFunc("/users", middleware.LogRequest(handlers.GetUsers))
	http.HandleFunc("/users/add", middleware.LogRequest(handlers.AddUser))

	http.HandleFunc("/products", middleware.LogRequest(handlers.GetProducts))
	http.HandleFunc("/products/add", middleware.LogRequest(handlers.AddProduct))
	http.HandleFunc("/products/", middleware.LogRequest(handlers.GetProductByID))

	// Start server
	utils.Logger.Info("Server is listening on port 8082")
	utils.Logger.Fatal(http.ListenAndServe(":8082", nil))
}
