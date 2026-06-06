// Package handler implements the SBOMVEXService gRPC handler.
package handler

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/osv/sbomvex/internal/domain/entity"
	"github.com/osv/sbomvex/internal/domain/service"
	sbomgenerators "github.com/osv/sbomvex/internal/generators/sbom"
	vexgenerators "github.com/osv/sbomvex/internal/generators/vex"
	"github.com/osv/sbomvex/internal/parsers/sbom/cyclonedx"
	"github.com/osv/sbomvex/internal/parsers/sbom/spdx"
	"github.com/osv/sbomvex/internal/parsers/sbom/swid"
	vexcdx "github.com/osv/sbomvex/internal/parsers/vex/cdx"
	vexcsaf "github.com/osv/sbomvex/internal/parsers/vex/csaf"
	vexopenvex "github.com/osv/sbomvex/internal/parsers/vex/openvex"

	pb "github.com/osv/proto/gen/go/sbomvex/v1"
)

// SBOMVEXHandler implements pb.SBOMVEXServiceServer.
type SBOMVEXHandler struct {
	pb.UnimplementedSBOMVEXServiceServer
	detectSvc       *service.SBOMDetectService
	spdxTVParser    *spdx.TagValueParser
	spdxJSONParser  *spdx.JSONParser
	cdxParser       *cyclonedx.JSONParser
	swidParser      *swid.XMLParser
	vexOpenParser   *vexopenvex.Parser
	vexCDXParser    *vexcdx.Parser
	vexCSAFParser   *vexcsaf.Parser
	spdxGen         *sbomgenerators.SPDXTagValueGenerator
	cdxGen          *sbomgenerators.CycloneDXGenerator
	vexGen          *vexgenerators.OpenVEXGenerator
	log             zerolog.Logger
}

// New creates a SBOMVEXHandler.
func New(log zerolog.Logger) *SBOMVEXHandler {
	return &SBOMVEXHandler{
		detectSvc:      service.New(),
		spdxTVParser:   spdx.NewTagValueParser(),
		spdxJSONParser: spdx.NewJSONParser(),
		cdxParser:      cyclonedx.New(),
		swidParser:     swid.New(),
		vexOpenParser:  vexopenvex.New(),
		vexCDXParser:   vexcdx.New(),
		vexCSAFParser:  vexcsaf.New(),
		spdxGen:        sbomgenerators.NewSPDX(),
		cdxGen:         sbomgenerators.NewCycloneDX(),
		vexGen:         vexgenerators.New(),
		log:            log,
	}
}

// ParseSBOM parses an SBOM document and returns product list.
func (h *SBOMVEXHandler) ParseSBOM(_ context.Context, req *pb.ParseSBOMRequest) (*pb.ParseSBOMResponse, error) {
	content := req.GetFileBytes()
	if len(content) == 0 {
		return nil, status.Error(codes.InvalidArgument, "file_bytes is required")
	}

	// Detect or use provided format
	sbomFormat := h.detectSvc.Detect(content)
	if req.GetFormat() != "" {
		sbomFormat = entity.SBOMFormat(req.GetFormat())
	}

	var doc *entity.SBOMDocument
	var err error

	switch sbomFormat {
	case entity.SBOMFormatCycloneDX:
		doc, err = h.cdxParser.Parse(content)
	case entity.SBOMFormatSPDX:
		// Try JSON first, fall back to tag-value
		doc, err = h.spdxJSONParser.Parse(content)
		if err != nil {
			doc, err = h.spdxTVParser.Parse(content)
		}
	case entity.SBOMFormatSWID:
		doc, err = h.swidParser.Parse(content)
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unknown SBOM format: %q (detected: %q)", req.GetFormat(), sbomFormat)
	}

	if err != nil {
		return nil, status.Errorf(codes.Internal, "parse SBOM: %v", err)
	}

	// Convert to proto products
	products := make([]*pb.ProductInfo, 0, len(doc.Components))
	for _, comp := range doc.Components {
		pi := comp.ToProductInfo()
		products = append(products, &pb.ProductInfo{
			Vendor:  pi.Vendor,
			Product: pi.Product,
			Version: pi.Version,
			Purl:    pi.PURL,
		})
	}

	return &pb.ParseSBOMResponse{
		Products:        products,
		DetectedFormat:  string(sbomFormat),
		ComponentCount:  int32(len(products)),
	}, nil
}

// GenerateSBOM generates an SBOM document from scan results.
func (h *SBOMVEXHandler) GenerateSBOM(ctx context.Context, req *pb.GenerateSBOMRequest) (*pb.GenerateSBOMResponse, error) {
	results := make([]sbomgenerators.ScanResult, 0, len(req.GetScanResults()))
	for _, r := range req.GetScanResults() {
		results = append(results, sbomgenerators.ScanResult{
			Vendor:  r.GetVendor(),
			Product: r.GetProduct(),
			Version: r.GetVersion(),
			PURL:    r.GetPurl(),
		})
	}

	opts := sbomgenerators.SBOMOptions{
		PackageName:     req.GetPackageName(),
		PackageVersion:  req.GetPackageVersion(),
		PackageSupplier: req.GetPackageSupplier(),
		StripScanDir:    req.GetStripScanDir(),
	}

	var (
		data        []byte
		err         error
		contentType string
		format      = req.GetFormat()
	)

	switch format {
	case "spdx", "spdx-tv", "":
		data, err = h.spdxGen.Generate(ctx, results, opts)
		contentType = h.spdxGen.ContentType()
		format = "spdx"
	case "cyclonedx", "cdx":
		data, err = h.cdxGen.Generate(ctx, results, opts)
		contentType = h.cdxGen.ContentType()
		format = "cyclonedx"
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unsupported SBOM format: %q", format)
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate SBOM: %v", err)
	}

	return &pb.GenerateSBOMResponse{
		Document:    data,
		Format:      format,
		ContentType: contentType,
	}, nil
}

