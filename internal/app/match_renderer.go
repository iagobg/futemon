package app

import (
	"fmt"
	"strings"
	"time"
)

const (
	narrationCharDuration = 30 * time.Millisecond
	defaultEventPause     = time.Second
	goalEventPause        = 2 * time.Second
	halftimeEventPause    = 5 * time.Second
	matchMinuteDuration   = time.Second
)

type MatchRenderState struct {
	Match             MatchResult
	Events            []RenderedMatchEvent
	ElapsedLabel      string
	MatchClockLabel   string
	ProgressPercent   int
	Finished          bool
	GoalLive          bool
	ScoreLabel        string
	FinalScoreLabel   string
	NextRefreshMS     int64
	NextRefreshAtMS   int64
	RenderedAtMS      int64
	StartedAtMS       int64
	EndedAtMS         int64
	Clock             MatchClockState
	AnimationConfig   AnimationConfig
	ScoreTeamA        int
	ScoreTeamB        int
	CurrentScoreTeamA int
	CurrentScoreTeamB int
	PlaybackMode      string
}

type AnimationConfig struct {
	CharDurationMS  int64
	DefaultPauseMS  int64
	GoalPauseMS     int64
	HalftimePauseMS int64
	MatchMinuteMS   int64
}

type MatchClockState struct {
	StartSecond      int
	EndSecond        int
	ElapsedMS        int64
	DurationMS       int64
	Running          bool
	PhaseStartedAtMS int64
}

type RenderedMatchEvent struct {
	Sequence         int
	Minute           int
	Type             string
	Label            string
	Text             string
	FullText         string
	Status           string
	EventElapsedMS   int64
	StartedAtMS      int64
	TextEndAtMS      int64
	PauseEndAtMS     int64
	ClockEndAtMS     int64
	ClockStartSecond int
	ClockEndSecond   int
	DurationMS       int64
	PauseMS          int64
	IsGoal           bool
	IsFulltime       bool
	IsMilestone      bool
	Attribution      string
	LabelHidden      bool
	RevealIndex      int
	GoalTrigger      bool
	GoalTeamSide     string
	ScoreAtMS        int64
}

