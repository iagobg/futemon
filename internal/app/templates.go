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
<body class="min-h-screen bg-zinc-950 text-zinc-100">
  <header class="border-b border-zinc-800 bg-zinc-950/95">
    <div class="mx-auto flex max-w-7xl items-center justify-between px-4 py-4">
      <a href="/teams" class="text-xl font-black tracking-wide text-lime-300">Futemon</a>
      <nav class="flex flex-wrap gap-2 text-sm">
        {{ template "navLink" dict "Href" "/teams" "Key" "teams" "Active" .Active "Label" "Meus Times" }}
        {{ template "navLink" dict "Href" "/duels" "Key" "duels" "Active" .Active "Label" "Duelos" }}
        {{ template "navLink" dict "Href" "/tournaments" "Key" "tournaments" "Active" .Active "Label" "Torneios" }}
        {{ template "navLink" dict "Href" "/global-teams" "Key" "global" "Active" .Active "Label" "Times Globais" }}
        {{ template "navLink" dict "Href" "/admin" "Key" "admin" "Active" .Active "Label" "Admin" }}
        {{ template "navLink" dict "Href" "/settings" "Key" "settings" "Active" .Active "Label" "Conta" }}
      </nav>
    </div>
  </header>

  <main class="mx-auto max-w-7xl px-4 py-6">
    {{ if eq .Active "teams" }}{{ template "teams" . }}{{ end }}
    {{ if eq .Active "duels" }}{{ template "duels" . }}{{ end }}
    {{ if eq .Active "match" }}{{ template "match" .MatchState }}{{ end }}
    {{ if eq .Active "tournaments" }}{{ template "tournaments" . }}{{ end }}
    {{ if eq .Active "global" }}{{ template "globalTeams" . }}{{ end }}
    {{ if eq .Active "admin" }}{{ template "admin" . }}{{ end }}
    {{ if eq .Active "settings" }}{{ template "settings" . }}{{ end }}
  </main>
</body>
</html>
{{ end }}

{{ define "navLink" }}
<a href="{{ .Href }}" class="rounded-md px-3 py-2 {{ if eq .Active .Key }}bg-lime-300 text-zinc-950{{ else }}text-zinc-300 hover:bg-zinc-900{{ end }}">{{ .Label }}</a>
{{ end }}

{{ define "teams" }}
<section class="grid gap-6 lg:grid-cols-[1fr_360px]">
  <div>
    <div class="mb-4 flex items-end justify-between gap-4">
      <div>
        <h1 class="text-2xl font-bold">Meus Times</h1>
        <p class="mt-1 text-sm text-zinc-400">{{ len .Teams }} de 6 slots usados. Times inscritos em torneios ficam congelados.</p>
      </div>
      <a href="/teams" class="rounded-md bg-lime-300 px-4 py-2 text-sm font-semibold text-zinc-950">Novo Time</a>
    </div>
    {{ if .Flash }}<div class="mb-4 rounded-md border border-lime-500 bg-lime-950 px-4 py-3 text-sm text-lime-100">{{ .Flash }}</div>{{ end }}
    {{ if .Error }}<div class="mb-4 rounded-md border border-red-500 bg-red-950 px-4 py-3 text-sm text-red-100">{{ .Error }}</div>{{ end }}
    <div class="grid gap-4 md:grid-cols-2">
      {{ range .Teams }}{{ template "teamCard" . }}{{ end }}
    </div>
  </div>

  <aside class="rounded-lg border border-zinc-800 bg-zinc-900 p-4">
    <h2 class="font-semibold">{{ if .EditingTeam }}Editar Escalacao{{ else }}Nova Escalacao{{ end }}</h2>
    <form action="/teams/save" method="post" class="mt-4 space-y-3">
      <input type="hidden" name="id" value="{{ .TeamForm.ID }}">
      <label class="block text-sm">Nome do time<input name="name" class="mt-1 w-full rounded-md border border-zinc-700 bg-zinc-950 px-3 py-2" value="{{ .TeamForm.Name }}" required maxlength="100"></label>
      <label class="block text-sm">Goleiro
        <select name="goalkeeper_id" class="mt-1 w-full rounded-md border border-zinc-700 bg-zinc-950 px-3 py-2">
          {{ range .Pokemon }}<option value="{{ .ID }}" {{ if eq .ID $.TeamForm.GoalkeeperID }}selected{{ end }}>{{ .Name }} - {{ .Type1 }}</option>{{ end }}
        </select>
      </label>
      <label class="block text-sm">Fixo
        <select name="fixo_id" class="mt-1 w-full rounded-md border border-zinc-700 bg-zinc-950 px-3 py-2">
          {{ range .Pokemon }}<option value="{{ .ID }}" {{ if eq .ID $.TeamForm.FixoID }}selected{{ end }}>{{ .Name }} - {{ .Type1 }}</option>{{ end }}
        </select>
      </label>
      <label class="block text-sm">Ala Esquerda
        <select name="ala_esquerda_id" class="mt-1 w-full rounded-md border border-zinc-700 bg-zinc-950 px-3 py-2">
          {{ range .Pokemon }}<option value="{{ .ID }}" {{ if eq .ID $.TeamForm.AlaEsquerdaID }}selected{{ end }}>{{ .Name }} - {{ .Type1 }}</option>{{ end }}
        </select>
      </label>
      <label class="block text-sm">Ala Direita
        <select name="ala_direita_id" class="mt-1 w-full rounded-md border border-zinc-700 bg-zinc-950 px-3 py-2">
          {{ range .Pokemon }}<option value="{{ .ID }}" {{ if eq .ID $.TeamForm.AlaDireitaID }}selected{{ end }}>{{ .Name }} - {{ .Type1 }}</option>{{ end }}
        </select>
      </label>
      <label class="block text-sm">Pivo
        <select name="pivo_id" class="mt-1 w-full rounded-md border border-zinc-700 bg-zinc-950 px-3 py-2">
          {{ range .Pokemon }}<option value="{{ .ID }}" {{ if eq .ID $.TeamForm.PivoID }}selected{{ end }}>{{ .Name }} - {{ .Type1 }}</option>{{ end }}
        </select>
      </label>
      <button class="w-full rounded-md bg-lime-300 px-4 py-2 font-semibold text-zinc-950">Salvar Escalacao</button>
    </form>
  </aside>
