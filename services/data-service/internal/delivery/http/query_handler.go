// Package http — CVE flexible query handler for data-service.
// Implements CR-008: flexible CVE query endpoint supporting both GET and POST.
//
// Routes:
//   GET  /cve/query?field=value&operator=eq|gt|lt|in...
//   POST /query  {"field": "...", "value": "...", "operator": "..."}
//
// Mirrors Python: web/restapi/query.py in cve-search
package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/rs/zerolog/log"
)

// QueryHandler handles flexible CVE query endpoints.
// The query is expressed as a MongoDB filter built from HTTP params.
type QueryHandler struct {
	col *mongo.Collection
}

// NewQueryHandler creates a QueryHandler.
// db: MongoDB database connection (uses "cves" collection).
func NewQueryHandler(db *mongo.Database) *QueryHandler {
	return &QueryHandler{col: db.Collection("cves")}
}

// queryRequest is the JSON body for POST /query.
type queryRequest struct {
	Field    string      `json:"field"`
	Value    interface{} `json:"value"`
	Operator string      `json:"operator"` // eq | ne | gt | gte | lt | lte | in | regex
	Limit    int         `json:"limit"`
	Skip     int         `json:"skip"`
}

// GetQuery handles: GET /cve/query
// Query params:
//   - field:    MongoDB field name (e.g. "cvss3", "vendors", "cwe", "summary")
//   - value:    target value (scalar, or comma-separated for $in)
//   - operator: eq|ne|gt|gte|lt|lte|in|regex (default: eq)
//   - limit:    max results (default 100)
//   - skip:     offset (default 0)
//
// Example: GET /cve/query?field=cvss3&value=9.8&operator=gte&limit=20
func (h *QueryHandler) GetQuery(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	field := q.Get("field")
	value := q.Get("value")
	operator := q.Get("operator")
	limit, _ := strconv.Atoi(q.Get("limit"))
	skip, _ := strconv.Atoi(q.Get("skip"))

	if field == "" {
		writeQueryError(w, http.StatusBadRequest, "field parameter is required")
		return
	}

	req := queryRequest{
		Field:    field,
		Operator: operator,
		Limit:    limit,
		Skip:     skip,
	}

	// For $in operator, value is comma-separated
	if strings.ToLower(operator) == "in" {
		parts := strings.Split(value, ",")
		vals := make([]interface{}, 0, len(parts))
		for _, p := range parts {
			vals = append(vals, strings.TrimSpace(p))
		}
		req.Value = vals
	} else {
		req.Value = value
	}

	h.executeQuery(w, r, req)
}

// PostQuery handles: POST /query
// Body: {"field": "cvss3", "value": 9.8, "operator": "gte", "limit": 20, "skip": 0}
func (h *QueryHandler) PostQuery(w http.ResponseWriter, r *http.Request) {
	var req queryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeQueryError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Field == "" {
		writeQueryError(w, http.StatusBadRequest, "field is required")
		return
	}
	h.executeQuery(w, r, req)
}

// executeQuery builds the MongoDB filter and executes the query.
func (h *QueryHandler) executeQuery(w http.ResponseWriter, r *http.Request, req queryRequest) {
	// Build MongoDB filter
	filter, err := buildQueryFilter(req.Field, req.Value, req.Operator)
	if err != nil {
		writeQueryError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Apply limits and caps
	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	findOpts := options.Find().
		SetSort(bson.D{{Key: "modified", Value: -1}}).
		SetLimit(int64(limit))
	if req.Skip > 0 {
		findOpts.SetSkip(int64(req.Skip))
	}

	cursor, err := h.col.Find(r.Context(), filter, findOpts)
	if err != nil {
		log.Error().Err(err).Str("field", req.Field).Msg("query execution failed")
		writeQueryError(w, http.StatusInternalServerError, "query execution failed")
		return
	}
	defer cursor.Close(r.Context())

	var results []bson.M
	if err := cursor.All(r.Context(), &results); err != nil {
		writeQueryError(w, http.StatusInternalServerError, "result decode failed")
		return
	}

	writeQueryJSON(w, http.StatusOK, map[string]interface{}{
		"results": results,
		"count":   len(results),
		"limit":   limit,
		"skip":    req.Skip,
		"query": map[string]interface{}{
			"field":    req.Field,
			"operator": req.Operator,
			"value":    req.Value,
		},
	})
}

// buildQueryFilter builds a MongoDB filter from field + value + operator.
// Allowed operators: eq, ne, gt, gte, lt, lte, in, regex
// Allowed fields (allowlist to prevent injection): see safeQueryFields.
func buildQueryFilter(field string, value interface{}, operator string) (bson.M, error) {
	// Allowlist of queryable fields (mirrors cve-search Python list)
	safeFields := map[string]bool{
		"id": true, "summary": true, "cvss": true, "cvss3": true, "cvss4": true,
		"cwe": true, "vendors": true, "products": true, "severity": true,
		"epss": true, "epssPercentile": true, "assigner": true,
		"vulnerable_configuration": true, "vulnerable_product": true,
		"published": true, "modified": true, "status": true,
	}

	if !safeFields[field] {
		return nil, errorf("field '%s' is not queryable; allowed fields: id, summary, cvss, cvss3, cwe, vendors, products, severity, published, modified", field)
	}

	op := strings.ToLower(strings.TrimPrefix(operator, "$"))
	if op == "" {
		op = "eq" // default operator
	}

	switch op {
	case "eq":
		return bson.M{field: value}, nil
	case "ne":
		return bson.M{field: bson.M{"$ne": value}}, nil
	case "gt":
		return bson.M{field: bson.M{"$gt": toNumber(value)}}, nil
	case "gte":
		return bson.M{field: bson.M{"$gte": toNumber(value)}}, nil
	case "lt":
		return bson.M{field: bson.M{"$lt": toNumber(value)}}, nil
	case "lte":
		return bson.M{field: bson.M{"$lte": toNumber(value)}}, nil
	case "in":
		return bson.M{field: bson.M{"$in": value}}, nil
	case "regex":
		strVal, ok := value.(string)
		if !ok {
			return nil, errorf("regex operator requires a string value")
		}
		return bson.M{field: bson.M{"$regex": strVal, "$options": "i"}}, nil
	default:
		return nil, errorf("unsupported operator '%s'; valid: eq, ne, gt, gte, lt, lte, in, regex", operator)
	}
}

// toNumber converts string or numeric value to float64 for comparison operators.
func toNumber(v interface{}) interface{} {
	switch n := v.(type) {
	case float64, float32, int, int64:
		return n
	case string:
		if f, err := strconv.ParseFloat(n, 64); err == nil {
			return f
		}
	}
	return v
}

func errorf(format string, args ...interface{}) error {
	return &queryError{msg: "query: " + sprintf(format, args...)}
}

type queryError struct{ msg string }

func (e *queryError) Error() string { return e.msg }

func sprintf(format string, args ...interface{}) string {
	if len(args) == 0 {
		return format
	}
	// Simple substitution without importing fmt here (fmt is imported in cve_handler.go)
	result := format
	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			result = replaceFirst(result, "%s", v)
		}
	}
	return result
}

func replaceFirst(s, old, new string) string {
	idx := strings.Index(s, old)
	if idx < 0 {
		return s
	}
	return s[:idx] + new + s[idx+len(old):]
}

func writeQueryJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeQueryError(w http.ResponseWriter, status int, msg string) {
	writeQueryJSON(w, status, map[string]string{"error": msg})
}
