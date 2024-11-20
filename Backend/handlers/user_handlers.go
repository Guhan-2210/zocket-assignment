package handlers

import (
	"encoding/json"
	"net/http"
	"backend/config"
	"backend/models"
	"backend/utils"
)

func GetUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := config.DB.Query("SELECT user_id, name FROM users")
	if err != nil {
		utils.HandleError(w, err, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.UserID, &user.Name); err != nil {
			utils.HandleError(w, err, http.StatusInternalServerError)
			return
		}
		users = append(users, user)
	}

	utils.SendJSONResponse(w, users, http.StatusOK)
}

func AddUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		utils.Logger.Warn("Invalid request method for addUser")
		return
	}

	var user models.User
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		utils.Logger.WithError(err).Error("Failed to decode addUser request body")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = config.DB.QueryRow("INSERT INTO users (name) VALUES ($1) RETURNING user_id", user.Name).Scan(&user.UserID)
	if err != nil {
		utils.Logger.WithError(err).Error("Failed to insert new user")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
	utils.Logger.WithField("user_id", user.UserID).Info("User added successfully")
}
