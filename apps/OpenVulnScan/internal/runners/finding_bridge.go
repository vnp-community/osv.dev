// Package runners — finding_bridge.go
// FindingBridge implement findingv1.FindingServiceServer trực tiếp.
// Bridge Pattern: tránh import finding-service/internal/ (Go restriction).
//
// Thay vì import finding-service/internal/delivery/grpc/server,
// chúng ta implement toàn bộ logic trực tiếp với:
// - finding-service/internal/infra/postgres (PostgreSQL repo)
// - finding-service/internal/usecase/finding (usecases)
// - finding-service/internal/domain/finding (domain types)
//
// Những packages này thuộc internal của finding-service, nhưng vì
// go.work workspace cho phép cross-module internal access trong cùng workspace.
//
// FALLBACK: Nếu workspace không cho phép, implement minimal bridge với direct SQL.
package runners

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	findingv1 "github.com/osv/shared/proto/gen/go/finding/v1"
)

// findingBridge implement findingv1.FindingServiceServer dùng direct Postgres queries.
// Đây là minimal implementation để compile và boot được.
// Full implementation sẽ delegate đến finding-service usecases.
type findingBridge struct {
	findingv1.UnimplementedFindingServiceServer
	db *pgxpool.Pool
}

// newFindingBridge tạo FindingBridge với direct DB connection.
func newFindingBridge(db *pgxpool.Pool) findingv1.FindingServiceServer {
	return &findingBridge{db: db}
}

// BatchCreateFindings tạo nhiều findings từ một scan.
func (b *findingBridge) BatchCreateFindings(ctx context.Context, req *findingv1.BatchCreateFindingsRequest) (*findingv1.BatchCreateFindingsResponse, error) {
	if req.TestId == "" {
		return nil, status.Error(codes.InvalidArgument, "test_id is required")
	}

	var findingIDs []string
	for _, fi := range req.Findings {
		id := uuid.New()
		findingIDs = append(findingIDs, id.String())

		_, err := b.db.Exec(ctx, `
			INSERT INTO findings (
				id, title, description, severity, cve, cwe,
				component_name, component_version, file_path,
				hash_code, active, verified,
				test_id, engagement_id, product_id,
				date, created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6,
				$7, $8, $9,
				$10, $11, $12,
				$13, $14, $15,
				NOW(), NOW(), NOW()
			) ON CONFLICT (hash_code, test_id) DO UPDATE SET
				updated_at = NOW(), active = $11
		`,
			id, fi.Title, fi.Description, fi.Severity, fi.Cve, fi.Cwe,
			fi.ComponentName, fi.ComponentVersion, fi.FilePath,
			fi.HashCode, fi.Active, fi.Verified,
			req.TestId, req.EngagementId, req.ProductId,
		)
		if err != nil {
			log.Error().Err(err).Str("title", fi.Title).Msg("finding insert error")
			// Tiếp tục batch — không abort toàn bộ
		}
	}

	return &findingv1.BatchCreateFindingsResponse{
		FindingIds: findingIDs,
		Created:    int32(len(findingIDs)),
	}, nil
}

// FindByHashCode tìm finding theo hash code để dedup.
func (b *findingBridge) FindByHashCode(ctx context.Context, req *findingv1.FindByHashCodeRequest) (*findingv1.FindByHashCodeResponse, error) {
	var findingID string
	var findingStatus string

	err := b.db.QueryRow(ctx, `
		SELECT id::text, state FROM findings
		WHERE hash_code = $1 AND test_id = $2
		LIMIT 1
	`, req.HashCode, req.TestId).Scan(&findingID, &findingStatus)

	if err != nil {
		return &findingv1.FindByHashCodeResponse{}, nil // not found
	}

	return &findingv1.FindByHashCodeResponse{
		FindingId: &findingID,
		Status:    &findingStatus,
	}, nil
}

// CloseOldFindings đóng findings không còn xuất hiện trong scan mới.
func (b *findingBridge) CloseOldFindings(ctx context.Context, req *findingv1.CloseOldFindingsRequest) (*findingv1.CloseOldFindingsResponse, error) {
	excludeList := make([]string, 0, len(req.ExcludeFindingIds))
	excludeList = append(excludeList, req.ExcludeFindingIds...)

	result, err := b.db.Exec(ctx, `
		UPDATE findings
		SET active = false, state = 'mitigated', updated_at = NOW()
		WHERE test_id = $1 AND active = true
		  AND id::text != ALL($2::text[])
	`, req.TestId, excludeList)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "close_old: %v", err)
	}

	return &findingv1.CloseOldFindingsResponse{
		Closed: int32(result.RowsAffected()),
	}, nil
}

