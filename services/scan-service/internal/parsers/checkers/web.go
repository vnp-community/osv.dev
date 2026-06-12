// Web/HTTP server checkers: nginx, Apache httpd, Apache Tomcat, lighttpd
package checkers

import parserentity "github.com/osv/scan-service/internal/parsers/entity"

func init() {
	// nginx
	Register(CheckerDef{
		Name: "nginx",
		ContainsPatterns: []string{
			`nginx/\d`,
			`nginx version: nginx/\d`,
		},
		VersionPatterns: []string{
			`nginx/([\d.]+)`,
		},
		FilenamePatterns: []string{
			`(?i)^nginx$`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "nginx", Product: "nginx"},
			{Vendor: "f5", Product: "nginx"},
		},
		IgnorePatterns: []string{
			`#\s*define\s+NGINX_VER`,
		},
	})

	// Apache httpd
	Register(CheckerDef{
		Name: "apache",
		ContainsPatterns: []string{
			`Apache/\d`,
			`Apache HTTP Server`,
			`httpd/\d`,
		},
		VersionPatterns: []string{
			`Apache/([\d.]+)`,
			`httpd/([\d.]+)`,
		},
		FilenamePatterns: []string{
			`(?i)^httpd$`,
			`(?i)^apache2$`,
			`(?i)^libhttpd`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "apache", Product: "http_server"},
		},
	})

	// Apache Tomcat
	Register(CheckerDef{
		Name: "apache_tomcat",
		ContainsPatterns: []string{
			`Apache Tomcat/\d`,
			`Tomcat/\d[\d.]+`,
		},
		VersionPatterns: []string{
			`Apache Tomcat/([\d.]+)`,
			`Tomcat/([\d.]+)`,
		},
		FilenamePatterns: []string{
			`(?i)catalina\.jar`,
			`(?i)tomcat-.*\.jar`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "apache", Product: "tomcat"},
		},
	})

	// lighttpd
	Register(CheckerDef{
		Name: "lighttpd",
		ContainsPatterns: []string{
			`lighttpd/\d`,
			`lighttpd-\d`,
		},
		VersionPatterns: []string{
			`lighttpd/([\d.]+)`,
		},
		FilenamePatterns: []string{
			`(?i)^lighttpd$`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "lighttpd", Product: "lighttpd"},
		},
	})
}
