package app

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const trainerIconCount = 152
const trainerIconColumns = 24
const trainerIconRows = 32
const defaultTrainerIconSheetPath = "data/icons/pngkey.com-3ds-png-2672768.png"

type Server struct {
	store          Store
	matchGenerator MatchGenerator
	byokGenerator  func(apiKey string) MatchGenerator
	duelLimiter    *DailyDuelLimiter
	authMode       string
	templates      *template.Template
	sessionKey     []byte
	googleOAuth    GoogleOAuthConfig
	artworkDir     string
}

func NewServer(store Store) *Server {
	sessionKey := []byte(os.Getenv("SESSION_SECRET"))
	artworkDir := os.Getenv("FUTEMON_ARTWORK_DIR")
	if artworkDir == "" {
		artworkDir = DefaultPokemonArtworkDir
	}
	authMode := normalizeAuthMode(os.Getenv("FUTEMON_AUTH_MODE"))
	return &Server{
		store:          store,
		matchGenerator: NewMatchGeneratorFromEnv(),
		byokGenerator:  func(apiKey string) MatchGenerator { return NewOpenRouterMatchGeneratorFromEnv(apiKey) },
		duelLimiter:    NewDailyDuelLimiter(dailyDuelLimitFromEnv(authMode)),
		authMode:       authMode,
		sessionKey:     sessionKey,
		googleOAuth:    loadGoogleOAuthConfig(),
		artworkDir:     artworkDir,
		templates: template.Must(template.New("app").Funcs(template.FuncMap{
			"dict":                 dict,
			"formatShortTime":      formatShortTime,
			"inc":                  inc,
			"iconChoices":          iconChoices,
			"list":                 list,
			"lower":                strings.ToLower,
			"matchGoalsA":          matchGoalsA,
			"matchGoalsB":          matchGoalsB,
			"abilityDisplayName":   abilityDisplayName,
			"pokemonAbilitiesJSON": pokemonAbilitiesJSON,
			"pokemonArtwork":       pokemonArtworkByID,
			"pokemonDisplayName":   pokemonDisplayName,
			"pokemonName":          pokemonNameByID,
			"pokemonTypeLabel":     pokemonTypeLabel,
			"pokemonTypePillClass": pokemonTypePillClass,
			"reverseEvents":        reverseRenderedMatchEvents,
			"resultLabel":          resultLabel,
			"teamRecordSum":        teamRecordSum,
			"trainerAvatarStyle":   trainerAvatarStyle,
			"trainerInitial":       trainerInitial,
			"transferKindLabel":    transferKindLabel,
		}).Parse(mustReadEmbeddedText("templates/page.html"))),
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleHome)
	mux.HandleFunc("/teams", s.handleTeams)
	mux.HandleFunc("/teams/new", s.handleNewTeam)
	mux.HandleFunc("/teams/edit", s.handleEditTeam)
	mux.HandleFunc("/teams/save", s.handleSaveTeam)
	mux.HandleFunc("/teams/delete", s.handleDeleteTeam)
	mux.HandleFunc("/teams/", s.handleTeamRoute)
	mux.HandleFunc("/users/", s.handleUserRoute)
	mux.HandleFunc("/profile", s.handleProfile)
	mux.HandleFunc("/duels", s.handleDuels)
	mux.HandleFunc("/duels/start", s.handleStartDuel)
	mux.HandleFunc("/match/", s.handleMatchRoute)
	mux.HandleFunc("/tournaments", s.handleTournaments)
	mux.HandleFunc("/global-teams", s.handleGlobalTeams)
	mux.HandleFunc("/admin", s.handleAdmin)
	mux.HandleFunc("/settings", s.handleSettings)
	mux.HandleFunc("/settings/save", s.handleSaveSettings)
	mux.HandleFunc("/auth/google", s.handleGoogleLogin)
	mux.HandleFunc("/auth/google/callback", s.handleGoogleCallback)
	mux.HandleFunc("/auth/logout", s.handleLogout)
	mux.HandleFunc("/static/app.js", s.handleAsset)
	mux.HandleFunc("/static/trainer-icons.png", s.handleTrainerIcons)
	mux.Handle("/static/pokemon-artwork/", s.pokemonArtworkHandler())
	return mux
}

