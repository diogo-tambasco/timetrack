// Cliente HTTP fino do Timetrack (nasce na fase 01; as fases 02/03
// estendem): base /api/v1, cookies de sessão inclusos e envelope de erro
// canônico {"error":{code,message}} virando exceção tipada.
import type { Presence } from "./realtime"; // fase 21 (type-only — sem ciclo de runtime)

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

type ErrorEnvelope = { error?: { code?: string; message?: string } };

export async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`/api/v1${path}`, {
    credentials: "include",
    ...init,
  });

  if (res.status === 204) return undefined as T;

  const body = (await res.json().catch(() => null)) as unknown;
  if (!res.ok) {
    const env = (body ?? {}) as ErrorEnvelope;
    throw new ApiError(
      res.status,
      env.error?.code ?? "unknown",
      env.error?.message ?? `HTTP ${res.status}`,
    );
  }
  return body as T;
}

export type WorkspaceRole = "owner" | "member" | "viewer";

export type Me = {
  user: { id: string; email: string; name: string; avatar_url: string | null };
  workspaces: { id: string; name: string; role: WorkspaceRole }[];
};

export const getMe = () => request<Me>("/me");

// detalhe do workspace (§3.5 da fase 01 + members, extensão aditiva da
// fase 02): fonte da seção "Membros do projeto" do modal (CA-28)
export type WorkspaceMember = {
  user_id: string;
  name: string;
  email: string;
  role: WorkspaceRole;
};

export type WorkspaceDetail = {
  id: string;
  name: string;
  role: WorkspaceRole;
  created_at: string;
  members: WorkspaceMember[];
};

export const getWorkspace = (ws: string) =>
  request<{ workspace: WorkspaceDetail }>(`/workspaces/${ws}`).then(
    (r) => r.workspace,
  );

// fase 21: snapshot inicial de presence — os membros do ws com timer rodando
// agora (cada um um Presence). Semeia o presenceStore no mount/reconexão.
export const getWorkspacePresence = (ws: string) =>
  request<{ members: Presence[] }>(`/workspaces/${ws}/presence`).then(
    (r) => r.members,
  );

// ——— fase 17: convites + gestão de membros (plan §4) ———
// O token NUNCA vem no corpo (RNF-04) — não está no tipo.
export type InviteRole = "member" | "viewer";

export type Invitation = {
  id: string;
  email: string;
  role: InviteRole;
  status: "pending" | "accepted" | "revoked";
  expires_at: string | null;
  created_at: string;
};

export const listInvitations = (ws: string) =>
  request<{ invitations: Invitation[] }>(`/workspaces/${ws}/invitations`).then(
    (r) => r.invitations,
  );

export const createInvitation = (
  ws: string,
  body: { email: string; role: InviteRole },
) => request<Invitation>(`/workspaces/${ws}/invitations`, json("POST", body));

export const revokeInvitation = (ws: string, id: string) =>
  request<void>(`/workspaces/${ws}/invitations/${id}`, { method: "DELETE" });

export const addMember = (
  ws: string,
  body: { email: string; role: InviteRole },
) => request<WorkspaceMember>(`/workspaces/${ws}/members`, json("POST", body));

export const removeMember = (ws: string, userId: string) =>
  request<void>(`/workspaces/${ws}/members/${userId}`, { method: "DELETE" });

export const changeRole = (ws: string, userId: string, role: WorkspaceRole) =>
  request<{ user_id: string; role: WorkspaceRole }>(
    `/workspaces/${ws}/members/${userId}`,
    json("PATCH", { role }),
  );

export const logout = () => request<void>("/auth/logout", { method: "POST" });

// ——— fase 02: domínio core (contratos do plan.md §3) ———

export type Client = {
  id: string;
  workspace_id: string;
  name: string;
  archived_at: string | null;
  created_at: string;
};

export type ProjectAccess = "edit" | "view";
export type ProjectVisibility = "private" | "shared";

export type ProjectMember = { user_id: string; access: ProjectAccess };

export type Project = {
  id: string;
  workspace_id: string;
  client_id: string | null;
  name: string;
  color: string;
  weekly_hours_goal: number | null;
  visibility: ProjectVisibility;
  owner_user_id: string;
  archived_at: string | null;
  created_at: string;
  // fase 28: prazo do projeto (marco local) — null = sem prazo (CA-01/02).
  deadline: string | null;
};

