# Zocket SDE Assignment

The Product Management Backend System is a high-performance RESTful API built with Golang, designed for efficient product data management. It features PostgreSQL for storage, asynchronous image processing with message queues, and Redis caching for optimal performance. With robust logging, error handling, and scalability, it ensures reliability and real-time responsiveness.

## Demo Video
[Click Here](https://www.loom.com/share/9666e16cb57e410e903504d7d8429f70?sid=ad7f4082-a280-4672-855f-a1c1105f255b)

## System Architecture

The system consists of two main components:

1. *Backend Service*: Handles product and user management with REST APIs
2. *Image Processing Microservice*: Processes product images asynchronously

### Technology Stack

- *Programming Language*: Go 1.23.3
- *Database*: PostgreSQL
- *Cache*: Redis
- *Message Queue*: RabbitMQ
- *Cloud Storage*: AWS S3
- *Testing*: Go testing framework with mocking

## Features

- User management (CRUD operations)
- Product management with image handling
- Asynchronous image processing and compression
- Redis caching for improved performance
- AWS S3 integration for image storage
- Comprehensive logging system
- Request middleware for logging and monitoring

## Prerequisites

- Go 1.23.3 or higher
- PostgreSQL
- Redis
- RabbitMQ
- AWS Account with S3 access

## Configuration

### Environment Variables Required (Create a .env file)
```
DB_PASSWORD=your_db_password
AWS_ACCESS_KEY=your_aws_access_key
AWS_SECRET_KEY=your_aws_secret_key
S3_BUCKET=your_s3_bucket_name
```

### Database Configuration
```
CREATE DATABASE zocket;
CREATE TABLE users (
user_id SERIAL PRIMARY KEY,
name VARCHAR(255) NOT NULL
);

CREATE TABLE products (
product_id SERIAL PRIMARY KEY,
user_id INTEGER REFERENCES users(user_id),
product_name VARCHAR(255) NOT NULL,
product_description TEXT,
product_images TEXT[],
compressed_product_images TEXT[],
product_price DECIMAL(10,2) NOT NULL
);
```

## Installation & Setup

1. Clone the repository:
```
git clone https://github.com/Guhan-2210/zocket-assignment.git
```
2. Install dependencies:
```
go mod download
```

3. Start the backend service:
```
cd Backend
go run main.go
```
4. Start the image processing microservice:
```
cd Microservice
go run main.go
```


## API Endpoints

### Users
- GET /users - Get all users
- POST /users/add - Add a new user

### Products
- GET /products - Get all products (with optional filters)
- POST /products/add - Add a new product
- GET /products/{id} - Get product by ID

### Query Parameters for Products
- user_id - Filter by user
- min_price - Minimum price filter
- max_price - Maximum price filter
- product_name - Search by product name

## Testing

Run the test suite:
```
go test ./... -v
```


## Architecture Details

### Backend Service
- RESTful API implementation
- Redis caching for product details
- PostgreSQL for data persistence
- Request logging middleware
- Error handling utilities

### Image Processing Microservice
- Asynchronous image processing using RabbitMQ
- Image compression functionality
- AWS S3 integration for storage
- Automatic database updates with processed image URLs

## Error Handling

The system implements comprehensive error handling with:
- HTTP status codes
- Detailed error logging
- Client-friendly error messages

## Performance Considerations

- Redis caching for frequently accessed products
- Asynchronous image processing
- Database query optimization
- Connection pooling
