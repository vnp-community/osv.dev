// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kev

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func sampleCatalog() *Catalog {
	return &Catalog{
		Title:          "CISA Known Exploited Vulnerabilities Catalog",
		CatalogVersion: "2026.06.01",
		DateReleased:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Count:          2,
		Vulnerabilities: []*Entry{
			{
				CVEID:              "CVE-2023-44487",
				VendorProject:      "IETF",
				Product:            "HTTP/2",
				VulnerabilityName:  "HTTP/2 Rapid Reset Attack",
				DateAdded:          "2023-10-10",
				ShortDescription:   "HTTP/2 protocol vulnerability allowing DDoS.",
				RequiredAction:     "Apply mitigations or vendor updates.",
				DueDate:            "2023-10-31",
				KnownRansomwareCampaignUse: "Unknown",
			},
			{
				CVEID:              "CVE-2021-44228",
				VendorProject:      "Apache",
				Product:            "Log4j2",
				VulnerabilityName:  "Apache Log4j2 Remote Code Execution Vulnerability",
				DateAdded:          "2021-12-10",
				ShortDescription:   "Apache Log4j2 JNDI injection vulnerability.",
				RequiredAction:     "Apply mitigations per vendor instructions.",
				DueDate:            "2021-12-24",
				KnownRansomwareCampaignUse: "Known",
			},
		},
	}
}

func TestBuildIndex(t *testing.T) {
	catalog := sampleCatalog()
	idx := BuildIndex(catalog)

	if len(idx) != 2 {
		t.Errorf("expected 2 entries, got %d", len(idx))
	}
	if _, ok := idx["CVE-2023-44487"]; !ok {
		t.Error("expected CVE-2023-44487 in index")
	}
	if _, ok := idx["CVE-2021-44228"]; !ok {
		t.Error("expected CVE-2021-44228 in index")
	}
}

func TestInMemoryLookup(t *testing.T) {
	catalog := sampleCatalog()
	lookup := NewInMemoryLookup(catalog)

	t.Run("IsKEV_true", func(t *testing.T) {
		if !lookup.IsKEV("CVE-2023-44487") {
			t.Error("expected CVE-2023-44487 to be in KEV")
		}
	})

	t.Run("IsKEV_false", func(t *testing.T) {
		if lookup.IsKEV("CVE-1999-00001") {
			t.Error("expected CVE-1999-00001 to NOT be in KEV")
		}
	})

	t.Run("Get_found", func(t *testing.T) {
		e := lookup.Get("CVE-2021-44228")
		if e == nil {
			t.Fatal("expected entry, got nil")
		}
		if e.VendorProject != "Apache" {
			t.Errorf("expected Apache, got %q", e.VendorProject)
		}
	})

	t.Run("Get_notfound", func(t *testing.T) {
		if lookup.Get("CVE-9999-99999") != nil {
			t.Error("expected nil for unknown CVE")
		}
	})

	t.Run("Count", func(t *testing.T) {
		if lookup.Count() != 2 {
			t.Errorf("expected 2, got %d", lookup.Count())
		}
	})
}

func TestFetchCatalog(t *testing.T) {
	catalog := sampleCatalog()
	data, err := json.Marshal(catalog)
	if err != nil {
		t.Fatalf("marshal test catalog: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}))
	defer srv.Close()

	client := NewClient(WithCatalogURL(srv.URL))
	got, err := client.FetchCatalog(context.Background())
	if err != nil {
		t.Fatalf("FetchCatalog: %v", err)
	}

	if got.Count != 2 {
		t.Errorf("expected Count=2, got %d", got.Count)
	}
	if len(got.Vulnerabilities) != 2 {
		t.Errorf("expected 2 vulnerabilities, got %d", len(got.Vulnerabilities))
	}
}

func TestFetchCatalog_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewClient(WithCatalogURL(srv.URL))
	_, err := client.FetchCatalog(context.Background())
	if err == nil {
		t.Error("expected error on 500 response, got nil")
	}
}