func RenderMatch(match MatchResult, now time.Time) MatchRenderState {
	elapsed := now.Sub(match.StartTime)
	if elapsed < 0 {
		elapsed = 0
	}

	total := matchDuration(match.Events)
	finished := elapsed >= total

	events := make([]RenderedMatchEvent, 0, len(match.Events))
	goalLive := false
	nextRefreshMS := int64(10_000)
	nextRefreshAtMS := int64(0)
	clock := MatchClockState{StartSecond: 0, EndSecond: 0}
	var cursor time.Duration
	currentScoreA, currentScoreB := 0, 0
	for i, event := range match.Events {
		displayNarrative, cue := parseNarrativeCues(event.Narrative)
		cue = suspenseCueForEvent(event, displayNarrative, cue)
		readDuration := eventReadDuration(event)
		start := cursor
		end := cursor + readDuration
		dramaticPause := eventDramaticPauseDuration(event)
		dramaticEnd := end + dramaticPause
		clockDuration := eventClockAdvanceDuration(match.Events, i)
		clockEnd := dramaticEnd + clockDuration
		clockStartSecond := event.Minute * 60
		clockEndSecond := clockStartSecond
		scoreAt := time.Duration(0)
		goalTeamSide := ""
		if event.Type == "goal" {
			scoreAt = dramaticEnd
			goalTeamSide = teamSideForEvent(match, event)
		}
		if i+1 < len(match.Events) && event.Type != "fulltime" {
			clockEndSecond = match.Events[i+1].Minute * 60
		}
		cursor = clockEnd

		if elapsed >= start && elapsed < end {
			second := event.Minute * 60
			clock = MatchClockState{StartSecond: second, EndSecond: second}
			nextRefreshMS = (end - elapsed + 50*time.Millisecond).Milliseconds()
			nextRefreshAtMS = match.StartTime.Add(end + 50*time.Millisecond).UnixMilli()
		} else if elapsed >= end && elapsed < dramaticEnd {
			second := event.Minute * 60
			clock = MatchClockState{StartSecond: second, EndSecond: second}
			nextRefreshMS = (dramaticEnd - elapsed + 50*time.Millisecond).Milliseconds()
			nextRefreshAtMS = match.StartTime.Add(dramaticEnd + 50*time.Millisecond).UnixMilli()
		} else if elapsed >= dramaticEnd && elapsed < clockEnd {
			clock = MatchClockState{
				StartSecond:      clockStartSecond,
				EndSecond:        clockEndSecond,
				ElapsedMS:        (elapsed - dramaticEnd).Milliseconds(),
				DurationMS:       clockDuration.Milliseconds(),
				Running:          clockEndSecond != clockStartSecond,
				PhaseStartedAtMS: match.StartTime.Add(dramaticEnd).UnixMilli(),
			}
			nextRefreshMS = (clockEnd - elapsed + 50*time.Millisecond).Milliseconds()
			nextRefreshAtMS = match.StartTime.Add(clockEnd + 50*time.Millisecond).UnixMilli()
		} else if elapsed >= clockEnd {
			second := event.Minute * 60
			clock = MatchClockState{StartSecond: second, EndSecond: second}
		}

		rendered := RenderedMatchEvent{
			Sequence:         i,
			Minute:           event.Minute,
			Type:             event.Type,
			Label:            eventLabel(event.Type),
			FullText:         displayNarrative,
			Status:           "pending",
			StartedAtMS:      match.StartTime.Add(start).UnixMilli(),
			TextEndAtMS:      match.StartTime.Add(end).UnixMilli(),
			PauseEndAtMS:     match.StartTime.Add(dramaticEnd).UnixMilli(),
			ClockEndAtMS:     match.StartTime.Add(clockEnd).UnixMilli(),
			ClockStartSecond: clockStartSecond,
			ClockEndSecond:   clockEndSecond,
			DurationMS:       readDuration.Milliseconds(),
			PauseMS:          dramaticPause.Milliseconds(),
			IsGoal:           event.Type == "goal",
			IsFulltime:       event.Type == "fulltime",
			IsMilestone: event.Type == "goal" || event.Type == "fulltime" ||
				event.Type == "injury" || event.Type == "halftime",
			Attribution:  eventAttribution(match, event),
			RevealIndex:  cue.RevealIndex,
			GoalTrigger:  cue.GoalTrigger,
			GoalTeamSide: goalTeamSide,
			ScoreAtMS:    scoreAnchorMS(match.StartTime, scoreAt),
		}
		switch {
		case elapsed >= dramaticEnd:
			rendered.Status = "done"
			rendered.Text = displayNarrative
			rendered.EventElapsedMS = readDuration.Milliseconds()
		case elapsed >= end:
			rendered.Status = "live"
			rendered.Text = displayNarrative
			rendered.EventElapsedMS = readDuration.Milliseconds()
		case elapsed >= start:
			rendered.Status = "live"
			eventElapsed := elapsed - start
			rendered.EventElapsedMS = eventElapsed.Milliseconds()
			rendered.Text = partialNarrative(displayNarrative, eventElapsed, readDuration)
			rendered.LabelHidden = visibleRuneCount(displayNarrative, eventElapsed, readDuration) < cue.RevealIndex
		default:
			rendered.Text = ""
		}
		if event.Type == "goal" && elapsed >= scoreAt {
			switch goalTeamSide {
			case "a":
				currentScoreA++
			case "b":
				currentScoreB++
			}
		}
		if rendered.Status == "live" && rendered.IsGoal {
			goalLive = true
		}
		events = append(events, rendered)
	}

	scoreA, scoreB := match.Score()
	scoreLabel := fmt.Sprintf("%s %d x %d %s", match.TeamA.Name, currentScoreA, currentScoreB, match.TeamB.Name)
	finalScoreLabel := fmt.Sprintf("%s %d x %d %s", match.TeamA.Name, scoreA, scoreB, match.TeamB.Name)
	if finished {
		scoreLabel = finalScoreLabel
		clock = MatchClockState{StartSecond: 40 * 60, EndSecond: 40 * 60}
		nextRefreshMS = 0
		nextRefreshAtMS = 0
	}
	clockSecond := interpolatedClockSecond(clock)

	return MatchRenderState{
		Match:           match,
		Events:          events,
		ElapsedLabel:    formatElapsed(elapsed),
		MatchClockLabel: formatClockSecond(clockSecond),
		ProgressPercent: progressFromClock(clockSecond),
		Finished:        finished,
		GoalLive:        goalLive,
		ScoreLabel:      scoreLabel,
		FinalScoreLabel: finalScoreLabel,
		NextRefreshMS:   nextRefreshMS,
		NextRefreshAtMS: nextRefreshAtMS,
		RenderedAtMS:    now.UnixMilli(),
		StartedAtMS:     match.StartTime.UnixMilli(),
		EndedAtMS:       match.StartTime.Add(total).UnixMilli(),
		Clock:           clock,
		AnimationConfig: AnimationConfig{
			CharDurationMS:  narrationCharDuration.Milliseconds(),
			DefaultPauseMS:  defaultEventPause.Milliseconds(),
			GoalPauseMS:     goalEventPause.Milliseconds(),
			HalftimePauseMS: halftimeEventPause.Milliseconds(),
			MatchMinuteMS:   matchMinuteDuration.Milliseconds(),
		},
		ScoreTeamA:        scoreA,
		ScoreTeamB:        scoreB,
		CurrentScoreTeamA: currentScoreA,
		CurrentScoreTeamB: currentScoreB,
	}
}

