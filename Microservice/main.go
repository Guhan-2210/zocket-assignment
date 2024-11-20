package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log"
	"net/http"
	// "net/url"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/joho/godotenv"
	"github.com/rabbitmq/amqp091-go"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"database/sql"

	_ "github.com/lib/pq"
)

var (
	host     = "localhost"
	port     = 5432
	user     = "postgres"
	password = os.Getenv("DB_PASSWORD")
	dbname   = "zocket"
)

var db *sql.DB

// RabbitMQ settings
const (
	rabbitMQURL = "amqp://guest:guest@localhost:5672/"
	queueName   = "image_processing"
)

// AWS S3 settings
var (
	awsRegion    = os.Getenv("AWS_REGION")
	awsAccessKey = os.Getenv("AWS_ACCESS_KEY")
	awsSecretKey = os.Getenv("AWS_SECRET_KEY")
	s3Bucket     = os.Getenv("S3_BUCKET")
	imageQuality = 50
)

type ImageMessage struct {
	ImageURL  string `json:"image_url"`
	ProductID int    `json:"product_id"`
}

// Helper function to connect to RabbitMQ
func connectToRabbitMQ() (*amqp091.Connection, *amqp091.Channel, error) {
	conn, err := amqp091.Dial(rabbitMQURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to RabbitMQ: %v", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open RabbitMQ channel: %v", err)
	}

	return conn, ch, nil
}

// Compress the image
func compressImage(src io.Reader, quality int) (*bytes.Buffer, error) {
	img, _, err := image.Decode(src)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %v", err)
	}

	var buf bytes.Buffer
	options := &jpeg.Options{Quality: quality}
	if err := jpeg.Encode(&buf, img, options); err != nil {
		return nil, fmt.Errorf("failed to encode image: %v", err)
	}

	return &buf, nil
}

// Download image from URL
func downloadImage(url string) (io.ReadCloser, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image from URL: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-200 response: %d", resp.StatusCode)
	}
	return resp.Body, nil
}
func sanitizeFileName(fileName string) string {
	replacements := map[string]string{
		"?": "_",
		"&": "_",
		"=": "_",
		"%": "_",
		" ": "_",
	}
	for old, new := range replacements {
		fileName = strings.ReplaceAll(fileName, old, new)
	}
	return fileName
}

// Upload image to S3
func uploadToS3(bucket, key string, file io.ReadSeeker) (string, error) {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(awsRegion),
		Credentials: credentials.NewStaticCredentials(awsAccessKey, awsSecretKey, ""),
	})
	if err != nil {
		return "", fmt.Errorf("failed to create AWS session: %v", err)
	}

	s3Client := s3.New(sess)
	encodedKey := sanitizeFileName(key)

	_, err = s3Client.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(encodedKey),
		Body:        file,
		ContentType: aws.String("image/jpeg"),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %v", err)
	}

	return fmt.Sprintf("https://%s.s3.amazonaws.com/%s", bucket, encodedKey), nil
}

// Process an image
func processImage(imageURL string, bucket string, quality int) (string, error) {
	imgReader, err := downloadImage(imageURL)
	if err != nil {
		return "", fmt.Errorf("error downloading image %s: %v", imageURL, err)
	}
	defer imgReader.Close()

	compressedImage, err := compressImage(imgReader, quality)
	if err != nil {
		return "", fmt.Errorf("error compressing image %s: %v", imageURL, err)
	}

	compressedImageReader := bytes.NewReader(compressedImage.Bytes())
	outputFile := filepath.Base(imageURL) + "_compressed.jpg"

	return uploadToS3(bucket, outputFile, compressedImageReader)
}

func updateCompressedImagesInDB(productID int, s3URL string) error {
	pgConnStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)

	conn, err := sql.Open("postgres", pgConnStr)
	if err != nil {
		return fmt.Errorf("failed to connect to the database: %v", err)
	}
	defer conn.Close()

	query := `UPDATE products
              SET compressed_product_images = array_append(compressed_product_images, $1)
              WHERE product_id = $2`

	_, err = conn.Exec(query, s3URL, productID)
	if err != nil {
		return fmt.Errorf("failed to update product ID %d with S3 URL: %v", productID, err)
	}
	return nil
}

func processQueueMessages(ch *amqp091.Channel, queue string, wg *sync.WaitGroup) {
	defer wg.Done()

	msgs, err := ch.Consume(
		queue,
		"",    // Consumer tag
		true,  // Auto-ack
		false, // Exclusive
		false, // No-local
		false, // No-wait
		nil,   // Args
	)
	if err != nil {
		log.Fatalf("Failed to register consumer: %v", err)
	}

	for msg := range msgs {
		var imageMessage ImageMessage
		err := json.Unmarshal(msg.Body, &imageMessage)
		if err != nil {
			log.Printf("Invalid message format: %v", err)
			continue
		}

		imageURL := imageMessage.ImageURL
		productID := imageMessage.ProductID
		log.Printf("Processing image: %s for product ID: %d", imageURL, productID)

		// Process the image (compress and upload to S3)
		s3URL, err := processImage(imageURL, s3Bucket, imageQuality)
		if err != nil {
			log.Printf("Error processing image %s: %v", imageURL, err)
			continue
		}

		log.Printf("Image for product ID %d successfully uploaded to S3: %s", productID, s3URL)

		// Update the database with the compressed image URL
		err = updateCompressedImagesInDB(productID, s3URL)
		if err != nil {
			log.Printf("Error updating database for product ID %d: %v", productID, err)
		} else {
			log.Printf("Database updated for product ID %d with S3 URL: %s", productID, s3URL)
		}
	}
}

func main() {

	// Load environment variables from .env file
	err := godotenv.Load("../.env")
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Continue with RabbitMQ setup and worker initialization
	conn, ch, err := connectToRabbitMQ()
	if err != nil {
		log.Fatalf("Failed to set up RabbitMQ connection: %v", err)
	}
	defer conn.Close()
	defer ch.Close()

	// Ensure the queue exists
	_, err = ch.QueueDeclare(
		queueName,
		true,  // Durable
		false, // Delete when unused
		false, // Exclusive
		false, // No-wait
		nil,   // Args
	)
	if err != nil {
		log.Fatalf("Failed to declare queue: %v", err)
	}

	// Start worker to process queue messages
	var wg sync.WaitGroup
	wg.Add(1)
	go processQueueMessages(ch, queueName, &wg)

	log.Println("Image processing microservice is running...")
	wg.Wait()
}
