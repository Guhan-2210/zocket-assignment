package middleware

import (
	"net/http"
	"time"
	"backend/utils"
)

func LogRequest(handlerFunc http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		handlerFunc(w, r)
		duration := time.Since(startTime)

		utils.Logger.WithFields(map[string]interface{}{
			"method":     r.Method,
			"path":       r.URL.Path,
			"duration":   duration.String(),
			"user_agent": r.UserAgent(),
		}).Info("Request handled")
	}
}
