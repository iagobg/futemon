package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const DefaultPokemonArtworkDir = "data/pokemon-artwork"

func CachePokemonArtwork(ctx context.Context, client *http.Client, pokemon Pokemon, dir string) (Pokemon, error) {
	pokemon = ensurePokemonArtwork(pokemon)
	if pokemon.ArtworkURL == "" {
		return pokemon, nil
	}
	if dir == "" {
		dir = DefaultPokemonArtworkDir
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Pokemon{}, err
	}

	filename := fmt.Sprintf("%d.png", pokemon.ID)
	path := filepath.Join(dir, filename)
	if _, err := os.Stat(path); err == nil {
		pokemon.LocalArtworkURL = pokemonLocalArtworkURL(pokemon.ID)
		return pokemon, nil
	}

	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pokemon.ArtworkURL, nil)
	if err != nil {
		return Pokemon{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return Pokemon{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Pokemon{}, fmt.Errorf("download artwork for #%d returned %s", pokemon.ID, resp.Status)
	}

	tmp := path + ".tmp"
	file, err := os.Create(tmp)
	if err != nil {
		return Pokemon{}, err
	}
	_, copyErr := io.Copy(file, resp.Body)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return Pokemon{}, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return Pokemon{}, closeErr
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return Pokemon{}, err
	}
	pokemon.LocalArtworkURL = pokemonLocalArtworkURL(pokemon.ID)
	return pokemon, nil
}
