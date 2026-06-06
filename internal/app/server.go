package app

import (
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	store     Store
	templates *template.Template
}

func NewServer(store Store) *Server {
	return &Server{
		store: store,
		templates: template.Must(template.New("app").Funcs(template.FuncMap{
			"dict":  dict,
			"list":  list,
			"lower": strings.ToLower,
		}).Parse(pageTemplates)),
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleHome)
	mux.HandleFunc("/teams", s.handleTeams)
	mux.HandleFunc("/teams/save", s.handleSaveTeam)
	mux.HandleFunc("/teams/delete", s.handleDeleteTeam)
	mux.HandleFunc("/duels", s.handleDuels)
	mux.HandleFunc("/duels/start", s.handleStartDuel)
	mux.HandleFunc("/match/latest", s.handleLatestMatch)
	mux.HandleFunc("/match/events", s.handleMatchEvents)
	mux.HandleFunc("/match/live", s.handleMatchLive)
	mux.HandleFunc("/tournaments", s.handleTournaments)
	mux.HandleFunc("/global-teams", s.handleGlobalTeams)
	mux.HandleFunc("/admin", s.handleAdmin)
	mux.HandleFunc("/settings", s.handleSettings)
	mux.HandleFunc("/settings/save", s.handleSaveSettings)
	mux.HandleFunc("/static/app.js", s.handleAsset)
	return mux
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/teams", http.StatusSeeOther)
}

func (s *Server) handleTeams(w http.ResponseWriter, r *http.Request) {
	data := s.teamViewData(r)
	s.render(w, "layout", data)
}

func (s *Server) handleSaveTeam(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "metodo nao permitido", http.StatusMethodNotAllowed)
		return
	}
	input, err := teamInputFromRequest(r)
	if err != nil {
		data := s.teamViewData(r)
		data.Error = "Revise a escalacao antes de salvar."
		s.render(w, "layout", data)
		return
	}
	currentUser, ok := s.store.CurrentUser()
	if !ok {
		http.Error(w, "usuario nao encontrado", http.StatusUnauthorized)
		return
	}
	input.UserID = currentUser.ID

	team, err := s.store.SaveTeam(input)
	if err != nil {
		data := s.teamViewData(r)
		data.TeamForm = input
		data.Error = teamErrorMessage(err)
		s.render(w, "layout", data)
		return
	}
	http.Redirect(w, r, "/teams?edit="+team.ID+"&saved=1", http.StatusSeeOther)
}

func (s *Server) handleDeleteTeam(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "metodo nao permitido", http.StatusMethodNotAllowed)
		return
	}
	currentUser, ok := s.store.CurrentUser()
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
	user, _ := s.store.CurrentUser()
	s.render(w, "layout", ViewData{Active: "duels", User: user, Teams: s.store.MyTeams(), GlobalTeams: s.store.GlobalTeams()})
}

func (s *Server) handleStartDuel(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	teamAID := r.FormValue("team_id")
	teamBID := r.FormValue("opponent_id")
	if teamBID == "" {
		teamBID = "team-paleta-bolada"
	}

	teamA, okA := s.store.FindTeam(teamAID)
	teamB, okB := s.store.FindTeam(teamBID)
	if !okA || !okB {
		http.Error(w, "time nao encontrado", http.StatusBadRequest)
		return
	}

	match := SimulateMatch(teamA, teamB)
	s.store.SetLatestMatch(match)
	w.Header().Set("HX-Redirect", "/match/latest")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleLatestMatch(w http.ResponseWriter, r *http.Request) {
	match, ok := s.store.LatestMatch()
	if !ok {
		http.Redirect(w, r, "/duels", http.StatusSeeOther)
		return
	}
	s.render(w, "layout", ViewData{Active: "match", Match: match, MatchState: RenderMatch(match, time.Now())})
}

func (s *Server) handleMatchEvents(w http.ResponseWriter, r *http.Request) {
	match, ok := s.store.LatestMatch()
	if !ok {
		http.Error(w, "partida nao encontrada", http.StatusNotFound)
		return
	}

	state := RenderMatch(match, time.Now())
	var done []RenderedMatchEvent
	for _, event := range state.Events {
		if event.Status == "done" {
			done = append(done, event)
		}
	}
	s.render(w, "eventList", done)
}

func (s *Server) handleMatchLive(w http.ResponseWriter, r *http.Request) {
	match, ok := s.store.LatestMatch()
	if !ok {
		http.Error(w, "partida nao encontrada", http.StatusNotFound)
		return
	}
	s.render(w, "matchLive", RenderMatch(match, time.Now()))
}

func (s *Server) handleTournaments(w http.ResponseWriter, r *http.Request) {
	user, _ := s.store.CurrentUser()
	s.render(w, "layout", ViewData{Active: "tournaments", User: user, Tournaments: s.store.Tournaments(), Teams: s.store.MyTeams()})
}

func (s *Server) handleGlobalTeams(w http.ResponseWriter, r *http.Request) {
	user, _ := s.store.CurrentUser()
	s.render(w, "layout", ViewData{Active: "global", User: user, GlobalTeams: s.store.GlobalTeams()})
}

func (s *Server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	user, ok := s.store.CurrentUser()
	if !ok || user.Role != "admin" {
		http.Error(w, "acesso restrito a administradores", http.StatusForbidden)
		return
	}
	s.render(w, "layout", ViewData{Active: "admin", User: user, Tournaments: s.store.Tournaments()})
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	data := s.accountViewData(r)
	s.render(w, "layout", data)
}

func (s *Server) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "metodo nao permitido", http.StatusMethodNotAllowed)
		return
	}
	user, ok := s.store.CurrentUser()
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
		UserID:       user.ID,
		DisplayName:  r.FormValue("display_name"),
		GeminiAPIKey: r.FormValue("gemini_api_key"),
		ClearAPIKey:  r.FormValue("clear_api_key") == "on",
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

