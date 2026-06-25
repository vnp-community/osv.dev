package http

import (
	"strings"

	"github.com/osv/search-service/internal/domain/entity"
	search "github.com/osv/search-service/internal/usecase/cvesearch"
)

func mapToCVEResponseItem(cve *entity.CVE) CVEResponseItem {
	var cweIds []string = []string{}
	var vendorStr, productStr string
	if len(cve.Vendors) > 0 { vendorStr = cve.Vendors[0] }
	if len(cve.Products) > 0 { productStr = cve.Products[0] }

	severityStr := string(cve.Severity)
	if severityStr == "" || severityStr == "UNKNOWN" {
		severityStr = "Info"
	} else {
		severityStr = strings.Title(strings.ToLower(severityStr))
	}

	return CVEResponseItem{
		ID:             cve.ID,
		Severity:       severityStr,
		CVSSv3:         cve.CVSS3Score,
		CVSSv2:         cve.CVSSScore,
		EPSSScore:      cve.EPSS,
		EPSSPercentile: cve.EPSSPct,
		IsKEV:          cve.IsKEV,
		HasExploit:     cve.IsExploit,
		Vendor:         vendorStr,
		Product:        productStr,
		CWEIds:         cweIds,
		CAPECIds:       []string{},
		Description:    cve.Description,
		PublishedAt:    cve.Published.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:      cve.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		Sources: []CVESource{
			{Name: string(cve.Source), URL: cve.Link},
		},
	}
}

func mapToSearchResponse(resp *search.Response) SearchResponse {
	data := make([]CVEResponseItem, 0, len(resp.CVEs))
	for _, c := range resp.CVEs {
		data = append(data, mapToCVEResponseItem(c))
	}
	return SearchResponse{
		Data:     data,
		Total:    int(resp.Total),
		Page:     resp.Page,
		PageSize: resp.Limit,
		Aggregations: &SearchAggregations{
			BySeverity: map[string]int{},
			TopVendors: []VendorCount{},
			ByYear:     []YearCount{},
		},
	}
}

func mapToCVEDetailResponse(cve *entity.CVE) CVEDetailResponse {
	item := mapToCVEResponseItem(cve)
	return CVEDetailResponse{
		CVEResponseItem:  item,
		AffectedProducts: []CPEEntry{},
		References:       []string{cve.Link},
		Notes:            []string{},
	}
}
