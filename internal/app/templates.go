package app

const pageTemplates = `
{{ define "layout" }}
<!doctype html>
<html lang="pt-BR">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Futemon</title>
  <script src="https://cdn.tailwindcss.com"></script>
  <script src="https://unpkg.com/htmx.org@1.9.12"></script>
  <script src="/static/app.js" defer></script>
  <style>
    @keyframes futemon-confetti {
      0% { transform: translateY(-20px) rotate(0deg); opacity: 0; }
      15% { opacity: 1; }
      100% { transform: translateY(150px) rotate(260deg); opacity: 0; }
    }
    .confetti-piece { animation: futemon-confetti 1200ms ease-out forwards; }
  </style>
</head>
<body class="{{ if eq .Active "match" }}flex h-screen flex-col overflow-hidden{{ else }}min-h-screen{{ end }} bg-zinc-950 text-zinc-100">
  <header class="flex-none border-b border-zinc-800 bg-zinc-950/95">
    <div class="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-3 px-4 py-4">
      <a href="/" class="text-xl font-black tracking-wide text-lime-300">Futemon</a>
      <div class="flex flex-wrap items-center gap-3">
        {{ if .User.ID }}
        <nav class="flex flex-wrap gap-2 text-sm">
          {{ template "navLink" dict "Href" "/teams" "Key" "teams" "Active" .Active "Label" "Meus Times" }}
          {{ template "navLink" dict "Href" "/duels" "Key" "duels" "Active" .Active "Label" "Duelos" }}
          {{ template "navLink" dict "Href" "/tournaments" "Key" "tournaments" "Active" .Active "Label" "Torneios" }}
          {{ template "navLink" dict "Href" "/global-teams" "Key" "global" "Active" .Active "Label" "Times Globais" }}
          {{ if eq .User.Role "admin" }}{{ template "navLink" dict "Href" "/admin" "Key" "admin" "Active" .Active "Label" "Admin" }}{{ end }}
          {{ template "navLink" dict "Href" "/settings" "Key" "settings" "Active" .Active "Label" "Conta" }}
        </nav>
        <a href="/profile" class="inline-flex items-center gap-2 rounded-md border border-zinc-800 px-3 py-2 text-sm text-zinc-300 hover:bg-zinc-900">{{ template "userAvatar" dict "User" .User "Size" "sm" }}<span>{{ .User.DisplayName }}</span></a>
        {{ else }}
        <a href="/auth/google" class="rounded-md bg-lime-300 px-3 py-2 text-sm font-semibold text-zinc-950">Entrar com Google</a>
        {{ end }}
      </div>
    </div>
  </header>

  <main class="mx-auto w-full max-w-7xl px-4 {{ if eq .Active "match" }}min-h-0 flex-1 overflow-hidden py-3{{ else }}py-6{{ end }}">
    {{ if eq .Active "home" }}{{ template "home" . }}{{ end }}
    {{ if eq .Active "teams" }}{{ template "teams" . }}{{ end }}
    {{ if eq .Active "team_detail" }}{{ template "teamDetail" . }}{{ end }}
    {{ if eq .Active "team_form" }}{{ template "teamForm" . }}{{ end }}
    {{ if eq .Active "duels" }}{{ template "duels" . }}{{ end }}
    {{ if eq .Active "match" }}{{ template "match" .MatchState }}{{ end }}
    {{ if eq .Active "tournaments" }}{{ template "tournaments" . }}{{ end }}
    {{ if eq .Active "global" }}{{ template "globalTeams" . }}{{ end }}
    {{ if eq .Active "admin" }}{{ template "admin" . }}{{ end }}
    {{ if eq .Active "settings" }}{{ template "settings" . }}{{ end }}
    {{ if eq .Active "profile" }}{{ template "profile" . }}{{ end }}
  </main>
</body>
</html>
{{ end }}

{{ define "navLink" }}
<a href="{{ .Href }}" class="rounded-md px-3 py-2 {{ if eq .Active .Key }}bg-lime-300 text-zinc-950{{ else }}text-zinc-300 hover:bg-zinc-900{{ end }}">{{ .Label }}</a>
{{ end }}


{{ define "home" }}
<section class="mx-auto max-w-xl rounded-lg border border-zinc-800 bg-zinc-900 p-6">
  <h1 class="text-3xl font-black text-lime-300">Futemon</h1>
  <p class="mt-3 text-zinc-300">Monte seus times de Pokemon e acompanhe partidas de futsal narradas lance a lance.</p>
  <div class="mt-6 grid gap-3">
    <a href="/auth/google" class="rounded-md bg-lime-300 px-4 py-3 text-center font-semibold text-zinc-950">Entrar com Google</a>
  </div>
</section>
{{ end }}

{{ define "typePill" }}
<span class="inline-flex min-w-0 items-center justify-center truncate rounded border px-1.5 py-0.5 text-[10px] font-bold uppercase tracking-wide {{ pokemonTypePillClass . }}">{{ pokemonTypeLabel . }}</span>
{{ end }}

{{ define "typePillTiny" }}
<span class="inline-flex min-w-0 items-center justify-center truncate rounded border px-1 py-0 text-[9px] font-bold uppercase leading-4 tracking-wide {{ pokemonTypePillClass . }}">{{ pokemonTypeLabel . }}</span>
{{ end }}

{{ define "userAvatar" }}
{{ $user := .User }}
{{ $size := .Size }}
{{ if $user.AvatarIcon }}
<span class="{{ if eq $size "lg" }}h-20 w-20{{ else }}h-8 w-8{{ end }} inline-block flex-none rounded-md bg-zinc-950 bg-no-repeat" style="{{ trainerAvatarStyle $user.AvatarIcon }}"></span>
{{ else if $user.PictureURL }}
<img src="{{ $user.PictureURL }}" alt="" class="{{ if eq $size "lg" }}h-20 w-20{{ else }}h-8 w-8{{ end }} flex-none rounded-md object-cover">
{{ else }}
<span class="{{ if eq $size "lg" }}h-20 w-20 text-2xl{{ else }}h-8 w-8 text-sm{{ end }} inline-flex flex-none items-center justify-center rounded-md bg-lime-300 font-black text-zinc-950">{{ trainerInitial $user }}</span>
{{ end }}
{{ end }}

{{ define "teams" }}
<section>
  <div class="mb-4 flex items-end justify-between gap-4">
    <div>
      <h1 class="text-2xl font-bold">Meus Times</h1>
      <p class="mt-1 text-sm text-zinc-400">{{ len .Teams }} de 6 slots usados. Times inscritos em torneios ficam congelados.</p>
    </div>
    <a href="/teams/new" class="rounded-md bg-lime-300 px-4 py-2 text-sm font-semibold text-zinc-950">Novo Time</a>
  </div>
  {{ if .Flash }}<div class="mb-4 rounded-md border border-lime-500 bg-lime-950 px-4 py-3 text-sm text-lime-100">{{ .Flash }}</div>{{ end }}
  {{ if .Error }}<div class="mb-4 rounded-md border border-red-500 bg-red-950 px-4 py-3 text-sm text-red-100">{{ .Error }}</div>{{ end }}
  <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
    {{ range .Teams }}{{ template "teamCard" dict "Team" . "Editable" true }}{{ end }}
  </div>
</section>
{{ end }}

{{ define "teamForm" }}
<section>
  <div class="mb-4 flex flex-wrap items-end justify-between gap-4">
    <div>
      <h1 class="text-2xl font-bold">{{ if .EditingTeam }}Editar Escalacao{{ else }}Nova Escalacao{{ end }}</h1>
      <p class="mt-1 text-sm text-zinc-400">Monte o quinteto em linha e escolha uma habilidade para cada posicao.</p>
    </div>
    <a href="/teams" class="rounded-md border border-zinc-700 px-4 py-2 text-sm text-zinc-200">Voltar</a>
  </div>
  {{ if .Error }}<div class="mb-4 rounded-md border border-red-500 bg-red-950 px-4 py-3 text-sm text-red-100">{{ .Error }}</div>{{ end }}
  {{ if .EditingTeam }}
  <div class="mb-4 rounded-md border {{ if .TransferWindow.Used }}border-amber-500 bg-amber-950 text-amber-100{{ else }}border-lime-500 bg-lime-950 text-lime-100{{ end }} px-4 py-3 text-sm">
    <div class="font-semibold">Janela de transferencia semanal</div>
    <p class="mt-1">{{ if .TransferWindow.Used }}Este time ja usou a troca de Pokemon desta semana. Nome e habilidades ainda podem ser ajustados.{{ else }}Este time ainda tem 1 troca de Pokemon disponivel nesta semana. Trocar qualquer Pokemon consome a janela.{{ end }}</p>
    <p class="mt-1 text-xs opacity-80">Periodo atual: {{ formatShortTime .TransferWindow.Start }} ate {{ formatShortTime .TransferWindow.End }}.</p>
  </div>
  {{ end }}
  <form action="/teams/save" method="post" class="rounded-lg border border-zinc-800 bg-zinc-900 p-4">
    <input type="hidden" name="id" value="{{ .TeamForm.ID }}">
    <div class="mb-4 max-w-lg">
      <label class="block text-sm">Nome do time<input name="name" class="mt-1 w-full rounded-md border border-zinc-700 bg-zinc-950 px-3 py-2" value="{{ .TeamForm.Name }}" required maxlength="100"></label>
    </div>
    <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-5">
      {{ template "lineupSlot" dict "Label" "Goleiro" "PokemonName" "goalkeeper_id" "AbilityName" "goalkeeper_ability" "Pokemon" .Pokemon "SelectedID" .TeamForm.GoalkeeperID "SelectedAbility" .TeamForm.GoalkeeperAbility }}
      {{ template "lineupSlot" dict "Label" "Fixo" "PokemonName" "fixo_id" "AbilityName" "fixo_ability" "Pokemon" .Pokemon "SelectedID" .TeamForm.FixoID "SelectedAbility" .TeamForm.FixoAbility }}
      {{ template "lineupSlot" dict "Label" "Ala Esquerda" "PokemonName" "ala_esquerda_id" "AbilityName" "ala_esquerda_ability" "Pokemon" .Pokemon "SelectedID" .TeamForm.AlaEsquerdaID "SelectedAbility" .TeamForm.AlaEsquerdaAbility }}
      {{ template "lineupSlot" dict "Label" "Ala Direita" "PokemonName" "ala_direita_id" "AbilityName" "ala_direita_ability" "Pokemon" .Pokemon "SelectedID" .TeamForm.AlaDireitaID "SelectedAbility" .TeamForm.AlaDireitaAbility }}
      {{ template "lineupSlot" dict "Label" "Pivo" "PokemonName" "pivo_id" "AbilityName" "pivo_ability" "Pokemon" .Pokemon "SelectedID" .TeamForm.PivoID "SelectedAbility" .TeamForm.PivoAbility }}
    </div>
    <div class="mt-4 flex justify-end gap-2">
      <a href="/teams" class="rounded-md border border-zinc-700 px-4 py-2 text-sm text-zinc-200">Cancelar</a>
      <button class="rounded-md bg-lime-300 px-5 py-2 font-semibold text-zinc-950">Salvar Escalacao</button>
    </div>
  </form>
</section>
{{ end }}

{{ define "lineupSlot" }}
<div data-lineup-slot class="rounded-md border border-zinc-800 bg-zinc-950/60 p-3">
  <div class="flex items-center justify-between gap-2">
    <div class="text-xs font-bold uppercase tracking-wide text-zinc-500">{{ .Label }}</div>
    <button type="button" data-clear-lineup-slot aria-label="Limpar {{ .Label }}" class="inline-flex h-6 w-6 items-center justify-center rounded text-zinc-500 hover:bg-zinc-800 hover:text-zinc-100 focus:outline-none focus:ring-2 focus:ring-lime-300">×</button>
  </div>
  <div class="mt-3 flex aspect-square items-center justify-center rounded-md bg-zinc-950 p-2">
    <div data-lineup-preview-placeholder class="{{ if pokemonArtwork .Pokemon .SelectedID }}hidden {{ end }}flex h-full w-full items-center justify-center rounded border border-dashed border-zinc-800 text-xs text-zinc-600">Pokemon</div>
    <img data-lineup-preview src="{{ pokemonArtwork .Pokemon .SelectedID }}" alt="" class="{{ if pokemonArtwork .Pokemon .SelectedID }}{{ else }}hidden {{ end }}h-full w-full object-contain">
  </div>
  <div class="mt-3 grid gap-2">
    {{ template "pokemonPicker" dict "Label" "Pokemon" "Name" .PokemonName "Pokemon" .Pokemon "SelectedID" .SelectedID }}
    {{ template "abilityPicker" dict "Label" "Habilidade" "Name" .AbilityName "SelectedAbility" .SelectedAbility }}
  </div>
</div>
{{ end }}

{{ define "abilityPicker" }}
<div data-ability-picker class="relative text-sm">
  <label class="block">{{ .Label }}
    <input data-ability-search autocomplete="off" spellcheck="false" class="mt-1 w-full rounded-md border border-zinc-700 bg-zinc-950 px-3 py-1.5" value="{{ abilityDisplayName .SelectedAbility }}" placeholder="Escolha uma habilidade">
  </label>
  <input data-ability-id type="hidden" name="{{ .Name }}" value="{{ .SelectedAbility }}">
  <div data-ability-options class="absolute z-30 mt-1 hidden max-h-56 w-full overflow-y-auto rounded-md border border-zinc-700 bg-zinc-950 shadow-xl"></div>
</div>
{{ end }}

{{ define "pokemonPicker" }}
<div data-pokemon-picker class="relative text-sm">
  <label class="block">{{ .Label }}
    <input data-pokemon-search autocomplete="off" spellcheck="false" class="mt-1 w-full rounded-md border border-zinc-700 bg-zinc-950 px-3 py-1.5" value="{{ pokemonName .Pokemon .SelectedID }}" placeholder="Digite aqui">
  </label>
  <input data-pokemon-id type="hidden" name="{{ .Name }}" value="{{ .SelectedID }}">
  <div data-pokemon-options class="absolute z-20 mt-1 hidden max-h-72 w-full overflow-y-auto rounded-md border border-zinc-700 bg-zinc-950 shadow-xl">
    {{ range .Pokemon }}
    <button type="button" data-pokemon-option data-id="{{ .ID }}" data-name="{{ pokemonDisplayName .Name }}" data-search="{{ lower (pokemonDisplayName .Name) }}" data-abilities='{{ pokemonAbilitiesJSON . }}' data-artwork="{{ .DisplayArtworkURL }}" class="hidden w-full items-center gap-2 px-2.5 py-1.5 text-left hover:bg-zinc-800">
      <span class="min-w-0 flex-1">
        <span class="flex min-w-0 items-baseline gap-1.5"><span class="flex-none text-[11px] font-semibold text-zinc-500">#{{ .ID }}</span><span class="truncate text-sm font-medium leading-5 text-zinc-100">{{ pokemonDisplayName .Name }}</span></span>
        <span class="mt-0.5 flex min-w-0 flex-wrap gap-1">{{ template "typePillTiny" .Type1 }}{{ if .Type2 }}{{ template "typePillTiny" .Type2 }}{{ end }}</span>
      </span>
      {{ if .DisplayArtworkURL }}<img src="{{ .DisplayArtworkURL }}" alt="" class="h-10 w-10 flex-none object-contain">{{ else }}<span class="h-10 w-10 flex-none rounded bg-zinc-800"></span>{{ end }}
    </button>
    {{ end }}
  </div>
</div>
{{ end }}

{{ define "teamCard" }}
{{ $team := .Team }}
<article class="rounded-lg border border-zinc-800 bg-zinc-900 p-4">
  <div class="flex items-start justify-between gap-3">
    <div>
      <h2 class="font-bold text-lime-200"><a href="/teams/{{ $team.ID }}" class="hover:underline">{{ $team.Name }}</a></h2>
      <p class="mt-1 text-xs text-zinc-500">{{ $team.ID }}</p>
    </div>
    <div class="flex flex-col items-end gap-1">
      {{ if $team.IsRetired }}<span class="rounded bg-zinc-600 px-2 py-1 text-xs font-semibold text-zinc-100">Aposentado</span>{{ end }}
      {{ if $team.IsFrozen }}<span class="rounded bg-sky-300 px-2 py-1 text-xs font-semibold text-sky-950">Congelado</span>{{ end }}
      <span class="rounded bg-zinc-800 px-2 py-1 text-xs font-semibold text-zinc-200">{{ $team.Record.Label }}</span>
      <span class="text-[11px] text-zinc-500">{{ $team.Record.WinPercent }}% vit.</span>
    </div>
  </div>
  <dl class="mt-4 grid gap-2 text-sm">
    {{ range $team.Roster }}
    <div class="flex items-center justify-between gap-3 rounded bg-zinc-950 px-3 py-2">
      <dt class="text-zinc-400">{{ .Position }}</dt>
      <dd class="flex min-w-0 items-center gap-2 font-medium"><span class="hidden flex-none gap-1 sm:flex">{{ template "typePill" .Pokemon.Type1 }}{{ if .Pokemon.Type2 }}{{ template "typePill" .Pokemon.Type2 }}{{ end }}</span><span class="min-w-0"><span class="block truncate">{{ pokemonDisplayName .Pokemon.Name }}</span>{{ if .Ability }}<span class="block truncate text-xs font-normal text-zinc-500">{{ abilityDisplayName .Ability }}</span>{{ end }}</span>{{ if .Pokemon.DisplayArtworkURL }}<img src="{{ .Pokemon.DisplayArtworkURL }}" alt="" class="h-9 w-9 flex-none object-contain">{{ end }}</dd>
    </div>
    {{ end }}
  </dl>
  {{ if .Editable }}
  <div class="mt-4 grid grid-cols-3 gap-2 text-sm">
    <a href="/teams/{{ $team.ID }}" class="rounded-md border border-zinc-700 px-3 py-2 text-center">Historico</a>
    <a href="/teams/edit?id={{ $team.ID }}" class="rounded-md border border-zinc-700 px-3 py-2 text-center">Editar</a>
    <form action="/teams/delete" method="post">
      <input type="hidden" name="id" value="{{ $team.ID }}">
      <button class="w-full rounded-md border border-zinc-700 px-3 py-2 {{ if $team.IsFrozen }}cursor-not-allowed opacity-50{{ end }}" {{ if $team.IsFrozen }}disabled{{ end }}>Excluir</button>
    </form>
  </div>
  {{ else }}
  <div class="mt-4 text-sm">
    <a href="/teams/{{ $team.ID }}" class="block rounded-md border border-zinc-700 px-3 py-2 text-center">Ver historico</a>
  </div>
  {{ end }}
</article>
{{ end }}

{{ define "teamDetail" }}
<section class="grid gap-6 lg:grid-cols-[420px_1fr]">
  <div>
    {{ template "teamCard" dict "Team" .Team "Editable" false }}
  </div>
  <div>
    <div class="mb-4 flex flex-wrap items-end justify-between gap-3">
      <div>
        <div class="flex flex-wrap items-center gap-2">
          <h1 class="text-2xl font-bold">Historico de {{ .Team.Name }}</h1>
          {{ if .Team.IsRetired }}<span class="rounded bg-zinc-700 px-2 py-1 text-xs font-semibold text-zinc-100">Aposentado</span>{{ end }}
        </div>
        <p class="mt-1 text-sm text-zinc-400">{{ .Team.Record.Label }} em {{ .Team.Record.Played }} partidas finalizadas.</p>
      </div>
      <a href="/global-teams" class="rounded-md border border-zinc-700 px-4 py-2 text-sm text-zinc-200">Times Globais</a>
    </div>
    {{ if .Trainer.ID }}
    <div class="mb-6 rounded-lg border border-zinc-800 bg-zinc-900 p-4">
      <div class="text-xs uppercase tracking-wide text-zinc-500">Treinador</div>
      <a href="/users/{{ .Trainer.ID }}" class="mt-3 inline-flex items-center gap-3 hover:underline">{{ template "userAvatar" dict "User" .Trainer "Size" "sm" }}<span class="font-semibold text-lime-200">{{ .Trainer.DisplayName }}</span></a>
    </div>
    {{ end }}
    <div class="mb-6 rounded-lg border border-zinc-800 bg-zinc-900 p-4">
      <h2 class="font-semibold text-lime-200">Historico de formacao</h2>
      <div class="mt-4 grid gap-3">
        {{ if .TeamTransfers }}
          {{ range .TeamTransfers }}
          <article class="rounded-md bg-zinc-950 p-3 text-sm">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <div class="font-semibold text-zinc-100">{{ transferKindLabel .Kind }}</div>
              <div class="text-xs uppercase tracking-wide text-zinc-500">{{ formatShortTime .CreatedAt }}</div>
            </div>
            <p class="mt-1 text-zinc-400">{{ .Summary }}</p>
            <div class="mt-3 flex flex-wrap gap-2">
              {{ range .After.Roster }}
              <span class="inline-flex items-center gap-1 rounded bg-zinc-900 px-2 py-1 text-xs text-zinc-300">{{ if .Pokemon.DisplayArtworkURL }}<img src="{{ .Pokemon.DisplayArtworkURL }}" alt="" class="h-6 w-6 object-contain">{{ end }}{{ .Position }}: {{ pokemonDisplayName .Pokemon.Name }}</span>
              {{ end }}
            </div>
          </article>
          {{ end }}
        {{ else }}
          <div class="text-sm text-zinc-400">Nenhum registro de formacao encontrado.</div>
        {{ end }}
      </div>
    </div>
    <div class="grid gap-3">
      {{ if .TeamHistory }}
        {{ range .TeamHistory }}
        <article class="rounded-lg border border-zinc-800 bg-zinc-900 p-4">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div>
              <div class="text-xs uppercase tracking-wide text-zinc-500">{{ formatShortTime .PlayedAt }} · {{ resultLabel .TeamResult }}</div>
              <h2 class="mt-1 font-semibold text-lime-200">{{ .TeamAName }} {{ .ScoreTeamA }} x {{ .ScoreTeamB }} {{ .TeamBName }}</h2>
              <p class="mt-1 text-sm text-zinc-400">Placar do time: {{ .TeamScoreLine }}</p>
            </div>
            <div class="flex flex-wrap gap-2 text-sm">
              <a href="/match/{{ .ID }}/replay" class="rounded-md bg-lime-300 px-3 py-2 font-semibold text-zinc-950">Narracao</a>
              <a href="/match/{{ .ID }}/recap" class="rounded-md border border-zinc-700 px-3 py-2 text-zinc-200">Relatorio</a>
            </div>
          </div>
          <div class="mt-4 grid gap-3 text-sm md:grid-cols-2">
            <div class="rounded-md bg-zinc-950 p-3">
              <div class="font-semibold text-zinc-200">{{ .TeamAName }}</div>
              <div class="mt-2 grid gap-1 text-zinc-400">
                {{ if .GoalsTeamA }}{{ range .GoalsTeamA }}<div>{{ .Minute }}' {{ .PokemonName }}</div>{{ end }}{{ else }}<div>Sem gols</div>{{ end }}
              </div>
            </div>
            <div class="rounded-md bg-zinc-950 p-3">
              <div class="font-semibold text-zinc-200">{{ .TeamBName }}</div>
              <div class="mt-2 grid gap-1 text-zinc-400">
                {{ if .GoalsTeamB }}{{ range .GoalsTeamB }}<div>{{ .Minute }}' {{ .PokemonName }}</div>{{ end }}{{ else }}<div>Sem gols</div>{{ end }}
              </div>
            </div>
          </div>
        </article>
        {{ end }}
      {{ else }}
        <div class="rounded-lg border border-zinc-800 bg-zinc-900 p-4 text-sm text-zinc-400">Nenhuma partida finalizada no historico deste time.</div>
      {{ end }}
    </div>
  </div>
</section>
{{ end }}

{{ define "duels" }}
<section class="grid gap-6 lg:grid-cols-[420px_1fr]">
  <form hx-post="/duels/start" class="rounded-lg border border-zinc-800 bg-zinc-900 p-4">
    <h1 class="text-2xl font-bold">Duelos</h1>
    <p class="mt-1 text-sm text-zinc-400">{{ if .User.HasGeminiAPIKey }}Modo Chave de API Ativo{{ else }}1/1 duelo disponivel hoje. BYOK desativa esse limite.{{ end }}</p>
    <label class="mt-5 block text-sm">Seu time
      <select name="team_id" class="mt-1 w-full rounded-md border border-zinc-700 bg-zinc-950 px-3 py-2">
        {{ range .Teams }}<option value="{{ .ID }}">{{ .Name }}</option>{{ end }}
      </select>
    </label>
    <button name="opponent_id" value="team-paleta-bolada" class="mt-4 w-full rounded-md bg-lime-300 px-4 py-2 font-semibold text-zinc-950">Buscar Duelo Aleatorio</button>
    <div class="mt-5 border-t border-zinc-800 pt-5">
      <label class="block text-sm">ID do time adversario
        <input name="opponent_id" placeholder="team-paleta-bolada" class="mt-1 w-full rounded-md border border-zinc-700 bg-zinc-950 px-3 py-2">
      </label>
      <button class="mt-3 w-full rounded-md border border-lime-300 px-4 py-2 font-semibold text-lime-200">Desafiar por ID</button>
    </div>
  </form>

  <div class="grid gap-4 md:grid-cols-2">
    {{ range .GlobalTeams }}{{ template "teamCard" dict "Team" . "Editable" false }}{{ end }}
  </div>
</section>
{{ end }}

{{ define "match" }}
<section class="h-full min-h-0">
  {{ template "matchLive" . }}
</section>
{{ end }}

{{ define "matchLive" }}
<div class="relative flex h-full min-h-0 flex-col overflow-hidden rounded-lg border border-zinc-800 bg-zinc-900 p-4" data-broadcast-state data-playback-mode="{{ .PlaybackMode }}" data-match-id="{{ .Match.ID }}" data-match-version="{{ .Match.ID }}" data-sync-url="/match/{{ .Match.ID }}/sync" data-rendered-at-ms="{{ .RenderedAtMS }}" data-match-started-at-ms="{{ .StartedAtMS }}" data-match-ended-at-ms="{{ .EndedAtMS }}" data-current-score-team-a="{{ .CurrentScoreTeamA }}" data-current-score-team-b="{{ .CurrentScoreTeamB }}" data-final-score-team-a="{{ .ScoreTeamA }}" data-final-score-team-b="{{ .ScoreTeamB }}">
  <div data-confetti class="pointer-events-none absolute inset-0 z-10 overflow-hidden"></div>
  <div class="flex-none border-b border-zinc-800 pb-4">
    <div class="grid items-center gap-3 md:grid-cols-[auto_minmax(0,1fr)_auto]">
      <span data-match-clock class="inline-flex w-[5.5ch] justify-self-center rounded-md border border-zinc-700 bg-zinc-950 px-2 py-1 font-mono text-lg font-bold tabular-nums text-lime-200 md:justify-self-start">{{ .MatchClockLabel }}</span>
      <h1 class="min-w-0 text-center text-xl font-bold sm:text-2xl">
        <span class="inline-block max-w-[36vw] truncate align-bottom sm:max-w-[30vw]">{{ .Match.TeamA.Name }}</span>
        <span data-score-team-a>{{ .CurrentScoreTeamA }}</span> x <span data-score-team-b>{{ .CurrentScoreTeamB }}</span>
        <span class="inline-block max-w-[36vw] truncate align-bottom sm:max-w-[30vw]">{{ .Match.TeamB.Name }}</span>
      </h1>
      <span data-live-badge class="justify-self-center rounded px-2 py-1 text-xs font-bold md:justify-self-end {{ if .Finished }}bg-lime-300 text-zinc-950{{ else }}bg-red-400 text-red-950{{ end }}">{{ if .Finished }}ENCERRADO{{ else }}AO VIVO{{ end }}</span>
    </div>
    <div data-team-summary class="mt-4 grid gap-3 text-sm md:grid-cols-2">
      <div class="rounded-md bg-zinc-950 p-3">
        <div class="font-semibold text-lime-200">{{ .Match.TeamA.Name }}</div>
        <div class="mt-2 grid gap-1 text-zinc-400">
          {{ range .Match.TeamA.Roster }}<div class="flex items-center justify-between gap-3"><span>{{ .Position }}</span><span class="flex min-w-0 items-center gap-2 text-zinc-100"><span class="hidden flex-none gap-1 sm:flex">{{ template "typePill" .Pokemon.Type1 }}{{ if .Pokemon.Type2 }}{{ template "typePill" .Pokemon.Type2 }}{{ end }}</span><span class="truncate">{{ pokemonDisplayName .Pokemon.Name }}</span>{{ if .Pokemon.DisplayArtworkURL }}<img src="{{ .Pokemon.DisplayArtworkURL }}" alt="" class="h-8 w-8 flex-none object-contain">{{ end }}</span></div>{{ end }}
        </div>
      </div>
      <div class="rounded-md bg-zinc-950 p-3">
        <div class="font-semibold text-lime-200">{{ .Match.TeamB.Name }}</div>
        <div class="mt-2 grid gap-1 text-zinc-400">
          {{ range .Match.TeamB.Roster }}<div class="flex items-center justify-between gap-3"><span>{{ .Position }}</span><span class="flex min-w-0 items-center gap-2 text-zinc-100"><span class="hidden flex-none gap-1 sm:flex">{{ template "typePill" .Pokemon.Type1 }}{{ if .Pokemon.Type2 }}{{ template "typePill" .Pokemon.Type2 }}{{ end }}</span><span class="truncate">{{ pokemonDisplayName .Pokemon.Name }}</span>{{ if .Pokemon.DisplayArtworkURL }}<img src="{{ .Pokemon.DisplayArtworkURL }}" alt="" class="h-8 w-8 flex-none object-contain">{{ end }}</span></div>{{ end }}
        </div>
      </div>
    </div>
    <div class="mt-4 h-2 overflow-hidden rounded bg-zinc-800">
      <div data-progress-bar class="h-full bg-lime-300" style="width: {{ .ProgressPercent }}%"></div>
    </div>
  </div>
  <div data-event-feed class="mt-4 min-h-0 flex-1 overflow-y-auto pr-2">
    {{ template "eventList" .Events }}
  </div>
</div>
{{ end }}

{{ define "eventList" }}
<ol class="space-y-3">
  {{ range reverseEvents . }}
  <li data-event-item data-event-key="{{ .Sequence }}" data-event-type="{{ .Type }}" data-event-started-at-ms="{{ .StartedAtMS }}" data-event-text-end-at-ms="{{ .TextEndAtMS }}" data-event-pause-end-at-ms="{{ .PauseEndAtMS }}" data-event-clock-end-at-ms="{{ .ClockEndAtMS }}" data-clock-start-second="{{ .ClockStartSecond }}" data-clock-end-second="{{ .ClockEndSecond }}" data-goal-team-side="{{ .GoalTeamSide }}" data-score-at-ms="{{ .ScoreAtMS }}" class="{{ if eq .Status "pending" }}hidden {{ end }}rounded-md border p-4 {{ if eq .Status "live" }}border-lime-300 bg-zinc-950{{ else }}border-zinc-800 bg-zinc-950{{ end }}">
    <div data-event-meta class="mb-1 flex items-center gap-2 text-xs uppercase tracking-wide {{ if eq .Status "live" }}text-lime-300{{ else }}text-zinc-500{{ end }}">
      <span>{{ .Minute }}'</span><span data-event-label data-reveal-index="{{ .RevealIndex }}" class="{{ if .LabelHidden }}hidden{{ end }}">{{ .Label }}</span><span data-generic-label class="{{ if .LabelHidden }}{{ else }}hidden{{ end }}">Lance</span>
      {{ if .Attribution }}<span data-event-attribution data-reveal-index="{{ .RevealIndex }}" class="normal-case tracking-normal text-zinc-400 {{ if .LabelHidden }}hidden{{ end }}">{{ .Attribution }}</span>{{ end }}
      <span data-now-pill class="{{ if eq .Status "live" }}{{ else }}hidden {{ end }}rounded bg-lime-300 px-1.5 py-0.5 text-[10px] font-bold text-zinc-950">AGORA</span>
    </div>
    <p class="text-zinc-100"><span data-typewriter data-event-key="{{ .Sequence }}" data-full-text="{{ .FullText }}" data-event-started-at-ms="{{ .StartedAtMS }}" data-event-elapsed-ms="{{ .EventElapsedMS }}" data-duration-ms="{{ .DurationMS }}" data-pause-ms="{{ .PauseMS }}" data-reveal-index="{{ .RevealIndex }}" data-goal-trigger="{{ .GoalTrigger }}">{{ .Text }}</span><span data-cursor class="{{ if eq .Status "live" }}{{ else }}hidden {{ end }}text-lime-300">|</span></p>
  </li>
  {{ end }}
</ol>
{{ end }}

{{ define "tournaments" }}
<section>
  <h1 class="text-2xl font-bold">Torneios</h1>
  <div class="mt-4 grid gap-4 lg:grid-cols-2">
    {{ range .Tournaments }}
    <article class="rounded-lg border border-zinc-800 bg-zinc-900 p-4">
      <div class="flex items-center justify-between">
        <h2 class="font-semibold text-lime-200">{{ .Name }}</h2>
        <span class="rounded bg-zinc-800 px-2 py-1 text-xs">{{ .Status }}</span>
      </div>
      <div class="mt-5 grid grid-cols-2 gap-3">
        {{ range .Teams }}
        <div class="rounded border border-zinc-700 p-3 text-sm">{{ .Name }}</div>
        {{ end }}
      </div>
      <button class="mt-4 rounded-md bg-lime-300 px-4 py-2 text-sm font-semibold text-zinc-950">Inscrever time elegivel</button>
    </article>
    {{ end }}
  </div>
</section>
{{ end }}

{{ define "globalTeams" }}
<section>
  <div class="mb-4 flex flex-wrap items-end justify-between gap-3">
    <div>
      <h1 class="text-2xl font-bold">Times Globais</h1>
      <p class="mt-1 text-sm text-zinc-400">Ordene por criacao recente ou por recorde ponderado.</p>
    </div>
    <div class="flex gap-2 text-sm">
      <a href="/global-teams?sort=recent" class="rounded-md px-3 py-2 {{ if eq .TeamSort "recent" }}bg-lime-300 font-semibold text-zinc-950{{ else }}border border-zinc-700 text-zinc-200{{ end }}">Recentes</a>
      <a href="/global-teams?sort=best" class="rounded-md px-3 py-2 {{ if eq .TeamSort "best" }}bg-lime-300 font-semibold text-zinc-950{{ else }}border border-zinc-700 text-zinc-200{{ end }}">Melhores</a>
    </div>
  </div>
  <div class="mt-4 grid gap-3">
    {{ range $i, $team := .GlobalTeams }}{{ template "globalTeamRow" dict "Team" $team "Rank" (inc $i) }}{{ end }}
  </div>
</section>
{{ end }}

{{ define "globalTeamRow" }}
{{ $team := .Team }}
<article class="grid gap-3 rounded-lg border border-zinc-800 bg-zinc-900 p-4 md:grid-cols-[64px_1fr_auto] md:items-center">
  <div class="flex items-center gap-3">
    <div class="flex h-12 w-12 items-center justify-center rounded-md bg-lime-300 text-xl font-black text-zinc-950">#{{ .Rank }}</div>
    <div class="md:hidden">
      <h2 class="font-bold text-lime-200"><a href="/teams/{{ $team.ID }}" class="hover:underline">{{ $team.Name }}</a></h2>
      <p class="text-xs text-zinc-500">{{ $team.Record.Label }}</p>
    </div>
  </div>
  <div class="min-w-0">
    <div class="hidden md:block">
      <h2 class="font-bold text-lime-200"><a href="/teams/{{ $team.ID }}" class="hover:underline">{{ $team.Name }}</a></h2>
      <p class="mt-1 text-xs text-zinc-500">{{ $team.ID }}</p>
    </div>
    <div class="mt-3 flex flex-wrap gap-2 md:mt-2">
      {{ range $team.Roster }}
      <span class="inline-flex h-12 w-12 items-center justify-center rounded-md bg-zinc-950 p-1" title="{{ .Position }}: {{ pokemonDisplayName .Pokemon.Name }}">{{ if .Pokemon.DisplayArtworkURL }}<img src="{{ .Pokemon.DisplayArtworkURL }}" alt="{{ pokemonDisplayName .Pokemon.Name }}" class="h-full w-full object-contain">{{ else }}<span class="text-xs text-zinc-500">{{ pokemonDisplayName .Pokemon.Name }}</span>{{ end }}</span>
      {{ end }}
    </div>
  </div>
  <div class="grid grid-cols-3 gap-2 text-center text-sm md:min-w-[240px]">
    <div class="rounded-md bg-zinc-950 px-3 py-2">
      <div class="text-xs text-zinc-500">Recorde</div>
      <div class="font-semibold text-zinc-100">{{ $team.Record.Label }}</div>
    </div>
    <div class="rounded-md bg-zinc-950 px-3 py-2">
      <div class="text-xs text-zinc-500">Vitoria</div>
      <div class="font-semibold text-zinc-100">{{ $team.Record.WinPercent }}%</div>
    </div>
    <a href="/teams/{{ $team.ID }}" class="rounded-md border border-zinc-700 px-3 py-2 text-zinc-200">
      <span class="block text-xs text-zinc-500">Abrir</span>
      <span class="font-semibold">Historico</span>
    </a>
  </div>
</article>
{{ end }}

{{ define "profile" }}
<section class="grid gap-6 lg:grid-cols-[360px_1fr]">
  <aside class="rounded-lg border border-zinc-800 bg-zinc-900 p-4">
    <div class="flex items-center gap-4">
      {{ template "userAvatar" dict "User" .ProfileUser "Size" "lg" }}
      <div class="min-w-0">
        <h1 class="truncate text-2xl font-bold">{{ .ProfileUser.DisplayName }}</h1>
        <p class="mt-1 truncate text-sm text-zinc-500">{{ .ProfileUser.Email }}</p>
      </div>
    </div>
    <div class="mt-5 grid grid-cols-2 gap-3 text-sm">
      <div class="rounded-md bg-zinc-950 p-3">
        <div class="text-xs text-zinc-500">Recorde total</div>
        <div class="mt-1 font-semibold text-zinc-100">{{ .ProfileRecord.Label }}</div>
      </div>
      <div class="rounded-md bg-zinc-950 p-3">
        <div class="text-xs text-zinc-500">Vitoria</div>
        <div class="mt-1 font-semibold text-zinc-100">{{ .ProfileRecord.WinPercent }}%</div>
      </div>
      <div class="rounded-md bg-zinc-950 p-3">
        <div class="text-xs text-zinc-500">Ativos</div>
        <div class="mt-1 font-semibold text-zinc-100">{{ len .ProfileTeams }}</div>
      </div>
      <div class="rounded-md bg-zinc-950 p-3">
        <div class="text-xs text-zinc-500">Aposentados</div>
        <div class="mt-1 font-semibold text-zinc-100">{{ len .ProfileRetiredTeams }}</div>
      </div>
    </div>
    {{ if .ProfileBestTeam.ID }}
    <div class="mt-4 rounded-md bg-zinc-950 p-3 text-sm">
      <div class="text-xs text-zinc-500">Melhor time ativo</div>
      <a href="/teams/{{ .ProfileBestTeam.ID }}" class="mt-1 block font-semibold text-lime-200 hover:underline">{{ .ProfileBestTeam.Name }}</a>
      <div class="mt-1 text-zinc-400">{{ .ProfileBestTeam.Record.Label }}</div>
    </div>
    {{ end }}
  </aside>
  <div class="grid gap-6">
    <section>
      <h2 class="text-xl font-bold">Times ativos</h2>
      <div class="mt-4 grid gap-4 md:grid-cols-2">
        {{ if .ProfileTeams }}{{ range .ProfileTeams }}{{ template "teamCard" dict "Team" . "Editable" false }}{{ end }}{{ else }}<div class="rounded-lg border border-zinc-800 bg-zinc-900 p-4 text-sm text-zinc-400">Nenhum time ativo.</div>{{ end }}
      </div>
    </section>
    {{ if .ProfileRetiredTeams }}
    <section>
      <h2 class="text-xl font-bold">Times aposentados</h2>
      <div class="mt-4 grid gap-4 md:grid-cols-2">
        {{ range .ProfileRetiredTeams }}{{ template "teamCard" dict "Team" . "Editable" false }}{{ end }}
      </div>
    </section>
    {{ end }}
  </div>
</section>
{{ end }}

{{ define "admin" }}
<section class="grid gap-6 lg:grid-cols-2">
  <div class="rounded-lg border border-zinc-800 bg-zinc-900 p-4">
    <h1 class="text-2xl font-bold">Painel de Admin</h1>
    <form class="mt-4 space-y-3">
      <label class="block text-sm">Nome do torneio<input class="mt-1 w-full rounded-md border border-zinc-700 bg-zinc-950 px-3 py-2" placeholder="Copa Silph"></label>
      <button class="rounded-md bg-lime-300 px-4 py-2 font-semibold text-zinc-950">Criar Torneio</button>
    </form>
  </div>
  <div class="rounded-lg border border-zinc-800 bg-zinc-900 p-4">
    <h2 class="font-semibold">Operacoes</h2>
    <div class="mt-4 grid gap-3">
      <button class="rounded-md border border-zinc-700 px-4 py-2 text-left">Simular Proxima Rodada</button>
      <button class="rounded-md border border-zinc-700 px-4 py-2 text-left">Cleanup de Times Orfaos</button>
      <button class="rounded-md border border-red-500 px-4 py-2 text-left text-red-200">Remocao Forcada</button>
    </div>
  </div>
</section>
{{ end }}

{{ define "settings" }}
<section class="max-w-xl rounded-lg border border-zinc-800 bg-zinc-900 p-4">
  <h1 class="text-2xl font-bold">Configuracoes de Conta</h1>
  {{ if .Flash }}<div class="mt-4 rounded-md border border-lime-500 bg-lime-950 px-4 py-3 text-sm text-lime-100">{{ .Flash }}</div>{{ end }}
  {{ if .Error }}<div class="mt-4 rounded-md border border-red-500 bg-red-950 px-4 py-3 text-sm text-red-100">{{ .Error }}</div>{{ end }}
  <div class="mt-4 flex flex-wrap gap-2 border-b border-zinc-800 pb-4 text-sm">
    <a href="/auth/logout" class="rounded-md border border-zinc-700 px-3 py-2 text-zinc-300">Sair</a>
  </div>
  <form action="/settings/save" method="post" class="mt-4 space-y-4">
    <label class="block text-sm">Nome publico<input name="display_name" class="mt-1 w-full rounded-md border border-zinc-700 bg-zinc-950 px-3 py-2" value="{{ .AccountForm.DisplayName }}" required maxlength="100"></label>
    <div>
      <div class="text-sm font-semibold text-zinc-200">Imagem publica</div>
      <div class="mt-3 flex flex-wrap gap-2">
        <label class="inline-flex cursor-pointer items-center gap-2 rounded-md border border-zinc-700 px-3 py-2 text-sm text-zinc-300 has-[:checked]:border-lime-300 has-[:checked]:text-lime-200">
          <input type="radio" name="avatar_icon" value="0" class="sr-only" {{ if eq .AccountForm.AvatarIcon 0 }}checked{{ end }}>
          {{ if .User.PictureURL }}<img src="{{ .User.PictureURL }}" alt="" class="h-8 w-8 flex-none rounded-md object-cover">{{ else }}<span class="inline-flex h-8 w-8 flex-none items-center justify-center rounded-md bg-lime-300 text-sm font-black text-zinc-950">{{ trainerInitial .User }}</span>{{ end }}
          <span>Google</span>
        </label>
      </div>
      <div class="mt-3 grid max-h-64 grid-cols-6 gap-2 overflow-y-auto rounded-md border border-zinc-800 bg-zinc-950 p-3 sm:grid-cols-8 md:grid-cols-10">
        {{ range iconChoices }}
        <label class="cursor-pointer rounded-md border border-transparent p-1 has-[:checked]:border-lime-300">
          <input type="radio" name="avatar_icon" value="{{ . }}" class="sr-only" {{ if eq $.AccountForm.AvatarIcon . }}checked{{ end }}>
          <span class="block h-12 w-12 rounded bg-zinc-900 bg-no-repeat" style="{{ trainerAvatarStyle . }}"></span>
        </label>
        {{ end }}
      </div>
    </div>
    <div>
      <label class="block text-sm">Chave Gemini API<input name="gemini_api_key" type="password" class="mt-1 w-full rounded-md border border-zinc-700 bg-zinc-950 px-3 py-2" placeholder="{{ if .User.HasGeminiAPIKey }}Chave armazenada com AES-GCM-256{{ else }}Cole sua chave Gemini{{ end }}"></label>
      <p class="mt-2 text-sm text-zinc-400">{{ if .User.HasGeminiAPIKey }}BYOK ativo. Envie uma nova chave para substituir a atual.{{ else }}Sem chave Gemini salva. Duelos casuais seguem no limite diario.{{ end }}</p>
    </div>
    {{ if .User.HasGeminiAPIKey }}
    <label class="flex items-center gap-2 text-sm text-zinc-300"><input name="clear_api_key" type="checkbox" class="h-4 w-4"> Remover chave Gemini salva</label>
    {{ end }}
    <button class="rounded-md bg-lime-300 px-4 py-2 font-semibold text-zinc-950">Salvar</button>
  </form>
</section>
{{ end }}
`
