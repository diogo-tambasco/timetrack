import { execFileSync } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { expect, type Page } from "@playwright/test";

const e2eDir = path.dirname(fileURLToPath(import.meta.url));

// Completa o login com o Google STUBBADO (cmd/googlestub atrás do backend
// local — TDD.md §3) e espera cair na home. Usado por todo E2E que precisa de
// sessão (a partir da fase 01, o shell exige auth — CA-17a).
// Fase 13: a home agora é `/` (Dashboard absorveu o /hoje — CA-04/05); o login
// cai em `/`, não mais em `/hoje` (exceção mecânica sancionada, plan 13 §3).
export async function loginViaStub(page: Page) {
  await page.goto("/login");
  await page.getByRole("link", { name: /entrar com google/i }).click();
  await expect(page).toHaveURL(/\/$/);
}

// waitTaskPatch (fase 24): promessa que resolve quando UM PATCH de task retorna
// 2xx. O popup central dispara um PATCH POR CAMPO, fire-and-forget (nome no
// blur/Enter, status/assignee/etc. no chip/select). Editar DOIS campos em sequência
// rápida deixa dois PATCHes em voo na MESMA task e o refetch de convergência do
// board (invalidate → GET /board) pode assentar refletindo só um deles (CA-20) —
// flake só sob concorrência (passa isolado). Serializar no teste (esperar o PATCH
// do 1º campo ANTES de mexer no 2º) espelha o uso real — o usuário muda o status
// DEPOIS do nome assentar — e torna a convergência determinística. Casa só PATCH de
// task de topo (/tasks/{uuid}[?...]), não sub-task (/tasks/{uuid}/subtasks/…). Uso:
//   const p = waitTaskPatch(page); await <ação que dispara o PATCH>; await p;
export function waitTaskPatch(page: Page) {
  return page.waitForResponse(
    (r) =>
      r.request().method() === "PATCH" &&
      /\/tasks\/[0-9a-fA-F-]+(\?|$)/.test(r.url()) &&
      r.ok(),
  );
}

// Semeia um segundo membro no workspace pessoal do usuário do stub
// (diogo.fixture@example.com). Não existe API de convite no MVP — assim
// como nos testes Go (addMember), membership extra nasce por SQL direto
// no Postgres de teste (container timetrack-db-test da :5433). Idempotente
// (ON CONFLICT), seguro pra rodar em todo teste.
export function seedWorkspaceMember(opts: {
  googleId: string;
  name: string;
  email: string;
  role: "member" | "viewer";
}) {
  const sql = `
    -- guard: explode ALTO se o user do stub ainda não logou (1/0 sem
    -- constant-folding: o divisor vem de subquery)
    SELECT 1 / (SELECT count(*)::int FROM users
                 WHERE email = 'diogo.fixture@example.com') AS fixture_user_guard;
    WITH dono AS (
      SELECT m.workspace_id AS id
        FROM workspace_members m
        JOIN users u ON u.id = m.user_id
       WHERE u.email = 'diogo.fixture@example.com' AND m.role = 'owner'
       LIMIT 1
    ), membro AS (
      INSERT INTO users (google_id, email, name)
      VALUES ('${opts.googleId}', '${opts.email}', '${opts.name}')
      ON CONFLICT (google_id) DO UPDATE SET name = EXCLUDED.name
      RETURNING id
    )
    INSERT INTO workspace_members (workspace_id, user_id, role)
    SELECT dono.id, membro.id, '${opts.role}' FROM dono, membro
    ON CONFLICT DO NOTHING;`;
  execFileSync("docker", [
    "compose",
    "-f",
    path.resolve(e2eDir, "../../docker-compose.dev.yml"),
    "exec",
    "-T",
    "timetrack-db-test",
    "psql",
    "-U",
    "postgres",
    "-d",
    "timetrack_test",
    "-v",
    "ON_ERROR_STOP=1",
    "-c",
    sql,
  ]);
}

