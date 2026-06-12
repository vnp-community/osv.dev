// Package tagging provides CWE-based and Package-based auto-tagging extensions.
// TASK-02-03a: CWE-based tags — map CWE IDs to attack-type and category tags.
// TASK-02-03b: Package-based tags — infer tags from affected package names.
package tagging

import (
	"strings"
)

// Tag is a string label applied to a CVE for categorization and search.
type Tag = string

// ---- CWE-Based Tagging (TASK-02-03a) ----

// cweTagMap maps CWE IDs to a list of associated tags.
// Based on CWE Research Concepts and established security taxonomy.
var cweTagMap = map[string][]Tag{
	// Injection
	"CWE-89":  {"sqli", "injection", "database"},
	"CWE-564": {"sqli", "injection"},
	"CWE-90":  {"ldap-injection", "injection"},
	"CWE-91":  {"xpath-injection", "injection"},
	"CWE-79":  {"xss", "injection", "web"},
	"CWE-80":  {"xss", "injection", "web"},
	"CWE-83":  {"xss", "injection", "web"},
	"CWE-87":  {"xss", "injection", "web"},
	"CWE-78":  {"command-injection", "injection", "rce"},
	"CWE-77":  {"command-injection", "injection"},
	"CWE-94":  {"code-injection", "injection", "rce"},
	"CWE-95":  {"code-injection", "injection"},
	"CWE-96":  {"code-injection", "injection"},

	// Memory safety
	"CWE-119": {"memory-safety", "buffer-overflow"},
	"CWE-120": {"memory-safety", "buffer-overflow"},
	"CWE-121": {"memory-safety", "stack-overflow"},
	"CWE-122": {"memory-safety", "heap-overflow"},
	"CWE-123": {"memory-safety"},
	"CWE-124": {"memory-safety"},
	"CWE-125": {"memory-safety", "out-of-bounds-read"},
	"CWE-126": {"memory-safety"},
	"CWE-127": {"memory-safety"},
	"CWE-416": {"memory-safety", "use-after-free"},
	"CWE-415": {"memory-safety", "double-free"},
	"CWE-787": {"memory-safety", "out-of-bounds-write"},
	"CWE-788": {"memory-safety"},
	"CWE-476": {"memory-safety", "null-pointer"},
	"CWE-824": {"memory-safety"},
	"CWE-825": {"memory-safety", "use-after-free"},

	// Path/File
	"CWE-22":  {"path-traversal", "file"},
	"CWE-23":  {"path-traversal", "file"},
	"CWE-24":  {"path-traversal", "file"},
	"CWE-36":  {"path-traversal", "file"},
	"CWE-73":  {"file", "open-redirect"},
	"CWE-434": {"file-upload", "file"},

	// Access control
	"CWE-284": {"access-control", "authorization"},
	"CWE-285": {"access-control", "authorization"},
	"CWE-862": {"missing-auth", "access-control"},
	"CWE-863": {"incorrect-auth", "access-control"},
	"CWE-269": {"privesc", "access-control"},
	"CWE-266": {"privesc", "access-control"},
	"CWE-267": {"privesc", "access-control"},
	"CWE-732": {"permissions", "access-control"},
	"CWE-276": {"permissions", "access-control"},

	// Authentication
	"CWE-287": {"auth-bypass", "authentication"},
	"CWE-288": {"auth-bypass", "authentication"},
	"CWE-290": {"auth-bypass", "authentication"},
	"CWE-295": {"cert-validation", "authentication"},
	"CWE-297": {"cert-validation", "authentication"},
	"CWE-298": {"cert-validation", "authentication"},
	"CWE-306": {"missing-auth", "authentication"},
	"CWE-307": {"brute-force", "authentication"},
	"CWE-308": {"authentication"},
	"CWE-322": {"authentication"},

	// Cryptography
	"CWE-310": {"crypto", "cryptography"},
	"CWE-320": {"crypto", "cryptography"},
	"CWE-326": {"weak-crypto", "cryptography"},
	"CWE-327": {"weak-crypto", "cryptography"},
	"CWE-330": {"weak-random", "cryptography"},
	"CWE-338": {"weak-random", "cryptography"},

	// Information disclosure
	"CWE-200": {"info-disclosure"},
	"CWE-201": {"info-disclosure"},
	"CWE-202": {"info-disclosure"},
	"CWE-203": {"info-disclosure", "timing-attack"},
	"CWE-209": {"info-disclosure"},
	"CWE-215": {"info-disclosure"},
	"CWE-312": {"info-disclosure", "cleartext"},
	"CWE-313": {"info-disclosure", "cleartext"},
	"CWE-319": {"info-disclosure", "cleartext"},

	// DoS
	"CWE-400": {"dos", "resource-exhaustion"},
	"CWE-401": {"dos", "memory-leak"},
	"CWE-404": {"dos"},
	"CWE-407": {"dos", "algorithmic-complexity"},
	"CWE-674": {"dos", "stack-overflow"},
	"CWE-770": {"dos", "resource-exhaustion"},
	"CWE-834": {"dos"},

	// SSRF/XXE
	"CWE-918": {"ssrf", "injection"},
	"CWE-611": {"xxe", "injection", "xml"},
	"CWE-776": {"xxe", "dos", "xml"},

	// Deserialization
	"CWE-502": {"deserialization", "rce"},

	// Race condition
	"CWE-362": {"race-condition"},
	"CWE-364": {"race-condition"},
	"CWE-366": {"race-condition"},
	"CWE-367": {"race-condition", "toctou"},

	// Redirect
	"CWE-601": {"open-redirect", "web"},

	// SQL / data format
	"CWE-643": {"xpath-injection", "injection"},
	"CWE-917": {"expression-injection", "injection"},

	// Integer
	"CWE-190": {"integer-overflow", "memory-safety"},
	"CWE-191": {"integer-underflow", "memory-safety"},
	"CWE-192": {"integer"},
	"CWE-193": {"off-by-one", "memory-safety"},
	"CWE-194": {"integer"},
	"CWE-195": {"integer"},
}

