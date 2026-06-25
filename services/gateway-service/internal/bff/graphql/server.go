// Package graphql — server.go
// GraphQLServer builds an HTTP handler for the GraphQL endpoint.
// S3-GW-01: POST /graphql and GET /graphql (GraphiQL dev playground)
package graphql

import (
	"encoding/json"
	"net/http"
	"os"

	gql "github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
	"github.com/rs/zerolog"

	"github.com/osv/gateway-service/internal/adapter/grpcclient"
)

// NewGraphQLHandler builds and returns an HTTP handler for GraphQL.
// Mounts the handler on POST /graphql (and GET /graphql for GraphiQL in dev mode).
//
// Usage in main.go (additive — does not modify existing mux):
//
//	gqlHandler := graphql.NewGraphQLHandler(aiClient, cvedbClient, log)
//	mux.Handle("/graphql", gqlHandler)
func NewGraphQLHandler(aiClient *grpcclient.AIClient, cvedbClient *grpcclient.CVEDBClient, log zerolog.Logger) (http.Handler, error) {
	resolver := NewResolver(aiClient, cvedbClient, log)

	schema, err := BuildSchema(resolver)
	if err != nil {
		return nil, err
	}

	// GraphiQL playground only in development mode
	graphiql := os.Getenv("DEV_MODE") == "true"

	h := handler.New(&handler.Config{
		Schema:   &schema,
		Pretty:   false,
		GraphiQL: graphiql,
	})

	return h, nil
}

// ExecuteQuery executes a GraphQL query directly (useful for testing and internal calls).
func ExecuteQuery(schema gql.Schema, query string, variables map[string]interface{}) *gql.Result {
	return gql.Do(gql.Params{
		Schema:         schema,
		RequestString:  query,
		VariableValues: variables,
	})
}

// ── GraphQL HTTP adapter (if not using handler package) ──────────────────────

// graphQLHandler implements the standard http.Handler for GraphQL requests.
// Used as fallback if github.com/graphql-go/handler is not available.
type graphQLHTTPHandler struct {
	schema gql.Schema
	log    zerolog.Logger
}

func (g *graphQLHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var params struct {
		Query         string                 `json:"query"`
		Variables     map[string]interface{} `json:"variables"`
		OperationName string                 `json:"operationName"`
	}

	if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
	} else {
		params.Query = r.URL.Query().Get("query")
	}

	result := gql.Do(gql.Params{
		Schema:         g.schema,
		RequestString:  params.Query,
		VariableValues: params.Variables,
		Context:        r.Context(),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result) //nolint:errcheck
}
