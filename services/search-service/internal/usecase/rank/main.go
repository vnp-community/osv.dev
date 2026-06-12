// Package main — ranking-service entry point.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// RankEntry holds a ranking group + rank value.
type RankEntry struct {
	Group string `bson:"group" json:"group"`
	Rank  int    `bson:"rank"  json:"rank"`
}

// RankingEntry is the MongoDB document in "ranking" collection.
type RankingEntry struct {
	ID  primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	CPE string             `bson:"cpe"           json:"cpe"`
	Rank []RankEntry       `bson:"rank"          json:"rank"`
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	mongoURI := envDefault("MONGO_URI", "mongodb://localhost:27017")
	mongoDB  := envDefault("MONGO_DB", "cvedb")
	port     := envDefault("PORT", "8080")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := mongo.Connect(ctx, mongoopts.Client().ApplyURI(mongoURI).SetAppName("ranking-service"))
	if err != nil {
		log.Fatal().Err(err).Msg("MongoDB connect failed")
	}
	pingCtx, pingCancel := context.WithTimeout(ctx, 10*time.Second)
	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		log.Fatal().Err(err).Msg("MongoDB ping failed")
	}
	pingCancel()
	defer client.Disconnect(ctx) //nolint:errcheck

	col := client.Database(mongoDB).Collection("ranking")

	r := chi.NewRouter()
	r.Use(middleware.RealIP, middleware.Recoverer, middleware.RequestID)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "ranking-service"})
	})

	// GET /ranking — list all entries
	r.Get("/ranking", func(w http.ResponseWriter, r *http.Request) {
		cursor, err := col.Find(r.Context(), bson.M{}, mongoopts.Find().SetLimit(1000))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		defer cursor.Close(r.Context())
		var entries []RankingEntry
		cursor.All(r.Context(), &entries) //nolint:errcheck
		writeJSON(w, http.StatusOK, map[string]interface{}{"entries": entries, "total": len(entries)})
	})

	// POST /ranking — create new entry
	r.Post("/ranking", func(w http.ResponseWriter, r *http.Request) {
		var entry RankingEntry
		if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		entry.ID = primitive.NewObjectID()
		if _, err := col.InsertOne(r.Context(), entry); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, entry)
	})

	// DELETE /ranking/{id}
	r.Delete("/ranking/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := primitive.ObjectIDFromHex(chi.URLParam(r, "id"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
			return
		}
		if _, err := col.DeleteOne(r.Context(), bson.M{"_id": id}); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	})

	// GET /ranking/lookup?cpe=... — loosy CPE lookup
	r.Get("/ranking/lookup", func(w http.ResponseWriter, r *http.Request) {
		cpeStr := r.URL.Query().Get("cpe")
		if cpeStr == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cpe parameter required"})
			return
		}

		entry, matched, err := loosyLookup(r.Context(), col, cpeStr)
		if err != nil || entry == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no ranking found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"cpe":          entry.CPE,
			"rank":         entry.Rank,
			"matched_part": matched,
		})
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		log.Info().Str("port", port).Msg("ranking-service started")
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

// loosyLookup implements the loosy CPE lookup algorithm:
// splits CPE on ":", tries each meaningful part as a regex against "cpe" field.
func loosyLookup(ctx context.Context, col *mongo.Collection, cpeStr string) (*RankingEntry, string, error) {
	skipParts := map[string]bool{
		"":    true,
		"cpe": true, "2.3": true,
		"a": true, "o": true, "h": true,
		"*": true, "-": true,
	}
	parts := strings.Split(cpeStr, ":")
	for _, part := range parts {
		if skipParts[part] {
			continue
		}
		var entry RankingEntry
		err := col.FindOne(ctx, bson.M{
			"cpe": bson.M{"$regex": regexp.QuoteMeta(part), "$options": "i"},
		}).Decode(&entry)
		if err == mongo.ErrNoDocuments {
			continue
		}
		if err != nil {
			return nil, "", err
		}
		return &entry, part, nil
	}
	return nil, "", nil
}

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
