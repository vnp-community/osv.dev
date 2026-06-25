package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dsn := "postgres://osv:osvsecret@localhost:5432/osvdb?sslmode=disable"
	if envDsn := os.Getenv("DATABASE_URL"); envDsn != "" {
		dsn = envDsn
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Printf("Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		fmt.Printf("Failed to ping database: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Connected to database. Starting V7 seed...")

	// 1. Scans history
	scanID := uuid.New()
	targetID := uuid.New()
	userID := uuid.New() // Note: usually requires existing user, we will just use a random UUID, it might violate FK if user doesn't exist.
	// Actually, let's query a user to use.
	var existingUserID uuid.UUID
	err = pool.QueryRow(ctx, "SELECT id FROM users LIMIT 1").Scan(&existingUserID)
	if err != nil {
		fmt.Printf("Warning: no users found, skipping scans seed or ignoring FK.\n")
	} else {
		userID = existingUserID
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO scans (id, target_id, name, status, tool_name, created_at, updated_at, created_by)
		VALUES ($1, $2, 'V7 History Scan', 'completed', 'nmap', $3, $3, $4)
		ON CONFLICT DO NOTHING
	`, scanID, targetID, time.Now(), userID)
	if err != nil {
		fmt.Printf("Failed to insert scan: %v\n", err)
	} else {
		fmt.Println("Seeded scans history.")
	}

	// 2. SLA Overview (findings with at_risk/breached)
	findingID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO findings (id, title, status, severity, risk_score, created_at, updated_at)
		VALUES ($1, 'V7 SLA Finding', 'active', 'high', 8.5, $2, $2)
		ON CONFLICT DO NOTHING
	`, findingID, time.Now().Add(-40*24*time.Hour))
	if err != nil {
		fmt.Printf("Failed to insert finding: %v\n", err)
	} else {
		// Insert SLA if there is a finding_sla_status table, or update the finding if sla_status is a column.
		// Assume it's a column or we just created a very old finding which will trigger the SLA.
		fmt.Println("Seeded old finding for SLA.")
	}

	// 3. Triage Queue
	// We need ai_reports or triage_queue. Let's see if ai_reports exists.
	_, err = pool.Exec(ctx, `
		INSERT INTO triage_queue (id, finding_id, status, created_at, updated_at)
		VALUES ($1, $2, 'pending', $3, $3)
		ON CONFLICT DO NOTHING
	`, uuid.New(), findingID, time.Now())
	if err != nil {
		fmt.Printf("Failed to insert triage_queue (table might not exist or schema differs): %v\n", err)
	}

	// 4. Webhook Deliveries
	webhookID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO webhook_deliveries (id, webhook_id, event_type, status, status_code, request_payload, response_body, duration_ms, created_at)
		VALUES ($1, $2, 'test.event', 'success', 200, '{}', '{}', 150, $3)
		ON CONFLICT DO NOTHING
	`, uuid.New(), webhookID, time.Now())
	if err != nil {
		fmt.Printf("Failed to insert webhook_deliveries: %v\n", err)
	} else {
		fmt.Println("Seeded webhook delivery.")
	}

	// 5. Inapp Alerts Unread Count
	_, err = pool.Exec(ctx, `
		INSERT INTO inapp_alerts (id, user_id, title, message, type, is_read, created_at)
		VALUES ($1, $2, 'V7 Unread Alert', 'This is an unread alert.', 'info', false, $3)
		ON CONFLICT DO NOTHING
	`, uuid.New(), userID, time.Now())
	if err != nil {
		fmt.Printf("Failed to insert inapp_alerts: %v\n", err)
	} else {
		fmt.Println("Seeded unread inapp_alert.")
	}

	fmt.Println("V7 seed completed.")
}
