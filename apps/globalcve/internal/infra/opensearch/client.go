// Package opensearch provides a shared OpenSearch client.
package opensearch

import (
	"crypto/tls"
	"fmt"
	"net/http"

	opensearchgo "github.com/opensearch-project/opensearch-go/v4"

	"github.com/globalcve/mono/internal/config"
)

// NewClient creates a new OpenSearch client.
func NewClient(cfg config.OpenSearchConfig) (*opensearchgo.Client, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // dev mode
	}

	client, err := opensearchgo.NewClient(opensearchgo.Config{
		Addresses: []string{cfg.URL},
		Transport: transport,
	})
	if err != nil {
		return nil, fmt.Errorf("create opensearch client: %w", err)
	}

	return client, nil
}
