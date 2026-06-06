package app

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestTeamsPageAndCreateTeamHandler(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()
	server := NewServer(store)

	page := httptest.NewRecorder()
	server.Routes().ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/teams", nil))
	if page.Code != http.StatusOK {
		t.Fatalf("GET /teams status = %d", page.Code)
	}
	if !strings.Contains(page.Body.String(), "Meus Times") {
		t.Fatal("teams page did not render expected content")
	}

	form := url.Values{
		"name":            {"Time Handler"},
		"goalkeeper_id":   {"9"},
		"fixo_id":         {"6"},
		"ala_esquerda_id": {"25"},
		"ala_direita_id":  {"4"},
		"pivo_id":         {"68"},
	}
	req := httptest.NewRequest(http.MethodPost, "/teams/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusSeeOther {
		t.Fatalf("POST /teams/save status = %d", res.Code)
	}
	if got := len(store.MyTeams()); got != 2 {
		t.Fatalf("my teams = %d, want 2", got)
	}
}

func TestSettingsHandlerUpdatesDisplayName(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()
	server := NewServer(store)

	form := url.Values{
		"display_name": {"Novo Nome"},
	}
	req := httptest.NewRequest(http.MethodPost, "/settings/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusSeeOther {
		t.Fatalf("POST /settings/save status = %d", res.Code)
	}
	user, ok := store.CurrentUser()
	if !ok {
		t.Fatal("current user not found")
	}
	if user.DisplayName != "Novo Nome" {
		t.Fatalf("display name = %q", user.DisplayName)
	}
}

func TestMatchLiveHandlerRendersBroadcastState(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()
	teamA, _ := store.FindTeam("team-kanto-press")
	teamB, _ := store.FindTeam("team-paleta-bolada")
	store.SetLatestMatch(SimulateMatch(teamA, teamB))
	server := NewServer(store)

	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/match/live", nil))

	if res.Code != http.StatusOK {
		t.Fatalf("GET /match/live status = %d", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, "Transmissao") && !strings.Contains(body, "AO VIVO") {
		t.Fatalf("live fragment did not look like broadcast state: %s", body)
	}
	if !strings.Contains(body, "data-typewriter") {
		t.Fatalf("live fragment did not include client typewriter hook: %s", body)
	}
	if !strings.Contains(body, "data-progress-bar") || !strings.Contains(body, "data-match-clock") {
		t.Fatalf("live fragment did not include client clock hooks: %s", body)
	}
	if !strings.Contains(body, "data-next-refresh-ms") {
		t.Fatalf("live fragment did not include client refresh timing: %s", body)
	}
}

func TestStaticAppJS(t *testing.T) {
	server := NewServer(NewMemoryStore())

	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/static/app.js", nil))

	if res.Code != http.StatusOK {
		t.Fatalf("GET /static/app.js status = %d", res.Code)
	}
	if contentType := res.Header().Get("Content-Type"); !strings.Contains(contentType, "text/javascript") {
		t.Fatalf("content type = %q", contentType)
	}
	if !strings.Contains(res.Body.String(), "requestAnimationFrame") {
		t.Fatal("app.js did not include animation loop")
	}
}

func TestAdminRequiresAdminRole(t *testing.T) {
	store := NewMemoryStore()
	store.user.Role = "user"
	server := NewServer(store)

	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/admin", nil))

	if res.Code != http.StatusForbidden {
		t.Fatalf("GET /admin status = %d, want %d", res.Code, http.StatusForbidden)
	}
}
