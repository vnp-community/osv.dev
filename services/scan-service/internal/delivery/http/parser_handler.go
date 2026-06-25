package http

import (
	"net/http"

	"github.com/rs/zerolog"
	importuc "github.com/osv/scan-service/internal/usecase/import"
)

type ParserHandler struct {
	factory importuc.ParserFactory
	log     zerolog.Logger
}

func NewParserHandler(factory importuc.ParserFactory, log zerolog.Logger) *ParserHandler {
	return &ParserHandler{
		factory: factory,
		log:     log,
	}
}

// ListParsers handles GET /api/v2/parsers
func (h *ParserHandler) ListParsers(w http.ResponseWriter, r *http.Request) {
	parsers := h.factory.ListScanTypes()
	
	// Create a simple response format matching DefectDojo if needed,
	// or just a list of strings. Let's return a list of objects.
	type ParserInfo struct {
		Name string `json:"name"`
	}
	
	var res []ParserInfo
	for _, p := range parsers {
		res = append(res, ParserInfo{Name: p})
	}
	
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"count":   len(res),
		"results": res,
	})
}
