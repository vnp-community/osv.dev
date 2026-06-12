# User Requirements Document (URD) - OSV.dev

## 1. Introduction
This document defines the user requirements for OSV.dev, focusing on the needs of its primary users: developers, security researchers, and automated tools.

## 2. User Personas
- **Developer (Alice):** Maintains a web application and wants to ensure her dependencies do not contain known vulnerabilities.
- **Tool Builder (Bob):** Develops a CI/CD security scanner and needs a reliable source of truth for vulnerability data.
- **Security Analyst (Charlie):** Researches vulnerabilities and needs to report them in a standard format or browse existing data.

## 3. User Requirements

### 3.1. Developers (Alice)
- **UR-01:** I want to search for vulnerabilities by package name and version to see if my software is affected.
- **UR-02:** I want a clear explanation of the vulnerability, including affected versions and remediation steps (e.g., which version to upgrade to).
- **UR-03:** I want to easily browse vulnerability details through a clean web interface.

### 3.2. Tool Builders (Bob)
- **UR-04:** I need a programmatic API to query whether a specific commit hash or package version is vulnerable.
- **UR-05:** I require the API to respond quickly (low latency) to avoid slowing down CI pipelines.
- **UR-06:** I need access to bulk data dumps so I can mirror the database or perform offline analysis.
- **UR-07:** The data format must be structured, machine-readable, and well-documented (OSV Schema).

### 3.3. Security Analysts (Charlie)
- **UR-08:** I want a standardized way to describe open-source vulnerabilities.
- **UR-09:** I need a mechanism to contribute vulnerability data or corrections to the database.
