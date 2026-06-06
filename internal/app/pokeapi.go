package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type PokeAPISeeder struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewPokeAPISeeder() PokeAPISeeder {
	return PokeAPISeeder{
		BaseURL: "https://pokeapi.co/api/v2",
		HTTPClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (s PokeAPISeeder) FetchPokemon(ctx context.Context, id int) (Pokemon, error) {
	client := s.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	var detail pokeAPIPokemon
	if err := getJSON(ctx, client, s.resourceURL("pokemon", id), &detail); err != nil {
		return Pokemon{}, err
	}

	var species pokeAPISpecies
	if err := getJSON(ctx, client, detail.Species.URL, &species); err != nil {
		return Pokemon{}, err
	}

	abilities := make([]PokemonAbility, 0, len(detail.Abilities))
	for _, abilityRef := range detail.Abilities {
		var ability pokeAPIAbility
		if err := getJSON(ctx, client, abilityRef.Ability.URL, &ability); err != nil {
			return Pokemon{}, err
		}
		abilities = append(abilities, PokemonAbility{
			Name:        ability.Name,
			Description: firstEffect(ability.EffectEntries),
		})
	}
	abilityJSON, err := json.Marshal(abilities)
	if err != nil {
		return Pokemon{}, err
	}

	pokemon := Pokemon{
		ID:          detail.ID,
		Name:        detail.Name,
		Type1:       detail.Types[0].Type.Name,
		Description: firstFlavorText(species.FlavorTextEntries),
		Abilities:   string(abilityJSON),
	}
	if len(detail.Types) > 1 {
		pokemon.Type2 = detail.Types[1].Type.Name
	}
	for _, stat := range detail.Stats {
		switch stat.Stat.Name {
		case "hp":
			pokemon.HP = stat.BaseStat
		case "attack":
			pokemon.Attack = stat.BaseStat
		case "defense":
			pokemon.Defense = stat.BaseStat
		case "special-attack":
			pokemon.SpecialAttack = stat.BaseStat
		case "special-defense":
			pokemon.SpecialDefense = stat.BaseStat
		case "speed":
			pokemon.Speed = stat.BaseStat
		}
	}
	return pokemon, nil
}

func (s PokeAPISeeder) resourceURL(resource string, id int) string {
	base := strings.TrimRight(s.BaseURL, "/")
	return fmt.Sprintf("%s/%s/%d", base, resource, id)
}

func (s *SQLiteStore) UpsertPokemon(pokemon Pokemon) error {
	_, err := s.db.Exec(`
		INSERT INTO pokemons (
			id, name, type_1, type_2, hp, attack, defense, special_attack,
			special_defense, speed, description, abilities
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			type_1 = excluded.type_1,
			type_2 = excluded.type_2,
			hp = excluded.hp,
			attack = excluded.attack,
			defense = excluded.defense,
			special_attack = excluded.special_attack,
			special_defense = excluded.special_defense,
			speed = excluded.speed,
			description = excluded.description,
			abilities = excluded.abilities`,
		pokemon.ID, pokemon.Name, pokemon.Type1, nullString(pokemon.Type2), pokemon.HP,
		pokemon.Attack, pokemon.Defense, pokemon.SpecialAttack, pokemon.SpecialDefense,
		pokemon.Speed, pokemon.Description, pokemon.Abilities,
	)
	return err
}

type PokemonAbility struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type pokeAPIPokemon struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Species struct {
		URL string `json:"url"`
	} `json:"species"`
	Types []struct {
		Slot int `json:"slot"`
		Type struct {
			Name string `json:"name"`
		} `json:"type"`
	} `json:"types"`
	Stats []struct {
		BaseStat int `json:"base_stat"`
		Stat     struct {
			Name string `json:"name"`
		} `json:"stat"`
	} `json:"stats"`
	Abilities []struct {
		Ability struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"ability"`
	} `json:"abilities"`
}

type pokeAPISpecies struct {
	FlavorTextEntries []localizedText `json:"flavor_text_entries"`
}

type pokeAPIAbility struct {
	Name          string          `json:"name"`
	EffectEntries []localizedText `json:"effect_entries"`
}

type localizedText struct {
	FlavorText string `json:"flavor_text"`
	Effect     string `json:"effect"`
	Language   struct {
		Name string `json:"name"`
	} `json:"language"`
}

func getJSON(ctx context.Context, client *http.Client, rawURL string, out any) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if parsed.Scheme == "" {
		return fmt.Errorf("invalid PokeAPI URL: %s", rawURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("PokeAPI returned %s for %s", resp.Status, rawURL)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func firstFlavorText(entries []localizedText) string {
	for _, entry := range entries {
		if entry.Language.Name == "en" && entry.FlavorText != "" {
			return normalizePokeAPIText(entry.FlavorText)
		}
	}
	return ""
}

func firstEffect(entries []localizedText) string {
	for _, entry := range entries {
		if entry.Language.Name == "en" && entry.Effect != "" {
			return normalizePokeAPIText(entry.Effect)
		}
	}
	return ""
}

func normalizePokeAPIText(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\f", " ")
	return strings.Join(strings.Fields(value), " ")
}
