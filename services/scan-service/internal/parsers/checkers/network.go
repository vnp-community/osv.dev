// Network checkers: curl, libcurl, OpenSSH, libssh, libssh2, OpenLDAP
package checkers

import parserentity "github.com/osv/scan-service/internal/parsers/entity"

func init() {
	// curl
	Register(CheckerDef{
		Name: "curl",
		ContainsPatterns: []string{
			`curl/\d[\d.]+`,
			`^curl\s[\d.]+`,
		},
		VersionPatterns: []string{
			`curl/([\d.]+)`,
			`^curl\s([\d.]+)`,
		},
		FilenamePatterns: []string{
			`(?i)^curl$`,
			`(?i)^curl-\d`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "haxx", Product: "curl"},
		},
	})

	// libcurl
	Register(CheckerDef{
		Name: "libcurl",
		ContainsPatterns: []string{
			`libcurl/\d`,
			`libcurl-\d`,
		},
		VersionPatterns: []string{
			`libcurl/([\d.]+)`,
		},
		FilenamePatterns: []string{
			`(?i)^libcurl\.so`,
			`(?i)^libcurl-\d`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "haxx", Product: "libcurl"},
		},
	})

	// OpenSSH
	Register(CheckerDef{
		Name: "openssh",
		ContainsPatterns: []string{
			`OpenSSH_[\d.]+[a-zA-Z0-9]*`,
		},
		VersionPatterns: []string{
			`OpenSSH_([\d.]+[a-zA-Z][0-9]*)`,
			`OpenSSH_([\d.]+)`,
		},
		FilenamePatterns: []string{
			`(?i)^sshd$`,
			`(?i)^ssh$`,
			`(?i)^libssh_utils`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "openbsd", Product: "openssh"},
		},
		IgnorePatterns: []string{
			`OpenSSH is a derivative`,
			`#\s*define\s+SSH_VERSION`,
		},
	})

	// libssh
	Register(CheckerDef{
		Name: "libssh",
		ContainsPatterns: []string{
			`libssh-\d`,
			`libssh version \d`,
		},
		VersionPatterns: []string{
			`libssh-([\d.]+)`,
			`libssh version ([\d.]+)`,
		},
		FilenamePatterns: []string{
			`(?i)^libssh\.so`,
			`(?i)^libssh-\d`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "libssh", Product: "libssh"},
		},
	})

	// libssh2
	Register(CheckerDef{
		Name: "libssh2",
		ContainsPatterns: []string{
			`libssh2/\d`,
			`LIBSSH2_VERSION\s`,
		},
		VersionPatterns: []string{
			`libssh2/([\d.]+)`,
			`LIBSSH2_VERSION\s+"([\d.]+)"`,
		},
		FilenamePatterns: []string{
			`(?i)^libssh2\.so`,
			`(?i)^libssh2-\d`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "libssh2", Product: "libssh2"},
		},
	})

	// OpenLDAP
	Register(CheckerDef{
		Name: "openldap",
		ContainsPatterns: []string{
			`OpenLDAP\s\d\.\d`,
			`OPENLDAP_API_VERSION`,
		},
		VersionPatterns: []string{
			`OpenLDAP\s([\d.]+)`,
		},
		FilenamePatterns: []string{
			`(?i)^libldap\.so`,
			`(?i)^slapd$`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "openldap", Product: "openldap"},
		},
	})
}
