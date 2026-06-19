package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvLoadsValuesWithoutOverridingEnvironment(t *testing.T) {
	t.Setenv("FUTEMON_ENV_TEST_KEEP", "from-env")
	t.Setenv("FUTEMON_ENV_TEST_PORT", "")
	_ = os.Unsetenv("FUTEMON_ENV_TEST_PORT")

	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(`
# local defaults
FUTEMON_ENV_TEST_PORT=8090
FUTEMON_ENV_TEST_KEEP=from-file
export FUTEMON_ENV_TEST_QUOTED="hello world"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := LoadDotEnv(path); err != nil {
		t.Fatal(err)
	}

	if got := os.Getenv("FUTEMON_ENV_TEST_PORT"); got != "8090" {
		t.Fatalf("loaded port = %q, want 8090", got)
	}
	if got := os.Getenv("FUTEMON_ENV_TEST_KEEP"); got != "from-env" {
		t.Fatalf("existing env was overwritten: %q", got)
	}
	if got := os.Getenv("FUTEMON_ENV_TEST_QUOTED"); got != "hello world" {
		t.Fatalf("quoted value = %q, want hello world", got)
	}
}

func TestLoadDotEnvIgnoresMissingFile(t *testing.T) {
	if err := LoadDotEnv(filepath.Join(t.TempDir(), ".env")); err != nil {
		t.Fatal(err)
	}
}
