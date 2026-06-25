package embedding

import (
    "context"
    "encoding/binary"
    "fmt"
    "math"
    "time"

    "github.com/redis/go-redis/v9"
    "github.com/rs/zerolog"
)

const (
    maxTextLength = 8000           // chars
    embedCacheTTL = 7 * 24 * time.Hour
    cachePrefix   = "osv:embed:"
)

// Provider defines the interface for embedding generation.
type Provider interface {
    GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
    Dimensions() int
}

// PGVectorStorage stores embeddings in PostgreSQL pgvector.
type PGVectorStorage interface {
    Store(ctx context.Context, id, text string, embedding []float32) error
    FindSimilar(ctx context.Context, embedding []float32, topK int) ([]SimilarResult, error)
}

// SimilarResult is returned from vector similarity search.
type SimilarResult struct {
    ID         string
    Text       string
    Similarity float64
}

// EmbeddingService generates, caches, and stores CVE embeddings.
type EmbeddingService struct {
    provider  Provider
    redis     *redis.Client
    pgvector  PGVectorStorage
    logger    zerolog.Logger
}

// New creates an EmbeddingService.
func New(provider Provider, redisClient *redis.Client, pgvector PGVectorStorage, logger zerolog.Logger) *EmbeddingService {
    return &EmbeddingService{
        provider: provider,
        redis:    redisClient,
        pgvector: pgvector,
        logger:   logger,
    }
}

// GenerateForVuln generates and caches an embedding for a CVE.
func (s *EmbeddingService) GenerateForVuln(ctx context.Context, cveID, summary, details string) ([]float32, error) {
    cacheKey := cachePrefix + cveID

    // 1. Check Redis cache
    if cached, err := s.redis.Get(ctx, cacheKey).Bytes(); err == nil {
        return decodeFloat32(cached), nil
    }

    // 2. Prepare text (truncate to API limit)
    text := summary + "\n\n" + details
    if len(text) > maxTextLength {
        text = text[:maxTextLength]
    }

    // 3. Generate embedding
    embedding, err := s.provider.GenerateEmbedding(ctx, text)
    if err != nil {
        return nil, fmt.Errorf("generate embedding for %s: %w", cveID, err)
    }

    // 4. Cache in Redis (little-endian float32 encoding)
    encoded := encodeFloat32(embedding)
    s.redis.Set(ctx, cacheKey, encoded, embedCacheTTL)

    // 5. Store in pgvector
    if s.pgvector != nil {
        if err := s.pgvector.Store(ctx, cveID, text, embedding); err != nil {
            s.logger.Warn().Err(err).Str("cve", cveID).Msg("pgvector store failed")
        }
    }

    s.logger.Debug().
        Str("cve", cveID).
        Int("dims", len(embedding)).
        Msg("embedding generated and cached")

    return embedding, nil
}

// SearchSimilar finds CVEs similar to a query text.
func (s *EmbeddingService) SearchSimilar(ctx context.Context, query string, topK int) ([]SimilarResult, error) {
    if s.pgvector == nil {
        return nil, fmt.Errorf("pgvector storage not configured")
    }

    embedding, err := s.provider.GenerateEmbedding(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("generate query embedding: %w", err)
    }

    return s.pgvector.FindSimilar(ctx, embedding, topK)
}

// encodeFloat32 encodes a float32 slice as little-endian bytes.
func encodeFloat32(data []float32) []byte {
    buf := make([]byte, len(data)*4)
    for i, f := range data {
        binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
    }
    return buf
}

// decodeFloat32 decodes little-endian bytes into a float32 slice.
func decodeFloat32(buf []byte) []float32 {
    data := make([]float32, len(buf)/4)
    for i := range data {
        data[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4:]))
    }
    return data
}
