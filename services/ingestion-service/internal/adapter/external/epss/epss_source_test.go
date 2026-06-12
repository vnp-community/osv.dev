package epss_test

import (
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/osv/ingestion-service/internal/adapter/external/epss"
	"github.com/osv/ingestion-service/internal/adapter/external/sources"
)

func TestEPSSSource_FetchCVEData(t *testing.T) {
	csvData := buildEPSSCSV()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		gz := gzip.NewWriter(w)
		gz.Write(csvData)
		gz.Close()
	}))
	defer srv.Close()

	src := epss.New(srv.URL, srv.Client())
	if src.Name() != "EPSS" {
		t.Errorf("expected Name=EPSS, got %s", src.Name())
	}

	data, err := src.FetchCVEData(context.Background())
	if err != nil {
		t.Fatalf("FetchCVEData: %v", err)
	}
	if len(data.Metrics) != 3 {
		t.Errorf("expected 3 metrics, got %d", len(data.Metrics))
	}
	if data.Metrics[0].CVENumber != "CVE-2022-26134" {
		t.Errorf("expected CVE-2022-26134, got %s", data.Metrics[0].CVENumber)
	}
	if data.Metrics[0].MetricID != sources.MetricIDEPSS {
		t.Errorf("expected MetricID=%d, got %d", sources.MetricIDEPSS, data.Metrics[0].MetricID)
	}
	if data.Metrics[0].MetricScore < 0.97 {
		t.Errorf("expected score ~0.97, got %f", data.Metrics[0].MetricScore)
	}
	if data.Metrics[0].MetricField != "1.0" {
		t.Errorf("expected percentile '1.0', got %q", data.Metrics[0].MetricField)
	}
}

func buildEPSSCSV() []byte {
	// EPSS CSV format: comment + header + data rows
	content := "#model_version:v2023.03.01,score_date:" + time.Now().UTC().Format("2006-01-02") + "\n"
	content += "cve,epss,percentile\n"
	content += "CVE-2022-26134,0.97553,1.0\n"
	content += "CVE-2021-44228,0.97534,1.0\n"
	content += "CVE-2022-0847,0.97531,0.99\n"
	return []byte(content)
}

func TestEPSSSource_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	src := epss.New(srv.URL, srv.Client())
	_, err := src.FetchCVEData(context.Background())
	if err == nil {
		t.Error("expected error for 404 response")
	}
}