func (s *Server) handleTrainerIcons(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "metodo nao permitido", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, filepath.Clean(defaultTrainerIconSheetPath))
}

func (s *Server) pokemonArtworkHandler() http.Handler {
	fileServer := http.StripPrefix("/static/pokemon-artwork/", http.FileServer(http.Dir(s.artworkDir)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		fileServer.ServeHTTP(w, r)
	})
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(r)
	if ok {
		http.Redirect(w, r, "/teams", http.StatusSeeOther)
		return
	}
	s.render(w, "layout", ViewData{Active: "home", User: user})
}

func (s *Server) requireUser(w http.ResponseWriter, r *http.Request) (User, bool) {
	user, ok := s.currentUser(r)
	if !ok {
		if isHTMXRequest(r) {
			w.Header().Set("HX-Redirect", "/")
			http.Error(w, "Sessao expirada. Entre novamente para continuar.", http.StatusUnauthorized)
			return User{}, false
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return User{}, false
	}
	return user, true
}

func isHTMXRequest(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("HX-Request"), "true")
}

func (s *Server) handleTeams(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireUser(w, r); !ok {
		return
	}
	if editID := r.URL.Query().Get("edit"); editID != "" {
		http.Redirect(w, r, "/teams/edit?id="+editID, http.StatusSeeOther)
		return
	}
	data := s.teamViewData(r)
	s.render(w, "layout", data)
}

func (s *Server) handleNewTeam(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	s.render(w, "layout", s.teamFormViewData(user, defaultTeamInput(s.store.Pokemon()), false))
}

func (s *Server) handleEditTeam(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	team, ok := s.store.FindTeam(r.URL.Query().Get("id"))
	if !ok || team.UserID != user.ID {
		data := s.teamViewData(r)
		data.Error = "Time nao encontrado."
		s.render(w, "layout", data)
		return
	}
	s.render(w, "layout", s.teamFormViewData(user, teamInputFromTeam(team), true))
}

func (s *Server) handleSaveTeam(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "metodo nao permitido", http.StatusMethodNotAllowed)
		return
	}
	input, err := teamInputFromRequest(r)
	currentUser, ok := s.currentUser(r)
	if !ok {
		http.Error(w, "usuario nao encontrado", http.StatusUnauthorized)
		return
	}
	if err != nil {
		data := s.teamFormViewData(currentUser, input, input.ID != "")
		data.Error = "Revise a escalacao antes de salvar."
		s.render(w, "layout", data)
		return
	}
	input.UserID = currentUser.ID

	team, err := s.store.SaveTeam(input)
	if err != nil {
		data := s.teamFormViewData(currentUser, input, input.ID != "")
		data.Error = teamErrorMessage(err)
		s.render(w, "layout", data)
		return
	}
	_ = team
	http.Redirect(w, r, "/teams?saved=1", http.StatusSeeOther)
}

func (s *Server) handleDeleteTeam(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "metodo nao permitido", http.StatusMethodNotAllowed)
		return
	}
	currentUser, ok := s.currentUser(r)
	if !ok {
		http.Error(w, "usuario nao encontrado", http.StatusUnauthorized)
		return
	}
	if err := s.store.DeleteTeam(r.FormValue("id"), currentUser.ID); err != nil {
		data := s.teamViewData(r)
		data.Error = teamErrorMessage(err)
		s.render(w, "layout", data)
		return
	}
	http.Redirect(w, r, "/teams?deleted=1", http.StatusSeeOther)
}

func (s *Server) handleDuels(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	s.render(w, "layout", ViewData{Active: "duels", User: user, Teams: s.store.MyTeams(user.ID), GlobalTeams: s.store.GlobalTeams("best")})
}