// seedInvitedLogin (fase 17, T-11/CA-35): simula o 1º LOGIN do email convidado
// SEM precisar logar um 2º usuário no googlestub (que só autentica
// diogo.fixture@example.com). Materializa o user do convidado E roda a MESMA
// resolução de convites do callback de auth (invitations.ResolveForEmail):
// cria a membership a partir dos convites pending válidos do email e marca os
// convites accepted. Assim o E2E prova o caminho convite→1º-login→membership
// ponta a ponta (limitação do stub documentada no cabeçalho do spec). Roda
// DEPOIS de o owner ter criado o convite.
export function seedInvitedLogin(opts: {
  googleId: string;
  name: string;
  email: string;
}) {
  const email = opts.email.toLowerCase();
  const sql = `
    -- guard: o owner do stub precisa existir (e ter criado o convite)
    SELECT 1 / (SELECT count(*)::int FROM users
                 WHERE email = 'diogo.fixture@example.com') AS fixture_user_guard;
    -- 1º login do convidado: cria o user (espelha o UpsertUserFromGoogle).
    WITH novo AS (
      INSERT INTO users (google_id, email, name)
      VALUES ('${opts.googleId}', '${email}', '${opts.name}')
      ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name
      RETURNING id
    )
    -- resolução dos convites pending válidos do email (ResolveForEmail):
    , inv AS (
      SELECT id, workspace_id, role FROM invitations
       WHERE lower(email) = '${email}'
         AND status = 'pending'
         AND (expires_at IS NULL OR expires_at > now())
       FOR UPDATE
    ), ins AS (
      INSERT INTO workspace_members (workspace_id, user_id, role)
      SELECT inv.workspace_id, novo.id, inv.role FROM inv, novo
      ON CONFLICT (workspace_id, user_id) DO NOTHING
    )
    UPDATE invitations SET status = 'accepted'
     WHERE id IN (SELECT id FROM inv);`;
  runSql(sql);
}

// Reseta a conexão do Google Calendar do usuário do stub (fase 06): zera os
// scopes pra base (sem calendar.readonly), deixando o status DESCONECTADO.
// O db-test é PERSISTENTE e o user do stub é único — sem isto, a 1ª conexão
// do calendario.spec gruda e o teste vira flaky (sem CTA na 2ª rodada).
// Idempotente; roda DEPOIS do login (o user já existe). NÃO deleta o token
// (o connect depois faz merge e re-adiciona o calendar.readonly).
export function resetCalendarConnection() {
  const sql = `
    SELECT 1 / (SELECT count(*)::int FROM users
                 WHERE email = 'diogo.fixture@example.com') AS fixture_user_guard;
    UPDATE oauth_tokens
       SET scopes = ARRAY['openid','email','profile']
     WHERE user_id = (SELECT id FROM users WHERE email = 'diogo.fixture@example.com');`;
  execFileSync("docker", [
    "compose",
    "-f",
    path.resolve(e2eDir, "../../docker-compose.dev.yml"),
    "exec",
    "-T",
    "timetrack-db-test",
    "psql",
    "-U",
    "postgres",
    "-d",
    "timetrack_test",
    "-v",
    "ON_ERROR_STOP=1",
    "-c",
    sql,
  ]);
}

// resetCalendarSettings (fase 11): zera as preferências de calendário do
// usuário do stub (db-test persistente). Sem a linha, o GET volta aos defaults
// (default_view='week'), deixando o calendario.spec SEMPRE começar na semana —
// senão o run anterior (que salva 'day') faz o próximo abrir no Dia.
export function resetCalendarSettings() {
  const sql = `
    SELECT 1 / (SELECT count(*)::int FROM users
                 WHERE email = 'diogo.fixture@example.com') AS fixture_user_guard;
    DELETE FROM calendar_settings
     WHERE user_id = (SELECT id FROM users WHERE email = 'diogo.fixture@example.com');`;
  runSql(sql);
}

