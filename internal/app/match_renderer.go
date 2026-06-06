package app

import (
	"fmt"
	"time"
)

const (
	narrationCharDuration = 50 * time.Millisecond
	eventBaseDelay        = 750 * time.Millisecond
	matchMinuteDuration   = time.Second
)

type MatchRenderState struct {
	Match           MatchResult
	Events          []RenderedMatchEvent
	ElapsedLabel    string
	MatchClockLabel string
	ProgressPercent int
	Finished        bool
	GoalLive        bool
	ScoreLabel      string
	NextRefreshMS   int64
	Clock           MatchClockState
	AnimationConfig AnimationConfig
	ScoreTeamA      int
	ScoreTeamB      int
}

type AnimationConfig struct {
	CharDurationMS int64
	BaseDelayMS    int64
	MatchMinuteMS  int64
}

type MatchClockState struct {
	StartSecond int
	EndSecond   int
	ElapsedMS   int64
	DurationMS  int64
	Running     bool
}

type RenderedMatchEvent struct {
	Minute         int
	Type           string
	Label          string
	Text           string
	FullText       string
	Status         string
	EventElapsedMS int64
	DurationMS     int64
	PauseMS        int64
	IsGoal         bool
	IsFulltime     bool
	IsMilestone    bool
	Attribution    string
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
	clock := MatchClockState{StartSecond: 0, EndSecond: 0}
	var cursor time.Duration
	for i, event := range match.Events {
		readDuration := eventReadDuration(event)
		start := cursor
		end := cursor + readDuration
		dramaticPause := eventDramaticPauseDuration(event)
		dramaticEnd := end + dramaticPause
		clockDuration := eventClockAdvanceDuration(match.Events, i)
		clockEnd := dramaticEnd + clockDuration
		cursor = clockEnd

		if elapsed >= start && elapsed < end {
			second := event.Minute * 60
			clock = MatchClockState{StartSecond: second, EndSecond: second}
			nextRefreshMS = (end - elapsed + 50*time.Millisecond).Milliseconds()
		} else if elapsed >= end && elapsed < dramaticEnd {
			second := event.Minute * 60
			clock = MatchClockState{StartSecond: second, EndSecond: second}
			nextRefreshMS = (dramaticEnd - elapsed + 50*time.Millisecond).Milliseconds()
		} else if elapsed >= dramaticEnd && elapsed < clockEnd {
			startSecond := event.Minute * 60
			endSecond := startSecond
			if i+1 < len(match.Events) && event.Type != "fulltime" {
				endSecond = match.Events[i+1].Minute * 60
			}
			clock = MatchClockState{
				StartSecond: startSecond,
				EndSecond:   endSecond,
				ElapsedMS:   (elapsed - dramaticEnd).Milliseconds(),
				DurationMS:  clockDuration.Milliseconds(),
				Running:     endSecond != startSecond,
			}
			nextRefreshMS = (clockEnd - elapsed + 50*time.Millisecond).Milliseconds()
		} else if elapsed >= clockEnd {
			second := event.Minute * 60
			clock = MatchClockState{StartSecond: second, EndSecond: second}
		}

		rendered := RenderedMatchEvent{
			Minute:     event.Minute,
			Type:       event.Type,
			Label:      eventLabel(event.Type),
			FullText:   event.Narrative,
			Status:     "pending",
			DurationMS: readDuration.Milliseconds(),
			PauseMS:    dramaticPause.Milliseconds(),
			IsGoal:     event.Type == "goal",
			IsFulltime: event.Type == "fulltime",
			IsMilestone: event.Type == "goal" || event.Type == "fulltime" ||
				event.Type == "injury" || event.Type == "halftime",
			Attribution: eventAttribution(match, event),
		}
		switch {
		case elapsed >= end:
			rendered.Status = "done"
			rendered.Text = event.Narrative
			rendered.EventElapsedMS = readDuration.Milliseconds()
		case elapsed >= start:
			rendered.Status = "live"
			eventElapsed := elapsed - start
			rendered.EventElapsedMS = eventElapsed.Milliseconds()
			rendered.Text = partialNarrative(event.Narrative, eventElapsed, readDuration)
		default:
			rendered.Text = ""
		}
		if rendered.Status == "pending" {
			continue
		}
		if rendered.Status == "live" && rendered.IsGoal {
			goalLive = true
		}
		events = append(events, rendered)
	}

	scoreLabel := "Placar em andamento"
	scoreA, scoreB := match.Score()
	if finished {
		scoreLabel = fmt.Sprintf("%s %d x %d %s", match.TeamA.Name, scoreA, scoreB, match.TeamB.Name)
		clock = MatchClockState{StartSecond: 40 * 60, EndSecond: 40 * 60}
		nextRefreshMS = 0
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
		NextRefreshMS:   nextRefreshMS,
		Clock:           clock,
		AnimationConfig: AnimationConfig{
			CharDurationMS: narrationCharDuration.Milliseconds(),
			BaseDelayMS:    eventBaseDelay.Milliseconds(),
			MatchMinuteMS:  matchMinuteDuration.Milliseconds(),
		},
		ScoreTeamA: scoreA,
		ScoreTeamB: scoreB,
	}
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
	if len(event.Narrative) == 0 {
		return narrationCharDuration
	}
	return time.Duration(len([]rune(event.Narrative))) * narrationCharDuration
}

func eventDramaticPauseDuration(event MatchEvent) time.Duration {
	return eventBaseDelay + time.Duration(event.DramaticPauseSeconds)*time.Second
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