// o detalhe (e o 201 do POST) embute members; a listagem não
export type ProjectDetail = Project & { members: ProjectMember[] };

// fase 24: status ganha 'backlog' (4ª coluna, antes de todo).
export type TaskStatus = "backlog" | "todo" | "doing" | "done";

export type Task = {
  id: string;
  workspace_id: string;
  project_id: string;
  // fase 12: vínculo opcional com um objetivo do projeto (null = "Geral")
  objective_id: string | null;
  name: string;
  status: TaskStatus;
  // fase 24: prioridade 0..3, link de doc (Notion), ordenação manual (DnD)
  priority: number;
  notion_url: string | null;
  position: number;
  due_date: string | null;
  recurrence: string | null;
  notes: string;
  created_by: string;
  assignee: string | null;
  requested_by: string | null;
  created_at: string;
};

// fase 24: sub-task "goal da task" (tabela task_subtasks).
export type Subtask = {
  id: string;
  task_id: string;
  title: string;
  done: boolean;
  is_goal: boolean;
  position: number;
  created_at: string;
};

export type SubtaskInput = {
  title?: string;
  done?: boolean;
  is_goal?: boolean;
  position?: number;
};

const json = (method: string, body: unknown): RequestInit => ({
  method,
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify(body),
});

const qs = (params: Record<string, string | undefined>) => {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined) search.set(key, value);
  }
  const out = search.toString();
  return out ? `?${out}` : "";
};

const wsPath = (ws: string, rest: string) => `/workspaces/${ws}${rest}`;

// — clientes (plan §3.1) —

export const listClients = (ws: string, opts?: { archived?: boolean }) =>
  request<{ clients: Client[] }>(
    wsPath(
      ws,
      `/clients${qs({ archived: opts?.archived ? "true" : undefined })}`,
    ),
  ).then((r) => r.clients);

export const createClient = (ws: string, input: { name: string }) =>
  request<Client>(wsPath(ws, "/clients"), json("POST", input));

export const renameClient = (ws: string, id: string, input: { name: string }) =>
  request<Client>(wsPath(ws, `/clients/${id}`), json("PATCH", input));

export const archiveClient = (ws: string, id: string) =>
  request<void>(wsPath(ws, `/clients/${id}`), { method: "DELETE" });

// — projetos (plan §3.2) —

export type ProjectInput = {
  name: string;
  color: string;
  visibility: ProjectVisibility;
  client_id?: string | null;
  weekly_hours_goal?: number | null;
  members?: ProjectMember[];
  // fase 28: tri-state no PATCH (ausente mantém, null remove, data define).
  deadline?: string | null;
};

// fase 28: card vivo da lista de Projetos (GET .../projects/summary).
export type ProjectSummary = {
  id: string;
  name: string;
  color: string;
  client_id: string | null;
  deadline: string | null;
  task_count: number;
  done_count: number;
  open_count: number;
  tracked_seconds: number;
  due_today_count: number;
};

// fase 28: item datado do calendário do projeto.
export type ProjectCalendarItem = {
  kind: "task" | "objective" | "project_deadline";
  id: string;
  name: string;
  date: string; // YYYY-MM-DD
  status: string | null;
};

export type ProjectCalendar = {
  items: ProjectCalendarItem[];
  from: string;
  to: string;
};

export const listProjects = (ws: string, opts?: { archived?: boolean }) =>
  request<{ projects: Project[] }>(
    wsPath(
      ws,
      `/projects${qs({ archived: opts?.archived ? "true" : undefined })}`,
    ),
  ).then((r) => r.projects);

export const getProject = (ws: string, id: string) =>
  request<ProjectDetail>(wsPath(ws, `/projects/${id}`));

export const createProject = (ws: string, input: ProjectInput) =>
  request<ProjectDetail>(wsPath(ws, "/projects"), json("POST", input));

export const patchProject = (
  ws: string,
  id: string,
  patch: Partial<ProjectInput>,
) => request<Project>(wsPath(ws, `/projects/${id}`), json("PATCH", patch));

export const archiveProject = (ws: string, id: string) =>
  request<void>(wsPath(ws, `/projects/${id}`), { method: "DELETE" });

export const replaceProjectMembers = (
  ws: string,
  id: string,
  members: ProjectMember[],
) =>
  request<{ members: ProjectMember[] }>(
    wsPath(ws, `/projects/${id}/members`),
    json("PUT", { members }),
  ).then((r) => r.members);