func (s *Server) handleStartDuel(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	_ = r.ParseForm()
	teamAID := r.FormValue("team_id")
	teamBID := r.FormValue("opponent_id")
	if r.FormValue("duel_mode") == "random" {
		teamBID = ""
	}

	teamA, okA := s.store.FindTeam(teamAID)
	if okA && teamA.UserID != user.ID {
		okA = false
	}
	if !okA {
		http.Error(w, "time nao encontrado", http.StatusBadRequest)
		return
	}
	if teamBID == "" {
		teamBID = s.randomOpponentID(teamA, user.ID)
	}
	teamB, okB := s.store.FindTeam(teamBID)
	if !okB {
		http.Error(w, "time nao encontrado", http.StatusBadRequest)
		return
	}

	userAPIKey, hasUserAPIKey, err := s.store.UserAPIKey(user.ID)
	if err != nil {
		log.Printf("load user API key failed: user=%s error=%v", user.ID, err)
		http.Error(w, "Nao foi possivel ler a chave BYOK da conta.", http.StatusInternalServerError)
		return
	}
	now := time.Now()
	consumedDailyDuel := false
	if !hasUserAPIKey {
		canUse, err := s.duelLimiter.CanUse(s.store, user.ID, now)
		if err != nil {
			log.Printf("load duel usage failed: user=%s error=%v", user.ID, err)
			http.Error(w, "Nao foi possivel verificar o limite diario.", http.StatusInternalServerError)
			return
		}
		if !canUse {
			http.Error(w, "Limite diario de duelos atingido. Configure uma chave OpenRouter na conta para usar BYOK.", http.StatusTooManyRequests)
			return
		}
		if err := s.duelLimiter.Record(s.store, user.ID, now); err != nil {
			log.Printf("record duel usage failed: user=%s error=%v", user.ID, err)
			http.Error(w, "Nao foi possivel registrar o limite diario.", http.StatusInternalServerError)
			return
		}
		consumedDailyDuel = true
	}

	generator := s.matchGenerator
	if hasUserAPIKey {
		generator = s.byokGenerator(userAPIKey)
	}
	match, err := generator.GenerateMatch(r.Context(), teamA, teamB)
	if err != nil {
		log.Printf("generate duel failed: team_a=%s team_b=%s error=%v", teamA.ID, teamB.ID, err)
		if consumedDailyDuel && shouldRefundDuelGenerationError(err) {
			s.refundDailyDuel(user.ID, now)
		}
		status, message := duelGenerationErrorResponse(err, hasUserAPIKey)
		http.Error(w, message, status)
		return
	}
	if err := s.store.SaveMatch(match); err != nil {
		log.Printf("save duel failed: match=%s error=%v", match.ID, err)
		if consumedDailyDuel {
			s.refundDailyDuel(user.ID, now)
		}
		http.Error(w, "A partida foi gerada, mas nao foi possivel salva-la.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Redirect", "/match/"+match.ID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) refundDailyDuel(userID string, now time.Time) {
	if err := s.duelLimiter.Refund(s.store, userID, now); err != nil {
		log.Printf("refund duel usage failed: user=%s error=%v", userID, err)
	}
}

func (s *Server) randomOpponentID(teamA Team, userID string) string {
	var candidates []Team
	for _, team := range s.store.GlobalTeams("recent") {
		if team.ID == "" || strings.EqualFold(team.ID, teamA.ID) || strings.EqualFold(team.UserID, userID) {
			continue
		}
		candidates = append(candidates, team)
	}
	if len(candidates) == 0 {
		return ""
	}
	index, err := randomIndex(len(candidates))
	if err != nil {
		index = int(time.Now().UnixNano() % int64(len(candidates)))
	}
	return candidates[index].ID
}

func randomIndex(max int) (int, error) {
	if max <= 0 {
		return 0, errors.New("max must be positive")
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

type DailyDuelLimiter struct {
	limit int
}

func NewDailyDuelLimiter(limit int) *DailyDuelLimiter {
	return &DailyDuelLimiter{limit: limit}
}

func (l *DailyDuelLimiter) CanUse(store Store, userID string, now time.Time) (bool, error) {
	if l == nil || l.limit <= 0 {
		return true, nil
	}
	count, err := store.DailyDuelCount(userID, duelUsageDate(now))
	if err != nil {
		return false, err
	}
	return count < l.limit, nil
}

func (l *DailyDuelLimiter) Record(store Store, userID string, now time.Time) error {
	if l == nil || l.limit <= 0 {
		return nil
	}
	return store.RecordDailyDuel(userID, duelUsageDate(now))
}

func (l *DailyDuelLimiter) Refund(store Store, userID string, now time.Time) error {
	if l == nil || l.limit <= 0 {
		return nil
	}
	return store.RefundDailyDuel(userID, duelUsageDate(now))
}

func shouldRefundDuelGenerationError(err error) bool {
	return !errors.Is(err, context.Canceled)
}

func duelGenerationErrorResponse(err error, hasUserAPIKey bool) (int, string) {
	var openRouterErr *OpenRouterError
	if errors.As(err, &openRouterErr) {
		switch openRouterErr.StatusCode {
		case http.StatusTooManyRequests:
			if hasUserAPIKey {
				return http.StatusTooManyRequests, "Sua chave OpenRouter atingiu um limite temporario. Tente novamente em alguns minutos ou revise seus limites no OpenRouter."
			}
			return http.StatusTooManyRequests, "O modelo gratuito do OpenRouter esta temporariamente limitado. Tente novamente em alguns minutos ou configure sua propria chave OpenRouter na conta."
		case http.StatusRequestTimeout, http.StatusGatewayTimeout:
			return http.StatusGatewayTimeout, "O OpenRouter demorou demais para responder. Tente novamente em instantes."
		case http.StatusUnauthorized, http.StatusForbidden:
			if hasUserAPIKey {
				return http.StatusBadGateway, "A chave OpenRouter salva nao foi aceita. Atualize a chave na conta e tente novamente."
			}
			return http.StatusBadGateway, "A chave OpenRouter do app nao foi aceita. Avise o administrador ou configure sua propria chave na conta."
		}
		if openRouterErr.StatusCode >= 500 {
			return http.StatusBadGateway, "O provedor de IA esta instavel no momento. Tente novamente em instantes."
		}
		return http.StatusBadGateway, "O OpenRouter rejeitou a geracao da partida. Tente novamente em instantes."
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return http.StatusGatewayTimeout, "O OpenRouter demorou demais para responder. Tente novamente em instantes."
	}
	if errors.Is(err, context.Canceled) {
		return http.StatusGatewayTimeout, "A geracao da partida foi interrompida antes de terminar. Tente novamente."
	}
	return http.StatusBadGateway, "Nao foi possivel gerar a partida pela API. Tente novamente em instantes."
}

func normalizeAuthMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "local", "none", "noauth", "disabled":
		return "local"
	default:
		return "google"
	}
}

func dailyDuelLimitFromEnv(authMode string) int {
	value := strings.TrimSpace(os.Getenv("FUTEMON_DAILY_DUEL_LIMIT"))
	if value != "" {
		limit, err := strconv.Atoi(value)
		if err == nil {
			return limit
		}
	}
	if authMode == "local" {
		return 0
	}
	return 1
}

func duelUsageDate(now time.Time) string {
	return now.UTC().Format("2006-01-02")
}

func (s *Server) handleMatchRoute(w http.ResponseWriter, r *http.Request) {
	suffix := strings.Trim(strings.TrimPrefix(r.URL.Path, "/match/"), "/")
	if suffix == "" {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(suffix, "/")
	matchID := parts[0]
	match, ok := s.store.MatchByID(matchID)
	if !ok {
		http.Error(w, "partida nao encontrada", http.StatusNotFound)
		return
	}
	if len(parts) == 1 {
		user, _ := s.currentUser(r)
		s.render(w, "layout", ViewData{Active: "match", User: user, Match: match, MatchState: matchRenderState(match, time.Now(), "live")})
		return
	}
	switch parts[1] {
	case "live":
		s.render(w, "matchLive", matchRenderState(match, time.Now(), "live"))
	case "replay":
		user, _ := s.currentUser(r)
		s.render(w, "layout", ViewData{Active: "match", User: user, Match: match, MatchState: replayMatchRenderState(match)})
	case "sync":
		s.renderMatchSync(w, match)
	case "events":
		s.renderDoneMatchEvents(w, match)
	case "recap":
		user, _ := s.currentUser(r)
		s.render(w, "layout", ViewData{Active: "match", User: user, Match: match, MatchState: reportMatchRenderState(match)})
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) renderMatchSync(w http.ResponseWriter, match MatchResult) {
	now := time.Now()
	state := RenderMatch(match, now)
	status := "live"
	if state.Finished {
		status = "finished"
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(MatchSyncState{
		MatchID:      match.ID,
		MatchVersion: match.ID,
		Status:       status,
		ServerNowMS:  now.UnixMilli(),
		StartTimeMS:  state.StartedAtMS,
		EndedAtMS:    state.EndedAtMS,
	})
}

func (s *Server) renderDoneMatchEvents(w http.ResponseWriter, match MatchResult) {
	state := RenderMatch(match, time.Now())
	var done []RenderedMatchEvent
	for _, event := range state.Events {
		if event.Status == "done" {
			done = append(done, event)
		}
	}
	s.render(w, "eventList", done)
}

func (s *Server) handleTournaments(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	s.render(w, "layout", ViewData{Active: "tournaments", User: user, Tournaments: s.store.Tournaments(), Teams: s.store.MyTeams(user.ID)})
}

func (s *Server) handleGlobalTeams(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	sortBy := r.URL.Query().Get("sort")
	if sortBy != "recent" {
		sortBy = "best"
	}
	s.render(w, "layout", ViewData{Active: "global", User: user, GlobalTeams: s.store.GlobalTeams(sortBy), TeamSort: sortBy})
}

func (s *Server) handleTeamRoute(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	teamID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/teams/"), "/")
	if teamID == "" {
		http.NotFound(w, r)
		return
	}
	team, ok := s.store.FindTeamIncludingRetired(teamID)
	if !ok {
		http.Error(w, "time nao encontrado", http.StatusNotFound)
		return
	}
	trainer, _ := s.store.UserByID(team.UserID)
	s.render(w, "layout", ViewData{
		Active:        "team_detail",
		User:          user,
		Trainer:       trainer,
		Team:          team,
		TeamHistory:   s.store.TeamHistory(team.ID),
		TeamTransfers: s.store.TeamTransfers(team.ID),
	})
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	http.Redirect(w, r, "/users/"+user.ID, http.StatusSeeOther)
}

func (s *Server) handleUserRoute(w http.ResponseWriter, r *http.Request) {
	viewer, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	userID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/users/"), "/")
	if userID == "" {
		http.NotFound(w, r)
		return
	}
	profile, ok := s.store.UserByID(userID)
	if !ok {
		http.Error(w, "usuario nao encontrado", http.StatusNotFound)
		return
	}
	activeTeams := s.store.MyTeams(profile.ID)
	retiredTeams := s.store.RetiredTeams(profile.ID)
	allTeams := append(append([]Team(nil), activeTeams...), retiredTeams...)
	bestTeam := bestTeamByRecord(activeTeams)
	s.render(w, "layout", ViewData{
		Active:              "profile",
		User:                viewer,
		ProfileUser:         profile,
		ProfileTeams:        activeTeams,
		ProfileRetiredTeams: retiredTeams,
		ProfileRecord:       teamRecordSum(allTeams),
		ProfileBestTeam:     bestTeam,
	})
}

func (s *Server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(r)
	if !ok || user.Role != "admin" {
		http.Error(w, "acesso restrito a administradores", http.StatusForbidden)
		return
	}
	s.render(w, "layout", ViewData{Active: "admin", User: user, Tournaments: s.store.Tournaments()})
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireUser(w, r); !ok {
		return
	}
	data := s.accountViewData(r)
	s.render(w, "layout", data)
}

func (s *Server) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "metodo nao permitido", http.StatusMethodNotAllowed)
		return
	}
	user, ok := s.currentUser(r)
	if !ok {
		http.Error(w, "usuario nao encontrado", http.StatusUnauthorized)
		return
	}
	if err := r.ParseForm(); err != nil {
		data := s.accountViewData(r)
		data.Error = "Nao foi possivel ler o formulario."
		s.render(w, "layout", data)
		return
	}
	_, err := s.store.UpdateAccount(AccountInput{
		UserID:           user.ID,
		DisplayName:      r.FormValue("display_name"),
		AvatarIcon:       parseAvatarIcon(r.FormValue("avatar_icon")),
		OpenRouterAPIKey: r.FormValue("openrouter_api_key"),
		ClearAPIKey:      r.FormValue("clear_api_key") == "on",
	})
	if err != nil {
		data := s.accountViewData(r)
		data.AccountForm = AccountInput{DisplayName: r.FormValue("display_name")}
		data.Error = accountErrorMessage(err)
		s.render(w, "layout", data)
		return
	}
	http.Redirect(w, r, "/settings?saved=1", http.StatusSeeOther)
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type ViewData struct {
	Active              string
	Teams               []Team
	GlobalTeams         []Team
	Pokemon             []Pokemon
	Tournaments         []Tournament
	Match               MatchResult
	MatchState          MatchRenderState
	Team                Team
	Trainer             User
	TeamHistory         []MatchSummary
	TeamTransfers       []TeamTransfer
	TransferWindow      TransferWindow
	TeamSort            string
	User                User
	ProfileUser         User
	ProfileTeams        []Team
	ProfileRetiredTeams []Team
	ProfileRecord       TeamRecord
	ProfileBestTeam     Team
	TeamForm            TeamInput
	AccountForm         AccountInput
	EditingTeam         bool
	Flash               string
	Error               string
}

func (m MatchResult) JSON() string {
	payload, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(payload)
}

func dict(values ...any) map[string]any {
	out := make(map[string]any, len(values)/2)
	for i := 0; i+1 < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			continue
		}
		out[key] = values[i+1]
	}
	return out
}

func list(values ...any) []any {
	return values
}

func inc(value int) int {
	return value + 1
}

func iconChoices() []int {
	choices := make([]int, trainerIconCount)
	for i := range choices {
		choices[i] = i + 1
	}
	return choices
}

func normalizeAvatarIcon(value int) int {
	if value < 0 || value > trainerIconCount {
		return 0
	}
	return value
}

func parseAvatarIcon(value string) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return normalizeAvatarIcon(parsed)
}

