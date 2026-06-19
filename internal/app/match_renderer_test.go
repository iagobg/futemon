package app

import (
	"strings"
	"testing"
	"time"
)

func TestRenderMatchRevealsNarrationProgressively(t *testing.T) {
	match := rendererTestMatch()

	state := RenderMatch(match, match.StartTime.Add(500*time.Millisecond))
	if state.Events[0].Status != "live" {
		t.Fatalf("first event status = %q, want live", state.Events[0].Status)
	}
	if state.Events[0].Text == "" {
		t.Fatal("expected partial text")
	}
	if state.Events[0].Text == match.Events[0].Narrative {
		t.Fatal("live event revealed full narrative too early")
	}
	if len(state.Events) != len(match.Events) {
		t.Fatalf("rendered events = %d, want full timeline %d", len(state.Events), len(match.Events))
	}
	if state.Events[1].Status != "pending" {
		t.Fatalf("second event status = %q, want pending", state.Events[1].Status)
	}
}

func TestRenderMatchUsesGoalCueForRevealAndCleanText(t *testing.T) {
	match := rendererTestMatch()
	match.Events = []MatchEvent{{Minute: 8, Type: "goal", TeamID: "a", Narrative: "Alakazam cruza para a area, Kangaskhan sobe {goal} GOOOL!", DramaticPauseSeconds: 1}}
	cueIndex := len([]rune("Alakazam cruza para a area, Kangaskhan sobe "))

	early := RenderMatch(match, match.StartTime.Add(250*time.Millisecond))
	if len(early.Events) != 1 {
		t.Fatalf("events = %d, want 1", len(early.Events))
	}
	if !early.Events[0].LabelHidden {
		t.Fatal("goal label should stay hidden before the cue")
	}
	if strings.Contains(early.Events[0].FullText, "{goal}") || strings.Contains(early.Events[0].Text, "{goal}") {
		t.Fatalf("goal marker leaked into rendered text: %+v", early.Events[0])
	}

	afterCue := RenderMatch(match, match.StartTime.Add(time.Duration(cueIndex+1)*narrationCharDuration))
	if afterCue.Events[0].LabelHidden {
		t.Fatal("goal label should reveal when the cue is reached")
	}
	if !afterCue.Events[0].GoalTrigger {
		t.Fatal("goal cue should enable the client celebration trigger")
	}
}

func TestRenderMatchUsesServerControlledPauseDurations(t *testing.T) {
	cases := []struct {
		eventType string
		want      time.Duration
	}{
		{eventType: "close_chance", want: defaultEventPause},
		{eventType: "goal", want: goalEventPause},
		{eventType: "halftime", want: halftimeEventPause},
	}

	for _, tc := range cases {
		event := MatchEvent{Type: tc.eventType, Narrative: "Texto.", DramaticPauseSeconds: 99}
		if got := eventDramaticPauseDuration(event); got != tc.want {
			t.Fatalf("pause for %s = %s, want %s", tc.eventType, got, tc.want)
		}
	}
}

func TestRenderMatchFinishesWithFinalScore(t *testing.T) {
	match := rendererTestMatch()
	state := RenderMatch(match, match.StartTime.Add(matchDuration(match.Events)+time.Second))

	if !state.Finished {
		t.Fatal("expected finished state")
	}
	if state.ProgressPercent != 100 {
		t.Fatalf("progress = %d, want 100", state.ProgressPercent)
	}
	if !strings.Contains(state.ScoreLabel, "2 x 1") {
		t.Fatalf("score label = %q", state.ScoreLabel)
	}
	for _, event := range state.Events {
		if event.Status != "done" {
			t.Fatalf("event %v status = %q, want done", event.Minute, event.Status)
		}
	}
}

func TestRenderMatchFreezesClockDuringNarration(t *testing.T) {
	match := rendererTestMatch()

	state := RenderMatch(match, match.StartTime.Add(500*time.Millisecond))

	if state.Clock.Running {
		t.Fatal("clock should not run while narration is being typed")
	}
	if state.MatchClockLabel != "00:00" {
		t.Fatalf("clock label = %q, want 00:00", state.MatchClockLabel)
	}
}

func TestRenderMatchExposesAbsoluteTimelineAnchors(t *testing.T) {
	match := rendererTestMatch()
	now := match.StartTime.Add(500 * time.Millisecond)

	state := RenderMatch(match, now)

	if state.RenderedAtMS != now.UnixMilli() {
		t.Fatalf("rendered at = %d, want %d", state.RenderedAtMS, now.UnixMilli())
	}
	if state.NextRefreshAtMS <= state.RenderedAtMS {
		t.Fatalf("next refresh anchor = %d, want after rendered at %d", state.NextRefreshAtMS, state.RenderedAtMS)
	}
	if state.Events[0].Sequence != 0 {
		t.Fatalf("event sequence = %d, want 0", state.Events[0].Sequence)
	}
	if state.Events[0].StartedAtMS != match.StartTime.UnixMilli() {
		t.Fatalf("event started at = %d, want %d", state.Events[0].StartedAtMS, match.StartTime.UnixMilli())
	}
	if state.Events[0].TextEndAtMS <= state.Events[0].StartedAtMS {
		t.Fatalf("event text end = %d, want after start %d", state.Events[0].TextEndAtMS, state.Events[0].StartedAtMS)
	}
	if state.Events[0].PauseEndAtMS <= state.Events[0].TextEndAtMS {
		t.Fatalf("event pause end = %d, want after text end %d", state.Events[0].PauseEndAtMS, state.Events[0].TextEndAtMS)
	}
}

