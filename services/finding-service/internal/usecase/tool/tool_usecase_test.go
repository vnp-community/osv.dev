package tool_usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/osv/finding-service/internal/domain/tool"
	"github.com/osv/finding-service/internal/infra/crypto"
	uc "github.com/osv/finding-service/internal/usecase/tool"
)

type mockToolRepo struct {
	tools map[uuid.UUID]*tool.ToolConfiguration
}

func newMockToolRepo() *mockToolRepo {
	return &mockToolRepo{
		tools: make(map[uuid.UUID]*tool.ToolConfiguration),
	}
}

func (m *mockToolRepo) Save(ctx context.Context, tc *tool.ToolConfiguration) error {
	m.tools[tc.ID] = tc
	return nil
}

func (m *mockToolRepo) FindByID(ctx context.Context, id uuid.UUID) (*tool.ToolConfiguration, error) {
	if tc, ok := m.tools[id]; ok {
		return tc, nil
	}
	return nil, errors.New("not found")
}

func (m *mockToolRepo) List(ctx context.Context) ([]*tool.ToolConfiguration, error) {
	return nil, nil // unused
}

func (m *mockToolRepo) Delete(ctx context.Context, id uuid.UUID) error {
	delete(m.tools, id)
	return nil
}

func TestCreateToolConfigUseCase(t *testing.T) {
	repo := newMockToolRepo()
	keyStr := "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI=" // 32 bytes base64
	c, _ := crypto.NewAES256GCM(keyStr)
	usecase := uc.NewCreateToolConfig(repo, c)

	t.Run("MissingName", func(t *testing.T) {
		in := uc.CreateToolConfigInput{
			AuthType: tool.AuthTypeHTTPBasic,
		}
		_, err := usecase.Execute(context.Background(), in)
		if err != uc.ErrNameRequired {
			t.Fatalf("expected ErrNameRequired, got %v", err)
		}
	})

	t.Run("Success", func(t *testing.T) {
		in := uc.CreateToolConfigInput{
			Name:     "Test Tool",
			AuthType: tool.AuthTypeHTTPBasic,
			Password: "my-secret-password",
		}
		tc, err := usecase.Execute(context.Background(), in)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if tc.Name != "Test Tool" {
			t.Fatal("name mismatch")
		}
		if tc.PasswordEnc == "my-secret-password" {
			t.Fatal("password should be encrypted")
		}
		decrypted, _ := c.Decrypt(tc.PasswordEnc)
		if decrypted != "my-secret-password" {
			t.Fatalf("expected decrypted password to match, got %s", decrypted)
		}
	})
}
