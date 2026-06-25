// Package grpcclient — notification_client.go
// NotificationClient wraps the notification-service gRPC NotificationServiceClient.
// Used by gateway to dispatch security alerts when KEV/SLA events occur.
package grpcclient

import (
	"context"
	"time"

	"google.golang.org/grpc"

	notifv1 "github.com/osv/shared/proto/gen/go/notification/v1"
)

// NotificationClient wraps the notification-service gRPC NotificationServiceClient.
type NotificationClient struct {
	conn   *grpc.ClientConn
	client notifv1.NotificationServiceClient
}

// NewNotificationClient creates a new NotificationClient connected to addr.
func NewNotificationClient(addr string, opts ...grpc.DialOption) (*NotificationClient, error) {
	all := append(defaultDialOptions(), opts...)
	conn, err := grpc.NewClient(addr, all...)
	if err != nil {
		return nil, err
	}
	return &NotificationClient{
		conn:   conn,
		client: notifv1.NewNotificationServiceClient(conn),
	}, nil
}

// Close tears down the underlying gRPC connection.
func (c *NotificationClient) Close() error { return c.conn.Close() }

// SendNotification dispatches a notification event to the notification-service.
// Returns (notificationID, error).
func (c *NotificationClient) SendNotification(ctx context.Context, req *notifv1.SendNotificationRequest) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := c.client.SendNotification(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.GetNotificationId(), nil
}

// GetAlerts retrieves recent alerts for an entity (e.g. a CVE ID or product ID).
func (c *NotificationClient) GetAlerts(ctx context.Context, entityID string, limit int32) ([]*notifv1.Alert, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := c.client.GetAlerts(ctx, &notifv1.GetAlertsRequest{
		EntityId: entityID,
		Limit:    limit,
	})
	if err != nil {
		return nil, err
	}
	return resp.GetAlerts(), nil
}

// AcknowledgeAlert marks an alert as acknowledged.
func (c *NotificationClient) AcknowledgeAlert(ctx context.Context, alertID string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := c.client.AcknowledgeAlert(ctx, &notifv1.AcknowledgeAlertRequest{
		AlertId: alertID,
	})
	return err
}
