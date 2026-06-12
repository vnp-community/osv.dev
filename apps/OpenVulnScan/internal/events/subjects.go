// Package events - NATS subject constants cho OpenVulnScan.
package events

// Scan lifecycle subjects.
const (
	SubjScanCreated   = "ovs.scan.created"
	SubjScanCompleted = "ovs.scan.completed"
	SubjScanFailed    = "ovs.scan.failed"
	SubjScanCancelled = "ovs.scan.cancelled"
)

// Finding lifecycle subjects.
const (
	SubjFindingBatchCreated  = "ovs.finding.batch_created"
	SubjFindingStatusChanged = "ovs.finding.status_changed"
)

// Agent subjects.
const (
	SubjAgentReportSubmitted = "ovs.agent.report.submitted"
	SubjAgentReportProcessed = "ovs.agent.report.processed"
)

// Notification subjects.
const (
	SubjNotificationDispatch = "ovs.notification.dispatch"
)

// CVE/Vulnerability subjects.
const (
	SubjCVEIngested = "ovs.cve.ingested"
)
