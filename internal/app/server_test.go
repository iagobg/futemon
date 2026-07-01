package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func addSession(t *testing.T, server *Server, req *http.Request, userID string) {
	t.Helper()
	res := httptest.NewRecorder()
	server.setSession(res, userID)
	cookies := res.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie")
	}
	req.AddCookie(cookies[0])
}

func TestTeamsPageAndCreateTeamHandler(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()
	server := NewServer(store)

	page := httptest.NewRecorder()
	pageReq := httptest.NewRequest(http.MethodGet, "/teams", nil)
	addSession(t, server, pageReq, demoUserID)
	server.Routes().ServeHTTP(page, pageReq)
	if page.Code != http.StatusOK {
		t.Fatalf("GET /teams status = %d", page.Code)
	}
	body := page.Body.String()
	if !strings.Contains(body, "Meus Times") {
		t.Fatal("teams page did not render expected content")
	}
	if !strings.Contains(body, "href=\"/teams/new\"") {
		t.Fatal("teams page did not include new team link")
	}
	if strings.Contains(body, "data-pokemon-picker") {
		t.Fatal("teams list should not render the team creation form")
	}

	formPage := httptest.NewRecorder()
	formReq := httptest.NewRequest(http.MethodGet, "/teams/new", nil)
	addSession(t, server, formReq, demoUserID)
	server.Routes().ServeHTTP(formPage, formReq)
	if formPage.Code != http.StatusOK {
		t.Fatalf("GET /teams/new status = %d", formPage.Code)
	}
	formBody := formPage.Body.String()
	if !strings.Contains(formBody, "data-pokemon-picker") || !strings.Contains(formBody, "official-artwork") || !strings.Contains(formBody, "data-lineup-preview") || !strings.Contains(formBody, "data-clear-lineup-slot") {
		t.Fatal("team form page did not include Pokemon typeahead, preview artwork, or clear controls")
	}
	if !strings.Contains(formBody, "data-ability-picker") || !strings.Contains(formBody, "data-abilities=") {
		t.Fatal("team form page did not include ability typeahead data")
	}

	if !strings.Contains(formBody, "xl:grid-cols-5") {
		t.Fatal("team form page did not include desktop five-slot lineup grid")
	}

	editPage := httptest.NewRecorder()
	editReq := httptest.NewRequest(http.MethodGet, "/teams/edit?id=team-kanto-press", nil)
	addSession(t, server, editReq, demoUserID)
	server.Routes().ServeHTTP(editPage, editReq)
	if editPage.Code != http.StatusOK {
		t.Fatalf("GET /teams/edit status = %d", editPage.Code)
	}
	if editBody := editPage.Body.String(); !strings.Contains(editBody, "Editar Escalacao") || !strings.Contains(editBody, "Kanto Press") {
		t.Fatalf("edit form did not render existing team: %s", editBody)
	} else if !strings.Contains(editBody, "permite trocar exatamente 1 Pokemon") {
		t.Fatalf("edit form did not explain weekly one-Pokemon transfer limit: %s", editBody)
	}

	form := url.Values{
		"name":                 {"Time Handler"},
		"goalkeeper_id":        {"9"},
		"goalkeeper_ability":   {"torrent"},
		"fixo_id":              {"6"},
		"fixo_ability":         {"blaze"},
		"ala_esquerda_id":      {"25"},
		"ala_esquerda_ability": {"static"},
		"ala_direita_id":       {"4"},
		"ala_direita_ability":  {"blaze"},
		"pivo_id":              {"68"},
		"pivo_ability":         {"no-guard"},
	}
	req := httptest.NewRequest(http.MethodPost, "/teams/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addSession(t, server, req, demoUserID)
	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusSeeOther {
		t.Fatalf("POST /teams/save status = %d", res.Code)
	}
	if got := len(store.MyTeams(demoUserID)); got != 2 {
		t.Fatalf("my teams = %d, want 2", got)
	}
}

func TestSettingsHandlerUpdatesDisplayName(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()
	server := NewServer(store)

	form := url.Values{
		"display_name": {"Novo Nome"},
		"avatar_icon":  {"18"},
	}
	req := httptest.NewRequest(http.MethodPost, "/settings/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addSession(t, server, req, demoUserID)
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
	if user.AvatarIcon != 18 {
		t.Fatalf("avatar icon = %d", user.AvatarIcon)
	}
}

func TestMatchLiveHandlerRendersBroadcastState(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()
	teamA, _ := store.FindTeam("team-kanto-press")
	teamB, _ := store.FindTeam("team-paleta-bolada")
	match := SimulateMatch(teamA, teamB)
	if err := store.SaveMatch(match); err != nil {
		t.Fatal(err)
	}
	server := NewServer(store)

	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/match/"+match.ID+"/live", nil))

	if res.Code != http.StatusOK {
		t.Fatalf("GET /match/{id}/live status = %d", res.Code)
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
	if !strings.Contains(body, "text-center") {
		t.Fatalf("live fragment did not center the score heading: %s", body)
	}
	summaryIndex := strings.Index(body, "data-team-summary")
	progressIndex := strings.Index(body, "data-progress-bar")
	feedIndex := strings.Index(body, "data-event-feed")
	if summaryIndex < 0 || progressIndex < 0 || feedIndex < 0 || !(summaryIndex < progressIndex && progressIndex < feedIndex) {
		t.Fatalf("progress bar should render between team summary and event feed: summary=%d progress=%d feed=%d", summaryIndex, progressIndex, feedIndex)
	}
	if !strings.Contains(body, "data-match-started-at-ms") || !strings.Contains(body, "data-match-ended-at-ms") {
		t.Fatalf("live fragment did not include client timeline anchors: %s", body)
	}
	if !strings.Contains(body, "data-event-clock-end-at-ms") || !strings.Contains(body, "data-clock-end-second") {
		t.Fatalf("live fragment did not include event playback anchors: %s", body)
	}
	if !strings.Contains(body, "data-score-team-a") || !strings.Contains(body, "data-sync-url=\"/match/"+match.ID+"/sync\"") {
		t.Fatalf("live fragment did not include score or sync hooks: %s", body)
	}
	eventKeys := renderedEventKeys(t, body)
	if len(eventKeys) < 2 {
		t.Fatalf("live fragment rendered too few events: %v", eventKeys)
	}
	if eventKeys[0] <= eventKeys[len(eventKeys)-1] {
		t.Fatalf("event list should render newest events first, got keys %v", eventKeys)
	}
}

func renderedEventKeys(t *testing.T, body string) []int {
	t.Helper()
	matches := regexp.MustCompile(`data-event-key="([0-9]+)"`).FindAllStringSubmatch(body, -1)
	keys := make([]int, 0, len(matches))
	for _, match := range matches {
		key, err := strconv.Atoi(match[1])
		if err != nil {
			t.Fatalf("invalid event key %q: %v", match[1], err)
		}
		keys = append(keys, key)
	}
	return keys
}

func TestMatchSyncHandlerReturnsClockContract(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()
	teamA, _ := store.FindTeam("team-kanto-press")
	teamB, _ := store.FindTeam("team-paleta-bolada")
	match := SimulateMatch(teamA, teamB)
	if err := store.SaveMatch(match); err != nil {
		t.Fatal(err)
	}
	server := NewServer(store)

	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/match/"+match.ID+"/sync", nil))

	if res.Code != http.StatusOK {
		t.Fatalf("GET /match/{id}/sync status = %d", res.Code)
	}
	var state MatchSyncState
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if state.MatchID != match.ID || state.MatchVersion != match.ID {
		t.Fatalf("sync identity = %+v, want match %q", state, match.ID)
	}
	if state.ServerNowMS == 0 || state.StartTimeMS != match.StartTime.UnixMilli() || state.EndedAtMS <= state.StartTimeMS {
		t.Fatalf("sync timing = %+v", state)
	}
}

func TestHomeShowsLoginAndAuthenticatedHomeRedirects(t *testing.T) {
	server := NewServer(NewMemoryStore())

	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("GET / status = %d", res.Code)
	}
	if body := res.Body.String(); !strings.Contains(body, "Entrar com Google") || strings.Contains(body, ">Meus Times</a>") {
		t.Fatalf("home did not render as login landing: %s", body)
	}

	authed := httptest.NewRequest(http.MethodGet, "/", nil)
	addSession(t, server, authed, demoUserID)
	res = httptest.NewRecorder()
	server.Routes().ServeHTTP(res, authed)
	if res.Code != http.StatusSeeOther || res.Header().Get("Location") != "/teams" {
		t.Fatalf("authenticated home redirect = %d %q", res.Code, res.Header().Get("Location"))
	}
}

func TestProtectedTeamsRedirectsWithoutSession(t *testing.T) {
	server := NewServer(NewMemoryStore())

	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/teams", nil))
	if res.Code != http.StatusSeeOther || res.Header().Get("Location") != "/" {
		t.Fatalf("GET /teams without session = %d %q", res.Code, res.Header().Get("Location"))
	}
}

func TestHTMXProtectedRouteReturnsUnauthorizedRedirectHint(t *testing.T) {
	server := NewServer(NewMemoryStore())

	req := httptest.NewRequest(http.MethodPost, "/duels/start", nil)
	req.Header.Set("HX-Request", "true")
	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("HTMX protected route status = %d, want 401", res.Code)
	}
	if got := res.Header().Get("HX-Redirect"); got != "/" {
		t.Fatalf("HTMX protected route HX-Redirect = %q, want /", got)
	}
	if body := res.Body.String(); strings.Contains(body, "<!doctype html>") || !strings.Contains(body, "Sessao expirada") {
		t.Fatalf("HTMX protected route body = %q", body)
	}
}

func TestLocalAuthModeAllowsAccessWithoutSession(t *testing.T) {
	t.Setenv("FUTEMON_AUTH_MODE", "local")
	server := NewServer(NewMemoryStore())

	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/teams", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("GET /teams in local auth mode = %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), "Treinador Demo") {
		t.Fatalf("local auth page did not render demo user: %s", res.Body.String())
	}
}

