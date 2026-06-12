// System library checkers: glibc, busybox, bash, sudo, systemd
// Database checkers: sqlite, mysql
package checkers

import parserentity "github.com/osv/scan-service/internal/parsers/entity"

func init() {
	// glibc
	Register(CheckerDef{
		Name: "glibc",
		ContainsPatterns: []string{
			`GNU C Library`,
			`GLIBC_\d`,
			`GNU libc version`,
		},
		VersionPatterns: []string{
			`GLIBC_([\d.]+)`,
			`GNU C Library[^\d]+([\d.]+)`,
			`glibc ([\d.]+)`,
		},
		FilenamePatterns: []string{
			`(?i)^libc\.so`,
			`(?i)^libc-\d`,
			`(?i)^ld-linux`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "gnu", Product: "glibc"},
		},
		IgnorePatterns: []string{
			`#\s*define.*GLIBC`,
		},
	})

	// BusyBox
	Register(CheckerDef{
		Name: "busybox",
		ContainsPatterns: []string{
			`BusyBox v\d`,
			`BusyBox, version \d`,
		},
		VersionPatterns: []string{
			`BusyBox v([\d.]+)`,
			`BusyBox, version ([\d.]+)`,
		},
		FilenamePatterns: []string{
			`(?i)^busybox$`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "busybox", Product: "busybox"},
		},
	})

	// bash
	Register(CheckerDef{
		Name: "bash",
		ContainsPatterns: []string{
			`GNU bash, version \d`,
			`bash.*version \d`,
		},
		VersionPatterns: []string{
			`GNU bash, version ([\d.]+)`,
			`bash.*version ([\d.]+)`,
		},
		FilenamePatterns: []string{
			`(?i)^bash$`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "gnu", Product: "bash"},
		},
	})

	// sudo
	Register(CheckerDef{
		Name: "sudo",
		ContainsPatterns: []string{
			`Sudo version \d`,
			`sudoedit`,
		},
		VersionPatterns: []string{
			`Sudo version ([\d.]+[a-z]?\d*)`,
		},
		FilenamePatterns: []string{
			`(?i)^sudo$`,
			`(?i)^sudoedit$`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "sudo", Product: "sudo"},
			{Vendor: "todd_miller", Product: "sudo"},
		},
	})

	// systemd
	Register(CheckerDef{
		Name: "systemd",
		ContainsPatterns: []string{
			`systemd \d`,
			`SYSTEMD_VERSION`,
		},
		VersionPatterns: []string{
			`systemd ([\d]+)`,
			`SYSTEMD_VERSION\s+"([\d]+)"`,
		},
		FilenamePatterns: []string{
			`(?i)^systemd$`,
			`(?i)^libsystemd\.so`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "freedesktop", Product: "systemd"},
			{Vendor: "systemd_project", Product: "systemd"},
		},
	})

	// SQLite
	Register(CheckerDef{
		Name: "sqlite",
		ContainsPatterns: []string{
			`SQLite[^\d]+(version\s+)?[\d.]+`,
			`SQLITE_VERSION`,
		},
		VersionPatterns: []string{
			`SQLite(?:\s+version)?\s+([\d.]+)`,
			`SQLITE_VERSION\s+"([\d.]+)"`,
		},
		FilenamePatterns: []string{
			`(?i)^libsqlite3\.so`,
			`(?i)^sqlite3$`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "sqlite", Product: "sqlite"},
		},
	})

	// MySQL / MariaDB client library
	Register(CheckerDef{
		Name: "mysql",
		ContainsPatterns: []string{
			`mysql\s+Ver\s+\d`,
			`MySQL server version \d`,
			`mysqld\s+Ver\s+\d`,
		},
		VersionPatterns: []string{
			`Ver\s+([\d.]+)`,
			`MySQL server version ([\d.]+)`,
		},
		FilenamePatterns: []string{
			`(?i)^libmysqlclient\.so`,
			`(?i)^mysqld$`,
			`(?i)^mysql$`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "mysql", Product: "mysql"},
			{Vendor: "oracle", Product: "mysql"},
		},
	})

	// OpenSSL 3.x additional pattern
	Register(CheckerDef{
		Name: "openssl3",
		ContainsPatterns: []string{
			`OpenSSL\s+3\.`,
		},
		VersionPatterns: []string{
			`OpenSSL\s+(3\.[\d.]+[a-z]?)`,
		},
		FilenamePatterns: []string{
			`(?i)^libssl\.so\.3`,
			`(?i)^libcrypto\.so\.3`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "openssl", Product: "openssl"},
		},
	})

	// GNU Wget
	Register(CheckerDef{
		Name: "wget",
		ContainsPatterns: []string{
			`GNU Wget \d`,
			`Wget/\d`,
		},
		VersionPatterns: []string{
			`GNU Wget ([\d.]+)`,
			`Wget/([\d.]+)`,
		},
		FilenamePatterns: []string{
			`(?i)^wget$`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "gnu", Product: "wget"},
		},
	})

	// OpenVPN
	Register(CheckerDef{
		Name: "openvpn",
		ContainsPatterns: []string{
			`OpenVPN\s+\d`,
			`OpenVPN \d[\d.]+`,
		},
		VersionPatterns: []string{
			`OpenVPN\s+([\d.]+)`,
		},
		FilenamePatterns: []string{
			`(?i)^openvpn$`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "openvpn", Product: "openvpn"},
		},
	})

	// Samba
	Register(CheckerDef{
		Name: "samba",
		ContainsPatterns: []string{
			`Samba version \d`,
			`Samba \d\.\d`,
		},
		VersionPatterns: []string{
			`Samba version ([\d.]+)`,
			`Samba ([\d.]+)`,
		},
		FilenamePatterns: []string{
			`(?i)^smbd$`,
			`(?i)^libsamba`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "samba", Product: "samba"},
		},
	})
}