// fase 28: cards vivos — 1 request com os contadores leves de todos os projetos.
export const getProjectsSummary = (
  ws: string,
  opts?: { tz?: string; archived?: boolean },
) =>
  request<{ projects: ProjectSummary[] }>(
    wsPath(
      ws,
      `/projects/summary${qs({
        tz: opts?.tz,
        archived: opts?.archived ? "true" : undefined,
      })}`,
    ),
  ).then((r) => r.projects);

// fase 28: itens datados do calendário do projeto (?month ou ?from/?to).
export const getProjectCalendar = (
  ws: string,
  id: string,
  opts: { month?: string; from?: string; to?: string; tz?: string },
) =>
  request<ProjectCalendar>(
    wsPath(
      ws,
      `/projects/${id}/calendar${qs({
        month: opts.month,
        from: opts.from,
        to: opts.to,
        tz: opts.tz,
      })}`,
    ),
  );

// — tasks (plan §3.3) —

export type TaskInput = {
  project_id: string;
  name: string;
  // fase 12: objective_id (null/ausente = bucket "Geral")
  objective_id?: string | null;
  // fase 24: status/priority na criação + notion_url (tri-state no PATCH)
  status?: TaskStatus;
  priority?: number;
  notion_url?: string | null;
  position?: number;
  due_date?: string | null;
  recurrence?: string | null;
  notes?: string;
  // fase 18: cross-assign (null/ausente = sem responsável/solicitante)
  assignee?: string | null;
  requested_by?: string | null;
};

// fase 18: item das visões cross-workspace ("Minhas tasks"/"Pedidas por mim") —
// a Task + o contexto do projeto/workspace/cliente (que cruzam tenants).
export type AssignedTask = Task & {
  project: { id: string; name: string; color: string };
  workspace: { id: string; name: string };
  client: string | null;
};

export const getAssignedTasks = () =>
  request<{ tasks: AssignedTask[] }>("/tasks/assigned").then((r) => r.tasks);

export const getRequestedTasks = () =>
  request<{ tasks: AssignedTask[] }>("/tasks/requested").then((r) => r.tasks);

export type TaskFilters = {
  project_id?: string;
  status?: TaskStatus;
  objective_id?: string;
};

export const listTasks = (ws: string, filters?: TaskFilters) =>
  request<{ tasks: Task[] }>(
    wsPath(
      ws,
      `/tasks${qs({
        project_id: filters?.project_id,
        status: filters?.status,
        objective_id: filters?.objective_id,
      })}`,
    ),
  ).then((r) => r.tasks);

export const createTask = (ws: string, input: TaskInput) =>
  request<Task>(wsPath(ws, "/tasks"), json("POST", input));

export const patchTask = (
  ws: string,
  id: string,
  patch: Partial<TaskInput> & { status?: TaskStatus },
) => request<Task>(wsPath(ws, `/tasks/${id}`), json("PATCH", patch));

export const deleteTask = (ws: string, id: string) =>
  request<void>(wsPath(ws, `/tasks/${id}`), { method: "DELETE" });

// — sub-tasks "goals da task" (fase 24, plan §4.1) —

export const listSubtasks = (ws: string, taskId: string) =>
  request<{ subtasks: Subtask[] }>(
    wsPath(ws, `/tasks/${taskId}/subtasks`),
  ).then((r) => r.subtasks);

export const createSubtask = (ws: string, taskId: string, title: string) =>
  request<Subtask>(
    wsPath(ws, `/tasks/${taskId}/subtasks`),
    json("POST", { title }),
  );

export const patchSubtask = (
  ws: string,
  taskId: string,
  subtaskId: string,
  patch: SubtaskInput,
) =>
  request<Subtask>(
    wsPath(ws, `/tasks/${taskId}/subtasks/${subtaskId}`),
    json("PATCH", patch),
  );

export const deleteSubtask = (ws: string, taskId: string, subtaskId: string) =>
  request<void>(wsPath(ws, `/tasks/${taskId}/subtasks/${subtaskId}`), {
    method: "DELETE",
  });

// — comentários (thread em tasks, fase 19, plan §6) —

export type Comment = {
  id: string;
  task_id: string;
  workspace_id: string;
  author_id: string;
  author_name: string;
  body: string;
  created_at: string;
};

