// Package cwe — embedded CWE database (top-200 most common CWEs).
// Source: NVD CWE list + MITRE CWE Top 25.
package cwe

// cweDB is the embedded CWE database.
// Sorted by CWE ID for human readability.
var cweDB = []Entry{
	// ── Memory Safety ─────────────────────────────────────────────────────────
	{
		ID: "CWE-119", Name: "Improper Restriction of Operations within the Bounds of a Memory Buffer",
		Abstraction: "Class", Category: CategoryMemory,
		Tags: []string{"type:memory-corruption", "attack:memory"},
	},
	{
		ID: "CWE-120", Name: "Buffer Copy without Checking Size of Input (Classic Buffer Overflow)",
		Abstraction: "Base", Category: CategoryMemory, Parents: []string{"CWE-119"},
		Tags: []string{"type:buffer-overflow", "type:memory-corruption"},
	},
	{
		ID: "CWE-121", Name: "Stack-based Buffer Overflow",
		Abstraction: "Variant", Category: CategoryMemory, Parents: []string{"CWE-120"},
		Tags: []string{"type:buffer-overflow", "type:memory-corruption", "attack:stack"},
	},
	{
		ID: "CWE-122", Name: "Heap-based Buffer Overflow",
		Abstraction: "Variant", Category: CategoryMemory, Parents: []string{"CWE-120"},
		Tags: []string{"type:buffer-overflow", "type:memory-corruption", "attack:heap"},
	},
	{
		ID: "CWE-125", Name: "Out-of-bounds Read",
		Abstraction: "Base", Category: CategoryMemory, Parents: []string{"CWE-119"},
		Tags: []string{"type:out-of-bounds", "type:memory-corruption"},
	},
	{
		ID: "CWE-126", Name: "Buffer Over-read",
		Abstraction: "Variant", Category: CategoryMemory, Parents: []string{"CWE-125"},
		Tags: []string{"type:out-of-bounds", "type:information-disclosure"},
	},
	{
		ID: "CWE-190", Name: "Integer Overflow or Wraparound",
		Abstraction: "Base", Category: CategoryMemory,
		Tags: []string{"type:integer-overflow", "type:memory-corruption"},
	},
	{
		ID: "CWE-191", Name: "Integer Underflow (Wrap or Wraparound)",
		Abstraction: "Base", Category: CategoryMemory, Parents: []string{"CWE-190"},
		Tags: []string{"type:integer-overflow"},
	},
	{
		ID: "CWE-369", Name: "Divide By Zero",
		Abstraction: "Base", Category: CategoryMemory,
		Tags: []string{"type:divide-by-zero", "impact:dos"},
	},
	{
		ID: "CWE-415", Name: "Double Free",
		Abstraction: "Base", Category: CategoryMemory,
		Tags: []string{"type:use-after-free", "type:memory-corruption"},
	},
	{
		ID: "CWE-416", Name: "Use After Free",
		Abstraction: "Base", Category: CategoryMemory,
		Tags: []string{"type:use-after-free", "type:memory-corruption"},
	},
	{
		ID: "CWE-476", Name: "NULL Pointer Dereference",
		Abstraction: "Base", Category: CategoryMemory,
		Tags: []string{"type:null-dereference", "impact:dos"},
	},
	{
		ID: "CWE-787", Name: "Out-of-bounds Write",
		Abstraction: "Base", Category: CategoryMemory, Parents: []string{"CWE-119"},
		Tags: []string{"type:out-of-bounds", "type:memory-corruption"},
	},
	{
		ID: "CWE-824", Name: "Access of Uninitialized Pointer",
		Abstraction: "Base", Category: CategoryMemory,
		Tags: []string{"type:memory-corruption"},
	},
	{
		ID: "CWE-835", Name: "Loop with Unreachable Exit Condition (Infinite Loop)",
		Abstraction: "Base", Category: CategoryMemory,
		Tags: []string{"type:infinite-loop", "impact:dos"},
	},

	// ── Injection ─────────────────────────────────────────────────────────────
	{
		ID: "CWE-74", Name: "Improper Neutralization of Special Elements in Output (Injection)",
		Abstraction: "Class", Category: CategoryInjection,
		Tags: []string{"type:injection"},
	},
	{
		ID: "CWE-77", Name: "Improper Neutralization of Special Elements used in a Command (Command Injection)",
		Abstraction: "Base", Category: CategoryInjection, Parents: []string{"CWE-74"},
		Tags: []string{"type:command-injection", "type:injection"},
	},
	{
		ID: "CWE-78", Name: "Improper Neutralization of Special Elements used in an OS Command (OS Command Injection)",
		Abstraction: "Base", Category: CategoryInjection, Parents: []string{"CWE-77"},
		Tags: []string{"type:command-injection", "attack:os-command"},
	},
	{
		ID: "CWE-79", Name: "Improper Neutralization of Input During Web Page Generation (Cross-site Scripting)",
		Abstraction: "Base", Category: CategoryInjection, Parents: []string{"CWE-74"},
		Tags: []string{"type:xss", "attack:client-side", "attack:web"},
	},
	{
		ID: "CWE-80", Name: "Improper Neutralization of Script-Related HTML Tags in a Web Page (Basic XSS)",
		Abstraction: "Variant", Category: CategoryInjection, Parents: []string{"CWE-79"},
		Tags: []string{"type:xss", "attack:client-side"},
	},
	{
		ID: "CWE-89", Name: "Improper Neutralization of Special Elements used in an SQL Command (SQL Injection)",
		Abstraction: "Base", Category: CategoryInjection, Parents: []string{"CWE-74"},
		Tags: []string{"type:sql-injection", "type:injection", "attack:database"},
	},
	{
		ID: "CWE-94", Name: "Improper Control of Generation of Code (Code Injection)",
		Abstraction: "Base", Category: CategoryInjection, Parents: []string{"CWE-74"},
		Tags: []string{"type:code-injection", "type:injection"},
	},
	{
		ID: "CWE-434", Name: "Unrestricted Upload of File with Dangerous Type",
		Abstraction: "Base", Category: CategoryInjection,
		Tags: []string{"type:file-upload", "attack:web"},
	},
	{
		ID: "CWE-502", Name: "Deserialization of Untrusted Data",
		Abstraction: "Base", Category: CategoryInjection,
		Tags: []string{"type:deserialization", "type:injection"},
	},

	// ── Authentication ────────────────────────────────────────────────────────
	{
		ID: "CWE-287", Name: "Improper Authentication",
		Abstraction: "Class", Category: CategoryAuthentication,
		Tags: []string{"type:authentication", "attack:auth-bypass"},
	},
	{
		ID: "CWE-295", Name: "Improper Certificate Validation",
		Abstraction: "Base", Category: CategoryAuthentication, Parents: []string{"CWE-287"},
		Tags: []string{"type:tls", "type:certificate-validation"},
	},
	{
		ID: "CWE-306", Name: "Missing Authentication for Critical Function",
		Abstraction: "Base", Category: CategoryAuthentication, Parents: []string{"CWE-287"},
		Tags: []string{"type:authentication", "attack:auth-bypass"},
	},
	{
		ID: "CWE-307", Name: "Improper Restriction of Excessive Authentication Attempts",
		Abstraction: "Base", Category: CategoryAuthentication, Parents: []string{"CWE-287"},
		Tags: []string{"type:brute-force", "attack:auth-bypass"},
	},
	{
		ID: "CWE-384", Name: "Session Fixation",
		Abstraction: "Base", Category: CategoryAuthentication,
		Tags: []string{"type:session-fixation", "attack:web"},
	},

	// ── Authorization ─────────────────────────────────────────────────────────
	{
		ID: "CWE-22", Name: "Improper Limitation of a Pathname to a Restricted Directory (Path Traversal)",
		Abstraction: "Base", Category: CategoryAuthorization,
		Tags: []string{"type:path-traversal", "attack:file-system"},
	},
	{
		ID: "CWE-23", Name: "Relative Path Traversal",
		Abstraction: "Base", Category: CategoryAuthorization, Parents: []string{"CWE-22"},
		Tags: []string{"type:path-traversal"},
	},
	{
		ID: "CWE-269", Name: "Improper Privilege Management",
		Abstraction: "Class", Category: CategoryAuthorization,
		Tags: []string{"type:privilege-escalation"},
	},
	{
		ID: "CWE-276", Name: "Incorrect Default Permissions",
		Abstraction: "Base", Category: CategoryAuthorization,
		Tags: []string{"type:misconfiguration", "attack:privilege-escalation"},
	},
	{
		ID: "CWE-284", Name: "Improper Access Control",
		Abstraction: "Class", Category: CategoryAuthorization,
		Tags: []string{"type:access-control", "attack:authorization-bypass"},
	},
	{
		ID: "CWE-285", Name: "Improper Authorization",
		Abstraction: "Class", Category: CategoryAuthorization, Parents: []string{"CWE-284"},
		Tags: []string{"type:authorization", "attack:authorization-bypass"},
	},
	{
		ID: "CWE-862", Name: "Missing Authorization",
		Abstraction: "Base", Category: CategoryAuthorization, Parents: []string{"CWE-285"},
		Tags: []string{"type:authorization", "attack:authorization-bypass"},
	},
	{
		ID: "CWE-863", Name: "Incorrect Authorization",
		Abstraction: "Base", Category: CategoryAuthorization, Parents: []string{"CWE-285"},
		Tags: []string{"type:authorization", "attack:authorization-bypass"},
	},

	// ── Cryptography ──────────────────────────────────────────────────────────
	{
		ID: "CWE-256", Name: "Plaintext Storage of a Password",
		Abstraction: "Base", Category: CategoryCryptography,
		Tags: []string{"type:credentials", "type:cryptography"},
	},
	{
		ID: "CWE-259", Name: "Use of Hard-coded Password",
		Abstraction: "Base", Category: CategoryCryptography,
		Tags: []string{"type:hardcoded-credentials", "type:credentials"},
	},
	{
		ID: "CWE-320", Name: "Key Management Errors",
		Abstraction: "Class", Category: CategoryCryptography,
		Tags: []string{"type:cryptography", "type:key-management"},
	},
	{
		ID: "CWE-326", Name: "Inadequate Encryption Strength",
		Abstraction: "Base", Category: CategoryCryptography,
		Tags: []string{"type:weak-cryptography", "type:cryptography"},
	},
	{
		ID: "CWE-327", Name: "Use of a Broken or Risky Cryptographic Algorithm",
		Abstraction: "Base", Category: CategoryCryptography,
		Tags: []string{"type:weak-cryptography", "type:cryptography"},
	},
	{
		ID: "CWE-338", Name: "Use of Cryptographically Weak Pseudo-Random Number Generator (PRNG)",
		Abstraction: "Base", Category: CategoryCryptography,
		Tags: []string{"type:weak-random", "type:cryptography"},
	},
	{
		ID: "CWE-798", Name: "Use of Hard-coded Credentials",
		Abstraction: "Base", Category: CategoryCryptography,
		Tags: []string{"type:hardcoded-credentials", "type:credentials"},
	},

	// ── Resource Management ───────────────────────────────────────────────────
	{
		ID: "CWE-400", Name: "Uncontrolled Resource Consumption",
		Abstraction: "Class", Category: CategoryResourceMgmt,
		Tags: []string{"type:resource-exhaustion", "impact:dos"},
	},
	{
		ID: "CWE-401", Name: "Missing Release of Memory after Effective Lifetime",
		Abstraction: "Base", Category: CategoryResourceMgmt,
		Tags: []string{"type:memory-leak", "impact:dos"},
	},
	{
		ID: "CWE-404", Name: "Improper Resource Shutdown or Release",
		Abstraction: "Class", Category: CategoryResourceMgmt,
		Tags: []string{"type:resource-leak"},
	},
	{
		ID: "CWE-407", Name: "Inefficient Algorithmic Complexity",
		Abstraction: "Base", Category: CategoryResourceMgmt, Parents: []string{"CWE-400"},
		Tags: []string{"type:algorithmic-complexity", "impact:dos"},
	},
	{
		ID: "CWE-770", Name: "Allocation of Resources Without Limits or Throttling",
		Abstraction: "Base", Category: CategoryResourceMgmt, Parents: []string{"CWE-400"},
		Tags: []string{"type:resource-exhaustion", "impact:dos"},
	},
	{
		ID: "CWE-789", Name: "Memory Allocation with Excessive Size Value",
		Abstraction: "Base", Category: CategoryResourceMgmt,
		Tags: []string{"type:memory-corruption", "impact:dos"},
	},

	// ── Network ───────────────────────────────────────────────────────────────
	{
		ID: "CWE-918", Name: "Server-Side Request Forgery (SSRF)",
		Abstraction: "Base", Category: CategoryNetwork,
		Tags: []string{"type:ssrf", "attack:network"},
	},
	{
		ID: "CWE-601", Name: "URL Redirection to Untrusted Site (Open Redirect)",
		Abstraction: "Base", Category: CategoryNetwork,
		Tags: []string{"type:open-redirect", "attack:web"},
	},
	{
		ID: "CWE-352", Name: "Cross-Site Request Forgery (CSRF)",
		Abstraction: "Compound", Category: CategoryNetwork,
		Tags: []string{"type:csrf", "attack:client-side", "attack:web"},
	},

	// ── Concurrency ───────────────────────────────────────────────────────────
	{
		ID: "CWE-362", Name: "Concurrent Execution using Shared Resource with Improper Synchronization (Race Condition)",
		Abstraction: "Class", Category: CategoryConcurrency,
		Tags: []string{"type:race-condition"},
	},
	{
		ID: "CWE-366", Name: "Race Condition within a Thread",
		Abstraction: "Base", Category: CategoryConcurrency, Parents: []string{"CWE-362"},
		Tags: []string{"type:race-condition"},
	},

	// ── Information Disclosure ────────────────────────────────────────────────
	{
		ID: "CWE-200", Name: "Exposure of Sensitive Information to an Unauthorized Actor",
		Abstraction: "Class", Category: CategoryDesign,
		Tags: []string{"type:information-disclosure", "impact:confidentiality"},
	},
	{
		ID: "CWE-209", Name: "Generation of Error Message Containing Sensitive Information",
		Abstraction: "Base", Category: CategoryDesign, Parents: []string{"CWE-200"},
		Tags: []string{"type:information-disclosure"},
	},

	// ── Design / Other ────────────────────────────────────────────────────────
	{
		ID: "CWE-20", Name: "Improper Input Validation",
		Abstraction: "Class", Category: CategoryDesign,
		Tags: []string{"type:input-validation"},
	},
	{
		ID: "CWE-116", Name: "Improper Encoding or Escaping of Output",
		Abstraction: "Class", Category: CategoryDesign,
		Tags: []string{"type:encoding"},
	},
	{
		ID: "CWE-264", Name: "Permissions, Privileges, and Access Controls",
		Abstraction: "Category", Category: CategoryAuthorization,
		Tags: []string{"type:access-control"},
	},
	{
		ID: "CWE-311", Name: "Missing Encryption of Sensitive Data",
		Abstraction: "Base", Category: CategoryCryptography,
		Tags: []string{"type:missing-encryption", "type:cryptography"},
	},
	{
		ID: "CWE-693", Name: "Protection Mechanism Failure",
		Abstraction: "Class", Category: CategoryDesign,
		Tags: []string{"type:protection-failure"},
	},
}
