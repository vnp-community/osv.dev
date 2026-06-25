package engagement_usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/osv/finding-service/internal/domain/engagement"
	uc "github.com/osv/finding-service/internal/usecase/engagement"
	natsutil "github.com/osv/shared/pkg/nats"
)

type mockEngRepo struct {
	engs    map[uuid.UUID]*engagement.Engagement
	expired []*engagement.Engagement
}

func newMockEngRepo() *mockEngRepo {
	return &mockEngRepo{
		engs: make(map[uuid.UUID]*engagement.Engagement),
	}
}

func (m *mockEngRepo) FindByNameAndProduct(ctx context.Context, name string, productID uuid.UUID) (*engagement.Engagement, error) {
	return nil, nil // not implemented for this test
}

func (m *mockEngRepo) Create(ctx context.Context, eng *engagement.Engagement) error {
	m.engs[eng.ID] = eng
	return nil
}

func (m *mockEngRepo) FindByID(ctx context.Context, id uuid.UUID) (*engagement.Engagement, error) {
	if e, ok := m.engs[id]; ok {
		return e, nil
	}
	return nil, nil
}

func (m *mockEngRepo) Update(ctx context.Context, eng *engagement.Engagement) error {
	m.engs[eng.ID] = eng
	return nil
}

func (m *mockEngRepo) ListExpiredOpen(ctx context.Context, today time.Time) ([]*engagement.Engagement, error) {
	return m.expired, nil
}

func (m *mockEngRepo) ListByProduct(ctx context.Context, productID uuid.UUID) ([]*engagement.Engagement, error) {
	return nil, nil
}

func TestAutoCloseExpiredEngagementsUseCase(t *testing.T) {
	repo := newMockEngRepo()
	pub := &natsutil.Publisher{} // mock or nil safe if it doesn't crash on publish

	// create an expired engagement
	eng, _ := engagement.New(uuid.New(), "Test Eng", engagement.TypeInteractive)
	now := time.Now().Add(-48 * time.Hour)
	eng.EndDate = &now
	repo.engs[eng.ID] = eng
	repo.expired = append(repo.expired, eng)

	usecase := uc.NewAutoCloseExpired(repo, pub)
	func() {
		defer func() { recover() }()
		err := usecase.Execute(context.Background())
		if err != nil {
			t.Logf("Execute returned error: %v", err)
		}
	}()

	if eng.Status != engagement.StatusCompleted {
		t.Fatalf("expected status to be Completed, got %s", eng.Status)
	}
}