export const listComments = (ws: string, task: string) =>
  request<{ comments: Comment[] }>(
    wsPath(ws, `/tasks/${task}/comments`),
  ).then((r) => r.comments);

export const createComment = (ws: string, task: string, body: string) =>
  request<Comment>(wsPath(ws, `/tasks/${task}/comments`), json("POST", { body }));

export const deleteComment = (ws: string, task: string, commentId: string) =>
  request<void>(wsPath(ws, `/tasks/${task}/comments/${commentId}`), {
    method: "DELETE",
  });

// — notas nativas .md (fase 26, plan §4) —

export type NoteLink = {
  id: string;
  ref_type: "task" | "note";
  ref_id: string;
  label: string;
  status?: string; // só p/ ref_type=task
  position: number;
};

export type Note = {
  id: string;
  workspace_id: string;
  project_id: string | null;
  task_id: string | null;
  title: string;
  body_md: string;
  preview?: string; // só na LISTA
  links: NoteLink[];
  created_by: string;
  created_at: string;
  updated_at: string;
};

export type CreateNoteInput = { project_id?: string; task_id?: string; title?: string };
export type PatchNoteInput = { title?: string; body_md?: string };

export type LinkResult = {
  link: NoteLink;
  task?: { id: string; project_id: string; name: string; status: string };
  note?: Note;
};

// projectId vazio/undefined → notas SOLTAS do user (CA-03.1).
export const listNotes = (ws: string, projectId?: string) =>
  request<{ notes: Note[] }>(
    wsPath(ws, `/notes${qs({ project_id: projectId || undefined })}`),
  ).then((r) => r.notes);

export const getNote = (ws: string, id: string) =>
  request<Note>(wsPath(ws, `/notes/${id}`));

export const createNote = (ws: string, input: CreateNoteInput) =>
  request<Note>(wsPath(ws, `/notes`), json("POST", input));

export const patchNote = (ws: string, id: string, input: PatchNoteInput) =>
  request<Note>(wsPath(ws, `/notes/${id}`), json("PATCH", input));

export const deleteNote = (ws: string, id: string) =>
  request<void>(wsPath(ws, `/notes/${id}`), { method: "DELETE" });

export const createLink = (
  ws: string,
  noteId: string,
  input: { kind: "task" | "note"; name?: string; title?: string },
) => request<LinkResult>(wsPath(ws, `/notes/${noteId}/links`), json("POST", input));

export const deleteLink = (ws: string, noteId: string, linkId: string) =>
  request<void>(wsPath(ws, `/notes/${noteId}/links/${linkId}`), { method: "DELETE" });

// — lembretes (due/manual, fase 20, plan §4) —

export type ReminderStatus = "pending" | "sent" | "canceled";

export type Reminder = {
  id: string;
  user_id: string;
  task_id: string;
  workspace_id: string;
  remind_at: string;
  kind: "due" | "manual";
  status: ReminderStatus;
  task_name?: string;
};

// GET /api/v1/reminders — os meus, cross-workspace (por usuário).
export const listReminders = (status?: ReminderStatus) =>
  request<{ reminders: Reminder[] }>(
    `/reminders${status ? `?status=${status}` : ""}`,
  ).then((r) => r.reminders);

export const createReminder = (
  ws: string,
  task: string,
  remind_at: string,
) =>
  request<Reminder>(
    wsPath(ws, `/tasks/${task}/reminders`),
    json("POST", { remind_at }),
  );

export const cancelReminder = (ws: string, task: string, reminderId: string) =>
  request<void>(wsPath(ws, `/tasks/${task}/reminders/${reminderId}`), {
    method: "DELETE",
  });

// ——— fase 03: timer / entries / settings (contratos do plan.md §3) ———

export type EntrySource = "timer" | "manual" | "calendar" | "claude";

export type TimeEntry = {
  id: string;
  workspace_id: string;
  project_id: string | null;
  task_id: string | null;
  description: string;
  tags: string[];
  started_at: string;
  stopped_at: string | null;
  duration_seconds: number | null;
  source: EntrySource;
};

// state.mode usa os valores da API ('pomodoro_focus' nunca vaza aqui)...
export type TimerMode = "countup" | "countdown" | "pomodoro" | "break";

// ...mas resume.mode expõe o valor INTERNO ('pomodoro_focus' incluso): é
// por ele que o front decide o auto-resume do ciclo (CA-34.1, plan §3.1).
export type TimerResume = {
  entry_id: string;
  mode: "countup" | "countdown" | "pomodoro_focus";
  target_seconds: number | null;
};

