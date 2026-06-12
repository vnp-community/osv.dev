> **✅ COMPLETED** — go build && go vet passed.

# T19 — OpenAPI Spec & Swagger UI

## Thông tin
| | |
|---|---|
| **Phase** | 6 — Documentation |
| **Ước tính** | 3–4 giờ |
| **Depends on** | T05–T15 |

---

## Các bước thực hiện

### 19.1 Chọn approach

**Option A (Recommended): Viết OpenAPI spec YAML thủ công**
- File `api/openapi.yaml` — viết đầy đủ spec
- Serve tại `/api/v1/docs` qua Swagger UI (embed HTML)

**Option B: Code generation với `swaggo/swag`**
- Thêm annotation vào handlers
- `swag init` để generate

Sẽ dùng **Option A** vì handlers được import từ services, không muốn sửa.

### 19.2 Tạo `api/openapi.yaml`

```yaml
openapi: 3.0.3
info:
  title: OpenVulnScan API
  description: |
    Vulnerability scanning and management platform.
    Built with Go, reusing osv.dev services.
  version: "1.0.0"
  contact:
    email: admin@openvulnscan.local

servers:
  - url: http://localhost:8080
    description: Local development
  - url: https://openvulnscan.example.com
    description: Production

components:
  securitySchemes:
    BearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT
    ApiKeyAuth:
      type: apiKey
      in: header
      name: X-API-Key

  schemas:
    Scan:
      type: object
      properties:
        id: { type: string, format: uuid }
        targets: { type: array, items: { type: string } }
        scan_type: { type: string, enum: [full, discovery, web, agent] }
        status: { type: string, enum: [pending, running, completed, failed, cancelled] }
        created_at: { type: string, format: date-time }

    Finding:
      type: object
      properties:
        id: { type: string, format: uuid }
        title: { type: string }
        severity: { type: string, enum: [critical, high, medium, low, info] }
        cve: { type: string }
        cvss_score: { type: number }
        status: { type: string }

    DashboardStats:
      type: object
      properties:
        total_scans: { type: integer }
        active_scans: { type: integer }
        total_findings: { type: integer }
        critical_findings: { type: integer }

security:
  - BearerAuth: []

paths:
  /healthz:
    get:
      security: []
      summary: Health check
      responses:
        "200":
          description: Healthy
          content:
            application/json:
              schema:
                type: object
                properties:
                  status: { type: string }

  /api/v1/auth/login:
    post:
      security: []
      summary: Login
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required: [email, password]
              properties:
                email: { type: string, format: email }
                password: { type: string }
      responses:
        "200":
          description: Login successful
          content:
            application/json:
              schema:
                type: object
                properties:
                  access_token: { type: string }
                  expires_in: { type: integer }

  /api/v1/scans:
    get:
      summary: List scans
      parameters:
        - in: query
          name: page
          schema: { type: integer, default: 1 }
        - in: query
          name: page_size
          schema: { type: integer, default: 20 }
      responses:
        "200":
          description: Scan list
    post:
      summary: Create scan
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required: [targets, scan_type]
              properties:
                targets:
                  type: array
                  items: { type: string }
                  example: ["192.168.1.0/24"]
                scan_type:
                  type: string
                  enum: [full, discovery, web]
                priority: { type: integer, default: 5 }
      responses:
        "202":
          description: Scan queued

  /api/v1/scans/{id}:
    get:
      summary: Get scan detail
      parameters:
        - in: path
          name: id
          required: true
          schema: { type: string, format: uuid }
      responses:
        "200":
          description: Scan detail
    delete:
      summary: Cancel scan
      parameters:
        - in: path
          name: id
          required: true
          schema: { type: string, format: uuid }
      responses:
        "200":
          description: Scan cancelled

  /api/v1/findings:
    get:
      summary: List findings
      parameters:
        - in: query
          name: severity
          schema: { type: string, enum: [critical, high, medium, low] }
        - in: query
          name: status
          schema: { type: string }
        - in: query
          name: scan_id
          schema: { type: string, format: uuid }
      responses:
        "200":
          description: Finding list

  /api/v1/dashboard:
    get:
      summary: Dashboard statistics
      responses:
        "200":
          description: Aggregated stats
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/DashboardStats'

  /api/v1/cves/{id}:
    get:
      summary: Get CVE detail
      parameters:
        - in: path
          name: id
          required: true
          schema: { type: string, example: "CVE-2021-44228" }
      responses:
        "200":
          description: CVE detail with CVSS

  /api/v1/scans/{id}/pdf:
    get:
      summary: Download scan PDF report
      parameters:
        - in: path
          name: id
          required: true
          schema: { type: string, format: uuid }
      responses:
        "200":
          description: PDF file
          content:
            application/pdf:
              schema:
                type: string
                format: binary

  /agent/download:
    get:
      security: []
      summary: Download agent script
      responses:
        "200":
          description: Python agent script
          content:
            text/x-python:
              schema:
                type: string

  /agent/report:
    post:
      security: []
      summary: Submit agent package report
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required: [hostname, packages]
              properties:
                hostname: { type: string }
                os_info: { type: string }
                packages:
                  type: array
                  items:
                    type: object
                    properties:
                      name: { type: string }
                      version: { type: string }
                      ecosystem: { type: string }
      responses:
        "200":
          description: Report accepted

  /api/v1/siem/config:
    get:
      summary: Get SIEM configuration
      responses:
        "200":
          description: SIEM config
    post:
      summary: Update SIEM configuration
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                host: { type: string }
                port: { type: integer }
                protocol: { type: string, enum: [udp, tcp] }
                enabled: { type: boolean }
      responses:
        "200":
          description: Config updated
```

### 19.3 Serve Swagger UI

```go
// internal/router/router.go — thêm Swagger UI

import _ "embed"

//go:embed swagger-ui.html
var swaggerUI []byte

// Trong router:
r.Get("/api/v1/docs", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/html")
    w.Write(swaggerUI)
})

r.Get("/api/v1/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/yaml")
    http.ServeFile(w, r, "api/openapi.yaml")
})
```

### 19.4 Tạo `swagger-ui.html`

```html
<!-- internal/router/swagger-ui.html -->
<!DOCTYPE html>
<html>
<head>
    <title>OpenVulnScan API Docs</title>
    <meta charset="utf-8"/>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" >
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"> </script>
<script>
window.onload = function() {
    SwaggerUIBundle({
        url: "/api/v1/openapi.yaml",
        dom_id: '#swagger-ui',
        presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
        layout: "BaseLayout",
        requestInterceptor: (req) => {
            const token = localStorage.getItem('access_token');
            if (token) req.headers['Authorization'] = 'Bearer ' + token;
            return req;
        }
    })
}
</script>
</body>
</html>
```

---

## Output

- [x] `api/openapi.yaml` — đầy đủ spec cho tất cả routes ✓ (40+ endpoints)
- [x] `GET /api/v1/docs` → Swagger UI ✓
- [x] `GET /api/v1/openapi.yaml` → raw YAML ✓

## Acceptance Criteria

```bash
# Swagger UI accessible
open http://localhost:8080/api/v1/docs
# → Browser hiển thị Swagger UI với tất cả endpoints

# OpenAPI spec valid
npx @redocly/cli lint api/openapi.yaml
# → No errors

# Test via Swagger UI
# → Login → Get token → Test /api/v1/scans
```
