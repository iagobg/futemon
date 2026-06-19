package app

import (
	"encoding/json"
	"fmt"
	"strings"
)

func pokemonDisplayName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	lower := strings.ToLower(name)
	special := map[string]string{
		"mr-mime":   "Mr. Mime",
		"mime-jr":   "Mime Jr.",
		"nidoran-f": "Nidoran♀",
		"nidoran-m": "Nidoran♂",
		"farfetchd": "Farfetch'd",
		"sirfetchd": "Sirfetch'd",
		"ho-oh":     "Ho-Oh",
		"porygon-z": "Porygon-Z",
		"jangmo-o":  "Jangmo-O",
		"hakamo-o":  "Hakamo-O",
		"kommo-o":   "Kommo-O",
		"type-null": "Type: Null",
		"flabebe":   "Flabébé",
	}
	if value, ok := special[lower]; ok {
		return value
	}
	parts := strings.Split(lower, "-")
	for i, part := range parts {
		parts[i] = titlePokemonPart(part)
	}
	return strings.Join(parts, " ")
}

func titlePokemonPart(part string) string {
	if part == "" {
		return ""
	}
	return strings.ToUpper(part[:1]) + part[1:]
}

func pokemonAbilities(pokemon Pokemon) []PokemonAbility {
	var abilities []PokemonAbility
	if err := json.Unmarshal([]byte(pokemon.Abilities), &abilities); err != nil {
		return nil
	}
	out := make([]PokemonAbility, 0, len(abilities))
	seen := map[string]bool{}
	for _, ability := range abilities {
		ability.Name = strings.TrimSpace(ability.Name)
		if ability.Name == "" {
			continue
		}
		key := strings.ToLower(ability.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, ability)
	}
	return out
}

func pokemonAbilitiesJSON(pokemon Pokemon) string {
	type abilityOption struct {
		Name        string `json:"name"`
		Label       string `json:"label"`
		Description string `json:"description"`
		Search      string `json:"search"`
	}
	abilities := pokemonAbilities(pokemon)
	options := make([]abilityOption, 0, len(abilities))
	for _, ability := range abilities {
		label := abilityDisplayName(ability.Name)
		options = append(options, abilityOption{
			Name:        ability.Name,
			Label:       label,
			Description: ability.Description,
			Search:      strings.ToLower(label + " " + ability.Name + " " + ability.Description),
		})
	}
	payload, err := json.Marshal(options)
	if err != nil {
		return "[]"
	}
	return string(payload)
}

func abilityDisplayName(name string) string {
	return pokemonDisplayName(name)
}

func normalizePokemonAbility(pokemon Pokemon, selected string) (string, bool) {
	abilities := pokemonAbilities(pokemon)
	selected = strings.TrimSpace(selected)
	if len(abilities) == 0 {
		return "", selected == ""
	}
	if selected == "" {
		return "", false
	}
	for _, ability := range abilities {
		if strings.EqualFold(ability.Name, selected) || strings.EqualFold(abilityDisplayName(ability.Name), selected) {
			return ability.Name, true
		}
	}
	return "", false
}

func pokemonArtworkURL(id int) string {
	if id <= 0 {
		return ""
	}
	return fmt.Sprintf("https://raw.githubusercontent.com/PokeAPI/sprites/master/sprites/pokemon/other/official-artwork/%d.png", id)
}

func pokemonTypeLabel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "normal":
		return "Normal"
	case "fire":
		return "Fogo"
	case "water":
		return "Agua"
	case "electric":
		return "Eletrico"
	case "grass":
		return "Grama"
	case "ice":
		return "Gelo"
	case "fighting":
		return "Lutador"
	case "poison":
		return "Venenoso"
	case "ground":
		return "Terra"
	case "flying":
		return "Voador"
	case "psychic":
		return "Psiquico"
	case "bug":
		return "Inseto"
	case "rock":
		return "Pedra"
	case "ghost":
		return "Fantasma"
	case "dragon":
		return "Dragao"
	case "dark":
		return "Sombrio"
	case "steel":
		return "Aco"
	case "fairy":
		return "Fada"
	default:
		return pokemonDisplayName(value)
	}
}
func pokemonTypePillClass(value string) string {
    switch strings.ToLower(strings.TrimSpace(value)) {
    case "normal":
        // #A8A77A: Muted greenish-gray/beige
        return "bg-stone-400 text-stone-950 border-stone-300"
    case "fire":
        // #EE8130: Vibrant orange
        return "bg-orange-500 text-orange-950 border-orange-400"
    case "water":
        // #6390F0: Mid-tone cornflower blue
        return "bg-blue-400 text-blue-950 border-blue-300"
    case "electric":
        // #F7D02C: Bright yellow
        return "bg-yellow-400 text-yellow-950 border-yellow-300"
    case "grass":
        // #7AC74C: Soft grass green
        return "bg-green-400 text-green-950 border-green-300"
    case "ice":
        // #96D9D6: Soft pastel cyan/teal
        return "bg-cyan-300 text-cyan-950 border-cyan-200"
    case "fighting":
        // #C22E28: Deep crimson red
        return "bg-red-600 text-red-50 border-red-500"
    case "poison":
        // #A33EA1: True purple
        return "bg-purple-600 text-purple-50 border-purple-500"
    case "ground":
        // #E2BF65: Sandy/khaki yellow-brown
        return "bg-yellow-600 text-yellow-950 border-yellow-500"
    case "flying":
        // #A98FF3: Light violet/indigo
        return "bg-indigo-300 text-indigo-950 border-indigo-200"
    case "psychic":
        // #F95587: Deep hot pink
        return "bg-pink-500 text-pink-950 border-pink-400"
    case "bug":
        // #A6B91A: Strong olive/yellow-green
        return "bg-lime-500 text-lime-950 border-lime-400"
    case "rock":
        // #B6A136: Dark tan/olive-gold
        return "bg-amber-600 text-amber-950 border-amber-500"
    case "ghost":
        // #735797: Muted lavender-purple
        return "bg-violet-600 text-violet-50 border-violet-500"
    case "dragon":
        // #6F35FC: Intense, deep blurple/indigo
        return "bg-indigo-600 text-indigo-50 border-indigo-500"
    case "dark":
        // #705746: Warm, muddy dark brown-gray
        return "bg-stone-600 text-stone-50 border-stone-500"
    case "steel":
        // #B7B7CE: Cool metallic blue-gray
        return "bg-slate-300 text-slate-950 border-slate-200"
    case "fairy":
        // #D685AD: Soft, dusty pink
        return "bg-rose-400 text-rose-950 border-rose-300"
    default:
        return "bg-zinc-700 text-zinc-100 border-zinc-600"
    }
}
func pokemonLocalArtworkURL(id int) string {
	if id <= 0 {
		return ""
	}
	return fmt.Sprintf("/static/pokemon-artwork/%d.png", id)
}

func ensurePokemonArtwork(pokemon Pokemon) Pokemon {
	pokemon.Name = pokemonDisplayName(pokemon.Name)
	if strings.TrimSpace(pokemon.ArtworkURL) == "" {
		pokemon.ArtworkURL = pokemonArtworkURL(pokemon.ID)
	}
	return pokemon
}
