package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"backend/config"
	"backend/models"
	"backend/utils"
	"time"
	"github.com/sirupsen/logrus"
	"github.com/lib/pq"
	"github.com/rabbitmq/amqp091-go"
	"context"
	"database/sql"
)




// Helper function to connect to RabbitMQ
func connectToRabbitMQ() (*amqp091.Connection, *amqp091.Channel) {
	conn, err := amqp091.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		utils.Logger.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		utils.Logger.Fatalf("Failed to open a channel: %v", err)
	}

	return conn, ch
}


func GetProducts(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	userID := r.URL.Query().Get("user_id")
	minPrice := r.URL.Query().Get("min_price")
	maxPrice := r.URL.Query().Get("max_price")
	productName := r.URL.Query().Get("product_name")

	query := `SELECT product_id, user_id, product_name, product_description, product_images, compressed_product_images, product_price
              FROM products WHERE 1=1`
	args := []interface{}{}

	if userID != "" {
		query += " AND user_id = $1"
		args = append(args, userID)
	}
	if minPrice != "" {
		query += " AND product_price >= $2"
		args = append(args, minPrice)
	}
	if maxPrice != "" {
		query += " AND product_price <= $3"
		args = append(args, maxPrice)
	}
	if productName != "" {
		query += " AND product_name ILIKE $4"
		args = append(args, "%"+productName+"%")
	}

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		utils.Logger.WithFields(logrus.Fields{
			"error":    err.Error(),
			"method":   r.Method,
			"endpoint": r.URL.Path,
		}).Error("Failed to fetch products")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var products []models.Product
	for rows.Next() {
		var product models.Product
		var productImages, compressedImages []string

		err := rows.Scan(&product.ID, &product.UserID, &product.ProductName, &product.ProductDescription,
			pq.Array(&productImages), pq.Array(&compressedImages), &product.ProductPrice)
		if err != nil {
			utils.Logger.WithFields(logrus.Fields{
				"error":    err.Error(),
				"method":   r.Method,
				"endpoint": r.URL.Path,
			}).Error("Failed to scan product row")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		product.ProductImages = productImages
		product.CompressedProductImages = compressedImages
		products = append(products, product)
	}

	// Log response time
	utils.Logger.WithFields(logrus.Fields{
		"method":        r.Method,
		"endpoint":      r.URL.Path,
		"response_time": time.Since(startTime),
	}).Info("Products fetched successfully")

	json.NewEncoder(w).Encode(products)
}

func AddProduct(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var product models.Product
	err := json.NewDecoder(r.Body).Decode(&product)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	query := `INSERT INTO products (user_id, product_name, product_description, product_images, compressed_product_images, product_price)
              VALUES ($1, $2, $3, $4, $5, $6) RETURNING product_id`

	err = config.DB.QueryRow(query,
		product.UserID,
		product.ProductName,
		product.ProductDescription,
		pq.Array(product.ProductImages),
		pq.Array(product.CompressedProductImages), // Initially empty
		product.ProductPrice,
	).Scan(&product.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Publish product images to RabbitMQ
	conn, ch := connectToRabbitMQ()
	defer conn.Close()
	defer ch.Close()

	queueName := "image_processing"
	_, err = ch.QueueDeclare(
		queueName, true, false, false, false, nil,
	)
	if err != nil {
		utils.Logger.Fatalf("Failed to declare a queue: %v", err)
	}

	// Publish each image URL to the queue
	for _, imageURL := range product.ProductImages {
		body, _ := json.Marshal(map[string]interface{}{
			"product_id": product.ID,
			"image_url":  imageURL,
		})

		err = ch.Publish(
			"", queueName, false, false,
			amqp091.Publishing{
				ContentType: "application/json",
				Body:        body,
			},
		)
		if err != nil {
			utils.Logger.WithFields(logrus.Fields{
				"product_id": product.ID,
				"image_url":  imageURL,
			}).WithError(err).Error("Failed to publish image for processing")
		} else {
			utils.Logger.WithFields(logrus.Fields{
				"product_id": product.ID,
				"image_url":  imageURL,
			}).Info("Image published successfully for processing")
		}
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"product_id": product.ID})
	utils.Logger.WithField("product_id", product.ID).Info("Product added successfully")
}

func GetProductByID(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	ctx := context.Background()
	id := r.URL.Path[len("/products/"):]

	// Convert the ID from the URL into an integer
	productID, err := strconv.Atoi(id)
	if err != nil {
		utils.Logger.WithFields(logrus.Fields{
			"error":    err.Error(),
			"method":   r.Method,
			"endpoint": r.URL.Path,
		}).Error("Invalid product ID")
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	// Check Redis cache for the product
	cacheKey := "product:" + strconv.Itoa(productID)
	cachedProduct, err := config.RDB.Get(ctx, cacheKey).Result()
	if err == nil {
		utils.Logger.WithFields(logrus.Fields{
			"method":    r.Method,
			"endpoint":  r.URL.Path,
			"cache_hit": true,
		}).Info("Cache hit for product")

		// Reset TTL on cache hit
		err := config.RDB.Expire(ctx, cacheKey, 10*time.Minute).Err()
		if err != nil {
			utils.Logger.WithFields(logrus.Fields{
				"error":    err.Error(),
				"method":   r.Method,
				"endpoint": r.URL.Path,
			}).Error("Failed to reset TTL")
			http.Error(w, "Failed to reset cache TTL", http.StatusInternalServerError)
			return
		}

		// Cache hit: Return the cached product
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(cachedProduct))
		return
	}

	utils.Logger.WithFields(logrus.Fields{
		"method":    r.Method,
		"endpoint":  r.URL.Path,
		"cache_hit": false,
	}).Info("Cache miss for product")

	// Cache miss: Query the database
	query := `SELECT product_id, user_id, product_name, product_description, product_images, compressed_product_images, product_price
              FROM products WHERE product_id = $1`

	var product models.Product
	var productImages, compressedImages []string

	err = config.DB.QueryRow(query, productID).Scan(
		&product.ID,
		&product.UserID,
		&product.ProductName,
		&product.ProductDescription,
		pq.Array(&productImages),
		pq.Array(&compressedImages),
		&product.ProductPrice,
	)

	if err == sql.ErrNoRows {
		utils.Logger.WithFields(logrus.Fields{
			"error":    "Product not found",
			"method":   r.Method,
			"endpoint": r.URL.Path,
		}).Error("Product not found")
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	} else if err != nil {
		utils.Logger.WithFields(logrus.Fields{
			"error":    err.Error(),
			"method":   r.Method,
			"endpoint": r.URL.Path,
		}).Error("Failed to query product")

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Assign the scanned arrays to the Product struct
	product.ProductImages = productImages
	product.CompressedProductImages = compressedImages

	// Convert the product to JSON
	productJSON, err := json.Marshal(product)
	if err != nil {
		utils.Logger.WithFields(logrus.Fields{
			"error":    err.Error(),
			"method":   r.Method,
			"endpoint": r.URL.Path,
		}).Error("Failed to encode product")
		http.Error(w, "Failed to encode product", http.StatusInternalServerError)
		return
	}

	// Store the product in Redis cache with a TTL (e.g., 10 minutes)
	config.RDB.Set(ctx, cacheKey, productJSON, 10*time.Minute)

	// Log response time
	utils.Logger.WithFields(logrus.Fields{
		"method":        r.Method,
		"endpoint":      r.URL.Path,
		"response_time": time.Since(startTime),
	}).Info("Product fetched successfully")

	// Respond with the product as JSON
	w.Header().Set("Content-Type", "application/json")
	w.Write(productJSON)
}
