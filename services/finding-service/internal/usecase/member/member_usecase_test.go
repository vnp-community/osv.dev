package member_usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/osv/finding-service/internal/domain/member"
	uc "github.com/osv/finding-service/internal/usecase/member"
)

type mockProductMemberRepo struct {
	roles   map[string]member.Role
	members map[string]*member.ProductMember
	saved   []*member.ProductMember
	deleted []string
}

func newMockRepo() *mockProductMemberRepo {
	return &mockProductMemberRepo{
		roles:   make(map[string]member.Role),
		members: make(map[string]*member.ProductMember),
	}
}

func (m *mockProductMemberRepo) key(pid, uid uuid.UUID) string {
	return pid.String() + "|" + uid.String()
}

func (m *mockProductMemberRepo) Save(ctx context.Context, mem *member.ProductMember) error {
	m.saved = append(m.saved, mem)
	return nil
}

func (m *mockProductMemberRepo) FindByProductAndUser(ctx context.Context, productID, userID uuid.UUID) (*member.ProductMember, error) {
	if mem, ok := m.members[m.key(productID, userID)]; ok {
		return mem, nil
	}
	return nil, errors.New("not found")
}

func (m *mockProductMemberRepo) ListByProduct(ctx context.Context, productID uuid.UUID) ([]*member.ProductMember, error) {
	return nil, nil // unused in these tests
}

func (m *mockProductMemberRepo) Delete(ctx context.Context, productID, userID uuid.UUID) error {
	m.deleted = append(m.deleted, m.key(productID, userID))
	return nil
}

func (m *mockProductMemberRepo) GetRole(ctx context.Context, productID, userID uuid.UUID) (*member.Role, error) {
	if role, ok := m.roles[m.key(productID, userID)]; ok {
		return &role, nil
	}
	return nil, errors.New("not found")
}

func TestAddProductMemberUseCase(t *testing.T) {
	repo := newMockRepo()
	usecase := uc.NewAddProductMember(repo)

	productID := uuid.New()
	ownerID := uuid.New()
	readerID := uuid.New()
	newUserID := uuid.New()

	repo.roles[repo.key(productID, ownerID)] = member.RoleOwner
	repo.roles[repo.key(productID, readerID)] = member.RoleReader

	t.Run("RequesterNotOwner", func(t *testing.T) {
		in := uc.AddProductMemberInput{
			RequesterUserID: readerID,
			ProductID:       productID,
			UserID:          newUserID,
			Role:            member.RoleWriter,
		}
		_, err := usecase.Execute(context.Background(), in)
		if err != uc.ErrNotOwner {
			t.Fatalf("expected ErrNotOwner, got %v", err)
		}
	})

	t.Run("MemberExists", func(t *testing.T) {
		existingUser := uuid.New()
		repo.members[repo.key(productID, existingUser)] = &member.ProductMember{}
		in := uc.AddProductMemberInput{
			RequesterUserID: ownerID,
			ProductID:       productID,
			UserID:          existingUser,
			Role:            member.RoleWriter,
		}
		_, err := usecase.Execute(context.Background(), in)
		if err != uc.ErrMemberExists {
			t.Fatalf("expected ErrMemberExists, got %v", err)
		}
	})

	t.Run("Success", func(t *testing.T) {
		in := uc.AddProductMemberInput{
			RequesterUserID: ownerID,
			ProductID:       productID,
			UserID:          newUserID,
			Role:            member.RoleWriter,
		}
		_, err := usecase.Execute(context.Background(), in)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(repo.saved) != 1 {
			t.Fatal("expected member to be saved")
		}
		if repo.saved[0].UserID != newUserID || repo.saved[0].Role != member.RoleWriter {
			t.Fatal("saved member mismatch")
		}
	})
}

func TestRemoveProductMemberUseCase(t *testing.T) {
	repo := newMockRepo()
	usecase := uc.NewRemoveProductMember(repo)

	productID := uuid.New()
	ownerID := uuid.New()
	maintainerID := uuid.New()
	targetID := uuid.New()

	repo.roles[repo.key(productID, ownerID)] = member.RoleOwner
	repo.roles[repo.key(productID, maintainerID)] = member.RoleMaintainer

	t.Run("RequesterNotOwner", func(t *testing.T) {
		in := uc.RemoveProductMemberInput{
			RequesterUserID: maintainerID,
			ProductID:       productID,
			UserID:          targetID,
		}
		err := usecase.Execute(context.Background(), in)
		if err != uc.ErrNotOwner {
			t.Fatalf("expected ErrNotOwner, got %v", err)
		}
	})

	t.Run("Success", func(t *testing.T) {
		in := uc.RemoveProductMemberInput{
			RequesterUserID: ownerID,
			ProductID:       productID,
			UserID:          targetID,
		}
		err := usecase.Execute(context.Background(), in)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(repo.deleted) != 1 {
			t.Fatal("expected member to be deleted")
		}
	})
}

func TestCheckProductPermissionUseCase(t *testing.T) {
	repo := newMockRepo()
	usecase := uc.NewCheckProductPermission(repo)

	productID := uuid.New()
	ownerID := uuid.New()
	readerID := uuid.New()

	repo.roles[repo.key(productID, ownerID)] = member.RoleOwner
	repo.roles[repo.key(productID, readerID)] = member.RoleReader

	t.Run("OwnerHasAllPerms", func(t *testing.T) {
		in := uc.CheckProductPermissionInput{
			UserID:     ownerID,
			ProductID:  productID,
			Permission: uc.PermProductDelete,
		}
		ok, err := usecase.Execute(context.Background(), in)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatal("expected true")
		}
	})

	t.Run("ReaderCannotDelete", func(t *testing.T) {
		in := uc.CheckProductPermissionInput{
			UserID:     readerID,
			ProductID:  productID,
			Permission: uc.PermFindingDelete,
		}
		ok, err := usecase.Execute(context.Background(), in)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Fatal("expected false")
		}
	})

	t.Run("ReaderCanView", func(t *testing.T) {
		in := uc.CheckProductPermissionInput{
			UserID:     readerID,
			ProductID:  productID,
			Permission: uc.PermFindingView,
		}
		ok, err := usecase.Execute(context.Background(), in)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatal("expected true")
		}
	})
}
