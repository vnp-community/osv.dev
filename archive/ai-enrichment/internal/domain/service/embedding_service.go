// domain/service/embedding_service.go
package service

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/osv/ai-enrichment/internal/application/port"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

const (
	maxTextLength  = 8000 // chars to send to embedding API
	embedCacheTTL  = 7 * 24 * time.Hour
)

// EmbeddingService generates and caches text embeddings for vulnerabilities.
type EmbeddingService struct {
	provider    port.EmbeddingProvider
	redisClient *redis.Client
	log         zerolog.Logger
}

// NewEmbeddingService creates an EmbeddingService with Redis cache.
func NewEmbeddingService(provider port.EmbeddingProvider, redisClient *redis.Client, log zerolog.Logger) *EmbeddingService {
	return &EmbeddingService{
		provider:    provider,
		redisClient: redisClient,
		log:         log,
	}
}

// GenerateForVuln generates an embedding for a vulnerability's text content.
// Cache key: "osv:embed:{vulnID}" with 7-day TTL.
func (s *EmbeddingService) GenerateForVuln(ctx context.Context, vulnID, summary, details string) ([]float32, error) {
	cacheKey := fmt.Sprintf("osv:embed:%s", vulnID)

	// 1. Check Redis cache
	cached, err := s.redisClient.Get(ctx, cacheKey).Bytes()
	if err == nil && len(cached) > 0 {
		embedding, decErr := decodeEmbedding(cached)
		if decErr == nil {
			s.log.Debug().Str("vuln_id", vulnID).Msg("embedding cache hit")
			return embedding, nil
		}
	}

	// 2. Prepare text: combine summary + details, truncate
	text := truncate(summary+"\n\n"+details, maxTextLength)

	// 3. Call embedding provider
	embedding, err := s.provider.GenerateEmbedding(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("generate embedding for %s: %w", vulnID, err)
	}

	// 4. Cache the result
	encoded, encErr := encodeEmbedding(embedding)
	if encErr == nil {
		s.redisClient.Set(ctx, cacheKey, encoded, embedCacheTTL) //nolint:errcheck
	}

	s.log.Debug().
		Str("vuln_id", vulnID).
		Int("dims", len(embedding)).
		Msg("embedding generated and cached")

	return embedding, nil
}

// truncate limits text to maxLen characters, preserving word boundaries.
func truncate(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	// Find last space before maxLen
	truncated := text[:maxLen]
	if idx := strings.LastIndex(truncated, " "); idx > maxLen/2 {
		return truncated[:idx]
	}
	return truncated
}

// encodeEmbedding encodes float32 slice as little-endian bytes.
func encodeEmbedding(v []float32) ([]byte, error) {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf, nil
}

// decodeEmbedding decodes little-endian bytes to float32 slice.
func decodeEmbedding(buf []byte) ([]float32, error) {
	if len(buf)%4 != 0 {
		return nil, fmt.Errorf("invalid embedding buffer length: %d", len(buf))
	}
	v := make([]float32, len(buf)/4)
	for i := range v {
		bits := binary.LittleEndian.Uint32(buf[i*4:])
		v[i] = math.Float32frombits(bits)
	}
	return v, nil
}