func visibleEvents(events []MatchEvent, elapsed time.Duration) []MatchEvent {
	var visible []MatchEvent
	var cursor time.Duration
	for _, event := range events {
		readTime := time.Duration(len(event.Narrative)*50) * time.Millisecond
		pause := time.Duration(event.DramaticPauseSeconds) * time.Second
		cursor += readTime + pause
		if cursor <= elapsed {
			visible = append(visible, event)
		}
	}
	if len(visible) == 0 && len(events) > 0 {
		return events[:1]
	}
	return visible
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type ViewData struct {
	Active      string
	Teams       []Team
	GlobalTeams []Team
	Pokemon     []Pokemon
	Tournaments []Tournament
	Match       MatchResult
	MatchState  MatchRenderState
	User        User
	TeamForm    TeamInput
	AccountForm AccountInput
	EditingTeam bool
	Flash       string
	Error       string
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

func (s *Server) teamViewData(r *http.Request) ViewData {
	user, _ := s.store.CurrentUser()
	data := ViewData{
		Active:   "teams",
		User:     user,
		Teams:    s.store.MyTeams(),
		Pokemon:  s.store.Pokemon(),
		TeamForm: defaultTeamInput(s.store.Pokemon()),
	}
	if r.URL.Query().Get("saved") == "1" {
		data.Flash = "Escalacao salva."
	}
	if r.URL.Query().Get("deleted") == "1" {
		data.Flash = "Time removido."
	}

	editID := r.URL.Query().Get("edit")
	if editID == "" {
		return data
	}
	team, ok := s.store.FindTeam(editID)
	if !ok || team.UserID != user.ID {
		data.Error = "Time nao encontrado."
		return data
	}
	data.TeamForm = teamInputFromTeam(team)
	data.EditingTeam = true
	return data
}

func (s *Server) accountViewData(r *http.Request) ViewData {
	user, ok := s.store.CurrentUser()
	data := ViewData{Active: "settings", User: user}
	if !ok {
		data.Error = "Usuario nao encontrado."
		return data
	}
	data.AccountForm = AccountInput{UserID: user.ID, DisplayName: user.DisplayName}
	if r.URL.Query().Get("saved") == "1" {
		data.Flash = "Configuracoes salvas."
	}
	return data
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
		ID:            r.FormValue("id"),
		Name:          strings.TrimSpace(r.FormValue("name")),
		GoalkeeperID:  goalkeeperID,
		FixoID:        fixoID,
		AlaEsquerdaID: alaEsquerdaID,
		AlaDireitaID:  alaDireitaID,
		PivoID:        pivoID,
	}, nil
}

func defaultTeamInput(pokemon []Pokemon) TeamInput {
	input := TeamInput{Name: "Novo Time"}
	if len(pokemon) > 0 {
		input.GoalkeeperID = pokemon[0].ID
		input.FixoID = pokemon[0].ID
		input.AlaEsquerdaID = pokemon[0].ID
		input.AlaDireitaID = pokemon[0].ID
		input.PivoID = pokemon[0].ID
	}
	return input
}

func teamInputFromTeam(team Team) TeamInput {
	return TeamInput{
		ID:            team.ID,
		UserID:        team.UserID,
		Name:          team.Name,
		GoalkeeperID:  team.Goalkeeper.ID,
		FixoID:        team.Fixo.ID,
		AlaEsquerdaID: team.AlaEsquerda.ID,
		AlaDireitaID:  team.AlaDireita.ID,
		PivoID:        team.Pivo.ID,
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
	default:
		return "Nao foi possivel salvar a alteracao."
	}
}

func accountErrorMessage(err error) string {
	switch {
	case errors.Is(err, ErrInvalidAccount):
		return "Informe um nome publico valido."
	case errors.Is(err, ErrEncryptionKeyRequired):
		return "Configure ENV_ENCRYPTION_KEY com 32 bytes antes de salvar uma chave Gemini."
	case errors.Is(err, ErrUserNotFound):
		return "Usuario nao encontrado."
	default:
		return "Nao foi possivel salvar as configuracoes."
	}
}