func scoreAnchorMS(start time.Time, offset time.Duration) int64 {
	if offset <= 0 {
		return 0
	}
	return start.Add(offset).UnixMilli()
}

func matchDuration(events []MatchEvent) time.Duration {
	var total time.Duration
	for _, event := range events {
		total += eventReadDuration(event) + eventDramaticPauseDuration(event)
	}
	for i := range events {
		total += eventClockAdvanceDuration(events, i)
	}
	return total
}

func eventReadDuration(event MatchEvent) time.Duration {
	narrative, _ := parseNarrativeCues(event.Narrative)
	if len(narrative) == 0 {
		return narrationCharDuration
	}
	return time.Duration(len([]rune(narrative))) * narrationCharDuration
}

func eventDramaticPauseDuration(event MatchEvent) time.Duration {
	switch event.Type {
	case "goal":
		return goalEventPause
	case "halftime":
		return halftimeEventPause
	default:
		return defaultEventPause
	}
}

func eventClockAdvanceDuration(events []MatchEvent, index int) time.Duration {
	if index+1 >= len(events) || events[index].Type == "fulltime" {
		return 0
	}
	gapMinutes := events[index+1].Minute - events[index].Minute
	if gapMinutes <= 0 {
		return 0
	}
	return time.Duration(gapMinutes) * matchMinuteDuration
}

func partialNarrative(narrative string, elapsed time.Duration, duration time.Duration) string {
	runes := []rune(narrative)
	if len(runes) == 0 {
		return ""
	}
	if elapsed >= duration {
		return narrative
	}
	visible := int((elapsed * time.Duration(len(runes))) / duration)
	if visible < 1 {
		visible = 1
	}
	if visible > len(runes) {
		visible = len(runes)
	}
	return string(runes[:visible])
}

func visibleRuneCount(narrative string, elapsed time.Duration, duration time.Duration) int {
	runes := []rune(narrative)
	if len(runes) == 0 {
		return 0
	}
	if elapsed >= duration {
		return len(runes)
	}
	visible := int((elapsed * time.Duration(len(runes))) / duration)
	if visible < 1 {
		visible = 1
	}
	if visible > len(runes) {
		visible = len(runes)
	}
	return visible
}