func TestRenderMatchExposesClockPhaseAnchor(t *testing.T) {
	match := rendererTestMatch()
	clockPhaseStart := eventReadDuration(match.Events[0]) + eventDramaticPauseDuration(match.Events[0])
	now := match.StartTime.Add(clockPhaseStart + 500*time.Millisecond)

	state := RenderMatch(match, now)

	if !state.Clock.Running {
		t.Fatal("expected clock to be running between events")
	}
	if state.Clock.PhaseStartedAtMS != match.StartTime.Add(clockPhaseStart).UnixMilli() {
		t.Fatalf("clock phase started at = %d, want %d", state.Clock.PhaseStartedAtMS, match.StartTime.Add(clockPhaseStart).UnixMilli())
	}
	if state.NextRefreshAtMS <= state.RenderedAtMS {
		t.Fatalf("next refresh anchor = %d, want after rendered at %d", state.NextRefreshAtMS, state.RenderedAtMS)
	}
}

func TestRenderMatchHalftimePauseKeepsClockStopped(t *testing.T) {
	match := rendererHalftimeTestMatch()
	elapsed := eventReadDuration(match.Events[0]) + eventDramaticPauseDuration(match.Events[0]) + eventClockAdvanceDuration(match.Events, 0) +
		eventReadDuration(match.Events[1]) + 2*time.Second

	state := RenderMatch(match, match.StartTime.Add(elapsed))

	if state.Clock.Running {
		t.Fatal("halftime pause should keep the match clock stopped")
	}
	if state.MatchClockLabel != "20:00" {
		t.Fatalf("clock label = %q, want 20:00", state.MatchClockLabel)
	}
}

func TestRenderMatchMystifiesOpenPlayEvents(t *testing.T) {
	match := rendererTestMatch()
	match.Events = []MatchEvent{
		{Minute: 0, Type: "kickoff", Narrative: "Comeca.", DramaticPauseSeconds: 1},
		{Minute: 4, Type: "close_chance", Narrative: "A jogada cresce pela ala direita, a defesa se desorganiza e o chute sai cruzado para assustar todo mundo.", DramaticPauseSeconds: 1},
	}
	elapsed := eventReadDuration(match.Events[0]) + eventDramaticPauseDuration(match.Events[0]) + eventClockAdvanceDuration(match.Events, 0) + 250*time.Millisecond

	state := RenderMatch(match, match.StartTime.Add(elapsed))

	if state.Events[0].RevealIndex != 0 || state.Events[0].LabelHidden {
		t.Fatalf("kickoff should not be mystified: %+v", state.Events[0])
	}
	if state.Events[1].RevealIndex == 0 {
		t.Fatal("open play event should have a suspense reveal index")
	}
	if !state.Events[1].LabelHidden {
		t.Fatal("open play label should start hidden")
	}
}

func TestRenderMatchKeepsGoalLiveUntilPauseEnds(t *testing.T) {
	match := rendererTestMatch()
	match.Events = []MatchEvent{{Minute: 8, Type: "goal", TeamID: "a", Narrative: "A bola sobra na entrada da area {goal} GOOOL!", DramaticPauseSeconds: 1}}
	readEnd := eventReadDuration(match.Events[0])
	pauseEnd := readEnd + eventDramaticPauseDuration(match.Events[0])

	duringPause := RenderMatch(match, match.StartTime.Add(readEnd+time.Millisecond))
	if duringPause.Events[0].Status != "live" {
		t.Fatalf("status during pause = %q, want live", duringPause.Events[0].Status)
	}
	if duringPause.CurrentScoreTeamA != 0 || duringPause.CurrentScoreTeamB != 0 {
		t.Fatalf("score during goal pause = %d x %d, want 0 x 0", duringPause.CurrentScoreTeamA, duringPause.CurrentScoreTeamB)
	}

	afterPause := RenderMatch(match, match.StartTime.Add(pauseEnd))
	if afterPause.Events[0].Status != "done" {
		t.Fatalf("status after pause = %q, want done", afterPause.Events[0].Status)
	}
	if afterPause.CurrentScoreTeamA != 1 || afterPause.CurrentScoreTeamB != 0 {
		t.Fatalf("score after goal pause = %d x %d, want 1 x 0", afterPause.CurrentScoreTeamA, afterPause.CurrentScoreTeamB)
	}
}

