// Package testutil — setup.go
// Shared test utilities và infrastructure setup cho integration tests.
package testutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestConfig cấu hình cho test environment.
// Sử dụng docker-compose test stack (localhost).
type TestConfig struct {
	DatabaseURL string
	RedisURL    string
	NATSURL     string
}

// DefaultTestConfig trả về cấu hình mặc định cho test.
func DefaultTestConfig() TestConfig {
	return TestConfig{
		DatabaseURL: "postgres://openvulnscan:secret@localhost:5432/openvulnscan_test",
		RedisURL:    "redis://localhost:6379",
		NATSURL:     "nats://localhost:4222",
	}
}

// DoRequest thực hiện HTTP request với JWT token.
func DoRequest(t *testing.T, method, url, token, body string) *http.Response {
	t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		t.Fatalf("DoRequest: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DoRequest %s %s: %v", method, url, err)
	}
	return resp
}

// LoginAndGetToken đăng nhập và trả về access token.
func LoginAndGetToken(t *testing.T, baseURL, email, password string) string {
	t.Helper()
	payload := fmt.Sprintf(`{"email":%q,"password":%q}`, email, password)
	resp, err := http.Post(baseURL+"/api/v1/auth/login", "application/json", bytes.NewBufferString(payload))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("login %d: %s", resp.StatusCode, body)
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result) //nolint:errcheck
	token, ok := result["access_token"].(string)
	if !ok || token == "" {
		t.Fatal("login: no access_token in response")
	}
	return token
}

// WaitForScanCompletion polls scan status until completed/failed/timeout.
func WaitForScanCompletion(t *testing.T, baseURL, token, scanID string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp := DoRequest(t, "GET", baseURL+"/api/v1/scans/"+scanID, token, "")
		if resp.StatusCode == http.StatusOK {
			var scan map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&scan) //nolint:errcheck
			resp.Body.Close()
			if status, ok := scan["status"].(string); ok {
				if status == "completed" || status == "failed" || status == "cancelled" {
					return status
				}
			}
		} else {
			resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}
	t.Logf("WaitForScanCompletion: timeout after %v", timeout)
	return "timeout"
}

// ReadJSON đọc và decode JSON response body.
func ReadJSON(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	defer resp.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Logf("ReadJSON decode warning: %v", err)
	}
	return result
}

// AssertStatusCode kiểm tra HTTP status code.
func AssertStatusCode(t *testing.T, expected, got int, context string) {
	t.Helper()
	if expected != got {
		t.Errorf("%s: expected status %d, got %d", context, expected, got)
	}
}

// AssertContainsKey kiểm tra JSON response chứa key.
func AssertContainsKey(t *testing.T, m map[string]interface{}, key, context string) {
	t.Helper()
	if _, ok := m[key]; !ok {
		t.Errorf("%s: expected key %q in response %v", context, key, m)
	}
}

// SkipIfNoInfra skip test nếu không kết nối được infra.
func SkipIfNoInfra(t *testing.T, baseURL string) {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(baseURL + "/health")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Skip("skipping: test infrastructure not available")
	}
	if resp != nil {
		resp.Body.Close()
	}
}
