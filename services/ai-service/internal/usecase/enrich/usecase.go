package enrich

import (
    "context"
    "fmt"
    "sync"

    "github.com/nats-io/nats.go"
    "github.com/rs/zerolog"

    "github.com/osv/ai-service/internal/domain/embedding"
    "github.com/osv/ai-service/internal/domain/epss"
    "github.com/osv/ai-service/internal/domain/severity"
    "github.com/osv/ai-service/internal/domain/triage"
)

// CVEInput is the input for enriching a CVE.
type CVEInput struct {
    CVEID        string
    Summary      string
    Details      string
    ExistingCVSS []severity.CVSSSeverity
}

// CVEOutput is the enrichment result.
type CVEOutput struct {
    CVEID       string
    Embedding   []float32
    Severity    *severity.SeverityPrediction
    EPSSScore   *epss.EPSSScore
    FindingTriage *triage.TriageResult
}

// UseCase orchestrates parallel CVE enrichment across all AI services.
type UseCase struct {
    embeddingSvc  *embedding.EmbeddingService
    severitySvc   *severity.Classifier
    epssClient    *epss.Client
    nc            *nats.Conn
    logger        zerolog.Logger
}

// New creates the EnrichCVE use-case.
func New(
    embeddingSvc *embedding.EmbeddingService,
    severitySvc *severity.Classifier,
    epssClient *epss.Client,
    nc *nats.Conn,
    logger zerolog.Logger,
) *UseCase {
    return &UseCase{
        embeddingSvc: embeddingSvc,
        severitySvc:  severitySvc,
        epssClient:   epssClient,
        nc:           nc,
        logger:       logger,
    }
}

// Execute runs all enrichment steps in parallel.
func (uc *UseCase) Execute(ctx context.Context, in CVEInput) (*CVEOutput, error) {
    out := &CVEOutput{CVEID: in.CVEID}
    var wg sync.WaitGroup
    var mu sync.Mutex
    errs := make([]error, 0)

    wg.Add(3)

    // 1. Generate embedding
    go func() {
        defer wg.Done()
        emb, err := uc.embeddingSvc.GenerateForVuln(ctx, in.CVEID, in.Summary, in.Details)
        mu.Lock()
        defer mu.Unlock()
        if err != nil {
            uc.logger.Warn().Err(err).Str("cve", in.CVEID).Msg("embedding failed")
            errs = append(errs, err)
            return
        }
        out.Embedding = emb
    }()

    // 2. Severity classification
    go func() {
        defer wg.Done()
        pred, err := uc.severitySvc.Classify(ctx, in.Summary, in.Details, in.ExistingCVSS)
        mu.Lock()
        defer mu.Unlock()
        if err != nil {
            uc.logger.Warn().Err(err).Str("cve", in.CVEID).Msg("severity classification failed")
            errs = append(errs, err)
            return
        }
        out.Severity = pred
    }()

    // 3. EPSS score
    go func() {
        defer wg.Done()
        score, err := uc.epssClient.GetScore(ctx, in.CVEID)
        mu.Lock()
        defer mu.Unlock()
        if err != nil {
            uc.logger.Warn().Err(err).Str("cve", in.CVEID).Msg("EPSS fetch failed")
            // Non-fatal
            return
        }
        out.EPSSScore = score
    }()

    wg.Wait()

    // Publish enrichment event
    if uc.nc != nil {
        subject := "ai.cve.enriched"
        payload := fmt.Sprintf(`{"cve_id":%q,"severity":%q,"has_epss":%v}`,
            in.CVEID,
            func() string {
                if out.Severity != nil {
                    return string(out.Severity.Severity)
                }
                return "UNKNOWN"
            }(),
            out.EPSSScore != nil,
        )
        _ = uc.nc.Publish(subject, []byte(payload))
    }

    uc.logger.Info().
        Str("cve", in.CVEID).
        Int("embedding_dims", len(out.Embedding)).
        Msg("CVE enrichment complete")

    return out, nil
}

// SubscribeToScanCompleted subscribes to NATS scan.completed events for auto-enrichment.
func (uc *UseCase) SubscribeToScanCompleted(ctx context.Context) error {
    _, err := uc.nc.Subscribe("scan.scan.completed", func(msg *nats.Msg) {
        // In production: parse event and enrich each CVE
        uc.logger.Debug().Msg("received scan.completed for enrichment")
    })
    return err
}
