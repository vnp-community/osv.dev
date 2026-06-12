package cveid_test

import (
	"testing"

	"github.com/osv/pkg/cveid"
)

func TestIsValid(t *testing.T) {
	valid := []string{
		"CVE-2021-44228",
		"CVE-2024-12345",
		"CVE-1999-0001",
		"CVE-2023-99999999", // long sequence
		"CVE-2024-1234",    // minimum 4 digits
	}
	for _, id := range valid {
		if !cveid.IsValid(id) {
			t.Errorf("IsValid(%q) = false, want true", id)
		}
	}

	invalid := []string{
		"",
		"CVE-2021-123",    // 3-digit sequence (too short)
		"cve-2021-44228",  // lowercase
		"CVE-21-44228",    // 2-digit year
		"CVE-2021",        // no sequence
		"GHSA-1234-5678",  // wrong prefix
		"CVE2021-44228",   // missing first hyphen
		"CVE-2021-44228x", // trailing garbage
		" CVE-2021-44228", // leading space (not normalized)
	}
	for _, id := range invalid {
		if cveid.IsValid(id) {
			t.Errorf("IsValid(%q) = true, want false", id)
		}
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"CVE-2021-44228", "CVE-2021-44228"},
		{" CVE-2021-44228 ", "CVE-2021-44228"},
		{"cve-2021-44228", "CVE-2021-44228"},
		{"Cve-2021-44228", "CVE-2021-44228"},
	}
	for _, tc := range tests {
		got := cveid.Normalize(tc.in)
		if got != tc.want {
			t.Errorf("Normalize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestYear(t *testing.T) {
	y, err := cveid.Year("CVE-2021-44228")
	if err != nil || y != 2021 {
		t.Errorf("Year() = %d, %v", y, err)
	}

	_, err = cveid.Year("invalid")
	if err == nil {
		t.Error("Year(invalid) should return error")
	}
}

func TestSequence(t *testing.T) {
	s, err := cveid.Sequence("CVE-2021-44228")
	if err != nil || s != 44228 {
		t.Errorf("Sequence() = %d, %v", s, err)
	}
}

func TestNormalizeAndValidate(t *testing.T) {
	n, ok := cveid.NormalizeAndValidate("cve-2021-44228")
	if !ok || n != "CVE-2021-44228" {
		t.Errorf("NormalizeAndValidate() = %q, %v", n, ok)
	}

	_, ok = cveid.NormalizeAndValidate("invalid")
	if ok {
		t.Error("NormalizeAndValidate(invalid) should return ok=false")
	}
}
