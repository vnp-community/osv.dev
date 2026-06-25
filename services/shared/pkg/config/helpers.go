// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// helpers.go cung cấp các standalone helper functions để đọc env vars với
// warning log khi dùng fallback. Dùng bổ sung cho Load[T] khi không cần
// full YAML config loading.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"
)

// EnvStr đọc env var, trả về defaultVal nếu không được set.
// Log WARN khi dùng fallback để engineer phát hiện misconfiguration trong production.
func EnvStr(envKey, defaultVal string) string {
	v := os.Getenv(envKey)
	if v != "" {
		return v
	}
	if defaultVal != "" {
		slog.Warn("env var not set, using default — configure in production",
			"env_key", envKey,
			"default", defaultVal,
		)
	}
	return defaultVal
}

// EnvStrRequired đọc env var bắt buộc — panic nếu không được set.
// Dùng cho credentials và security-critical config (không có default an toàn).
func EnvStrRequired(envKey string) string {
	v := os.Getenv(envKey)
	if v == "" {
		panic(fmt.Sprintf("config: required environment variable %q is not set", envKey))
	}
	return v
}

// EnvInt đọc env var dạng integer với fallback.
// Bỏ qua giá trị không hợp lệ và dùng defaultVal.
func EnvInt(envKey string, defaultVal int) int {
	v := os.Getenv(envKey)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		slog.Warn("invalid int value for env var, using default",
			"env_key", envKey,
			"value", v,
			"fallback", defaultVal,
		)
		return defaultVal
	}
	return n
}

// EnvDuration đọc env var dạng duration string (e.g. "5m", "30s") với fallback.
// Bỏ qua giá trị không hợp lệ và dùng defaultVal.
func EnvDuration(envKey string, defaultVal time.Duration) time.Duration {
	v := os.Getenv(envKey)
	if v == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		slog.Warn("invalid duration value for env var, using default",
			"env_key", envKey,
			"value", v,
			"fallback", defaultVal.String(),
		)
		return defaultVal
	}
	return d
}

// ServiceAddr đọc địa chỉ service (host:port) từ env var.
// Log WARN khi trỏ về localhost để nhắc nhở cấu hình production.
//
// Ví dụ:
//
//	addr := config.ServiceAddr("FINDING_SERVICE_GRPC", "localhost", 50060)
//	// → "finding-service:50060" nếu env set, hoặc "localhost:50060" + WARN
func ServiceAddr(envKey, defaultHost string, defaultPort int) string {
	v := os.Getenv(envKey)
	if v != "" {
		return v
	}
	fallback := fmt.Sprintf("%s:%d", defaultHost, defaultPort)
	slog.Warn("service address env var not set, using localhost — configure in production",
		"env_key", envKey,
		"fallback", fallback,
	)
	return fallback
}

// HTTPServiceAddr tương tự ServiceAddr nhưng thêm "http://" prefix.
//
// Ví dụ:
//
//	url := config.HTTPServiceAddr("SEARCH_SERVICE_HTTP", "localhost", 8083)
//	// → "http://search-service:8083" nếu env set, hoặc "http://localhost:8083" + WARN
func HTTPServiceAddr(envKey, defaultHost string, defaultPort int) string {
	v := os.Getenv(envKey)
	if v != "" {
		return v
	}
	fallback := fmt.Sprintf("http://%s:%d", defaultHost, defaultPort)
	slog.Warn("HTTP service addr not set, using localhost — configure in production",
		"env_key", envKey,
		"fallback", fallback,
	)
	return fallback
}

// Coalesce trả về giá trị string đầu tiên không rỗng trong danh sách.
// Thứ tự: giá trị từ EmbeddedConfig → env var → hardcode default.
//
// Ví dụ:
//
//	searchURL := config.Coalesce(cfg.SearchAddr, os.Getenv("SEARCH_SERVICE_HTTP"), "http://localhost:8083")
func Coalesce(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
