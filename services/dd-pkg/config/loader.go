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

// Package config provides configuration loading utilities.
//
// Priority: 1. Env vars (SCREAMING_SNAKE_CASE) 2. config.yaml 3. struct defaults.
//
// # Struct Tags
//
// Fields may be annotated with `env` struct tags:
//
//	type Config struct {
//	    // Required: startup fails with a clear error if REDIS_ADDR is not set
//	    RedisAddr string `env:"REDIS_ADDR,required"`
//
//	    // Optional with default; env var name is derived from field name (GRPC_PORT)
//	    GRPCPort  string `env:"GRPC_PORT"`
//
//	    // No tag: env var name is derived automatically (HTTP_PORT)
//	    HTTPPort  string
//	}
package config

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads configuration into T from the given YAML file path,
// then overlays environment variable overrides.
// Returns an error if any field tagged with `env:",required"` is empty
// after all sources have been applied.
func Load[T any](path string) (*T, error) {
	var cfg T

	// 1. Load from YAML file (if exists)
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("config: read file %q: %w", path, err)
		}
		if err == nil {
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return nil, fmt.Errorf("config: parse YAML %q: %w", path, err)
			}
		}
	}

	// 2. Overlay env vars (SCREAMING_SNAKE_CASE, using struct tags when present)
	applyEnv(reflect.TypeOf(cfg), reflect.ValueOf(&cfg).Elem(), "")

	// 3. Validate required fields
	if err := validateRequired(reflect.TypeOf(cfg), reflect.ValueOf(cfg), ""); err != nil {
		return nil, fmt.Errorf("config: missing required fields:\n%w", err)
	}

	return &cfg, nil
}

// MustLoad is like Load but panics on error. Useful in main() when a config
// error is unrecoverable.
func MustLoad[T any](path string) *T {
	cfg, err := Load[T](path)
	if err != nil {
		panic(err)
	}
	return cfg
}

// envKey returns the environment variable name for a struct field.
// Priority: `env:"KEY"` tag > SCREAMING_SNAKE_CASE of field name.
// The prefix is prepended with underscore separator when non-empty.
func envKey(field reflect.StructField, prefix string) (key string, required bool) {
	tag := field.Tag.Get("env")
	parts := strings.SplitN(tag, ",", 2)
	name := strings.TrimSpace(parts[0])
	if len(parts) == 2 && strings.TrimSpace(parts[1]) == "required" {
		required = true
	}

	if name != "" && name != "-" {
		key = name
	} else {
		// Convert CamelCase to SCREAMING_SNAKE_CASE: HTTPPort → HTTP_PORT
		key = camelToScreamingSnake(field.Name)
	}

	if prefix != "" {
		key = prefix + "_" + key
	}
	return key, required
}

// camelToScreamingSnake converts a CamelCase identifier to SCREAMING_SNAKE_CASE.
// Examples: HTTPPort → HTTP_PORT, GRPCAddr → GRPC_ADDR, RedisAddr → REDIS_ADDR.
func camelToScreamingSnake(s string) string {
	var result strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 {
			prev := runes[i-1]
			// Insert underscore before a transition from lowercase to uppercase,
			// or from a sequence of uppercase letters into a lowercase letter (e.g. HTTPPort → HTTP_Port).
			if isUpper(r) && isLower(prev) {
				result.WriteByte('_')
			} else if isUpper(r) && i+1 < len(runes) && isLower(runes[i+1]) && isUpper(prev) {
				result.WriteByte('_')
			}
		}
		result.WriteRune(r)
	}
	return strings.ToUpper(result.String())
}

func isUpper(r rune) bool { return r >= 'A' && r <= 'Z' }
func isLower(r rune) bool { return r >= 'a' && r <= 'z' }

// applyEnv walks struct fields recursively and sets values from environment variables.
func applyEnv(rt reflect.Type, rv reflect.Value, prefix string) {
	for i := range rt.NumField() {
		field := rt.Field(i)
		fv := rv.Field(i)

		if !fv.CanSet() {
			continue
		}

		key, _ := envKey(field, prefix)

		switch field.Type.Kind() { //nolint:exhaustive
		case reflect.Struct:
			applyEnv(field.Type, fv, key)

		case reflect.String:
			if val := os.Getenv(key); val != "" {
				fv.SetString(val)
			}

		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if val := os.Getenv(key); val != "" {
				var n int64
				if _, err := fmt.Sscanf(val, "%d", &n); err == nil {
					fv.SetInt(n)
				}
			}

		case reflect.Bool:
			if val := os.Getenv(key); val != "" {
				fv.SetBool(val == "true" || val == "1" || val == "yes")
			}

		case reflect.Float64:
			if val := os.Getenv(key); val != "" {
				var f float64
				if _, err := fmt.Sscanf(val, "%f", &f); err == nil {
					fv.SetFloat(f)
				}
			}
		}
	}
}

// validateRequired checks that all fields annotated with `env:",required"` are
// non-zero after env overlay. Returns a joined multi-error describing every
// missing field so the operator sees all problems at once.
func validateRequired(rt reflect.Type, rv reflect.Value, prefix string) error {
	var errs []error

	for i := range rt.NumField() {
		field := rt.Field(i)
		fv := rv.Field(i)

		key, required := envKey(field, prefix)

		if field.Type.Kind() == reflect.Struct {
			if err := validateRequired(field.Type, fv, key); err != nil {
				errs = append(errs, err)
			}
			continue
		}

		if required && fv.IsZero() {
			errs = append(errs, fmt.Errorf("  - %s (env: %s) is required but not set", field.Name, key))
		}
	}

	return errors.Join(errs...)
}
