package models

type Product struct {
	ID                      int      `json:"product_id"`
	UserID                  int      `json:"user_id"`
	ProductName             string   `json:"product_name"`
	ProductDescription      string   `json:"product_description"`
	ProductImages           []string `json:"product_images"`
	CompressedProductImages []string `json:"compressed_product_images"`
	ProductPrice            float64  `json:"product_price"`
}