// ParseVEX parses a VEX document and returns triage data.
func (h *SBOMVEXHandler) ParseVEX(_ context.Context, req *pb.ParseVEXRequest) (*pb.ParseVEXResponse, error) {
	content := req.GetFileBytes()
	if len(content) == 0 {
		return nil, status.Error(codes.InvalidArgument, "file_bytes is required")
	}

	format := req.GetFormat()

	var (
		doc *entity.VEXDocument
		err error
	)

	// Auto-detect or use provided format
	switch format {
	case "openvex":
		doc, err = h.vexOpenParser.Parse(content)
	case "cyclonedx", "cdx":
		doc, err = h.vexCDXParser.Parse(content)
	case "csaf":
		doc, err = h.vexCSAFParser.Parse(content)
	default:
		// Auto-detect
		doc, err = h.autoDetectVEX(content)
	}

	if err != nil {
		return nil, status.Errorf(codes.Internal, "parse VEX: %v", err)
	}

	// Convert triage data to proto
	td := doc.ToTriageData()
	protoTriage := make(map[string]*pb.TriageEntry, len(td))
	for cve, entry := range td {
		protoTriage[cve] = &pb.TriageEntry{
			Remarks:       remarksToString(entry.Remarks),
			Justification: entry.Justification,
			Comments:      entry.Comments,
			Response:      entry.Response,
		}
	}

	return &pb.ParseVEXResponse{
		TriageData:     protoTriage,
		DetectedFormat: string(doc.Format),
		StatementCount: int32(len(doc.Statements)),
	}, nil
}

// GenerateVEX generates a VEX document from CVE data.
func (h *SBOMVEXHandler) GenerateVEX(ctx context.Context, req *pb.GenerateVEXRequest) (*pb.GenerateVEXResponse, error) {
	data := req.GetCveData()
	products := make([]vexgenerators.ProductCVEs, 0, len(data))
	for _, pc := range data {
		if pc.GetProduct() == nil {
			continue
		}
		cves := make([]vexgenerators.CVEEntry, 0, len(pc.GetCves()))
		for _, c := range pc.GetCves() {
			cves = append(cves, vexgenerators.CVEEntry{
				CVENumber:  c.GetCveNumber(),
				Severity:   c.GetSeverity(),
				IsExploit:  c.GetIsExploit(),
				DataSource: c.GetDataSource(),
			})
		}
		products = append(products, vexgenerators.ProductCVEs{
			Vendor:  pc.GetProduct().GetVendor(),
			Product: pc.GetProduct().GetProduct(),
			Version: pc.GetProduct().GetVersion(),
			CVEs:    cves,
		})
	}

	doc, err := h.vexGen.Generate(ctx, products, vexgenerators.VEXOptions{
		Format: req.GetFormat(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate VEX: %v", err)
	}

	return &pb.GenerateVEXResponse{
		Document:    doc,
		Format:      "openvex",
		ContentType: "application/json",
	}, nil
}

// DetectSBOMFormat detects the SBOM format of provided bytes.
func (h *SBOMVEXHandler) DetectSBOMFormat(_ context.Context, req *pb.DetectFormatRequest) (*pb.DetectFormatResponse, error) {
	if len(req.GetFileBytes()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "file_bytes is required")
	}
	sbomFormat := h.detectSvc.Detect(req.GetFileBytes())
	return &pb.DetectFormatResponse{Format: string(sbomFormat)}, nil
}

// autoDetectVEX tries each VEX parser in order.
func (h *SBOMVEXHandler) autoDetectVEX(content []byte) (*entity.VEXDocument, error) {
	parsers := []interface {
		Parse([]byte) (*entity.VEXDocument, error)
	}{h.vexOpenParser, h.vexCDXParser, h.vexCSAFParser}

	var lastErr error
	for _, p := range parsers {
		doc, err := p.Parse(content)
		if err == nil && len(doc.Statements) > 0 {
			return doc, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("could not detect VEX format: %w", lastErr)
}

// remarksToString converts a domain Remarks integer to the proto string representation.
func remarksToString(r int) string {
	switch r {
	case 1:
		return "NotAffected"
	case 2:
		return "Confirmed"
	case 3:
		return "Mitigated"
	case 4:
		return "NewFound"
	default:
		return ""
	}
}

// Ensure interface compliance.
var _ pb.SBOMVEXServiceServer = (*SBOMVEXHandler)(nil)
