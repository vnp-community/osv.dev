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

// Package testutil provides test helpers for OSV microservices.
package testutil

import (
	"context"
	"sync"
)

// InMemoryStore is a generic thread-safe in-memory store for testing.
type InMemoryStore[K comparable, V any] struct {
	mu    sync.RWMutex
	items map[K]V
}

// NewInMemoryStore creates a new empty InMemoryStore.
func NewInMemoryStore[K comparable, V any]() *InMemoryStore[K, V] {
	return &InMemoryStore[K, V]{items: make(map[K]V)}
}

// Set stores a value.
func (s *InMemoryStore[K, V]) Set(_ context.Context, key K, value V) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[key] = value
	return nil
}

// Get retrieves a value. Returns the zero value and false if not found.
func (s *InMemoryStore[K, V]) Get(_ context.Context, key K) (V, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.items[key]
	return v, ok
}

// Delete removes a value.
func (s *InMemoryStore[K, V]) Delete(_ context.Context, key K) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, key)
}

// List returns all stored items as a slice.
func (s *InMemoryStore[K, V]) List(_ context.Context) []V {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]V, 0, len(s.items))
	for _, v := range s.items {
		result = append(result, v)
	}
	return result
}

// Count returns the number of items.
func (s *InMemoryStore[K, V]) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

// FakeEventPublisher records published events for test assertions.
type FakeEventPublisher struct {
	mu     sync.Mutex
	events []PublishedEvent
}

// PublishedEvent records a single published event.
type PublishedEvent struct {
	Topic   string
	Payload interface{}
}

// Publish records the event.
func (f *FakeEventPublisher) Publish(_ context.Context, topic string, payload interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, PublishedEvent{Topic: topic, Payload: payload})
	return nil
}

// Events returns all recorded events.
func (f *FakeEventPublisher) Events() []PublishedEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]PublishedEvent, len(f.events))
	copy(out, f.events)
	return out
}

// EventsForTopic returns all events published to the given topic.
func (f *FakeEventPublisher) EventsForTopic(topic string) []PublishedEvent {
	all := f.Events()
	var result []PublishedEvent
	for _, e := range all {
		if e.Topic == topic {
			result = append(result, e)
		}
	}
	return result
}

// Reset clears all recorded events.
func (f *FakeEventPublisher) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = nil
}