// seedReminder (fase 20): semeia um projeto + task + 1 reminder MANUAL pending no
// workspace pessoal do usuário do stub. when='past' (default) põe remind_at no
// passado (now() - 1min) → o worker (REMINDER_TICK curto no E2E) despacha no 1º
// tick (notice in-app + push); when='future' deixa o reminder pendente (não
// dispara) pra o teste de listar/cancelar em "Meus lembretes". O nome da task
// (único) isola o assert. Roda DEPOIS do login. O guard 1/0 explode ALTO se o
// stub ainda não logou.
export function seedReminder(taskName: string, when: "past" | "future" = "past") {
  const at = when === "future" ? "now() + interval '7 days'" : "now() - interval '1 minute'";
  const sql = `
    SELECT 1 / (SELECT count(*)::int FROM users
                 WHERE email = 'diogo.fixture@example.com') AS fixture_user_guard;
    WITH alvo AS (
      SELECT u.id AS user_id, w.id AS ws_id
        FROM users u
        JOIN workspace_members m ON m.user_id = u.id AND m.role = 'owner'
        JOIN workspaces w ON w.id = m.workspace_id
       WHERE u.email = 'diogo.fixture@example.com'
       ORDER BY w.created_at
       LIMIT 1
    ), proj AS (
      INSERT INTO projects (workspace_id, owner_user_id, name, color, visibility)
      SELECT ws_id, user_id, 'Lembretes E2E', '#4f9dde', 'shared' FROM alvo
      RETURNING id, workspace_id
    ), task AS (
      INSERT INTO tasks (workspace_id, project_id, name, created_by)
      SELECT proj.workspace_id, proj.id, '${taskName}', alvo.user_id FROM proj, alvo
      RETURNING id, workspace_id
    )
    INSERT INTO reminders (user_id, task_id, workspace_id, remind_at, kind, status)
    SELECT alvo.user_id, task.id, task.workspace_id, ${at}, 'manual', 'pending'
      FROM task, alvo;`;
  runSql(sql);
}

// runSql roda SQL arbitrário no Postgres de teste (container db-test, :5433).
function runSql(sql: string) {
  execFileSync("docker", [
    "compose",
    "-f",
    path.resolve(e2eDir, "../../docker-compose.dev.yml"),
    "exec",
    "-T",
    "timetrack-db-test",
    "psql",
    "-U",
    "postgres",
    "-d",
    "timetrack_test",
    "-v",
    "ON_ERROR_STOP=1",
    "-c",
    sql,
  ]);
}

// seedForgottenTimer (fase 16): semeia uma entry RODANDO começada há >2h
// (started_at = now() - interval) no workspace pessoal do usuário do stub, com
// a linha de timer_states correspondente — simula o timer esquecido ligado. O
// worker de corte (GUARD_TICK curto no E2E) vai fechá-la no marco das 2h. A
// descrição única (sufixo) isola o assert. Roda DEPOIS do login (o user já
// existe). Limpa qualquer entry rodando/estado anterior do user antes (o
// db-test é reusado).
export function seedForgottenTimer(description: string) {
  const sql = `
    SELECT 1 / (SELECT count(*)::int FROM users
                 WHERE email = 'diogo.fixture@example.com') AS fixture_user_guard;
    WITH alvo AS (
      -- o workspace pessoal = o 1º do /me (ORDER BY created_at — load-bearing,
      -- fase 10): o assert do spec usa me.workspaces[0], então a entry precisa
      -- nascer NESSE workspace, não no de menor uuid.
      SELECT u.id AS user_id, w.id AS ws_id
        FROM users u
        JOIN workspace_members m ON m.user_id = u.id AND m.role = 'owner'
        JOIN workspaces w ON w.id = m.workspace_id
       WHERE u.email = 'diogo.fixture@example.com'
       ORDER BY w.created_at
       LIMIT 1
    ), limpa_estado AS (
      DELETE FROM timer_states
       WHERE user_id = (SELECT user_id FROM alvo)
    ), limpa_rodando AS (
      UPDATE time_entries
         SET stopped_at = started_at + interval '1 second', duration_seconds = 1
       WHERE user_id = (SELECT user_id FROM alvo) AND stopped_at IS NULL
    ), nova AS (
      INSERT INTO time_entries (workspace_id, user_id, description, started_at, source)
      SELECT ws_id, user_id, '${description}', now() - interval '3 hours', 'timer' FROM alvo
      RETURNING id, user_id, started_at
    )
    INSERT INTO timer_states (user_id, mode, entry_id, started_at)
    SELECT user_id, 'countup', id, started_at FROM nova;`;
  runSql(sql);
}