type narrativeCue struct {
	RevealIndex int
	GoalTrigger bool
}

func suspenseCueForEvent(event MatchEvent, narrative string, cue narrativeCue) narrativeCue {
	if !mystifiesEvent(event.Type) {
		cue.RevealIndex = 0
		return cue
	}
	if cue.RevealIndex > 0 {
		return cue
	}
	runes := len([]rune(narrative))
	if runes == 0 {
		return cue
	}
	reveal := (runes * 55) / 100
	if reveal < 24 {
		reveal = 24
	}
	if reveal > runes-8 {
		reveal = runes - 8
	}
	if reveal < 1 {
		reveal = 1
	}
	cue.RevealIndex = reveal
	return cue
}

func mystifiesEvent(eventType string) bool {
	switch eventType {
	case "kickoff", "halftime", "fulltime":
		return false
	default:
		return true
	}
}

func parseNarrativeCues(narrative string) (string, narrativeCue) {
	marker := "{goal}"
	index := strings.Index(narrative, marker)
	if index < 0 {
		return narrative, narrativeCue{RevealIndex: 0}
	}
	before := narrative[:index]
	after := narrative[index+len(marker):]
	clean := before + after
	return clean, narrativeCue{
		RevealIndex: len([]rune(before)),
		GoalTrigger: true,
	}
}

func formatElapsed(elapsed time.Duration) string {
	seconds := int(elapsed.Seconds())
	minutes := seconds / 60
	seconds = seconds % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func interpolatedClockSecond(clock MatchClockState) int {
	if !clock.Running || clock.DurationMS <= 0 {
		return clock.StartSecond
	}
	if clock.ElapsedMS >= clock.DurationMS {
		return clock.EndSecond
	}
	delta := clock.EndSecond - clock.StartSecond
	return clock.StartSecond + int((int64(delta)*clock.ElapsedMS)/clock.DurationMS)
}

func formatClockSecond(totalSeconds int) string {
	if totalSeconds < 0 {
		totalSeconds = 0
	}
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func progressFromClock(clockSecond int) int {
	if clockSecond <= 0 {
		return 0
	}
	if clockSecond >= 40*60 {
		return 100
	}
	return (clockSecond * 100) / (40 * 60)
}

func eventLabel(eventType string) string {
	switch eventType {
	case "kickoff":
		return "Inicio"
	case "foul":
		return "Falta"
	case "goal":
		return "Gol"
	case "close_chance":
		return "Quase"
	case "injury":
		return "Atendimento"
	case "halftime":
		return "Intervalo"
	case "fulltime":
		return "Fim"
	default:
		return eventType
	}
}

func teamSideForEvent(match MatchResult, event MatchEvent) string {
	switch event.TeamID {
	case match.TeamA.ID:
		return "a"
	case match.TeamB.ID:
		return "b"
	default:
		return ""
	}
}

func eventAttribution(match MatchResult, event MatchEvent) string {
	if event.TeamID == "" && event.PokemonID == 0 {
		return ""
	}
	teamName := ""
	switch event.TeamID {
	case match.TeamA.ID:
		teamName = match.TeamA.Name
	case match.TeamB.ID:
		teamName = match.TeamB.Name
	}
	pokemonName := pokemonNameForEvent(match, event.PokemonID)
	switch {
	case teamName != "" && pokemonName != "":
		return pokemonName + " · " + teamName
	case pokemonName != "":
		return pokemonName
	case teamName != "":
		return teamName
	default:
		return ""
	}
}

func pokemonNameForEvent(match MatchResult, pokemonID int) string {
	if pokemonID == 0 {
		return ""
	}
	for _, positioned := range append(match.TeamA.Roster(), match.TeamB.Roster()...) {
		if positioned.Pokemon.ID == pokemonID {
			return positioned.Pokemon.Name
		}
	}
	return ""
}
