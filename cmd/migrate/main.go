package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"futemon/internal/app"
)

func main() {
	dbPath := flag.String("db", "futemon.db", "SQLite database path")
	seedPokemon := flag.Bool("seed-pokemon", false, "Fetch Pokemon data from PokeAPI and upsert it locally")
	pokemonLimit := flag.Int("pokemon-limit", 151, "Number of Pokemon IDs to fetch when --seed-pokemon is set")
	artworkDir := flag.String("artwork-dir", os.Getenv("FUTEMON_ARTWORK_DIR"), "Directory where Pokemon official artwork PNGs are cached")
	flag.Parse()

	store, err := app.NewSQLiteStore(*dbPath)
	if err != nil {
		log.Fatalf("open sqlite store: %v", err)
	}
	defer store.Close()

	if !*seedPokemon {
		fmt.Println("migrations applied")
		return
	}

	seeder := app.NewPokeAPISeeder()
	if *artworkDir == "" {
		*artworkDir = app.DefaultPokemonArtworkDir
	}
	for id := 1; id <= *pokemonLimit; id++ {
		pokemon, err := seeder.FetchPokemon(context.Background(), id)
		if err != nil {
			log.Fatalf("fetch pokemon %d: %v", id, err)
		}
		pokemon, err = app.CachePokemonArtwork(context.Background(), seeder.HTTPClient, pokemon, *artworkDir)
		if err != nil {
			log.Fatalf("cache artwork %d: %v", id, err)
		}
		if err := store.UpsertPokemon(pokemon); err != nil {
			log.Fatalf("save pokemon %d: %v", id, err)
		}
		fmt.Printf("seeded #%d %s\n", pokemon.ID, pokemon.Name)
	}
}
