# Product Requirements Document (PRD) - OSV.dev

## 1. Product Overview
**Product Name:** Open Source Vulnerabilities (OSV)
**Description:** OSV is an open, distributed, and easily queryable vulnerability database designed to help developers and tools accurately identify vulnerabilities in open-source dependencies.

## 2. Objectives
- Simplify vulnerability tracking for open-source developers.
- Provide a precise mapping of vulnerabilities to package versions and commits.
- Offer an easy-to-use API for automated querying and integration into CI/CD pipelines.

## 3. Key Features
- **Vulnerability Query API:** A highly available API to query vulnerabilities by commit hash, package name, or version.
- **Web Interface:** A searchable user interface for humans to browse and read vulnerability details.
- **Data Ingestion:** Automated systems to aggregate vulnerabilities from various sources (NVD, GitHub Advisories, OSS-Fuzz, Debian, Alpine, etc.).
- **Data Dumps:** Publicly accessible Google Cloud Storage buckets containing full database dumps.
- **Open Schema:** Adoption of the OpenSSF OSV schema for standardized vulnerability reporting.

## 4. Target Audience
- **Developers:** Want to know if their dependencies are vulnerable.
- **Security Researchers:** Need to report and track vulnerabilities.
- **Tool Builders:** Integrate vulnerability scanning into package managers, IDEs, and CI/CD tools (e.g., osv-scanner, Renovate, Trivy).

## 5. Success Metrics
- API Response Time.
- Number of integrated ecosystems and total vulnerabilities tracked.
- Uptake by third-party tools and platforms.