// break_kind ("short"|"long") rotula a pausa (CA-06.1 da fase 15) — vem
// PERSISTIDO do backend (break_is_long), só presente quando mode==='break'.
export type BreakKind = "short" | "long";

export type TimerState = {
  mode: TimerMode;
  entry: TimeEntry | null;
  elapsed_seconds: number;
  target_seconds: number | null;
  remaining_seconds: number | null;
  resume?: TimerResume;
  break_kind?: BreakKind;
};

// Result canônico: corpo de TODA rota de timer, inclusive o GET (RNF-05).
export type TimerResult = {
  state: TimerState | null;
  stopped_entry?: TimeEntry;
  server_time: string;
};

export type StartTimerInput = {
  workspace_id: string;
  description?: string;
  project_id?: string | null;
  task_id?: string | null;
  tags?: string[];
  mode?: "countup" | "countdown" | "pomodoro";
  target_seconds?: number;
};

// fase 27 (allowance HITL 23/06): edita SÓ description/project_id/tags da
// entry RODANDO via PATCH /timer (timing intocado — CA-13.1). project_id é
// tri-state (string | null | ausente). Devolve o mesmo TimerResult.
export type EditTimerInput = {
  description?: string;
  project_id?: string | null;
  tags?: string[];
};

export const getTimer = () => request<TimerResult>("/timer");
export const editTimer = (input: EditTimerInput) =>
  request<TimerResult>("/timer", json("PATCH", input));
export const startTimer = (input: StartTimerInput) =>
  request<TimerResult>("/timer/start", json("POST", input));
export const stopTimer = () =>
  request<TimerResult>("/timer/stop", { method: "POST" });
export const pauseTimer = (breakMinutes?: number) =>
  request<TimerResult>(
    "/timer/pause",
    breakMinutes === undefined
      ? { method: "POST" }
      : json("POST", { break_minutes: breakMinutes }),
  );
export const resumeTimer = () =>
  request<TimerResult>("/timer/resume", { method: "POST" });
export const skipBreak = () =>
  request<TimerResult>("/timer/break/skip", { method: "POST" });

// Pomodoro 2.0 (fase 15): além de foco/pausa, ciclos até a pausa longa,
// duração da pausa longa, auto-start (pausa e foco) e alerta sonoro. O GET
// sempre devolve os 7 campos; o PUT manda o objeto completo (CA-02).
export type PomodoroSettings = {
  focus_minutes: number;
  break_minutes: number;
  cycles_before_long: number;
  long_break_minutes: number;
  auto_start_break: boolean;
  auto_start_focus: boolean;
  sound_enabled: boolean;
};

export const getPomodoroSettings = () =>
  request<PomodoroSettings>("/settings/pomodoro");
export const putPomodoroSettings = (s: PomodoroSettings) =>
  request<PomodoroSettings>("/settings/pomodoro", json("PUT", s));

export type EntriesList = { entries: TimeEntry[]; total_seconds: number };

export const listEntries = (
  ws: string,
  opts?: { from?: string; to?: string; tz?: string },
) =>
  request<EntriesList>(
    wsPath(
      ws,
      `/entries${qs({ from: opts?.from, to: opts?.to, tz: opts?.tz })}`,
    ),
  );

export type EntryInput = {
  description?: string;
  project_id?: string | null;
  task_id?: string | null;
  tags?: string[];
  started_at: string;
  stopped_at: string;
};

export const createEntry = (ws: string, input: EntryInput) =>
  request<TimeEntry>(wsPath(ws, "/entries"), json("POST", input));

export const patchEntry = (
  ws: string,
  id: string,
  patch: Partial<EntryInput>,
) => request<TimeEntry>(wsPath(ws, `/entries/${id}`), json("PATCH", patch));

export const deleteEntry = (ws: string, id: string) =>
  request<void>(wsPath(ws, `/entries/${id}`), { method: "DELETE" });

export const continueEntry = (ws: string, id: string) =>
  request<TimerResult>(wsPath(ws, `/entries/${id}/continue`), {
    method: "POST",
  });

// ——— fase 07: relatório semanal (contrato §3.1 do plan) ———

export type ReportProjectBucket = {
  project_id: string | null;
  name: string | null;
  color: string | null;
  client_id: string | null;
  client_name: string | null;
  total_seconds: number;
};