func TestHeaderHidesAdminForNonAdminUser(t *testing.T) {
	store := NewMemoryStore()
	store.user.Role = "user"
	server := NewServer(store)

	req := httptest.NewRequest(http.MethodGet, "/teams", nil)
	addSession(t, server, req, demoUserID)
	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("GET /teams status = %d", res.Code)
	}
	if strings.Contains(res.Body.String(), ">Admin</a>") {
		t.Fatal("non-admin header rendered Admin tab")
	}
}

func TestDuelsPageRendersLoadingFeedback(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()
	server := NewServer(store)

	req := httptest.NewRequest(http.MethodGet, "/duels", nil)
	addSession(t, server, req, demoUserID)
	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("GET /duels status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"data-duel-form", "data-duel-loading", "Aguardando o narrador montar a partida", "Sair desta pagina cancela a geracao", "data-duel-cancel-warning", "data-duel-error"} {
		if !strings.Contains(body, want) {
			t.Fatalf("duels page missing %q: %s", want, body)
		}
	}
	if strings.Contains(body, `name="opponent_id" value="team-paleta-bolada"`) {
		t.Fatalf("random duel button should not submit a fixed opponent: %s", body)
	}
}

func TestStartDuelWithoutOpponentChoosesAvailableGlobalOpponent(t *testing.T) {
	store := NewMemoryStore()
	teamA := store.globalTeams[0]
	teamC := store.globalTeams[2]
	store.myTeams = []Team{teamA}
	store.globalTeams = []Team{teamA, teamC}
	generator := &captureMatchGenerator{}
	server := NewServer(store)
	server.matchGenerator = generator

	form := url.Values{
		"team_id": {teamA.ID},
	}
	req := httptest.NewRequest(http.MethodPost, "/duels/start", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addSession(t, server, req, demoUserID)
	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("duel status = %d, body=%s", res.Code, res.Body.String())
	}
	if generator.teamB.ID != teamC.ID {
		t.Fatalf("random opponent = %q, want %q", generator.teamB.ID, teamC.ID)
	}
}

