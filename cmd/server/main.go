package main

import (
	"log"
	"net/http"
	"os"

	"futemon/internal/app"
)

func main() {
	addr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}

	dbPath := os.Getenv("FUTEMON_DB_PATH")
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
