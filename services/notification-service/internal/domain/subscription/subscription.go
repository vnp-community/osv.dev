package subscription

import "time"

// SubscriptionType constants
type SubscriptionType string

const (
	SubscriptionVendor  SubscriptionType = "vendor"
	SubscriptionProduct SubscriptionType = "product"
	SubscriptionKEV     SubscriptionType = "kev"
)

// AlertSubscription for vendor/product/kev alerts.
type AlertSubscription struct {
	ID          string
	OwnerID     string
	Type        SubscriptionType // "vendor"|"product"|"kev"
	Value       string           // e.g. "apache" for vendor
	MinSeverity string           // "CRITICAL"|"HIGH"|"MEDIUM"|"LOW"
	MinEPSS     *float64
	IsActive    bool
	CreatedAt   time.Time
}