// BatchUpdateSLADates cập nhật SLA expiration dates.
func (b *findingBridge) BatchUpdateSLADates(ctx context.Context, req *findingv1.BatchUpdateSLADatesRequest) (*findingv1.BatchUpdateSLADatesResponse, error) {
	for _, u := range req.Updates {
		_, err := b.db.Exec(ctx, `
			UPDATE findings SET sla_expiration_date = $1, updated_at = NOW()
			WHERE id = $2::uuid
		`, u.ExpirationDate.AsTime(), u.FindingId)
		if err != nil {
			log.Error().Err(err).Str("finding_id", u.FindingId).Msg("sla update error")
		}
	}
	return &findingv1.BatchUpdateSLADatesResponse{Updated: int32(len(req.Updates))}, nil
}

// ListFindingsForSLACheck trả về findings cần SLA check.
func (b *findingBridge) ListFindingsForSLACheck(ctx context.Context, req *findingv1.ListFindingsForSLACheckRequest) (*findingv1.ListFindingsForSLACheckResponse, error) {
	query := `
		SELECT id::text, severity, product_id::text, date, sla_expiration_date
		FROM findings WHERE active = true
	`
	if req.HasSlaDate {
		query += " AND sla_expiration_date IS NOT NULL"
	}
	query += " LIMIT 1000"

	rows, err := b.db.Query(ctx, query)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list_for_sla: %v", err)
	}
	defer rows.Close()

	var findings []*findingv1.FindingForSLA
	for rows.Next() {
		var (
			id, severity, productID string
			date                    time.Time
			slaDate                 *time.Time
		)
		if err := rows.Scan(&id, &severity, &productID, &date, &slaDate); err != nil {
			continue
		}
		f := &findingv1.FindingForSLA{
			Id:        id,
			Severity:  severity,
			ProductId: productID,
			Date:      timestamppb.New(date),
		}
		if slaDate != nil {
			f.SlaExpirationDate = timestamppb.New(*slaDate)
		}
		findings = append(findings, f)
	}

	return &findingv1.ListFindingsForSLACheckResponse{Findings: findings}, nil
}

// ListFindingsForReport streams findings cho report generation.
func (b *findingBridge) ListFindingsForReport(req *findingv1.ListFindingsForReportRequest, stream findingv1.FindingService_ListFindingsForReportServer) error {
	ctx := stream.Context()

	query := `
		SELECT id::text, title, description, severity, cve, cwe,
			   component_name, component_version, product_id::text, test_id::text,
			   date, sla_expiration_date, active, state
		FROM findings WHERE 1=1
	`
	var args []interface{}
	idx := 1

	if req.ActiveOnly {
		query += fmt.Sprintf(" AND active = $%d", idx)
		args = append(args, true)
		idx++
	}
	if req.ProductId != nil && *req.ProductId != "" {
		query += fmt.Sprintf(" AND product_id = $%d::uuid", idx)
		args = append(args, *req.ProductId)
	}
	query += " LIMIT 100"

	rows, err := b.db.Query(ctx, query, args...)
	if err != nil {
		return status.Errorf(codes.Internal, "list_for_report: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id, title, desc, sev, cve     string
			cwe                           int32
			compName, compVer             string
			productID, testID             string
			date                          time.Time
			slaDate                       *time.Time
			active                        bool
			state                         string
		)
		if err := rows.Scan(
			&id, &title, &desc, &sev, &cve, &cwe,
			&compName, &compVer, &productID, &testID,
			&date, &slaDate, &active, &state,
		); err != nil {
			continue
		}
		proto := &findingv1.FindingProto{
			Id:               id,
			Title:            title,
			Description:      desc,
			Severity:         sev,
			Cve:              cve,
			Cwe:              cwe,
			ComponentName:    compName,
			ComponentVersion: compVer,
			ProductId:        productID,
			TestId:           testID,
			Date:             timestamppb.New(date),
			Status:           state,
		}
		if slaDate != nil {
			proto.SlaExpirationDate = timestamppb.New(*slaDate)
		}
		if err := stream.Send(proto); err != nil {
			return err
		}
	}
	return nil
}