export type ReportFocus = {
  focus_seconds: number;
  fragment_seconds: number;
  focus_count: number;
  fragment_count: number;
  longest_seconds: number;
  switches: number;
};

export type ReportGoal = {
  project_id: string;
  name: string;
  color: string;
  goal_seconds: number;
  realized_seconds: number;
};

export type WeekReport = {
  week_start: string;
  week_end: string;
  total_seconds: number;
  by_project: ReportProjectBucket[];
  focus: ReportFocus;
  goals: ReportGoal[];
};

export const getWeekReport = (
  ws: string,
  opts?: { date?: string; tz?: string },
) =>
  request<WeekReport>(
    wsPath(ws, `/reports/week${qs({ date: opts?.date, tz: opts?.tz })}`),
  );

// ——— fase 14: relatório de período genérico (contrato §3.1) ———

export type ReportObjective = {
  project_id: string;
  project_name: string;
  project_color: string;
  objective_id: string;
  objective_name: string;
  status: ObjectiveStatus;
  tasks_total: number;
  tasks_done: number;
  progress_pct: number;
  realized_seconds: number;
  goal_seconds?: number;
};

export type ReportAvailability = {
  tracked_seconds: number;
  expected_seconds: number;
  untracked_seconds: number;
  tracked_pct: number;
  expected_basis: string;
};

export type FocusHourBucket = { hour: number; focus_seconds: number };

export type ReportKpis = {
  tracked_seconds: number;
  deep_focus_pct: number;
  avg_session_seconds: number;
  untracked_seconds: number;
};

export type ReportPrevious = { total_seconds: number; kpis: ReportKpis };

export type PeriodReport = {
  from: string;
  to: string;
  total_seconds: number;
  by_project: ReportProjectBucket[];
  focus: ReportFocus;
  goals: ReportGoal[];
  objectives: ReportObjective[];
  availability: ReportAvailability;
  focus_by_hour: FocusHourBucket[];
  kpis: ReportKpis;
  previous?: ReportPrevious | null;
};

export type ReportPeriod = "day" | "week" | "month";

export type ReportParams = {
  period?: ReportPeriod;
  date?: string;
  from?: string;
  to?: string;
  tz?: string;
  project_id?: string;
  client_id?: string;
  objective_id?: string;
  compare?: "previous";
};

export const getReport = (ws: string, params: ReportParams) =>
  request<PeriodReport>(
    wsPath(
      ws,
      `/reports${qs({
        period: params.period,
        date: params.date,
        from: params.from,
        to: params.to,
        tz: params.tz,
        project_id: params.project_id,
        client_id: params.client_id,
        objective_id: params.objective_id,
        compare: params.compare,
      })}`,
    ),
  );

export type ReportSettings = {
  work_hours_per_day: number;
  workdays_only: boolean;
};

export const getReportSettings = () =>
  request<ReportSettings>("/settings/reports");
export const putReportSettings = (s: ReportSettings) =>
  request<ReportSettings>("/settings/reports", json("PUT", s));

// ——— fase 06: Google Calendar (leitura) ———

// caminho de navegação full-page do connect (OAuth incremental) — NÃO via
// request() (é um redirect 302 de browser, não JSON)
export const CALENDAR_CONNECT_PATH = "/api/v1/calendar/connect";

export type CalendarStatus = { connected: boolean };

export type CalendarEvent = {
  id: string;
  summary: string;
  start: string; // ISO
  end: string; // ISO
  all_day: boolean;
  html_link: string;
};

export const getCalendarStatus = () =>
  request<CalendarStatus>("/calendar/status");

export const getCalendarEvents = (opts: {
  from: string;
  to: string;
  tz: string;
}) =>
  request<{ events: CalendarEvent[] }>(
    `/calendar/events${qs({ from: opts.from, to: opts.to, tz: opts.tz })}`,
  ).then((r) => r.events);

// ângulo 4 dos relatórios (Planejado × Real)
export type PlannedDay = {
  date: string;
  planned_seconds: number;
  realized_seconds: number;
};

export type PlannedReport = {
  week_start: string;
  week_end: string;
  connected: boolean;
  days: PlannedDay[];
};

export const getPlannedReport = (
  ws: string,
  opts?: { date?: string; tz?: string },
) =>
  request<PlannedReport>(
    wsPath(ws, `/reports/planned${qs({ date: opts?.date, tz: opts?.tz })}`),
  );

