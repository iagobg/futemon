package app

import "testing"

func TestPokemonDisplayNameFormatsSpecialCases(t *testing.T) {
	cases := map[string]string{
		"bulbasaur": "Bulbasaur",
		"mr-mime":   "Mr. Mime",
		"mime-jr":   "Mime Jr.",
		"ho-oh":     "Ho-Oh",
		"nidoran-f": "Nidoran♀",
	}
	for input, want := range cases {
		if got := pokemonDisplayName(input); got != want {
			t.Fatalf("pokemonDisplayName(%q) = %q, want %q", input, got, want)
		}
	}
}
