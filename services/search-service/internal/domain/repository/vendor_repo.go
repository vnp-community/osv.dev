package repository

import "context"

// VendorEntry contains vendor metadata from CPE dictionary.
type VendorEntry struct {
	Vendor       string `db:"vendor"        json:"vendor"`
	ProductCount int    `db:"product_count" json:"product_count"`
}

// VendorRepository queries vendor/product data from CPE dictionary.
type VendorRepository interface {
	ListVendors(ctx context.Context, q string, limit int) ([]*VendorEntry, int64, error)
	GetProductsByVendor(ctx context.Context, vendor string) ([]string, error)
	ListProducts(ctx context.Context, vendor, q string, limit int) ([]string, error)
}
