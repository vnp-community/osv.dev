// Package main — taxonomy-service entry point.
// Provides CWE and CAPEC data via REST and gRPC.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
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

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	mongoURI := envDefault("MONGO_URI", "mongodb://localhost:27017")
	mongoDB  := envDefault("MONGO_DB", "cvedb")
	port     := envDefault("PORT", "8080")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := mongo.Connect(ctx, mongoopts.Client().ApplyURI(mongoURI).SetAppName("taxonomy-service"))
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
	cweCol := db.Collection("cwe")
	capecCol := db.Collection("capec")

	r := chi.NewRouter()
	r.Use(middleware.RealIP, middleware.Recoverer, middleware.RequestID)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "taxonomy-service"})
	})

	// GET /cwe/{id} — accepts "502", "CWE-502", "CWE502"
	r.Get("/cwe/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := normalizeCWEID(chi.URLParam(r, "id"))
		var result bson.M
		err := cweCol.FindOne(r.Context(), bson.M{"id": id}).Decode(&result)
		if err == mongo.ErrNoDocuments {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "CWE not found", "id": id})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, result)
	})

	// GET /cwe/{id}/capec — CAPECs related to a CWE
	r.Get("/cwe/{id}/capec", func(w http.ResponseWriter, r *http.Request) {
		cweID := normalizeCWEID(chi.URLParam(r, "id"))
		cursor, err := capecCol.Find(r.Context(), bson.M{
			"related_weakness": bson.M{
				"$elemMatch": bson.M{"$regex": "^" + cweID + "$"},
			},
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		defer cursor.Close(r.Context())
		var results []bson.M
		cursor.All(r.Context(), &results) //nolint:errcheck
		writeJSON(w, http.StatusOK, map[string]interface{}{"capec": results, "total": len(results)})
	})

	// GET /capec/{id}
	r.Get("/capec/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var result bson.M
		err := capecCol.FindOne(r.Context(), bson.M{"id": id}).Decode(&result)
		if err == mongo.ErrNoDocuments {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "CAPEC not found", "id": id})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, result)
	})

	// GET /capec/{id}/cwe — CWEs related to a CAPEC
	r.Get("/capec/{id}/cwe", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var capec struct {
			RelatedWeakness []string `bson:"related_weakness"`
		}
		if err := capecCol.FindOne(r.Context(), bson.M{"id": id}).Decode(&capec); err != nil {
			if err == mongo.ErrNoDocuments {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "CAPEC not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"cwe_ids": capec.RelatedWeakness})
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		log.Info().Str("port", port).Msg("taxonomy-service started")
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

// normalizeCWEID accepts "502", "CWE-502", "CWE502" → "502"
func normalizeCWEID(id string) string {
	id = strings.TrimPrefix(id, "CWE-")
	id = strings.TrimPrefix(id, "CWE")
	return id
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
