package cpe_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osv/pkg/cpe"
)

var testCases = []struct {
	name string
	uri  string
	fs   string
}{
	{
		name: "log4j",
		uri:  "cpe:/a:apache:log4j:2.14.1",
		fs:   "cpe:2.3:a:apache:log4j:2.14.1:*:*:*:*:*:*:*",
	},
	{
		name: "linux kernel",
		uri:  "cpe:/o:linux:linux_kernel:5.15.0",
		fs:   "cpe:2.3:o:linux:linux_kernel:5.15.0:*:*:*:*:*:*:*",
	},
	{
		name: "openssl application",
		uri:  "cpe:/a:openssl:openssl:1.0.1",
		fs:   "cpe:2.3:a:openssl:openssl:1.0.1:*:*:*:*:*:*:*",
	},
}

func TestURIToFS(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := cpe.URIToFS(tc.uri)
			require.NoError(t, err)
			assert.Equal(t, tc.fs, result)
		})
	}
}

func TestFSToURI(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := cpe.FSToURI(tc.fs)
			require.NoError(t, err)
			assert.Equal(t, tc.uri, result)
		})
	}
}

func TestRoundTrip_URI_to_FS_to_URI(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fs, err := cpe.URIToFS(tc.uri)
			require.NoError(t, err)
			uri, err := cpe.FSToURI(fs)
			require.NoError(t, err)
			assert.Equal(t, tc.uri, uri)
		})
	}
}

func TestVendorProduct_FS(t *testing.T) {
	vendor, product := cpe.VendorProduct("cpe:2.3:a:apache:log4j:2.14.1:*:*:*:*:*:*:*")
	assert.Equal(t, "apache", vendor)
	assert.Equal(t, "log4j", product)
}

func TestVendorProduct_URI(t *testing.T) {
	vendor, product := cpe.VendorProduct("cpe:/a:apache:log4j:2.14.1")
	assert.Equal(t, "apache", vendor)
	assert.Equal(t, "log4j", product)
}

func TestVendorProduct_InvalidReturnsEmpty(t *testing.T) {
	vendor, product := cpe.VendorProduct("not-a-cpe")
	assert.Empty(t, vendor)
	assert.Empty(t, product)
}

func TestIsFS(t *testing.T) {
	assert.True(t, cpe.IsFS("cpe:2.3:a:apache:log4j:2.14.1:*:*:*:*:*:*:*"))
	assert.False(t, cpe.IsFS("cpe:/a:apache:log4j"))
}

func TestIsURI(t *testing.T) {
	assert.True(t, cpe.IsURI("cpe:/a:apache:log4j"))
	assert.False(t, cpe.IsURI("cpe:2.3:a:apache:log4j:2.14.1:*:*:*:*:*:*:*"))
}

func TestParseFS_Invalid(t *testing.T) {
	_, err := cpe.ParseFS("not-a-cpe")
	assert.Error(t, err)

	_, err = cpe.ParseFS("cpe:1.0:a:apache:log4j")
	assert.Error(t, err)
}

func TestParseFS_AllWildcards(t *testing.T) {
	w, err := cpe.ParseFS("cpe:2.3:a:*:*:*:*:*:*:*:*:*:*")
	require.NoError(t, err)
	assert.Equal(t, "a", w.Part)
	assert.Equal(t, "*", w.Vendor)
	assert.Equal(t, "*", w.Product)
}

func TestToFS_EmptyFieldsBecomesWildcard(t *testing.T) {
	w := &cpe.WFN{Part: "a", Vendor: "test", Product: "prod"}
	fs := w.ToFS()
	assert.Contains(t, fs, "*")
	assert.Equal(t, "cpe:2.3:a:test:prod:*:*:*:*:*:*:*:*", fs)
}