</section>
{{ end }}

{{ define "teamCard" }}
<article class="rounded-lg border border-zinc-800 bg-zinc-900 p-4">
  <div class="flex items-start justify-between gap-3">
    <div>
      <h2 class="font-bold text-lime-200">{{ .Name }}</h2>
      <p class="mt-1 text-xs text-zinc-500">{{ .ID }}</p>
    </div>
    {{ if .IsFrozen }}<span class="rounded bg-sky-300 px-2 py-1 text-xs font-semibold text-sky-950">Congelado</span>{{ end }}
  </div>
  <dl class="mt-4 grid gap-2 text-sm">
    {{ range .Roster }}
    <div class="flex justify-between gap-3 rounded bg-zinc-950 px-3 py-2">
      <dt class="text-zinc-400">{{ .Position }}</dt>
      <dd class="font-medium">{{ .Pokemon.Name }}</dd>
    </div>
    {{ end }}
  </dl>
  <div class="mt-4 grid grid-cols-2 gap-2 text-sm">
    <a href="/teams?edit={{ .ID }}" class="rounded-md border border-zinc-700 px-3 py-2 text-center">Editar</a>
    <form action="/teams/delete" method="post">
      <input type="hidden" name="id" value="{{ .ID }}">
      <button class="w-full rounded-md border border-zinc-700 px-3 py-2 {{ if .IsFrozen }}cursor-not-allowed opacity-50{{ end }}" {{ if .IsFrozen }}disabled{{ end }}>Excluir</button>
    </form>
  </div>
</article>
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
    {{ range .GlobalTeams }}{{ template "teamCard" . }}{{ end }}
  </div>
</section>
{{ end }}

{{ define "match" }}
<section>
  {{ template "matchLive" . }}
</section>
{{ end }}

