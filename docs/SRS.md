# Software Requirements Specification (SRS) - OSV.dev

## 1. Introduction
This SRS outlines the system architecture and functional/non-functional requirements for the OSV.dev platform.

## 2. System Architecture
The system is built primarily on Google Cloud Platform (GCP).
- **Frontend / Web UI:** Web interface to browse and display vulnerabilities (`gcp/website`).
- **API Server:** High-performance API handling query requests (`gcp/api`).
- **Data Storage:** GCP Datastore/Firestore for persistent vulnerability records (`models.py`, `index.yaml`).
- **Background Workers:** Worker instances for impact analysis, bisection, and data ingestion (`gcp/workers`).
- **Ecosystem Importers:** Scripts and Go modules (`vulnfeeds`) to convert data from various ecosystems (NVD, Alpine, Debian, PyPI) into the OSV schema.

## 3. Functional Requirements
- **FR-01 (API Query):** The API must support POST requests to query vulnerabilities by (package, version) and by commit hash.
- **FR-02 (Data Ingestion):** The system must routinely fetch, parse, and ingest vulnerability feeds from configured external sources.
- **FR-03 (Bisection/Impact):** The system must be capable of analyzing commit ranges to precisely determine affected versions (impact analysis).
- **FR-04 (Web Interface):** The system must provide a public-facing website rendering vulnerability records in human-readable HTML.
- **FR-05 (Data Export):** The system must generate and upload complete data dumps to Google Cloud Storage periodically.

## 4. Non-Functional Requirements
- **NFR-01 (Performance):** The API should serve average queries in under 100ms.
- **NFR-02 (Availability):** The core API and web interface must have high availability (99.9% uptime).
- **NFR-03 (Scalability):** The system must auto-scale to handle traffic spikes, particularly for the API server.
- **NFR-04 (Maintainability):** Code must be linted and tested. Python uses standard formatting, and Go uses `golangci-lint`.
- **NFR-05 (Security):** The infrastructure must be deployed securely using Terraform, with appropriate IAM roles and principle of least privilege.
