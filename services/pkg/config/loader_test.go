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

package config_test

import (
	"os"
	"testing"

	"github.com/osv/pkg/config"
)

type testConfig struct {
	RedisAddr string `env:"REDIS_ADDR,required"`
	GRPCPort  string `env:"GRPC_PORT"`
	HTTPPort  string
	Debug     bool
	Workers   int
}

func TestLoad_RequiredMissing(t *testing.T) {
	// Note: cannot use t.Parallel() with t.Setenv() (enforced since Go 1.21)
	t.Setenv("REDIS_ADDR", "")

	_, err := config.Load[testConfig]("")
	if err == nil {
		t.Fatal("expected error for missing required field, got nil")
	}
	if want := "REDIS_ADDR"; !contains(err.Error(), want) {
		t.Errorf("error %q does not mention field name %q", err.Error(), want)
	}
}

func TestLoad_RequiredSet(t *testing.T) {
	// Note: cannot use t.Parallel() with t.Setenv() (enforced since Go 1.21)
	t.Setenv("REDIS_ADDR", "redis:6379")

	cfg, err := config.Load[testConfig]("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RedisAddr != "redis:6379" {
		t.Errorf("RedisAddr = %q, want %q", cfg.RedisAddr, "redis:6379")
	}
}

func TestLoad_EnvOverridesDefault(t *testing.T) {
	// Note: cannot use t.Parallel() with t.Setenv() (enforced since Go 1.21)
	t.Setenv("REDIS_ADDR", "myredis:6380")
	t.Setenv("GRPC_PORT", "9999")
	t.Setenv("HTTP_PORT", "8888") // auto-derived name

	cfg, err := config.Load[testConfig]("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.GRPCPort != "9999" {
		t.Errorf("GRPCPort = %q, want %q", cfg.GRPCPort, "9999")
	}
	if cfg.HTTPPort != "8888" {
		t.Errorf("HTTPPort = %q, want %q", cfg.HTTPPort, "8888")
	}
}

func TestLoad_BoolAndInt(t *testing.T) {
	// Note: cannot use t.Parallel() with t.Setenv() (enforced since Go 1.21)
	t.Setenv("REDIS_ADDR", "redis:6379")
	t.Setenv("DEBUG", "true")
	t.Setenv("WORKERS", "4")

	cfg, err := config.Load[testConfig]("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Debug {
		t.Error("Debug should be true")
	}
	if cfg.Workers != 4 {
		t.Errorf("Workers = %d, want 4", cfg.Workers)
	}
}

func TestLoad_YAMLFile(t *testing.T) {
	// Note: cannot use t.Parallel() with t.Setenv() (enforced since Go 1.21)
	// Write a temp YAML config
	f, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("redis_addr: yaml-redis:6379\ngrpc_port: \"7777\"\n")
	_ = f.Close()

	// Env should override YAML
	t.Setenv("REDIS_ADDR", "env-redis:6380")

	cfg, err := config.Load[testConfig](f.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Env takes priority
	if cfg.RedisAddr != "env-redis:6380" {
		t.Errorf("RedisAddr = %q, want env override %q", cfg.RedisAddr, "env-redis:6380")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := range len(s) - len(sub) + 1 {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