func TestStartDuelRedirectsToDynamicMatchRoute(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()
	server := NewServer(store)
	server.matchGenerator = LocalMatchGenerator{}

	form := url.Values{
		"team_id":     {"team-kanto-press"},
		"opponent_id": {"team-paleta-bolada"},
	}
	req := httptest.NewRequest(http.MethodPost, "/duels/start", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addSession(t, server, req, demoUserID)
	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, req)

	location := res.Header().Get("HX-Redirect")
	if res.Code != http.StatusNoContent || !strings.HasPrefix(location, "/match/") || location == "/match/latest" {
		t.Fatalf("duel redirect = status %d, HX-Redirect %q", res.Code, location)
	}

	matchPage := httptest.NewRecorder()
	server.Routes().ServeHTTP(matchPage, httptest.NewRequest(http.MethodGet, location, nil))
	if matchPage.Code != http.StatusOK {
		t.Fatalf("GET dynamic match status = %d", matchPage.Code)
	}
	if !strings.Contains(matchPage.Body.String(), "data-sync-url=\""+location+"/sync\"") {
		t.Fatalf("dynamic match page missing scoped sync url: %s", matchPage.Body.String())
	}
}

func TestStartDuelAppliesDailyLimitWithoutBYOK(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()
	server := NewServer(store)
	server.matchGenerator = LocalMatchGenerator{}

	form := url.Values{
		"team_id":     {"team-kanto-press"},
		"opponent_id": {"team-paleta-bolada"},
	}
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/duels/start", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		addSession(t, server, req, demoUserID)
		res := httptest.NewRecorder()
		server.Routes().ServeHTTP(res, req)
		if i == 0 && res.Code != http.StatusNoContent {
			t.Fatalf("first duel status = %d", res.Code)
		}
		if i == 1 && res.Code != http.StatusTooManyRequests {
			t.Fatalf("second duel status = %d, want 429", res.Code)
		}
	}
}