func trainerAvatarStyle(icon int) template.CSS {
	icon = normalizeAvatarIcon(icon)
	if icon == 0 {
		return ""
	}
	index := icon - 1
	column := index % trainerIconColumns
	row := index / trainerIconColumns
	xPercent := float64(column) * 100 / float64(trainerIconColumns-1)
	yPercent := float64(row) * 100 / float64(trainerIconRows-1)
	return template.CSS("background-image: url('/static/trainer-icons.png'); background-size: 2400% 3200%; background-position: " + strconv.FormatFloat(xPercent, 'f', 4, 64) + "% " + strconv.FormatFloat(yPercent, 'f', 4, 64) + "%;")
}

func trainerInitial(user User) string {
	name := strings.TrimSpace(user.DisplayName)
	if name == "" {
		name = strings.TrimSpace(user.Email)
	}
	if name == "" {
		return "?"
	}
	return strings.ToUpper(string([]rune(name)[0]))
}

func teamRecordSum(teams []Team) TeamRecord {
	var record TeamRecord
	for _, team := range teams {
		record.Wins += team.Record.Wins
		record.Draws += team.Record.Draws
		record.Losses += team.Record.Losses
		record.Played += team.Record.Played
	}
	return record
}

func bestTeamByRecord(teams []Team) Team {
	var best Team
	for _, team := range teams {
		if best.ID == "" || team.LeaderboardScore > best.LeaderboardScore {
			best = team
		}
	}
	return best
}

func formatShortTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("02/01 15:04")
}

func resultLabel(result string) string {
	switch result {
	case "win":
		return "Vitoria"
	case "draw":
		return "Empate"
	case "loss":
		return "Derrota"
	default:
		return "Partida"
	}
}

func transferKindLabel(kind string) string {
	switch kind {
	case "formation_created":
		return "Formacao original"
	case "pokemon_transfer":
		return "Troca semanal"
	default:
		return "Alteracao"
	}
}

func reverseRenderedMatchEvents(events []RenderedMatchEvent) []RenderedMatchEvent {
	reversed := make([]RenderedMatchEvent, len(events))
	for i := range events {
		reversed[len(events)-1-i] = events[i]
	}
	return reversed
}

func matchRenderState(match MatchResult, now time.Time, playbackMode string) MatchRenderState {
	state := RenderMatch(match, now)
	state.PlaybackMode = playbackMode
	return state
}

func replayMatchRenderState(match MatchResult) MatchRenderState {
	start := match.StartTime
	if start.IsZero() {
		start = time.Now()
	}
	return matchRenderState(match, start, "replay")
}

func reportMatchRenderState(match MatchResult) MatchRenderState {
	return matchRenderState(match, matchReportTime(match), "report")
}

func matchReportTime(match MatchResult) time.Time {
	if !match.EndTime.IsZero() {
		return match.EndTime.Add(time.Millisecond)
	}
	if !match.StartTime.IsZero() {
		return match.StartTime.Add(matchDuration(match.Events) + time.Millisecond)
	}
	return time.Now()
}