func TestRenderMatchCurrentScoreAdvancesWhenGoalEventEnds(t *testing.T) {
	match := rendererTestMatch()
	match.Events = []MatchEvent{{Minute: 8, Type: "goal", TeamID: "a", Narrative: "A bola sobra na entrada da area {goal} GOOOL!", DramaticPauseSeconds: 1}}
	scoreAt := eventReadDuration(match.Events[0]) + eventDramaticPauseDuration(match.Events[0])

	beforeEnd := RenderMatch(match, match.StartTime.Add(scoreAt-time.Millisecond))
	if beforeEnd.CurrentScoreTeamA != 0 || beforeEnd.CurrentScoreTeamB != 0 {
		t.Fatalf("score before goal event end = %d x %d, want 0 x 0", beforeEnd.CurrentScoreTeamA, beforeEnd.CurrentScoreTeamB)
	}

	afterEnd := RenderMatch(match, match.StartTime.Add(scoreAt))
	if afterEnd.CurrentScoreTeamA != 1 || afterEnd.CurrentScoreTeamB != 0 {
		t.Fatalf("score after goal event end = %d x %d, want 1 x 0", afterEnd.CurrentScoreTeamA, afterEnd.CurrentScoreTeamB)
	}
	if afterEnd.Events[0].GoalTeamSide != "a" || afterEnd.Events[0].ScoreAtMS != match.StartTime.Add(scoreAt).UnixMilli() {
		t.Fatalf("goal score anchor = %+v", afterEnd.Events[0])
	}
}

func TestRenderMatchNonGoalEventsNeverExposeScoreAnchors(t *testing.T) {
	match := rendererTestMatch()
	match.Events = []MatchEvent{
		{Minute: 4, Type: "close_chance", TeamID: "a", Narrative: "Pikachu arma uma jogada perigosa e quase abre o placar.", DramaticPauseSeconds: 1},
		{Minute: 8, Type: "goal", TeamID: "a", Narrative: "Machamp recebe, gira e bate {goal} GOOOL!", DramaticPauseSeconds: 1},
	}
	firstEventComplete := eventReadDuration(match.Events[0]) + eventDramaticPauseDuration(match.Events[0])

	afterChance := RenderMatch(match, match.StartTime.Add(firstEventComplete+100*time.Millisecond))
	if afterChance.CurrentScoreTeamA != 0 || afterChance.CurrentScoreTeamB != 0 {
		t.Fatalf("score after non-goal = %d x %d, want 0 x 0", afterChance.CurrentScoreTeamA, afterChance.CurrentScoreTeamB)
	}
	if afterChance.Events[0].GoalTeamSide != "" || afterChance.Events[0].ScoreAtMS != 0 {
		t.Fatalf("non-goal exposed score anchor: %+v", afterChance.Events[0])
	}
}

func TestMatchScoreIsDerivedFromGoalEvents(t *testing.T) {
	match := rendererHalftimeTestMatch()

	scoreA, scoreB := match.Score()

	if scoreA != 1 || scoreB != 0 {
		t.Fatalf("score = %d x %d, want 1 x 0", scoreA, scoreB)
	}
}

func rendererTestMatch() MatchResult {
	return MatchResult{
		TeamA:      Team{ID: "a", Name: "Time A"},
		TeamB:      Team{ID: "b", Name: "Time B"},
		ScoreTeamA: 2,
		ScoreTeamB: 1,
		StartTime:  time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC),
		Events: []MatchEvent{
			{Minute: 0, Type: "kickoff", Narrative: "Comeca a partida com respeito tatico.", DramaticPauseSeconds: 1},
			{Minute: 7, Type: "goal", TeamID: "a", Narrative: "Gol do Time A.", DramaticPauseSeconds: 1},
			{Minute: 14, Type: "goal", TeamID: "b", Narrative: "Gol do Time B.", DramaticPauseSeconds: 1},
			{Minute: 23, Type: "goal", TeamID: "a", Narrative: "Outro gol do Time A.", DramaticPauseSeconds: 1},
			{Minute: 40, Type: "fulltime", Narrative: "Fim de jogo.", DramaticPauseSeconds: 1},
		},
	}
}

func rendererHalftimeTestMatch() MatchResult {
	return MatchResult{
		TeamA:      Team{ID: "a", Name: "Time A"},
		TeamB:      Team{ID: "b", Name: "Time B"},
		ScoreTeamA: 2,
		ScoreTeamB: 1,
		StartTime:  time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC),
		Events: []MatchEvent{
			{Minute: 0, Type: "kickoff", Narrative: "Comeca.", DramaticPauseSeconds: 1},
			{Minute: 20, Type: "halftime", Narrative: "Intervalo.", DramaticPauseSeconds: 5},
			{Minute: 23, Type: "goal", TeamID: "a", PokemonID: 1, Narrative: "Gol.", DramaticPauseSeconds: 1},
			{Minute: 40, Type: "fulltime", Narrative: "Fim.", DramaticPauseSeconds: 1},
		},
	}
}
