// Package http — Atom/RSS feed renderers for CVE data.
// Supports Atom 1.0 and RSS 2.0 feed formats for GET /cve/last/{n}?format=atom|rss.
// Mirrors Python: bin/dump_last.py -f atom|rss in cve-search
package http

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"time"

	"github.com/osv/data-service/internal/domain/entity"
)

// ── Atom 1.0 ─────────────────────────────────────────────────────────────────

// atomFeedXML is the Atom 1.0 feed root element.
// Spec: https://tools.ietf.org/html/rfc4287
type atomFeedXML struct {
	XMLName xml.Name    `xml:"feed"`
	XMLNS   string      `xml:"xmlns,attr"`
	ID      string      `xml:"id"`
	Title   string      `xml:"title"`
	Updated string      `xml:"updated"`
	Link    atomLinkXML `xml:"link"`
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	ID      string      `xml:"id"`
	Title   string      `xml:"title"`
	Updated string      `xml:"updated"`
	Summary string      `xml:"summary,omitempty"`
	Link    atomLinkXML `xml:"link"`
}

type atomLinkXML struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
}

// renderAtomFeed writes CVEs as Atom 1.0 XML to the response writer.
// result can be []*entity.CVE or []interface{} or any slice type.
func renderAtomFeed(w http.ResponseWriter, result interface{}) {
	w.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")

	feed := atomFeedXML{
		XMLNS:   "http://www.w3.org/2005/Atom",
		ID:      "urn:osv:cve:feed:atom",
		Title:   "OSV CVE Feed",
		Updated: time.Now().UTC().Format(time.RFC3339),
	}
	feed.Link.Rel = "self"
	feed.Link.Href = "https://osv.dev/api/v1/cve/last"

	// Convert result to atom entries
	switch v := result.(type) {
	case []*entity.CVE:
		for _, cve := range v {
			feed.Entries = append(feed.Entries, cveToAtomEntry(cve))
		}
	}

	w.Write([]byte(xml.Header)) //nolint:errcheck
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	enc.Encode(feed) //nolint:errcheck
}

func cveToAtomEntry(cve *entity.CVE) atomEntry {
	id := cve.ID
	summary := cve.Summary
	updated := time.Now().UTC().Format(time.RFC3339)
	if !cve.Modified.IsZero() {
		updated = cve.Modified.UTC().Format(time.RFC3339)
	}

	entry := atomEntry{
		ID:      "urn:osv:cve:" + id,
		Title:   fmt.Sprintf("[%s] %s", id, truncateFeedStr(summary, 100)),
		Updated: updated,
		Summary: summary,
	}
	entry.Link.Rel = "alternate"
	entry.Link.Href = "https://osv.dev/cve/" + id
	return entry
}

// ── RSS 2.0 ──────────────────────────────────────────────────────────────────

// rssFeedXML is the RSS 2.0 feed root element.
// Spec: https://www.rssboard.org/rss-specification
type rssFeedXML struct {
	XMLName xml.Name      `xml:"rss"`
	Version string        `xml:"version,attr"`
	Channel rssChannelXML `xml:"channel"`
}

type rssChannelXML struct {
	Title       string       `xml:"title"`
	Link        string       `xml:"link"`
	Description string       `xml:"description"`
	LastBuild   string       `xml:"lastBuildDate"`
	Items       []rssItemXML `xml:"item"`
}

type rssItemXML struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}

// renderRSSFeed writes CVEs as RSS 2.0 XML to the response writer.
func renderRSSFeed(w http.ResponseWriter, result interface{}) {
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")

	rss := rssFeedXML{Version: "2.0"}
	rss.Channel.Title = "OSV CVE Feed"
	rss.Channel.Link = "https://osv.dev"
	rss.Channel.Description = "Latest CVE Vulnerability Feed from OSV"
	rss.Channel.LastBuild = time.Now().UTC().Format(time.RFC1123Z)

	switch v := result.(type) {
	case []*entity.CVE:
		for _, cve := range v {
			rss.Channel.Items = append(rss.Channel.Items, cveToRSSItem(cve))
		}
	}

	w.Write([]byte(xml.Header)) //nolint:errcheck
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	enc.Encode(rss) //nolint:errcheck
}

func cveToRSSItem(cve *entity.CVE) rssItemXML {
	id := cve.ID
	link := "https://osv.dev/cve/" + id
	pubDate := time.Now().UTC().Format(time.RFC1123Z)
	if !cve.Published.IsZero() {
		pubDate = cve.Published.UTC().Format(time.RFC1123Z)
	}

	title := fmt.Sprintf("[%s] %s", id, truncateFeedStr(cve.Summary, 80))

	return rssItemXML{
		Title:       title,
		Link:        link,
		Description: cve.Summary,
		PubDate:     pubDate,
		GUID:        id,
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func truncateFeedStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
