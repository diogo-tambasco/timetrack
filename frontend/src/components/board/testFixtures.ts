// testFixtures.ts (fase 24): fábricas de Board/BoardTask completas (com os
// campos novos da fase 24) pros specs jsdom da aba Tasks. Reusado por
// ProjectShell / TaskBoard / TaskPanel / TasksTab specs.
import type { Board, BoardObjective, BoardProject, BoardTask } from '../../lib/api';
import { PROJECT_PALETTE } from '../../lib/palette';

export function mkTask(over: Partial<BoardTask> = {}): BoardTask {
  return {
    id: 't1',
    workspace_id: 'w1',
    project_id: 'p1',
    objective_id: null,
    name: 'Task',
    status: 'todo',
    priority: 0,
    notion_url: null,
    position: 0,
    due_date: null,
    recurrence: null,
    notes: '',
    created_by: 'u1',
    assignee: null,
    requested_by: null,
    created_at: '2026-06-04T10:00:00Z',
    tracked_seconds: 0,
    subtask_count: 0,
    subtask_done: 0,
    goal_count: 0,
    goal_done: 0,
    property_values: [], // fase 25
    ...over,
  };
}

// fase 29: fábrica de BoardObjective com os campos de modo defaultados (by_tasks,
// sem marco, progress_pct=done_pct). Specs que precisam de objetivos no board usam.
export function mkObjective(over: Partial<BoardObjective> = {}): BoardObjective {
  const done_pct = over.done_pct ?? 0;
  return {
    id: 'o1',
    name: 'Goal',
    description: '',
    status: 'open',
    goal_hours: null,
    position: 0,
    task_count: 0,
    done_count: 0,
    done_pct,
    tracked_seconds: 0,
    completion_mode: 'by_tasks',
    target_date: null,
    progress_pct: done_pct,
    tasks: [],
    ...over,
  };
}

export function mkProject(over: Partial<BoardProject> = {}): BoardProject {
  return {
    id: 'p1',
    name: 'App',
    color: PROJECT_PALETTE[0],
    client_name: null,
    weekly_hours_goal: null,
    archived_at: null,
    deadline: null,
    tracked_seconds: 7200,
    can_write: true,
    property_defs: [], // fase 25
    ...over,
  };
}

export function mkBoard(over: Partial<Board> = {}): Board {
  const objectives: BoardObjective[] = over.objectives ?? [];
  const general = over.general ?? {
    task_count: 4,
    done_count: 1,
    tracked_seconds: 0,
    tasks: [
      mkTask({ id: 'b1', name: 'Backlog item', status: 'backlog', position: 0 }),
      mkTask({ id: 't-todo', name: 'To do item', status: 'todo', position: 0 }),
      mkTask({ id: 't-doing', name: 'Doing item', status: 'doing', position: 0 }),
      mkTask({ id: 't-done', name: 'Done item', status: 'done', position: 0 }),
    ],
  };
  return {
    project: over.project ?? mkProject(),
    objectives,
    general,
  };
}