// seedColleagueRunning (fase 21): semeia um COLEGA (member) no workspace
// pessoal do usuário do stub rodando uma entry num projeto SHARED — pro snapshot
// GET /workspaces/{id}/presence listar esse colega pro usuário logado (a
// cobertura A→B no nível do snapshot, já que o stub só autentica 1 usuário —
// CA-22; o delta cross-user fica nos testes de integração Go). Idempotente.
// Roda DEPOIS do login (o user do stub já existe).
export function seedColleagueRunning(opts: {
  googleId: string;
  name: string;
  email: string;
  projectName: string;
}) {
  // Statements SEPARADOS (não um CTE só): o db-test é PERSISTENTE, então o
  // colega/projeto/entry podem já existir de uma rodada anterior. Fechar a entry
  // rodando ANTES do INSERT precisa ser commitado pra não bater no
  // uq_entries_one_running (CTEs veem o mesmo snapshot). Idempotente.
  const sql = `
    SELECT 1 / (SELECT count(*)::int FROM users
                 WHERE email = 'diogo.fixture@example.com') AS fixture_user_guard;
    -- 1) colega (member do ws pessoal do stub)
    INSERT INTO users (google_id, email, name)
    VALUES ('${opts.googleId}', '${opts.email}', '${opts.name}')
    ON CONFLICT (google_id) DO UPDATE SET name = EXCLUDED.name;
    INSERT INTO workspace_members (workspace_id, user_id, role)
    SELECT w.id, c.id, 'member'
      FROM users u
      JOIN workspace_members m ON m.user_id = u.id AND m.role = 'owner'
      JOIN workspaces w ON w.id = m.workspace_id
      JOIN users c ON c.google_id = '${opts.googleId}'
     WHERE u.email = 'diogo.fixture@example.com'
     ORDER BY w.created_at LIMIT 1
    ON CONFLICT DO NOTHING;
    -- 2) fecha qualquer entry rodando do colega (COMMIT antes do INSERT novo)
    UPDATE time_entries
       SET stopped_at = started_at + interval '1 second', duration_seconds = 1
     WHERE user_id = (SELECT id FROM users WHERE google_id = '${opts.googleId}')
       AND stopped_at IS NULL;
    -- 3) projeto shared do colega (reusa por nome; idempotente)
    INSERT INTO projects (workspace_id, owner_user_id, name, color, visibility)
    SELECT w.id, c.id, '${opts.projectName}', '#4f9dde', 'shared'
      FROM users u
      JOIN workspace_members m ON m.user_id = u.id AND m.role = 'owner'
      JOIN workspaces w ON w.id = m.workspace_id
      JOIN users c ON c.google_id = '${opts.googleId}'
     WHERE u.email = 'diogo.fixture@example.com'
       AND NOT EXISTS (
         SELECT 1 FROM projects p
          WHERE p.workspace_id = w.id AND p.name = '${opts.projectName}' AND p.owner_user_id = c.id)
     ORDER BY w.created_at LIMIT 1;
    -- 4) entry RODANDO do colega no projeto shared
    INSERT INTO time_entries (workspace_id, user_id, project_id, description, started_at, source)
    SELECT w.id, c.id, p.id, '', now() - interval '7 minutes', 'timer'
      FROM users u
      JOIN workspace_members m ON m.user_id = u.id AND m.role = 'owner'
      JOIN workspaces w ON w.id = m.workspace_id
      JOIN users c ON c.google_id = '${opts.googleId}'
      JOIN projects p ON p.workspace_id = w.id AND p.name = '${opts.projectName}' AND p.owner_user_id = c.id
     WHERE u.email = 'diogo.fixture@example.com'
     ORDER BY w.created_at LIMIT 1;`;
  runSql(sql);
}

