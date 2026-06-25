# F14 — Asset Management: Data Flow

---

## 1. Auto-Registration Flow (post-scan)

```
scan-service-ovs publish NATS: scan.completed {scan_id, targets[], findings[]}
    │
    ▼
asset-service subscriber:
    For each target in scan.targets:
        UPSERT assets WHERE identifier=target AND product_id=product_id
        link_findings: INSERT asset_findings (asset_id, finding_id)
        │
        ▼
        calculateRiskScore(asset_id) → UPDATE assets.risk_score
```

---

## 2. Asset Lookup & Detail Flow

```
Client → GET /api/v1/assets/{id}
    │
    ▼
asset-service:
    1. SELECT asset + tags
    2. SELECT findings WHERE asset_id=$1 AND state='Active'
    3. Real-time risk score calculation
    4. last_scanned_at, scan history
    │
    ▼
Client ← 200 {
    id, identifier, type, tags, risk_score, risk_level,
    last_scanned_at, product_id,
    findings_summary: {critical: 1, high: 3, ...},
    active_findings_count: 4
}
```

---

## 3. Risk Score Recalculation Trigger

```
finding-service publish NATS: finding.state_changed {finding_id, to_state}
    │
    ▼
asset-service subscriber:
    lookup asset_id from asset_findings WHERE finding_id=$1
    if asset_id:
        calculateRiskScore(asset_id)
        UPDATE assets SET risk_score = new_score
```

---

## 4. Ad-hoc Scan Trigger

```
Client → POST /api/v1/assets/{id}/scan {scan_type: "nmap"}
    │
    ▼
asset-service:
    1. Fetch asset {identifier, product_id}
    2. Call scan-service-ovs.CreateScan:
        POST /api/v1/scans {targets: [asset.identifier], type, product_id}
    3. Return 202 {scan_id}
```

---

## 5. NATS Events

| Event | Publisher | Trigger |
|-------|-----------|---------|
| `scan.completed` | scan-service-ovs | Auto-register/update asset + link findings |
| `finding.state_changed` | finding-service-ovs | Recalculate asset risk score |
| `asset.risk_changed` | asset-service | Risk score crossed threshold | → notification-service |
