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
	if len(state.Events) != 1 {
		t.Fatalf("rendered events = %d, want only the active event", len(state.Events))
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
