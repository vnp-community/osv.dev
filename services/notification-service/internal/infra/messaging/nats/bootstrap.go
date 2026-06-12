// Package nats provides unified NATS consumers for notification-service.
// Consumes events from both OSV and GlobalCVE namespaces.
package nats

// ConsumedSubjects lists all NATS subjects this service subscribes to.
var ConsumedSubjects = []string{
	// OSV vulnerability events (từ notification)
	"osv.vuln.imported",
	"osv.vuln.updated",
	"osv.vuln.withdrawn",
	// GlobalCVE events (từ notification-service/globalcve)
	"cve.created",
	"cve.updated",
	"kev.added",
	"sync.completed",
}
