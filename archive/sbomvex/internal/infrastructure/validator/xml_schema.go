// Package validator provides safe XML parsing to prevent XXE attacks.
package validator

import (
	"bytes"
	"encoding/xml"
)

// SafeParseXML decodes XML from content into v without external entity resolution.
// This prevents XXE (XML External Entity) injection attacks.
func SafeParseXML(content []byte, v interface{}) error {
	d := xml.NewDecoder(bytes.NewReader(content))
	// Disable entity resolution by providing empty entity map
	d.Entity = map[string]string{}
	return d.Decode(v)
}
