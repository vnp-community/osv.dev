// Package server implements the ProductService gRPC server.
package server

import (
	"context"

	"github.com/google/uuid"
	engagement_uc "github.com/defectdojo/product-service/internal/usecase/engagement"
	product_uc "github.com/defectdojo/product-service/internal/usecase/product"
	test_uc "github.com/defectdojo/product-service/internal/usecase/test"
	"github.com/defectdojo/product-service/internal/domain/engagement"
	productv1 "github.com/osv/shared/proto/gen/go/product/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ProductServer implements productv1.ProductServiceServer.
type ProductServer struct {
	productv1.UnimplementedProductServiceServer
	getOrCreateProduct    *product_uc.GetOrCreateProductUseCase
	getOrCreateEngagement *engagement_uc.GetOrCreateEngagementUseCase
	getOrCreateTest       *test_uc.GetOrCreateTestUseCase
	closeEngagement       *engagement_uc.CloseEngagementUseCase
}

// New creates a ProductServer with all required use cases.
func New(
	gcp *product_uc.GetOrCreateProductUseCase,
	gce *engagement_uc.GetOrCreateEngagementUseCase,
	gct *test_uc.GetOrCreateTestUseCase,
	closeEng *engagement_uc.CloseEngagementUseCase,
) *ProductServer {
	return &ProductServer{
		getOrCreateProduct:    gcp,
		getOrCreateEngagement: gce,
		getOrCreateTest:       gct,
		closeEngagement:       closeEng,
	}
}

// GetOrCreateProduct resolves or creates a Product+ProductType by name.
func (s *ProductServer) GetOrCreateProduct(ctx context.Context, req *productv1.GetOrCreateProductRequest) (*productv1.GetOrCreateProductResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	requestorID, err := uuid.Parse(req.RequestorUserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid requestor_user_id")
	}

	p, pt, created, err := s.getOrCreateProduct.Execute(ctx, product_uc.GetOrCreateProductInput{
		Name:            req.Name,
		ProductTypeName: req.ProductTypeName,
		RequestorID:     requestorID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get_or_create_product: %v", err)
	}

	return &productv1.GetOrCreateProductResponse{
		ProductId:     p.ID.String(),
		ProductTypeId: pt.ID.String(),
		Created:       created,
	}, nil
}

// GetOrCreateEngagement resolves or creates an Engagement for a given Product.
func (s *ProductServer) GetOrCreateEngagement(ctx context.Context, req *productv1.GetOrCreateEngagementRequest) (*productv1.GetOrCreateEngagementResponse, error) {
	productID, err := uuid.Parse(req.ProductId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid product_id")
	}

	engType := engagement.TypeInteractive
	if req.EngagementType == "CI/CD" {
		engType = engagement.TypeCICD
	}

	eng, created, err := s.getOrCreateEngagement.Execute(ctx, engagement_uc.GetOrCreateInput{
		ProductID:                 productID,
		Name:                      req.Name,
		EngagementType:            engType,
		Version:                   req.Version,
		BuildID:                   req.BuildId,
		CommitHash:                req.CommitHash,
		BranchTag:                 req.BranchTag,
		DeduplicationOnEngagement: req.DeduplicationOnEngagement,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get_or_create_engagement: %v", err)
	}

	return &productv1.GetOrCreateEngagementResponse{
		EngagementId: eng.ID.String(),
		Created:      created,
	}, nil
}

// GetOrCreateTest resolves or creates a Test for a given Engagement + scan type.
func (s *ProductServer) GetOrCreateTest(ctx context.Context, req *productv1.GetOrCreateTestRequest) (*productv1.GetOrCreateTestResponse, error) {
	engagementID, err := uuid.Parse(req.EngagementId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid engagement_id")
	}

	t, created, err := s.getOrCreateTest.Execute(ctx, test_uc.GetOrCreateTestInput{
		EngagementID: engagementID,
		ScanType:     req.ScanType,
		Title:        req.Title,
		Version:      req.Version,
		BuildID:      req.BuildId,
		CommitHash:   req.CommitHash,
		BranchTag:    req.BranchTag,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get_or_create_test: %v", err)
	}

	return &productv1.GetOrCreateTestResponse{
		TestId:  t.ID.String(),
		Created: created,
	}, nil
}
