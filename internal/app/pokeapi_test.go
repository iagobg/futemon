package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPokeAPISeederFetchPokemon(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/pokemon/1", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"id": 1,
			"name": "bulbasaur",
			"sprites": {"other": {"official-artwork": {"front_default": "https://img.example/bulbasaur.png"}}},
			"species": {"url": "` + server.URL + `/pokemon-species/1"},
			"types": [{"slot": 1, "type": {"name": "grass"}}, {"slot": 2, "type": {"name": "poison"}}],
			"stats": [
				{"base_stat": 45, "stat": {"name": "hp"}},
				{"base_stat": 49, "stat": {"name": "attack"}},
				{"base_stat": 49, "stat": {"name": "defense"}},
				{"base_stat": 65, "stat": {"name": "special-attack"}},
				{"base_stat": 65, "stat": {"name": "special-defense"}},
				{"base_stat": 45, "stat": {"name": "speed"}}
			],
			"abilities": [{"ability": {"name": "overgrow", "url": "` + server.URL + `/ability/65"}}]
		}`))
	})
	mux.HandleFunc("/pokemon-species/1", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"flavor_text_entries": [
				{"flavor_text": "A strange seed was planted\non its back.", "language": {"name": "en"}}
			]
		}`))
	})
	mux.HandleFunc("/ability/65", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"name": "overgrow",
			"effect_entries": [
				{"effect": "Strengthens Grass moves in a pinch.", "language": {"name": "en"}}
			]
		}`))
	})

	seeder := PokeAPISeeder{BaseURL: server.URL, HTTPClient: server.Client()}
	pokemon, err := seeder.FetchPokemon(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}

	if pokemon.ID != 1 || pokemon.Name != "Bulbasaur" {
		t.Fatalf("pokemon identity = #%d %s", pokemon.ID, pokemon.Name)
	}
	if pokemon.ArtworkURL != "https://img.example/bulbasaur.png" {
		t.Fatalf("artwork url = %q", pokemon.ArtworkURL)
	}
	if pokemon.DisplayArtworkURL() != pokemon.ArtworkURL {
		t.Fatalf("display artwork should fall back to external url")
	}
	if pokemon.Type1 != "grass" || pokemon.Type2 != "poison" {
		t.Fatalf("types = %s/%s", pokemon.Type1, pokemon.Type2)
	}
	if pokemon.HP != 45 || pokemon.SpecialAttack != 65 || pokemon.Speed != 45 {
		t.Fatalf("stats were not mapped correctly: %+v", pokemon)
	}
	if pokemon.Description != "A strange seed was planted on its back." {
		t.Fatalf("description = %q", pokemon.Description)
	}
	if pokemon.Abilities == "" || pokemon.Abilities == "[]" {
		t.Fatal("abilities were not serialized")
	}
}