func matchGoalsA(match MatchResult) []MatchGoalSummary {
	teamA, _ := goalSummariesForMatch(match)
	return teamA
}

func matchGoalsB(match MatchResult) []MatchGoalSummary {
	_, teamB := goalSummariesForMatch(match)
	return teamB
}

func (s *Server) teamViewData(r *http.Request) ViewData {
	user, _ := s.currentUser(r)
	data := ViewData{
		Active:   "teams",
		User:     user,
		Teams:    s.store.MyTeams(user.ID),
		Pokemon:  s.store.Pokemon(),
		TeamForm: defaultTeamInput(s.store.Pokemon()),
	}
	if r.URL.Query().Get("saved") == "1" {
		data.Flash = "Escalacao salva."
	}
	if r.URL.Query().Get("deleted") == "1" {
		data.Flash = "Time removido."
	}

	return data
}

func (s *Server) teamFormViewData(user User, form TeamInput, editing bool) ViewData {
	data := ViewData{
		Active:      "team_form",
		User:        user,
		Pokemon:     s.store.Pokemon(),
		TeamForm:    form,
		EditingTeam: editing,
	}
	if editing && form.ID != "" {
		data.TransferWindow = s.store.TransferWindow(form.ID)
		data.TeamTransfers = s.store.TeamTransfers(form.ID)
	}
	return data
}

