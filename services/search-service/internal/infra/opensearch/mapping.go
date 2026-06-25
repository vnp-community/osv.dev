package opensearch

// CVEIndexMapping defines the OpenSearch mapping for the cves index.
const CVEIndexMapping = `{
  "settings": {
    "number_of_shards": 1,
    "number_of_replicas": 0,
    "analysis": {
      "analyzer": {
        "cve_analyzer": {
          "type": "custom",
          "tokenizer": "standard",
          "filter": ["lowercase", "stop", "asciifolding"]
        }
      }
    }
  },
  "mappings": {
    "properties": {
      "id":          { "type": "keyword" },
      "description": { "type": "text", "analyzer": "cve_analyzer" },
      "severity":    { "type": "keyword" },
      "source":      { "type": "keyword" },
      "cwe":         { "type": "keyword" },
      "vendors":     { "type": "keyword" },
      "products":    { "type": "keyword" },
      "epss":        { "type": "float" },
      "epss_percentile": { "type": "float" },
      "is_kev":      { "type": "boolean" },
      "is_exploit":  { "type": "boolean" },
      "cvss3":       { "type": "float" },
      "published":   { "type": "date" },
      "updated_at":  { "type": "date" }
    }
  }
}`

const CVEIndexName = "cves"
