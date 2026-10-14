import { defineConfig } from '@playwright/test';

// E2E roda a stack real local: googlestub (Google falso, :9100) → backend
// Go (:8080, Postgres de teste 5433 — make test-e2e sobe o container) →
// vite preview (:4173, proxy /api|/auth|/healthz pro backend).
// ATENÇÃO: derrube o `make up` antes (o app do compose também usa :8080 e
// seria reusado sem o Google stubbado).
const TEST_DATABASE_URL =
  process.env.TEST_DATABASE_URL ??
  'postgres://postgres:postgres@localhost:5433/timetrack_test?sslmode=disable';

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  retries: 0,
  // A stack de E2E tem SINGLETONS globais: 1 backend `go run`, 1 usuário do
  // googlestub (logo, 1 timer global) e 1 Postgres de teste compartilhado.
  // QUALQUER concorrência contra esse backend único é contenção pura — não há
  // paralelismo real a ganhar, só races de carga: o refetch de convergência do
  // board pós-PATCH atrasa (cross-assign: o assignee "não persiste"), e o FocoPicker
  // não recebe useTodayEntries/useProjects a tempo (o cartão do projeto recente não
  // aparece → o clique trava ~30s → segura o timerLock → cascata "did not run").
  // Nenhuma dessas é bug de app (passam isoladas/serial; o app está correto). A
  // topologia pede SERIAL: workers:1 = gate DETERMINÍSTICO, alinhado ao "GATE VERDE
  // determinístico" do projeto. Não mascara bug — uma falha real (stale permanente,
  // ordenação errada) estoura igual, agora sem ruído de carga. O timerLock e o
  // waitTaskPatch ficam como defesa caso alguém volte a subir workers.
  workers: 1,
  reporter: [['list']],
  // expect 10s (default é 5s): teto de segurança pra convergência web-first
  // (invalidate → GET /board após um PATCH). Sob serial (workers:1) raramente é
  // exercido, mas dá margem se a máquina estiver carregada; não mascara stale
  // permanente (que nunca converge). As asserções de orçamento de perf
  // (reload ≤2s/sync ≤3s) têm timeout EXPLÍCITO próprio e não são afetadas.
  expect: { timeout: 10_000 },
  use: {
    // 127.0.0.1, NÃO localhost: o `vite preview` liga só no loopback IPv6 (::1).
    // No macOS `localhost` resolve pra ::1 E 127.0.0.1; quando o Chromium tenta o
    // ramo IPv4 dá ERR_CONNECTION_REFUSED intermitente (a falha "anda" entre specs).
    // Fixar tudo que o browser acessa em IPv4 elimina a corrida de dual-stack — ESTE
    // era o motivo do gate flaky (não a contenção). O preview é forçado a 127.0.0.1
    // no webServer abaixo e o OAUTH_REDIRECT_URL acompanha (mesma origem).
    baseURL: 'http://127.0.0.1:4173',
  },
  webServer: [
    {
      command: 'go run ./cmd/googlestub -addr :9100 -fixtures ./testdata/google',
      cwd: '../backend',
      url: 'http://localhost:9100/healthz',
      reuseExistingServer: !process.env.CI,
      timeout: 60_000,
    },
    {
      // fase 06: Google Calendar de mentira (eventos ancorados no timeMin)
      command: 'go run ./cmd/calendarstub -addr :9101',
      cwd: '../backend',
      url: 'http://localhost:9101/healthz',
      reuseExistingServer: !process.env.CI,
      timeout: 60_000,
    },
    {
      command: 'go run ./cmd/server',
      cwd: '../backend',
      url: 'http://localhost:8080/healthz',
      reuseExistingServer: !process.env.CI,
      timeout: 60_000,
      env: {
        DATABASE_URL: TEST_DATABASE_URL,
        ADDR: ':8080',
        GOOGLE_AUTH_URL: 'http://localhost:9100/authorize',
        GOOGLE_TOKEN_URL: 'http://localhost:9100/token',
        GOOGLE_USERINFO_URL: 'http://localhost:9100/userinfo',
        OAUTH_REDIRECT_URL: 'http://127.0.0.1:4173/auth/google/callback',
        GOOGLE_CALENDAR_API_URL: 'http://localhost:9101',
        // fase 16: o worker de corte tica rápido no E2E (o timer-guard.spec
        // semeia started_at no passado >2h; o clock do worker é o real)
        GUARD_TICK: '1s',
        // fase 20: o worker de lembretes tica rápido no E2E (o reminders.spec
        // semeia um reminder com remind_at no passado → despacha no 1º tick)
        REMINDER_TICK: '1s',
      },
    },
    {
      command: 'npm run build && npm run preview -- --port 4173 --strictPort --host 127.0.0.1',
      url: 'http://127.0.0.1:4173',
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
    },
  ],
});