// ——— fase 11: preferências da UI de calendário (por usuário, sincroniza devices) ———

export type CalendarSettings = {
  default_view: "day" | "week" | "month";
  hour_start: number;
  hour_end: number;
  density: "comfortable" | "compact";
};

export const getCalendarSettings = () =>
  request<CalendarSettings>("/settings/calendar");
export const putCalendarSettings = (s: CalendarSettings) =>
  request<CalendarSettings>("/settings/calendar", json("PUT", s));

// ——— fase 12: objetivos + board (gerenciador de tarefas) ———

export type ObjectiveStatus = "open" | "done";

// fase 29: modo de conclusão do objetivo (Goal). by_tasks = % de tasks done
// (comportamento da fase 12, default); by_hours = tracked vs goal_hours;
// by_date = target_date é marco (fecho manual, alimenta o calendário da fase 28).
export type CompletionMode = "by_tasks" | "by_hours" | "by_date";

export type Objective = {
  id: string;
  workspace_id: string;
  project_id: string;
  name: string;
  description: string;
  status: ObjectiveStatus;
  goal_hours: number | null;
  // fase 29 (CA-10): modo de conclusão + marco. completion_mode sempre presente
  // (default by_tasks no backend); target_date YYYY-MM-DD ou null.
  completion_mode: CompletionMode;
  target_date: string | null;
  position: number;
  created_by: string;
  created_at: string;
};

export type ObjectiveInput = {
  name: string;
  description?: string;
  goal_hours?: number | null;
  // fase 29: ausente → by_tasks (default no backend); target_date só em by_date
  completion_mode?: CompletionMode;
  target_date?: string | null;
};

// goal_hours: null REMOVE a meta (tri-state — ausente mantém)
export type ObjectivePatch = {
  name?: string;
  description?: string;
  status?: ObjectiveStatus;
  goal_hours?: number | null;
  position?: number;
  // fase 29 tri-state: completion_mode ausente mantém o modo; target_date ausente
  // mantém, null limpa o marco, valor define (CA-03.1/04).
  completion_mode?: CompletionMode;
  target_date?: string | null;
};

// a task do board = a Task da fase 02 + tempo agregado (CA-16) + agregados de
// sub-task (fase 24, CA-14)
export type BoardTask = Task & {
  tracked_seconds: number;
  subtask_count: number;
  subtask_done: number;
  goal_count: number;
  goal_done: number;
  // fase 25 (CA-16/17): só as defs COM value pra esta task ([] quando nenhuma).
  property_values: PropertyValue[];
};

export type BoardObjective = {
  id: string;
  name: string;
  description: string;
  status: ObjectiveStatus;
  goal_hours: number | null;
  position: number;
  task_count: number;
  done_count: number;
  done_pct: number;
  tracked_seconds: number;
  // fase 29 (CA-10/10.1): modo + marco + progresso coerente com o modo.
  // progress_pct: by_tasks→done_pct; by_hours→round(tracked/(goal*3600)*100)
  // clampado [0,100] (0 sem meta); by_date→done_pct. Sempre presente no board.
  completion_mode: CompletionMode;
  target_date: string | null;
  progress_pct: number;
  tasks: BoardTask[];
};

export type BoardGeneral = {
  task_count: number;
  done_count: number;
  tracked_seconds: number;
  tasks: BoardTask[];
};

export type BoardProject = {
  id: string;
  name: string;
  color: string;
  // fase 24 (CA-14.1): nome do cliente via LEFT JOIN; null sem cliente
  client_name: string | null;
  weekly_hours_goal: number | null;
  archived_at: string | null;
  // fase 28: prazo do projeto (campo aditivo no board — CA-01); null = sem prazo
  deadline: string | null;
  tracked_seconds: number;
  // can_write: permissão de escrita do user da sessão NESTE projeto (CA-22 —
  // gating do front; a role do workspace sozinha não distingue access=view/edit)
  can_write: boolean;
  // fase 25 (CA-16): schema de propriedades custom do projeto, ordenado por position.
  property_defs: PropertyDef[];
};

export type Board = {
  project: BoardProject;
  objectives: BoardObjective[];
  general: BoardGeneral;
};

export const getBoard = (ws: string, projectId: string) =>
  request<Board>(wsPath(ws, `/projects/${projectId}/board`));

