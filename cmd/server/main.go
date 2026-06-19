package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"futemon/internal/app"
)

func main() {
	if err := app.LoadDotEnv(".env"); err != nil {
		log.Fatalf("load .env: %v", err)
	}
	authMode := flag.String("auth-mode", os.Getenv("FUTEMON_AUTH_MODE"), "authentication mode: google or local")
	portFlag := flag.String("port", os.Getenv("PORT"), "HTTP port")
	dbFlag := flag.String("db", os.Getenv("FUTEMON_DB_PATH"), "SQLite database path")
	flag.Parse()
	if *authMode != "" {
		_ = os.Setenv("FUTEMON_AUTH_MODE", *authMode)
	}

	addr := ":8080"
	if port := *portFlag; port != "" {
		addr = ":" + port
	}

	dbPath := *dbFlag
	if dbPath == "" {
		dbPath = "futemon.db"
	}

	store, err := app.NewSQLiteStore(dbPath)
	if err != nil {
		log.Fatalf("open sqlite store: %v", err)
	}
	defer store.Close()
	if key := os.Getenv("ENV_ENCRYPTION_KEY"); key != "" {
		cipher, err := app.NewKeyCipher([]byte(key))
		if err != nil {
			log.Fatalf("configure encryption: %v", err)
		}
		store.WithCipher(cipher)
	}

	server := app.NewServer(store)

	log.Printf("Futemon listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, server.Routes()); err != nil {
		log.Fatal(err)
	}
}