func TestStartDuelReturnsFriendlyOpenRouterRateLimit(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()
	server := NewServer(store)
	server.matchGenerator = errorMatchGenerator{err: &OpenRouterError{
		StatusCode: http.StatusTooManyRequests,
		Status:     "429 Too Many Requests",
		Body:       `{"error":{"message":"Provider returned error","metadata":{"raw":"temporarily rate-limited upstream","user_id":"private"}}}`,
	}}

	form := url.Values{
		"team_id":     {"team-kanto-press"},
		"opponent_id": {"team-paleta-bolada"},
	}
	req := httptest.NewRequest(http.MethodPost, "/duels/start", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addSession(t, server, req, demoUserID)
	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, req)

	body := res.Body.String()
	if res.Code != http.StatusTooManyRequests {
		t.Fatalf("duel status = %d, want 429; body=%s", res.Code, body)
	}
	if !strings.Contains(body, "modelo gratuito do OpenRouter") {
		t.Fatalf("duel rate-limit body = %q", body)
	}
	if strings.Contains(body, "private") || strings.Contains(body, "Provider returned error") {
		t.Fatalf("duel rate-limit leaked upstream body: %q", body)
	}
	count, err := store.DailyDuelCount(demoUserID, duelUsageDate(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("duel usage after provider failure = %d, want refunded to 0", count)
	}
}

func TestStartDuelKeepsDailyLimitWhenUserCancelsRequest(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()
	server := NewServer(store)
	server.matchGenerator = errorMatchGenerator{err: context.Canceled}

	form := url.Values{
		"team_id":     {"team-kanto-press"},
		"opponent_id": {"team-paleta-bolada"},
	}
	req := httptest.NewRequest(http.MethodPost, "/duels/start", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addSession(t, server, req, demoUserID)
	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusGatewayTimeout {
		t.Fatalf("duel status = %d, want 504; body=%s", res.Code, res.Body.String())
	}
	count, err := store.DailyDuelCount(demoUserID, duelUsageDate(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("duel usage after user cancellation = %d, want 1", count)
	}
}

func TestStartDuelBypassesDailyLimitWithBYOK(t *testing.T) {
	store := NewMemoryStore()
	store.user.OpenRouterAPIKey = "user-openrouter-key"
	store.user.HasOpenRouterAPIKey = true
	server := NewServer(store)
	var usedKey string
	server.byokGenerator = func(apiKey string) MatchGenerator {
		usedKey = apiKey
		return LocalMatchGenerator{}
	}

	form := url.Values{
		"team_id":     {"team-kanto-press"},
		"opponent_id": {"team-paleta-bolada"},
	}
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/duels/start", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		addSession(t, server, req, demoUserID)
		res := httptest.NewRecorder()
		server.Routes().ServeHTTP(res, req)
		if res.Code != http.StatusNoContent {
			t.Fatalf("duel %d status = %d", i+1, res.Code)
		}
	}
	if usedKey != "user-openrouter-key" {
		t.Fatalf("BYOK key = %q", usedKey)
	}
}

type errorMatchGenerator struct {
	err error
}

func (g errorMatchGenerator) GenerateMatch(_ context.Context, _ Team, _ Team) (MatchResult, error) {
	return MatchResult{}, g.err
}

type captureMatchGenerator struct {
	teamA Team
	teamB Team
}

func (g *captureMatchGenerator) GenerateMatch(_ context.Context, teamA Team, teamB Team) (MatchResult, error) {
	g.teamA = teamA
	g.teamB = teamB
	return completedTestMatch("match-captured-duel", teamA, teamB, []MatchEvent{
		{Minute: 40, Type: "fulltime", Narrative: "Fim."},
	}), nil
}

func TestTeamHistoryPageLinksReplayAndRecap(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()
	teamA, _ := store.FindTeam("team-kanto-press")
	teamB, _ := store.FindTeam("team-paleta-bolada")
	match := completedTestMatch("match-team-page", teamA, teamB, []MatchEvent{
		{Minute: 7, Type: "goal", TeamID: teamA.ID, PokemonID: teamA.Pivo.ID, Narrative: "Machamp marca."},
		{Minute: 40, Type: "fulltime", Narrative: "Fim."},
	})
	store.SaveMatch(match)
	server := NewServer(store)

	req := httptest.NewRequest(http.MethodGet, "/teams/"+teamA.ID, nil)
	addSession(t, server, req, demoUserID)
	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("GET team history status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"Historico de Kanto Press", "Treinador Demo", "/users/user-demo", "/match/" + match.ID + "/replay", "/match/" + match.ID + "/recap", "Machamp"} {
		if !strings.Contains(body, want) {
			t.Fatalf("team history missing %q: %s", want, body)
		}
	}
}

func TestRetiredTeamPageRendersButCannotBeChallenged(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()
	teamA, _ := store.FindTeam("team-kanto-press")
	retired, err := store.SaveTeam(validAbilityInput(TeamInput{
		UserID:        demoUserID,
		Name:          "Veteranos da Liga",
		GoalkeeperID:  9,
		FixoID:        6,
		AlaEsquerdaID: 25,
		AlaDireitaID:  4,
		PivoID:        68,
	}))
	if err != nil {
		t.Fatal(err)
	}
	match := completedTestMatch("match-retired-page", retired, teamA, []MatchEvent{
		{Minute: 12, Type: "goal", TeamID: retired.ID, PokemonID: retired.Pivo.ID, Narrative: "Dragonite abre o placar."},
		{Minute: 40, Type: "fulltime", Narrative: "Fim."},
	})
	store.SaveMatch(match)
	if err := store.DeleteTeam(retired.ID, demoUserID); err != nil {
		t.Fatal(err)
	}
	server := NewServer(store)

	pageReq := httptest.NewRequest(http.MethodGet, "/teams/"+retired.ID, nil)
	addSession(t, server, pageReq, demoUserID)
	page := httptest.NewRecorder()
	server.Routes().ServeHTTP(page, pageReq)
	if page.Code != http.StatusOK {
		t.Fatalf("GET retired team status = %d", page.Code)
	}
	for _, want := range []string{"Veteranos da Liga", "Aposentado", "/match/" + match.ID, "Machamp"} {
		if !strings.Contains(page.Body.String(), want) {
			t.Fatalf("retired team page missing %q: %s", want, page.Body.String())
		}
	}

	form := url.Values{"team_id": {teamA.ID}, "opponent_id": {retired.ID}}
	duelReq := httptest.NewRequest(http.MethodPost, "/duels/start", strings.NewReader(form.Encode()))
	duelReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addSession(t, server, duelReq, demoUserID)
	duel := httptest.NewRecorder()
	server.Routes().ServeHTTP(duel, duelReq)
	if duel.Code != http.StatusBadRequest {
		t.Fatalf("retired challenge status = %d", duel.Code)
	}
}

func TestProfilePageRendersTeamsRecordAndRetiredTeams(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()
	retired, err := store.SaveTeam(validAbilityInput(TeamInput{
		UserID:        demoUserID,
		Name:          "Time de Museu",
		GoalkeeperID:  9,
		FixoID:        6,
		AlaEsquerdaID: 25,
		AlaDireitaID:  4,
		PivoID:        68,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteTeam(retired.ID, demoUserID); err != nil {
		t.Fatal(err)
	}
	server := NewServer(store)

	req := httptest.NewRequest(http.MethodGet, "/users/"+demoUserID, nil)
	addSession(t, server, req, demoUserID)
	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("GET profile status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"Treinador Demo", "Recorde total", "Kanto Press", "Time de Museu", "Aposentados"} {
		if !strings.Contains(body, want) {
			t.Fatalf("profile missing %q: %s", want, body)
		}
	}
}

func TestGlobalTeamsCanSortByBestRecord(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()
	teamA, _ := store.FindTeam("team-kanto-press")
	teamB, _ := store.FindTeam("team-paleta-bolada")
	store.SaveMatch(completedTestMatch("match-global-best", teamA, teamB, []MatchEvent{
		{Minute: 4, Type: "goal", TeamID: teamA.ID, PokemonID: teamA.Pivo.ID, Narrative: "Gol."},
		{Minute: 40, Type: "fulltime", Narrative: "Fim."},
	}))
	server := NewServer(store)

	req := httptest.NewRequest(http.MethodGet, "/global-teams?sort=best", nil)
	addSession(t, server, req, demoUserID)
	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("GET global teams status = %d", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, "Ordene por criacao recente") || !strings.Contains(body, "#1") {
		t.Fatalf("global teams page missing leaderboard details: %s", body)
	}
	if strings.Index(body, "Kanto Press") > strings.Index(body, "Paleta Bolada") {
		t.Fatalf("best sort did not place winner first: %s", body)
	}
}

func TestGlobalTeamsDefaultsToBestRecord(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()
	teamA, _ := store.FindTeam("team-kanto-press")
	teamB, _ := store.FindTeam("team-paleta-bolada")
	store.SaveMatch(completedTestMatch("match-global-default-best", teamA, teamB, []MatchEvent{
		{Minute: 4, Type: "goal", TeamID: teamA.ID, PokemonID: teamA.Pivo.ID, Narrative: "Gol."},
		{Minute: 40, Type: "fulltime", Narrative: "Fim."},
	}))
	server := NewServer(store)

	req := httptest.NewRequest(http.MethodGet, "/global-teams", nil)
	addSession(t, server, req, demoUserID)
	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("GET global teams status = %d", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, "href=\"/global-teams?sort=best\" class=\"rounded-md px-3 py-2 bg-lime-300") {
		t.Fatalf("best sort tab was not active by default: %s", body)
	}
	if strings.Index(body, "Kanto Press") > strings.Index(body, "Paleta Bolada") {
		t.Fatalf("default sort did not place winner first: %s", body)
	}
}

func TestMatchRecapUsesLoadedBroadcastRenderer(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()
	teamA, _ := store.FindTeam("team-kanto-press")
	teamB, _ := store.FindTeam("team-paleta-bolada")
	match := completedTestMatch("match-recap", teamA, teamB, []MatchEvent{
		{Minute: 11, Type: "goal", TeamID: teamB.ID, PokemonID: teamB.Pivo.ID, Narrative: "Dragonite marca para o visitante."},
		{Minute: 40, Type: "fulltime", Narrative: "Fim de jogo."},
	})
	store.SaveMatch(match)
	server := NewServer(store)

	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/match/"+match.ID+"/recap", nil))

	if res.Code != http.StatusOK {
		t.Fatalf("GET match recap status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"data-broadcast-state", "data-playback-mode=\"report\"", "ENCERRADO", "Dragonite", "Dragonite marca para o visitante."} {
		if !strings.Contains(body, want) {
			t.Fatalf("match recap missing %q: %s", want, body)
		}
	}
}

func TestMatchReplayUsesClientSideReplayMode(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()
	teamA, _ := store.FindTeam("team-kanto-press")
	teamB, _ := store.FindTeam("team-paleta-bolada")
	match := completedTestMatch("match-replay", teamA, teamB, []MatchEvent{
		{Minute: 11, Type: "goal", TeamID: teamB.ID, PokemonID: teamB.Pivo.ID, Narrative: "Dragonite marca para o visitante."},
		{Minute: 40, Type: "fulltime", Narrative: "Fim de jogo."},
	})
	store.SaveMatch(match)
	server := NewServer(store)

	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/match/"+match.ID+"/replay", nil))

	if res.Code != http.StatusOK {
		t.Fatalf("GET match replay status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"data-broadcast-state", "data-playback-mode=\"replay\"", "AO VIVO"} {
		if !strings.Contains(body, want) {
			t.Fatalf("match replay missing %q: %s", want, body)
		}
	}
}

func TestSessionRoundTripUsesSignedCookie(t *testing.T) {
	store := NewMemoryStore()
	server := NewServer(store)
	server.sessionKey = []byte("test-session-secret")

	res := httptest.NewRecorder()
	server.setSession(res, demoUserID)
	cookies := res.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie")
	}

	req := httptest.NewRequest(http.MethodGet, "/teams", nil)
	req.AddCookie(cookies[0])
	user, ok := server.currentUser(req)
	if !ok || user.ID != demoUserID {
		t.Fatalf("session user = %+v, ok = %v", user, ok)
	}
}

func TestGoogleLoginRequiresOAuthConfig(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("GOOGLE_CLIENT_SECRET", "")
	server := NewServer(NewMemoryStore())

	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/auth/google", nil))

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /auth/google status = %d, want %d", res.Code, http.StatusServiceUnavailable)
	}
}

func TestPokemonArtworkHandlerServesLocalFilesWithLongCache(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "25.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	server := NewServer(NewMemoryStore())
	server.artworkDir = dir

	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/static/pokemon-artwork/25.png", nil))

	if res.Code != http.StatusOK {
		t.Fatalf("GET artwork status = %d", res.Code)
	}
	if cache := res.Header().Get("Cache-Control"); !strings.Contains(cache, "max-age=31536000") || !strings.Contains(cache, "immutable") {
		t.Fatalf("Cache-Control = %q", cache)
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
	js := res.Body.String()
	if !strings.Contains(js, "requestAnimationFrame") {
		t.Fatal("app.js did not include animation loop")
	}
	if !strings.Contains(js, "syncMatch") || !strings.Contains(js, "match_version") {
		t.Fatal("app.js did not include passive match sync")
	}
	if !strings.Contains(js, `item.dataset.eventType === "goal"`) {
		t.Fatal("app.js did not guard score updates to goal events")
	}
	if !strings.Contains(js, "hydratePokemonPickers") || !strings.Contains(js, "ArrowDown") {
		t.Fatal("app.js did not include Pokemon typeahead controls")
	}
	if !strings.Contains(js, "hydrateAbilityPickers") || !strings.Contains(js, "selectedPokemonIds") || !strings.Contains(js, "focusout") || !strings.Contains(js, "updateLineupPreview") || !strings.Contains(js, "hydrateLineupClearButtons") {
		t.Fatal("app.js did not include team form picker controls")
	}
}

func TestStaticAppCSS(t *testing.T) {
	server := NewServer(NewMemoryStore())

	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/static/app.css", nil))

	if res.Code != http.StatusOK {
		t.Fatalf("GET /static/app.css status = %d", res.Code)
	}
	if contentType := res.Header().Get("Content-Type"); !strings.Contains(contentType, "text/css") {
		t.Fatalf("content type = %q", contentType)
	}
	css := res.Body.String()
	if !strings.Contains(css, ".bg-zinc-950") || !strings.Contains(css, ".text-lime-300") {
		t.Fatal("app.css did not include expected Tailwind utilities")
	}
}

func TestLayoutUsesLocalTailwindCSS(t *testing.T) {
	server := NewServer(NewMemoryStore())

	req := httptest.NewRequest(http.MethodGet, "/teams", nil)
	addSession(t, server, req, demoUserID)
	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, req)

	body := res.Body.String()
	if !strings.Contains(body, `href="/static/app.css"`) {
		t.Fatalf("layout did not load local app.css: %s", body)
	}
	if strings.Contains(body, "cdn.tailwindcss.com") {
		t.Fatalf("layout still loads Tailwind CDN: %s", body)
	}
}

func TestAdminRequiresAdminRole(t *testing.T) {
	store := NewMemoryStore()
	store.user.Role = "user"
	server := NewServer(store)

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	addSession(t, server, req, demoUserID)
	res := httptest.NewRecorder()
	server.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("GET /admin status = %d, want %d", res.Code, http.StatusForbidden)
	}
}
