package handlers_test

import (
	"bytes"

	"encoding/json"

	"net/http"
	"net/http/httptest"

	"testing"
    "github.com/lib/pq"

	"backend/config"
	"backend/handlers"
	"backend/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-redis/redismock/v9"

	"github.com/stretchr/testify/assert"

	
	"strconv"

	"time"

	

)

func TestGetProducts(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	config.DB = db

	mockRows := sqlmock.NewRows([]string{"product_id", "user_id", "product_name", "product_description", "product_images", "compressed_product_images", "product_price"}).
		AddRow(1, 1, "Product A", "Description A", `{"image1.jpg", "image2.jpg"}`, `{"compressed1.jpg"}`, 100.0).
		AddRow(2, 2, "Product B", "Description B", `{"image3.jpg"}`, `{"compressed2.jpg"}`, 200.0)

	mock.ExpectQuery("SELECT product_id, user_id, product_name, product_description, product_images, compressed_product_images, product_price FROM products WHERE 1=1").
		WillReturnRows(mockRows)

	req := httptest.NewRequest(http.MethodGet, "/products", nil)
	w := httptest.NewRecorder()

	handlers.GetProducts(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var products []models.Product
	err = json.NewDecoder(resp.Body).Decode(&products)
	assert.NoError(t, err)
	assert.Len(t, products, 2)
}

func TestAddProduct(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	config.DB = db

	redisMock, _ := redismock.NewClientMock()
	config.RDB = redisMock

	product := models.Product{
		UserID:             1,
		ProductName:        "New Product",
		ProductDescription: "New Product Description",
		ProductImages:      []string{"image1.jpg"},
		ProductPrice:       150.0,
	}

	mock.ExpectQuery("INSERT INTO products").WithArgs(product.UserID, product.ProductName, product.ProductDescription, sqlmock.AnyArg(), sqlmock.AnyArg(), product.ProductPrice).
		WillReturnRows(sqlmock.NewRows([]string{"product_id"}).AddRow(1))

	body, _ := json.Marshal(product)
	req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handlers.AddProduct(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, float64(1), response["product_id"])
}

func TestGetProductByID(t *testing.T) {
	// Set up mocks for Redis and SQL
	db, sqlMock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	config.DB = db

	redisMock, redisExpect := redismock.NewClientMock()
	config.RDB = redisMock

	productID := 21
	cacheKey := "product:" + strconv.Itoa(productID)

	product := models.Product{
		ID:                      productID,
		UserID:                  1,
		ProductName:             "Test Product",
		ProductDescription:      "A sample product for testing",
		ProductImages:           []string{"image1.jpg", "image2.jpg"},
		CompressedProductImages: []string{"compressed1.jpg", "compressed2.jpg"},
		ProductPrice:            99.99,
	}
	productJSON, _ := json.Marshal(product)

	t.Run("Cache Hit", func(t *testing.T) {
		// Mock Redis cache hit
		redisExpect.ExpectGet(cacheKey).SetVal(string(productJSON))
		redisExpect.ExpectExpire(cacheKey, 10*time.Minute).SetVal(true)

		// Create a test request and response recorder
		req := httptest.NewRequest(http.MethodGet, "/products/"+strconv.Itoa(productID), nil)
		w := httptest.NewRecorder()

		// Call the handler
		handlers.GetProductByID(w, req)

		// Verify Redis mock expectations
		assert.NoError(t, redisExpect.ExpectationsWereMet())

		// Validate HTTP response
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var fetchedProduct models.Product
		err := json.NewDecoder(resp.Body).Decode(&fetchedProduct)
		assert.NoError(t, err)
		assert.Equal(t, product, fetchedProduct)
	})

	t.Run("Cache Miss", func(t *testing.T) {
		// Mock Redis cache miss
		redisExpect.ExpectGet(cacheKey).RedisNil()

		// Mock database query
		sqlMock.ExpectQuery(`SELECT product_id, user_id, product_name, product_description, product_images, compressed_product_images, product_price`).
			WithArgs(productID).
			WillReturnRows(sqlmock.NewRows([]string{"product_id", "user_id", "product_name", "product_description", "product_images", "compressed_product_images", "product_price"}).
				AddRow(product.ID, product.UserID, product.ProductName, product.ProductDescription, pq.Array(product.ProductImages), pq.Array(product.CompressedProductImages), product.ProductPrice))

		// Mock Redis SET operation to store fetched product
		redisExpect.ExpectSet(cacheKey, productJSON, 10*time.Minute).SetVal("OK")

		// Create a test request and response recorder
		req := httptest.NewRequest(http.MethodGet, "/products/"+strconv.Itoa(productID), nil)
		w := httptest.NewRecorder()

		// Call the handler
		handlers.GetProductByID(w, req)

		// Verify Redis and SQL mock expectations
		assert.NoError(t, redisExpect.ExpectationsWereMet())
		assert.NoError(t, sqlMock.ExpectationsWereMet())

		// Validate HTTP response
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var fetchedProduct models.Product
		err := json.NewDecoder(resp.Body).Decode(&fetchedProduct)
		assert.NoError(t, err)
		assert.Equal(t, product, fetchedProduct)
	})
}