func (s *Server) accountViewData(r *http.Request) ViewData {
	user, ok := s.currentUser(r)
	data := ViewData{Active: "settings", User: user}
	if !ok {
		data.Error = "Usuario nao encontrado."
		return data
	}
	data.AccountForm = AccountInput{UserID: user.ID, DisplayName: user.DisplayName, AvatarIcon: user.AvatarIcon}
	if r.URL.Query().Get("saved") == "1" {
		data.Flash = "Configuracoes salvas."
	}
	return data
}

func pokemonNameByID(pokemon []Pokemon, id int) string {
	for _, item := range pokemon {
		if item.ID == id {
			return pokemonDisplayName(item.Name)
		}
	}
	return ""
}

func pokemonArtworkByID(pokemon []Pokemon, id int) string {
	for _, item := range pokemon {
		if item.ID == id {
			return item.DisplayArtworkURL()
		}
	}
	return ""
}

func teamInputFromRequest(r *http.Request) (TeamInput, error) {
	if err := r.ParseForm(); err != nil {
		return TeamInput{}, err
	}
	goalkeeperID, err := strconv.Atoi(r.FormValue("goalkeeper_id"))
	if err != nil {
		return TeamInput{}, err
	}
	fixoID, err := strconv.Atoi(r.FormValue("fixo_id"))
	if err != nil {
		return TeamInput{}, err
	}
	alaEsquerdaID, err := strconv.Atoi(r.FormValue("ala_esquerda_id"))
	if err != nil {
		return TeamInput{}, err
	}
	alaDireitaID, err := strconv.Atoi(r.FormValue("ala_direita_id"))
	if err != nil {
		return TeamInput{}, err
	}
	pivoID, err := strconv.Atoi(r.FormValue("pivo_id"))
	if err != nil {
		return TeamInput{}, err
	}

	return TeamInput{
		ID:                 r.FormValue("id"),
		Name:               strings.TrimSpace(r.FormValue("name")),
		GoalkeeperID:       goalkeeperID,
		GoalkeeperAbility:  r.FormValue("goalkeeper_ability"),
		FixoID:             fixoID,
		FixoAbility:        r.FormValue("fixo_ability"),
		AlaEsquerdaID:      alaEsquerdaID,
		AlaEsquerdaAbility: r.FormValue("ala_esquerda_ability"),
		AlaDireitaID:       alaDireitaID,
		AlaDireitaAbility:  r.FormValue("ala_direita_ability"),
		PivoID:             pivoID,
		PivoAbility:        r.FormValue("pivo_ability"),
	}, nil
}