// TagsFromCWEs returns tags inferred from CWE IDs.
// TASK-02-03a implementation.
func TagsFromCWEs(cweIDs []string) []Tag {
	seen := make(map[Tag]bool)
	var tags []Tag
	for _, cweID := range cweIDs {
		// Normalize: "CWE-79" or "79"
		id := strings.ToUpper(strings.TrimSpace(cweID))
		if !strings.HasPrefix(id, "CWE-") {
			id = "CWE-" + id
		}
		if mappedTags, ok := cweTagMap[id]; ok {
			for _, t := range mappedTags {
				if !seen[t] {
					seen[t] = true
					tags = append(tags, t)
				}
			}
		}
	}
	return tags
}

// ---- Package-Based Tagging (TASK-02-03b) ----

// packageRule maps package name patterns (lowercase substring) to tags.
type packageRule struct {
	pattern string // substring match on package name (lowercase)
	tags    []Tag
}

// packageTagRules maps package name patterns to auto-assigned tags.
var packageTagRules = []packageRule{
	// Web frameworks
	{"django", []Tag{"web", "python-framework"}},
	{"flask", []Tag{"web", "python-framework"}},
	{"fastapi", []Tag{"web", "python-framework"}},
	{"rails", []Tag{"web", "ruby-framework"}},
	{"spring", []Tag{"web", "java-framework"}},
	{"laravel", []Tag{"web", "php-framework"}},
	{"express", []Tag{"web", "nodejs-framework"}},
	{"next.js", []Tag{"web", "nodejs-framework"}},
	{"nuxt", []Tag{"web", "nodejs-framework"}},
	{"gin-gonic", []Tag{"web", "go-framework"}},
	{"echo", []Tag{"web", "go-framework"}},
	{"fiber", []Tag{"web", "go-framework"}},

	// Databases
	{"mysql", []Tag{"database", "sql"}},
	{"postgresql", []Tag{"database", "sql"}},
	{"sqlite", []Tag{"database", "sql"}},
	{"mongodb", []Tag{"database", "nosql"}},
	{"redis", []Tag{"database", "cache"}},
	{"elasticsearch", []Tag{"database", "search"}},
	{"opensearch", []Tag{"database", "search"}},
	{"kafka", []Tag{"messaging", "streaming"}},
	{"rabbitmq", []Tag{"messaging"}},

	// Crypto / TLS
	{"openssl", []Tag{"crypto", "tls", "network"}},
	{"mbedtls", []Tag{"crypto", "tls", "embedded"}},
	{"boringssl", []Tag{"crypto", "tls"}},
	{"gnutls", []Tag{"crypto", "tls"}},
	{"libressl", []Tag{"crypto", "tls"}},
	{"cryptography", []Tag{"crypto", "python"}},
	{"bcrypt", []Tag{"crypto", "password-hashing"}},

	// OS / Kernel
	{"linux", []Tag{"kernel", "os"}},
	{"windows", []Tag{"os", "microsoft"}},
	{"android", []Tag{"os", "mobile", "android"}},
	{"ios", []Tag{"os", "mobile", "apple"}},
	{"freebsd", []Tag{"os", "bsd"}},
	{"openbsd", []Tag{"os", "bsd"}},

	// Container / Cloud
	{"docker", []Tag{"container", "cloud"}},
	{"kubernetes", []Tag{"container", "cloud", "orchestration"}},
	{"containerd", []Tag{"container", "cloud"}},
	{"helm", []Tag{"kubernetes", "cloud"}},
	{"terraform", []Tag{"iac", "cloud"}},

	// Parser / Serialization
	{"libxml", []Tag{"xml", "parser"}},
	{"expat", []Tag{"xml", "parser"}},
	{"pyyaml", []Tag{"yaml", "parser", "python"}},
	{"log4j", []Tag{"logging", "java"}},
	{"jackson", []Tag{"json", "deserialization", "java"}},

	// Network
	{"curl", []Tag{"network", "http"}},
	{"wget", []Tag{"network", "http"}},
	{"bind", []Tag{"dns", "network"}},
	{"nginx", []Tag{"web-server", "network"}},
	{"apache", []Tag{"web-server", "network"}},

	// Security tooling
	{"vault", []Tag{"secrets-management", "security"}},
	{"keycloak", []Tag{"auth", "sso", "security"}},
	{"oauth", []Tag{"auth", "security"}},
	{"jwt", []Tag{"auth", "token"}},
}

// TagsFromPackages returns tags inferred from affected package names.
// TASK-02-03b implementation.
func TagsFromPackages(packageNames []string) []Tag {
	seen := make(map[Tag]bool)
	var tags []Tag
	for _, pkg := range packageNames {
		pkgLower := strings.ToLower(pkg)
		for _, rule := range packageTagRules {
			if strings.Contains(pkgLower, rule.pattern) {
				for _, t := range rule.tags {
					if !seen[t] {
						seen[t] = true
						tags = append(tags, t)
					}
				}
			}
		}
	}
	return tags
}

// MergeTags combines and deduplicates multiple tag slices.
func MergeTags(tagSets ...[]Tag) []Tag {
	seen := make(map[Tag]bool)
	var result []Tag
	for _, tags := range tagSets {
		for _, t := range tags {
			if !seen[t] {
				seen[t] = true
				result = append(result, t)
			}
		}
	}
	return result
}
