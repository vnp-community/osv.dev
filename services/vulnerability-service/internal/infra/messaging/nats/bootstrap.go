// Package nats provides unified NATS event handling for vulnerability-service.
package nats

// ConsumedSubjects lists all subjects this service subscribes to.
var ConsumedSubjects = []string{
	"osv.vuln.imported",           // → alias detect
	"osv.ai.enrichment.completed", // → alias embedding update
}

// PublishedSubjects lists NATS subjects this service publishes.
var PublishedSubjects = []string{
	"osv.vuln.imported",
	"osv.vuln.updated",
	"osv.vuln.withdrawn",
}
