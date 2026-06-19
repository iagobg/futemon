package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCachePokemonArtworkDownloadsLocalCopy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("png"))
	}))
	defer server.Close()

	dir := t.TempDir()
	pokemon, err := CachePokemonArtwork(context.Background(), server.Client(), Pokemon{ID: 7, Name: "squirtle", ArtworkURL: server.URL + "/7.png"}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if pokemon.LocalArtworkURL != "/static/pokemon-artwork/7.png" {
		t.Fatalf("local artwork url = %q", pokemon.LocalArtworkURL)
	}
	if _, err := os.Stat(filepath.Join(dir, "7.png")); err != nil {
		t.Fatal(err)
	}
	if pokemon.DisplayArtworkURL() != pokemon.LocalArtworkURL {
		t.Fatalf("display artwork url = %q", pokemon.DisplayArtworkURL())
	}
}