func defaultTeamInput(pokemon []Pokemon) TeamInput {
	return TeamInput{Name: "Novo Time"}
}

func teamInputFromTeam(team Team) TeamInput {
	return TeamInput{
		ID:                 team.ID,
		UserID:             team.UserID,
		Name:               team.Name,
		GoalkeeperID:       team.Goalkeeper.ID,
		GoalkeeperAbility:  team.GoalkeeperAbility,
		FixoID:             team.Fixo.ID,
		FixoAbility:        team.FixoAbility,
		AlaEsquerdaID:      team.AlaEsquerda.ID,
		AlaEsquerdaAbility: team.AlaEsquerdaAbility,
		AlaDireitaID:       team.AlaDireita.ID,
		AlaDireitaAbility:  team.AlaDireitaAbility,
		PivoID:             team.Pivo.ID,
		PivoAbility:        team.PivoAbility,
	}
}

func teamErrorMessage(err error) string {
	switch {
	case errors.Is(err, ErrTeamLimitReached):
		return "Limite de 6 times atingido."
	case errors.Is(err, ErrTeamFrozen):
		return "Este time esta congelado por torneio ativo."
	case errors.Is(err, ErrTeamNotFound):
		return "Time nao encontrado."
	case errors.Is(err, ErrPokemonNotFound):
		return "Um dos Pokemon selecionados nao existe no cache local."
	case errors.Is(err, ErrInvalidTeam):
		return "Informe um nome valido para o time."
	case errors.Is(err, ErrDuplicatePokemon):
		return "Cada Pokemon so pode aparecer uma vez na escalacao."
	case errors.Is(err, ErrInvalidAbility):
		return "Escolha uma habilidade disponivel para cada Pokemon."
	case errors.Is(err, ErrTransferLimit):
		return "A janela semanal deste time ja foi usada. Uma nova troca abre no proximo domingo."
	case errors.Is(err, ErrTransferTooLarge):
		return "A janela semanal permite trocar apenas 1 Pokemon por vez."
	default:
		return "Nao foi possivel salvar a alteracao."
	}
}

func accountErrorMessage(err error) string {
	switch {
	case errors.Is(err, ErrInvalidAccount):
		return "Informe um nome publico valido."
	case errors.Is(err, ErrEncryptionKeyRequired):
		return "Configure ENV_ENCRYPTION_KEY com 32 bytes antes de salvar uma chave de API."
	case errors.Is(err, ErrUserNotFound):
		return "Usuario nao encontrado."
	default:
		return "Nao foi possivel salvar as configuracoes."
	}
}
