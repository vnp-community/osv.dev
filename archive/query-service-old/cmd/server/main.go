// Package main — query-service entry point.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// QueryFilter holds the header-based query parameters.
type QueryFilter struct {
	CVSSScore    *float64
	CVSSVersion  string // "V2", "V3", "V4"
	CVSSOperator string // "above", "below"
	StartDate    *time.Time
	EndDate      *time.Time
	TimeOperator string // "from", "until", "between", "outside"
	TimeField    string // "modified", "published"
	HideRejected bool
	Limit        int
	Skip         int
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	mongoURI := envDefault("MONGO_URI", "mongodb://localhost:27017")
	mongoDB  := envDefault("MONGO_DB", "cvedb")
	port     := envDefault("PORT", "8080")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := mongo.Connect(ctx, mongoopts.Client().ApplyURI(mongoURI).SetAppName("query-service"))
	if err != nil {
		log.Fatal().Err(err).Msg("MongoDB connect failed")
	}
	pingCtx, pingCancel := context.WithTimeout(ctx, 10*time.Second)
	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		log.Fatal().Err(err).Msg("MongoDB ping failed")
	}
	pingCancel()
	defer client.Disconnect(ctx) //nolint:errcheck

	db := client.Database(mongoDB)
	cveCol := db.Collection("cves")

	r := chi.NewRouter()
	r.Use(middleware.RealIP, middleware.Recoverer, middleware.RequestID)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "query-service"})
	})

	// GET /query — header-based CVE filter
	r.Get("/query", func(w http.ResponseWriter, r *http.Request) {
		f := parseFilter(r)
		filter := buildFilter(f)

		findOpts := mongoopts.Find().
			SetSort(bson.D{{Key: "cvss3", Value: -1}}).
			SetSkip(int64(f.Skip)).
			SetLimit(int64(f.Limit))

		cursor, err := cveCol.Find(r.Context(), filter, findOpts)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		defer cursor.Close(r.Context())

		var results []bson.M
		if err := cursor.All(r.Context(), &results); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"results": results, "total": len(results)})
	})

	// POST /query — raw MongoDB query (whitelist only)
	r.Post("/query", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Collection string      `json:"collection"`
			Query      interface{} `json:"query"`
			Limit      int         `json:"limit"`
			Skip       int         `json:"skip"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}

		// Whitelist allowed collections
		allowed := map[string]bool{"cves": true, "cpe": true, "cwe": true, "capec": true, "via4": true}
		if !allowed[body.Collection] {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error":   "collection not allowed",
				"allowed": "cves, cpe, cwe, capec, via4",
			})
			return
		}

		limit := body.Limit
		if limit <= 0 || limit > 1000 {
			limit = 100
		}

		// Convert body.Query to bson.M
		rawJSON, _ := json.Marshal(body.Query)
		var filter bson.M
		if err := bson.UnmarshalExtJSON(rawJSON, false, &filter); err != nil {
			filter = bson.M{}
		}

		col := db.Collection(body.Collection)
		opts := mongoopts.Find().SetLimit(int64(limit)).SetSkip(int64(body.Skip))
		cursor, err := col.Find(r.Context(), filter, opts)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		defer cursor.Close(r.Context())

		var results []bson.M
		cursor.All(r.Context(), &results) //nolint:errcheck
		writeJSON(w, http.StatusOK, map[string]interface{}{"results": results, "total": len(results)})
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		log.Info().Str("port", port).Msg("query-service started")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutCancel()
	srv.Shutdown(shutCtx) //nolint:errcheck
}

func parseFilter(r *http.Request) QueryFilter {
	f := QueryFilter{
		CVSSVersion:  r.Header.Get("cvssVersion"),
		CVSSOperator: r.Header.Get("cvssSelect"),
		TimeOperator: r.Header.Get("timeSelect"),
		TimeField:    r.Header.Get("timeTypeSelect"),
		HideRejected: r.Header.Get("rejected") == "hide",
	}
	if f.TimeField == "" {
		f.TimeField = "modified"
	}

	if s := r.Header.Get("cvss"); s != "" {
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			f.CVSSScore = &v
		}
	}
	if s := r.Header.Get("limit"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			f.Limit = v
		}
	}
	if f.Limit <= 0 || f.Limit > 1000 {
		f.Limit = 100
	}
	if s := r.Header.Get("skip"); s != "" {
		f.Skip, _ = strconv.Atoi(s)
	}

	for _, layout := range []string{"02-01-2006", "02/01/2006", "2006-01-02"} {
		if s := r.Header.Get("startDate"); s != "" {
			if t, err := time.Parse(layout, s); err == nil {
				f.StartDate = &t
				break
			}
		}
	}
	for _, layout := range []string{"02-01-2006", "02/01/2006", "2006-01-02"} {
		if s := r.Header.Get("endDate"); s != "" {
			if t, err := time.Parse(layout, s); err == nil {
				f.EndDate = &t
				break
			}
		}
	}
	return f
}

func buildFilter(f QueryFilter) bson.M {
	conds := bson.A{}

	if f.HideRejected {
		conds = append(conds, bson.M{"status": bson.M{"$ne": "Rejected"}})
	}

	if f.CVSSScore != nil {
		cvssField := map[string]string{"V2": "cvss", "V3": "cvss3", "V4": "cvss4"}[strings.ToUpper(f.CVSSVersion)]
		if cvssField == "" {
			cvssField = "cvss3"
		}
		op := map[string]string{"above": "$gt", "below": "$lt"}[strings.ToLower(f.CVSSOperator)]
		if op == "" {
			op = "$gte"
		}
		conds = append(conds, bson.M{cvssField: bson.M{op: *f.CVSSScore}})
	}

	timeField := f.TimeField
	if timeField == "" {
		timeField = "modified"
	}
	switch strings.ToLower(f.TimeOperator) {
	case "from":
		if f.StartDate != nil {
			conds = append(conds, bson.M{timeField: bson.M{"$gte": *f.StartDate}})
		}
	case "until":
		if f.EndDate != nil {
			conds = append(conds, bson.M{timeField: bson.M{"$lte": *f.EndDate}})
		}
	case "between":
		if f.StartDate != nil && f.EndDate != nil {
			conds = append(conds, bson.M{timeField: bson.M{"$gte": *f.StartDate, "$lte": *f.EndDate}})
		}
	case "outside":
		if f.StartDate != nil && f.EndDate != nil {
			conds = append(conds, bson.M{"$or": bson.A{
				bson.M{timeField: bson.M{"$lt": *f.StartDate}},
				bson.M{timeField: bson.M{"$gt": *f.EndDate}},
			}})
		}
	}

	if len(conds) == 0 {
		return bson.M{}
	}
	return bson.M{"$and": conds}
}

var _ = regexp.MustCompile // prevent unused import

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