// fase 25: propriedades tipadas custom por projeto ("database por projeto").
// v1 = text·number·select·date·url·checkbox (person/multi_select = backlog).
export type PropertyType =
  | "text"
  | "number"
  | "select"
  | "date"
  | "url"
  | "checkbox";

// option de um select: id estável (o value guarda o id, não o label — D-08), label,
// cor opcional (token de paleta ou hex via lib/palette).
export type PropertyOption = { id: string; label: string; color: string | null };
// na criação/edição o id/color são opcionais (o servidor gera o id faltante — CA-03).
export type PropertyOptionInput = {
  id?: string;
  label: string;
  color?: string | null;
};

export type PropertyDef = {
  id: string;
  project_id: string;
  workspace_id: string;
  name: string;
  type: PropertyType;
  options: PropertyOption[];
  position: number;
};

// o value chega tipado conforme def.type (string/number/boolean); null nunca é
// materializado (a ausência = ausência de linha — CA-09.1/17).
export type PropertyValue = { def_id: string; value: unknown };

export type CreatePropertyInput = {
  name: string;
  type: PropertyType;
  options?: PropertyOptionInput[];
};
export type PatchPropertyInput = {
  name?: string;
  position?: number;
  options?: PropertyOptionInput[];
};

export const listProperties = (ws: string, projectId: string) =>
  request<{ properties: PropertyDef[] }>(
    wsPath(ws, `/projects/${projectId}/properties`),
  ).then((r) => r.properties);

export const createProperty = (
  ws: string,
  projectId: string,
  input: CreatePropertyInput,
) =>
  request<PropertyDef>(
    wsPath(ws, `/projects/${projectId}/properties`),
    json("POST", input),
  );

export const patchProperty = (
  ws: string,
  projectId: string,
  defId: string,
  patch: PatchPropertyInput,
) =>
  request<PropertyDef>(
    wsPath(ws, `/projects/${projectId}/properties/${defId}`),
    json("PATCH", patch),
  );

export const deleteProperty = (ws: string, projectId: string, defId: string) =>
  request<void>(wsPath(ws, `/projects/${projectId}/properties/${defId}`), {
    method: "DELETE",
  });

// setValue: upsert do value (200) OU clear quando value === null (204 — CA-09.1).
export const setPropertyValue = (
  ws: string,
  projectId: string,
  taskId: string,
  defId: string,
  value: unknown,
) =>
  request<{ def_id: string; value: unknown } | void>(
    wsPath(ws, `/projects/${projectId}/tasks/${taskId}/properties/${defId}`),
    json("PUT", { value }),
  );

export const clearPropertyValue = (
  ws: string,
  projectId: string,
  taskId: string,
  defId: string,
) =>
  request<void>(
    wsPath(ws, `/projects/${projectId}/tasks/${taskId}/properties/${defId}`),
    { method: "DELETE" },
  );

export const listObjectives = (ws: string, projectId: string) =>
  request<{ objectives: Objective[] }>(
    wsPath(ws, `/projects/${projectId}/objectives`),
  ).then((r) => r.objectives);

export const createObjective = (
  ws: string,
  projectId: string,
  input: ObjectiveInput,
) =>
  request<Objective>(
    wsPath(ws, `/projects/${projectId}/objectives`),
    json("POST", input),
  );

export const patchObjective = (
  ws: string,
  projectId: string,
  objectiveId: string,
  patch: ObjectivePatch,
) =>
  request<Objective>(
    wsPath(ws, `/projects/${projectId}/objectives/${objectiveId}`),
    json("PATCH", patch),
  );

export const deleteObjective = (
  ws: string,
  projectId: string,
  objectiveId: string,
) =>
  request<void>(
    wsPath(ws, `/projects/${projectId}/objectives/${objectiveId}`),
    {
      method: "DELETE",
    },
  );

// ----- Fase 16: Web Push (CA-13/14) -----

export type PushKey = { key: string };

/** VAPID public key pra montar a subscription (vazia → push desligado). */
export const getPushKey = () => request<PushKey>("/push/key");

export type PushSubscribeInput = {
  endpoint: string;
  keys: { p256dh: string; auth: string };
};

/** persiste a subscription do navegador sob o user da sessão. */
export const postPushSubscribe = (input: PushSubscribeInput) =>
  request<{ ok: boolean }>("/push/subscribe", json("POST", input));
