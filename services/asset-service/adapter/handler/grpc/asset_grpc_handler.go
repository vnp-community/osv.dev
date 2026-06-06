// Package grpc implements the asset gRPC server.
package grpc

import (
	"context"
	"fmt"
	"net"

	"github.com/google/uuid"
	"github.com/osv/asset-service/internal/domain/entity"
	"github.com/osv/asset-service/internal/domain/repository"
	upsertasset "github.com/osv/asset-service/internal/usecase/upsert_asset"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"github.com/rs/zerolog"

	// Generated proto — use flat fields from GetAssetResponse (no nested Asset)
	assetpb "github.com/osv/proto/gen/go/asset/v1"
)

// AssetGRPCServer implements assetpb.AssetServiceServer.
type AssetGRPCServer struct {
	assetpb.UnimplementedAssetServiceServer
	upsertUC  *upsertasset.UseCase
	assetRepo repository.AssetRepository
	vulnRepo  repository.VulnerabilityRepository
	log       zerolog.Logger
}

func NewAssetGRPCServer(
	upsertUC *upsertasset.UseCase,
	assetRepo repository.AssetRepository,
	vulnRepo repository.VulnerabilityRepository,
	log zerolog.Logger,
) *AssetGRPCServer {
	return &AssetGRPCServer{upsertUC: upsertUC, assetRepo: assetRepo, vulnRepo: vulnRepo, log: log}
}

// UpsertAsset is called by scan-service after finding a host.
func (s *AssetGRPCServer) UpsertAsset(ctx context.Context, req *assetpb.UpsertAssetRequest) (*assetpb.UpsertAssetResponse, error) {
	resp, err := s.upsertUC.Execute(ctx, upsertasset.Request{
		IPAddress: req.GetIpAddress(),
		Hostname:  req.GetHostname(),
		OS:        req.GetOs(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "upsert asset: %v", err)
	}
	return &assetpb.UpsertAssetResponse{
		AssetId: resp.AssetID.String(),
		Created: resp.Created,
	}, nil
}

// LinkVulnerability links a CVE to an asset.
func (s *AssetGRPCServer) LinkVulnerability(ctx context.Context, req *assetpb.LinkVulnerabilityRequest) (*assetpb.LinkVulnerabilityResponse, error) {
	assetID, err := uuid.Parse(req.GetAssetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid asset_id")
	}

	vuln := &entity.Vulnerability{
		AssetID:  assetID,
		CVEID:    req.GetCveId(),
		Severity: entity.Severity(req.GetSeverity()),
		CVSS:     float64(req.GetCvss()),
	}
	// Wire scan_id if present
	if req.GetScanId() != "" {
		if sid, err2 := uuid.Parse(req.GetScanId()); err2 == nil {
			vuln.ScanID = sid
		}
	}
	if err := s.vulnRepo.Upsert(ctx, vuln); err != nil {
		return nil, status.Errorf(codes.Internal, "link vulnerability: %v", err)
	}
	return &assetpb.LinkVulnerabilityResponse{VulnId: vuln.ID.String()}, nil
}

// GetAsset returns an asset by ID — GetAssetResponse has flat fields (no nested Asset message).
func (s *AssetGRPCServer) GetAsset(ctx context.Context, req *assetpb.GetAssetRequest) (*assetpb.GetAssetResponse, error) {
	id, err := uuid.Parse(req.GetAssetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid asset_id")
	}
	asset, err := s.assetRepo.FindByID(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "asset not found: %v", err)
	}
	return &assetpb.GetAssetResponse{
		AssetId:   asset.ID.String(),
		IpAddress: asset.IPAddress,
		Hostname:  asset.Hostname,
		Os:        asset.OS,
	}, nil
}

// StartGRPCServer starts the asset gRPC server.
func StartGRPCServer(srv *AssetGRPCServer, port string, log zerolog.Logger) error {
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	grpcSrv := grpc.NewServer()
	assetpb.RegisterAssetServiceServer(grpcSrv, srv)
	log.Info().Str("port", port).Msg("asset gRPC server starting")
	return grpcSrv.Serve(lis)
}
