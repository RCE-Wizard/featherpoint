package main

import (
	"context"
	"crypto/tls"
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
	serverCertFile := env("TLS_CERT", "")
	serverKeyFile := env("TLS_KEY", "")
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

	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	// If TLS cert+key are provided, serve HTTPS with mTLS for agent endpoints.
	// The CA pool requires client certs on /v1/* routes (enforced in middleware).
	if serverCertFile != "" && serverKeyFile != "" {
		tlsCfg, err := authSvc.TLSConfig(serverCertFile, serverKeyFile)
		if err != nil {
			log.Fatalf("TLS config: %v", err)
		}
		// Use RequestClientCert rather than RequireAndVerifyClientCert so that
		// the web UI (which has no client cert) can still reach /api routes.
		// The mTLS middleware enforces verification for /v1/* explicitly.
		tlsCfg.ClientAuth = tls.RequestClientCert
		srv.TLSConfig = tlsCfg
		log.Printf("swinv-server %s listening on %s (HTTPS+mTLS)", version, addr)
		if err := srv.ListenAndServeTLS("", ""); err != nil {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}
	} else {
		log.Printf("swinv-server %s listening on %s (plain HTTP — set TLS_CERT/TLS_KEY for production)", version, addr)
		if err := srv.ListenAndServe(); err != nil {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