// Semeia um SEGUNDO workspace pro usuário do stub (fase 10): o seletor de
// workspace ativo só aparece com >1 no /me. Idempotente por nome fixo; o
// pessoal segue workspaces[0] (ordem por created_at — plan da 10 §6).
// seedProjectVivo (fase 28, CA-22): cria um projeto SHARED com deadline (último
// dia do mês corrente) + 1 task com due_date HOJE + 1 entry FECHADA (3600s) no
// workspace pessoal do usuário do stub. Prova os cards vivos (horas reais ≠ 0h,
// "X pendentes", dot "hoje"), o Upcoming da Overview e os itens do Calendar
// (deadline do projeto + due da task). Não há UI de editar deadline (set por SQL,
// como os outros seeds). Roda DEPOIS do login. Nome único isola o assert.
export function seedProjectVivo(opts: { project: string; task: string }) {
  // due_date = HOJE no fuso LOCAL do host (não o current_date do db-test, que é
  // UTC). O Summary calcula due_today com time.Now().In(tz-do-browser); o browser
  // do Playwright e o servidor Go herdam o fuso do host. Entre 21h–24h BRT o
  // current_date (UTC) já virou o dia seguinte e divergiria do "hoje" do Summary
  // → o dot "due hoje" sumia (flake tz pré-existente da fase 28). Computar o hoje
  // local aqui torna o teste determinístico em qualquer horário.
  const d = new Date();
  const localToday = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(
    d.getDate(),
  ).padStart(2, '0')}`;
  const sql = `
    SELECT 1 / (SELECT count(*)::int FROM users
                 WHERE email = 'diogo.fixture@example.com') AS fixture_user_guard;
    WITH alvo AS (
      SELECT u.id AS user_id, w.id AS ws_id
        FROM users u
        JOIN workspace_members m ON m.user_id = u.id AND m.role = 'owner'
        JOIN workspaces w ON w.id = m.workspace_id
       WHERE u.email = 'diogo.fixture@example.com'
       ORDER BY w.created_at
       LIMIT 1
    ), proj AS (
      INSERT INTO projects (workspace_id, owner_user_id, name, color, visibility, deadline)
      SELECT ws_id, user_id, '${opts.project}', '#4f9dde', 'shared',
             (date_trunc('month', current_date) + interval '1 month - 1 day')::date
        FROM alvo
      RETURNING id, workspace_id, owner_user_id
    ), task AS (
      INSERT INTO tasks (workspace_id, project_id, name, created_by, status, due_date)
      SELECT proj.workspace_id, proj.id, '${opts.task}', alvo.user_id, 'doing', '${localToday}'::date
        FROM proj, alvo
      RETURNING id, workspace_id, project_id
    )
    INSERT INTO time_entries (workspace_id, user_id, project_id, started_at, stopped_at, duration_seconds, source)
    SELECT task.workspace_id, alvo.user_id, task.project_id,
           now() - interval '2 hours', now() - interval '1 hour', 3600, 'manual'
      FROM task, alvo;`;
  runSql(sql);
}

export function seedSecondWorkspace(name: string) {
  const sql = `
    -- guard: explode ALTO se o user do stub ainda não logou (1/0 sem
    -- constant-folding: o divisor vem de subquery)
    SELECT 1 / (SELECT count(*)::int FROM users
                 WHERE email = 'diogo.fixture@example.com') AS fixture_user_guard;
    WITH alvo AS (
      SELECT id AS user_id FROM users WHERE email = 'diogo.fixture@example.com'
    ), novo AS (
      INSERT INTO workspaces (name)
      SELECT '${name}'
      WHERE NOT EXISTS (SELECT 1 FROM workspaces WHERE name = '${name}')
      RETURNING id
    ), ws AS (
      SELECT id FROM novo
      UNION ALL
      SELECT id FROM workspaces WHERE name = '${name}'
      LIMIT 1
    )
    INSERT INTO workspace_members (workspace_id, user_id, role)
    SELECT ws.id, alvo.user_id, 'owner' FROM ws, alvo
    ON CONFLICT DO NOTHING;`;
  execFileSync("docker", [
    "compose",
    "-f",
    path.resolve(e2eDir, "../../docker-compose.dev.yml"),
    "exec",
    "-T",
    "timetrack-db-test",
    "psql",
    "-U",
    "postgres",
    "-d",
    "timetrack_test",
    "-v",
    "ON_ERROR_STOP=1",
    "-c",
    sql,
  ]);
}
