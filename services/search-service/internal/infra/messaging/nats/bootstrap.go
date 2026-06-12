// Package nats provides NATS event consumers for search-service indexing.
package nats

// ConsumedSubjects lists NATS subjects this service subscribes to for index maintenance.
var ConsumedSubjects = []string{
	"osv.vuln.imported",   // → index new vulnerability
	"osv.vuln.updated",    // → update index entry
	"osv.vuln.withdrawn",  // → delete from index
}