{{ define "matchLive" }}
<div class="relative overflow-hidden rounded-lg border border-zinc-800 bg-zinc-900 p-4" hx-get="/match/live" hx-trigger="broadcast-refresh from:body, every 10s" hx-swap="outerHTML" data-broadcast-state data-next-refresh-ms="{{ .NextRefreshMS }}" data-goal-live="{{ .GoalLive }}" data-clock-running="{{ .Clock.Running }}" data-clock-start-second="{{ .Clock.StartSecond }}" data-clock-end-second="{{ .Clock.EndSecond }}" data-clock-elapsed-ms="{{ .Clock.ElapsedMS }}" data-clock-duration-ms="{{ .Clock.DurationMS }}">
  <div data-confetti class="pointer-events-none absolute inset-0 z-10 overflow-hidden"></div>
  <div class="border-b border-zinc-800 pb-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold">{{ .Match.TeamA.Name }} x {{ .Match.TeamB.Name }}</h1>
        <p class="mt-1 text-sm text-zinc-400"><span data-match-clock>{{ .MatchClockLabel }}</span> · {{ .ScoreLabel }}</p>
      </div>
      {{ if .Finished }}
      <span class="rounded bg-lime-300 px-2 py-1 text-xs font-bold text-zinc-950">ENCERRADO</span>
      {{ else }}
      <span class="rounded bg-red-400 px-2 py-1 text-xs font-bold text-red-950">AO VIVO {{ .ElapsedLabel }}</span>
      {{ end }}
    </div>
    <div class="mt-4 h-2 overflow-hidden rounded bg-zinc-800">
      <div data-progress-bar class="h-full bg-lime-300" style="width: {{ .ProgressPercent }}%"></div>
    </div>
    <div class="mt-4 grid gap-3 text-sm md:grid-cols-2">
      <div class="rounded-md bg-zinc-950 p-3">
        <div class="font-semibold text-lime-200">{{ .Match.TeamA.Name }}</div>
        <div class="mt-2 grid gap-1 text-zinc-400">
          {{ range .Match.TeamA.Roster }}<div class="flex justify-between gap-3"><span>{{ .Position }}</span><span class="text-zinc-100">{{ .Pokemon.Name }}</span></div>{{ end }}
        </div>
      </div>
      <div class="rounded-md bg-zinc-950 p-3">
        <div class="font-semibold text-lime-200">{{ .Match.TeamB.Name }}</div>
        <div class="mt-2 grid gap-1 text-zinc-400">
          {{ range .Match.TeamB.Roster }}<div class="flex justify-between gap-3"><span>{{ .Position }}</span><span class="text-zinc-100">{{ .Pokemon.Name }}</span></div>{{ end }}
        </div>
      </div>
    </div>
  </div>
  <div data-event-feed class="mt-4 max-h-[520px] overflow-y-auto pr-2">
    {{ template "eventList" .Events }}
  </div>
</div>
{{ end }}

{{ define "eventList" }}
<ol class="space-y-3">
  {{ range . }}
  <li class="rounded-md border p-4 {{ if eq .Status "live" }}border-lime-300 bg-zinc-950{{ else }}border-zinc-800 bg-zinc-950{{ end }}">
    <div class="mb-1 flex items-center gap-2 text-xs uppercase tracking-wide {{ if eq .Status "live" }}text-lime-300{{ else }}text-zinc-500{{ end }}">
      <span>{{ .Minute }}'</span><span>{{ .Label }}</span>
      {{ if .Attribution }}<span class="normal-case tracking-normal text-zinc-400">{{ .Attribution }}</span>{{ end }}
      {{ if eq .Status "live" }}<span class="rounded bg-lime-300 px-1.5 py-0.5 text-[10px] font-bold text-zinc-950">AGORA</span>{{ end }}
    </div>
    {{ if eq .Status "live" }}
    <p class="text-zinc-100"><span data-typewriter data-full-text="{{ .FullText }}" data-event-elapsed-ms="{{ .EventElapsedMS }}" data-duration-ms="{{ .DurationMS }}" data-pause-ms="{{ .PauseMS }}">{{ .Text }}</span><span class="text-lime-300">|</span></p>
    {{ else }}
    <p class="text-zinc-100">{{ .Text }}</p>
    {{ end }}
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
  <h1 class="text-2xl font-bold">Times Globais</h1>
  <div class="mt-4 grid gap-4 md:grid-cols-2 lg:grid-cols-3">
    {{ range .GlobalTeams }}{{ template "teamCard" . }}{{ end }}
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
  <form action="/settings/save" method="post" class="mt-4 space-y-4">
    <label class="block text-sm">Nome publico<input name="display_name" class="mt-1 w-full rounded-md border border-zinc-700 bg-zinc-950 px-3 py-2" value="{{ .AccountForm.DisplayName }}" required maxlength="100"></label>
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
