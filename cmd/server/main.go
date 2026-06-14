package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/featherpoint/swinv/internal/server/api"
	"github.com/featherpoint/swinv/internal/server/auth"
	"github.com/featherpoint/swinv/internal/server/store"
)

const version = "0.1.0"

func main() {
	dbURL := env("DATABASE_URL", "postgres://swinv:swinv_dev@localhost:5432/swinv")
	addr := env("LISTEN_ADDR", ":8080")
	enrollToken := env("ENROLLMENT_TOKEN", "dev-token")
	caCertFile := env("CA_CERT", "")
	caKeyFile := env("CA_KEY", "")
	jwtSecret := []byte(env("WEB_JWT_SECRET", "dev-jwt-secret-change-in-prod"))

	ctx := context.Background()

	db, err := store.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("store.New: %v", err)
	}
	defer db.Close()

	authSvc, err := auth.New(enrollToken, caCertFile, caKeyFile, jwtSecret)
	if err != nil {
		log.Fatalf("auth.New: %v", err)
	}

	r := api.NewRouter(db, authSvc)

	log.Printf("swinv-server %s listening on %s", version, addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
