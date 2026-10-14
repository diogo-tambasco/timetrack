// board.go — fase 12 T-05: read agregado único do projeto (CA-16/16.1/17).
// 3 queries fixas + 1 de metadados (D-05, sem N+1): objetivos, tasks, soma de
// tempo por task. Os buckets projeto×task são INDEPENDENTES (D-06): a soma por
// task é por task_id ∈ tasks do projeto (SEM filtrar project_id da entry); a do
// projeto é por project_id. project.can_write é computado pelo gate (CA-22).
package objectives

import (
	"context"
	"encoding/json"
	"math"
	"time"
)

// Board é a árvore que o /board devolve num único request (CA-16).
type Board struct {
	Project    BoardProject     `json:"project"`
	Objectives []BoardObjective `json:"objectives"`
	General    BoardGeneral     `json:"general"`
}

// BoardPropertyDef = o schema de uma propriedade custom do projeto (fase 25, CA-16).
// Tipo LOCAL do pacote objectives (D-06: board faz SQL inline, NÃO importa properties
// — sem ciclo de import). options sai cru (jsonb) e o cliente resolve o select (D-08).
type BoardPropertyDef struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	Options  json.RawMessage `json:"options"`
	Position int             `json:"position"`
}

// BoardPropertyValue = o valor de uma def numa task (fase 25, CA-16). Só as defs
// COM value vão pra task (CA-17 — defs sem value são derivadas no cliente).
type BoardPropertyValue struct {
	DefID string          `json:"def_id"`
	Value json.RawMessage `json:"value"`
}

// BoardProject = metadados do projeto + tempo total (por project_id) +
// can_write (gating do front — CA-22).
type BoardProject struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Color           string     `json:"color"`
	ClientName      *string    `json:"client_name"` // fase 24 (CA-14.1): LEFT JOIN clients; null sem cliente
	WeeklyHoursGoal *float64   `json:"weekly_hours_goal"`
	ArchivedAt      *time.Time `json:"archived_at"`
	Deadline        *string    `json:"deadline"` // fase 28 (CA-01): "YYYY-MM-DD" ou null; campo aditivo (RNF-05)
	TrackedSeconds  int64      `json:"tracked_seconds"`
	CanWrite        bool       `json:"can_write"`
	// fase 25 (CA-16): schema de propriedades custom do projeto, ordenado por position. Aditivo (RNF-06).
	PropertyDefs []BoardPropertyDef `json:"property_defs"`
}

// BoardObjective = objetivo + progresso (CA-16.1) + suas tasks. Fase 29:
// CompletionMode/TargetDate (aditivo, CA-10) + ProgressPct por modo (CA-10.1).
type BoardObjective struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	Description    string      `json:"description"`
	Status         string      `json:"status"`
	GoalHours      *float64    `json:"goal_hours"`
	CompletionMode string      `json:"completion_mode"` // fase 29 (CA-10)
	TargetDate     *string     `json:"target_date"`     // fase 29 (CA-10)
	Position       int         `json:"position"`
	TaskCount      int         `json:"task_count"`
	DoneCount      int         `json:"done_count"`
	DonePct        int         `json:"done_pct"`
	ProgressPct    int         `json:"progress_pct"` // fase 29 (CA-10.1, D-05): por modo
	TrackedSeconds int64       `json:"tracked_seconds"`
	Tasks          []BoardTask `json:"tasks"`
}

// BoardGeneral = bucket "Geral" das tasks sem objetivo (CA-11).
type BoardGeneral struct {
	TaskCount      int         `json:"task_count"`
	DoneCount      int         `json:"done_count"`
	TrackedSeconds int64       `json:"tracked_seconds"`
	Tasks          []BoardTask `json:"tasks"`
}

// BoardTask = a task da fase 02 + objective_id + tracked_seconds (CA-16) +
// priority/notion_url/position (fase 24, colunas de tasks) + agregados de
// sub-task (CA-14, query única — sem N+1).
type BoardTask struct {
	ID             string    `json:"id"`
	WorkspaceID    string    `json:"workspace_id"`
	ProjectID      string    `json:"project_id"`
	ObjectiveID    *string   `json:"objective_id"`
	Name           string    `json:"name"`
	Status         string    `json:"status"`
	Priority       int       `json:"priority"`
	NotionURL      *string   `json:"notion_url"`
	Position       int       `json:"position"`
	DueDate        *string   `json:"due_date"`
	Recurrence     *string   `json:"recurrence"`
	Notes          string    `json:"notes"`
	CreatedBy      string    `json:"created_by"`
	Assignee       *string   `json:"assignee"`
	RequestedBy    *string   `json:"requested_by"`
	CreatedAt      time.Time `json:"created_at"`
	TrackedSeconds int64     `json:"tracked_seconds"`
	SubtaskCount   int       `json:"subtask_count"`
	SubtaskDone    int       `json:"subtask_done"`
	GoalCount      int       `json:"goal_count"`
	GoalDone       int       `json:"goal_done"`
	// fase 25 (CA-16/17): só as defs COM value pra esta task ([] quando nenhuma). Aditivo (RNF-06).
	PropertyValues []BoardPropertyValue `json:"property_values"`
}

// Board monta a árvore do projeto. Visibilidade pelo gate (404 se invisível —
// CA-13); can_write = (dono ∥ owner-ws ∥ access=edit) E não arquivado (CA-22).
func (s *Service) Board(ctx context.Context, workspaceID, userID, role, projectID string) (Board, error) {
	g, err := s.loadProjectGate(ctx, workspaceID, userID, projectID)
	if err != nil {
		return Board{}, err
	}

	var b Board
	b.Objectives = []BoardObjective{}
	b.General.Tasks = []BoardTask{}

	// metadados do projeto + tempo total (por project_id, incl. entries sem task — CA-17)
	// + client_name via LEFT JOIN clients (CA-14.1; null sem cliente).
	if err := s.db.QueryRow(ctx, `
		SELECT p.id, p.name, p.color, c.name, p.weekly_hours_goal, p.archived_at,
		       to_char(p.deadline, 'YYYY-MM-DD'),
		       COALESCE((SELECT SUM(e.duration_seconds) FROM time_entries e
		                  WHERE e.workspace_id = $1 AND e.user_id = $2
		                    AND e.project_id = p.id AND e.duration_seconds IS NOT NULL), 0)
		  FROM projects p
		  LEFT JOIN clients c ON c.id = p.client_id
		 WHERE p.id = $3`,
		workspaceID, userID, projectID).Scan(
		&b.Project.ID, &b.Project.Name, &b.Project.Color, &b.Project.ClientName, &b.Project.WeeklyHoursGoal,
		&b.Project.ArchivedAt, &b.Project.Deadline, &b.Project.TrackedSeconds); err != nil {
		return Board{}, err
	}
	b.Project.CanWrite = g.canWrite(userID, role) && !g.archived

	// soma de tempo por task: task_id ∈ tasks do projeto, SEM filtrar project_id
	// da entry (D-06 — entry com task do projeto mas project_id nulo/diverso conta).
	tracked := map[string]int64{}
	rows, err := s.db.Query(ctx, `
		SELECT e.task_id, SUM(e.duration_seconds)
		  FROM time_entries e
		 WHERE e.workspace_id = $1 AND e.user_id = $2 AND e.duration_seconds IS NOT NULL
		   AND e.task_id IN (SELECT id FROM tasks WHERE workspace_id = $1 AND project_id = $3)
		 GROUP BY e.task_id`, workspaceID, userID, projectID)
	if err != nil {
		return Board{}, err
	}
	for rows.Next() {
		var taskID string
		var sum int64
		if err := rows.Scan(&taskID, &sum); err != nil {
			rows.Close()
			return Board{}, err
		}
		tracked[taskID] = sum
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Board{}, err
	}

	// objetivos do projeto, ordenados (CA-03)
	byObjective := map[string]*BoardObjective{}
	objRows, err := s.db.Query(ctx, `
		SELECT id, name, description, status, goal_hours, completion_mode,
		       to_char(target_date, 'YYYY-MM-DD'), position
		  FROM objectives WHERE workspace_id = $1 AND project_id = $2
		 ORDER BY position, created_at, id`, workspaceID, projectID)
	if err != nil {
		return Board{}, err
	}
	for objRows.Next() {
		var o BoardObjective
		o.Tasks = []BoardTask{}
		if err := objRows.Scan(&o.ID, &o.Name, &o.Description, &o.Status, &o.GoalHours,
			&o.CompletionMode, &o.TargetDate, &o.Position); err != nil {
			objRows.Close()
			return Board{}, err
		}
		b.Objectives = append(b.Objectives, o)
	}
	objRows.Close()
	if err := objRows.Err(); err != nil {
		return Board{}, err
	}
	for i := range b.Objectives {
		byObjective[b.Objectives[i].ID] = &b.Objectives[i]
	}

	// agregados de sub-task por task do projeto (CA-14, query ÚNICA — sem N+1).
	// task ausente do resultado → contagens 0 (mapa zero-value).
	type subAgg struct{ count, done, goalCount, goalDone int }
	subByTask := map[string]subAgg{}
	subRows, err := s.db.Query(ctx, `
		SELECT task_id,
		       count(*),
		       count(*) FILTER (WHERE done),
		       count(*) FILTER (WHERE is_goal),
		       count(*) FILTER (WHERE is_goal AND done)
		  FROM task_subtasks
		 WHERE task_id IN (SELECT id FROM tasks WHERE workspace_id = $1 AND project_id = $2)
		 GROUP BY task_id`, workspaceID, projectID)
	if err != nil {
		return Board{}, err
	}
	for subRows.Next() {
		var taskID string
		var a subAgg
		if err := subRows.Scan(&taskID, &a.count, &a.done, &a.goalCount, &a.goalDone); err != nil {
			subRows.Close()
			return Board{}, err
		}
		subByTask[taskID] = a
	}
	subRows.Close()
	if err := subRows.Err(); err != nil {
		return Board{}, err
	}

	// fase 25 (CA-16/RNF-02): schema de propriedades do projeto (1 query) + valores
	// por task (1 query, WHERE def_id = ANY($defs)), agrupados em memória — sem N+1.
	b.Project.PropertyDefs = []BoardPropertyDef{}
	defIDs := []string{}
	selectValid := map[string]map[string]bool{} // defID select → set de option.id válidos (CA-13)
	defType := map[string]string{}
	defRows, err := s.db.Query(ctx, `
		SELECT id, name, type, options, position
		  FROM project_property_defs
		 WHERE workspace_id = $1 AND project_id = $2
		 ORDER BY position, id`, workspaceID, projectID)
	if err != nil {
		return Board{}, err
	}
	for defRows.Next() {
		var d BoardPropertyDef
		var opts []byte
		if err := defRows.Scan(&d.ID, &d.Name, &d.Type, &opts, &d.Position); err != nil {
			defRows.Close()
			return Board{}, err
		}
		if len(opts) == 0 {
			opts = []byte("[]")
		}
		d.Options = json.RawMessage(opts)
		b.Project.PropertyDefs = append(b.Project.PropertyDefs, d)
		defIDs = append(defIDs, d.ID)
		defType[d.ID] = d.Type
		if d.Type == "select" {
			var parsed []struct {
				ID string `json:"id"`
			}
			_ = json.Unmarshal(opts, &parsed)
			set := map[string]bool{}
			for _, o := range parsed {
				set[o.ID] = true
			}
			selectValid[d.ID] = set
		}
	}
	defRows.Close()
	if err := defRows.Err(); err != nil {
		return Board{}, err
	}

	// valores por task: 1 query (def_id ∈ defs do projeto), map[task_id][]value.
	// value de select cujo option.id sumiu de def.options → OMITIDO (CA-13/D-11).
	valsByTask := map[string][]BoardPropertyValue{}
	if len(defIDs) > 0 {
		valRows, err := s.db.Query(ctx, `
			SELECT task_id, def_id, value
			  FROM task_property_values
			 WHERE workspace_id = $1 AND def_id = ANY($2)`, workspaceID, defIDs)
		if err != nil {
			return Board{}, err
		}
		for valRows.Next() {
			var taskID, defID string
			var raw []byte
			if err := valRows.Scan(&taskID, &defID, &raw); err != nil {
				valRows.Close()
				return Board{}, err
			}
			if defType[defID] == "select" {
				var sel string
				if json.Unmarshal(raw, &sel) != nil || !selectValid[defID][sel] {
					continue // value órfão de option → tratado como limpo na leitura (CA-13)
				}
			}
			valsByTask[taskID] = append(valsByTask[taskID], BoardPropertyValue{DefID: defID, Value: json.RawMessage(raw)})
		}
		valRows.Close()
		if err := valRows.Err(); err != nil {
			return Board{}, err
		}
	}

	// tasks do projeto, distribuídas nos objetivos / Geral. ORDER BY position
	// (CA-14): o resultado sai ordenado por position (desempate created_at, id) e o
	// front agrupa por status preservando a ordem → cada bucket fica position-asc.
	taskRows, err := s.db.Query(ctx, `
		SELECT id, workspace_id, project_id, objective_id, name, status,
		       priority, notion_url, position,
		       to_char(due_date, 'YYYY-MM-DD'), recurrence, notes,
		       created_by, assignee, requested_by, created_at
		  FROM tasks WHERE workspace_id = $1 AND project_id = $2
		 ORDER BY position, created_at, id`, workspaceID, projectID)
	if err != nil {
		return Board{}, err
	}
	for taskRows.Next() {
		var t BoardTask
		if err := taskRows.Scan(&t.ID, &t.WorkspaceID, &t.ProjectID, &t.ObjectiveID, &t.Name,
			&t.Status, &t.Priority, &t.NotionURL, &t.Position,
			&t.DueDate, &t.Recurrence, &t.Notes, &t.CreatedBy, &t.Assignee,
			&t.RequestedBy, &t.CreatedAt); err != nil {
			taskRows.Close()
			return Board{}, err
		}
		t.TrackedSeconds = tracked[t.ID]
		if a, ok := subByTask[t.ID]; ok {
			t.SubtaskCount, t.SubtaskDone, t.GoalCount, t.GoalDone = a.count, a.done, a.goalCount, a.goalDone
		}
		t.PropertyValues = valsByTask[t.ID]
		if t.PropertyValues == nil {
			t.PropertyValues = []BoardPropertyValue{} // CA-17: task sem value → [] (não null)
		}
		if t.ObjectiveID != nil {
			if o := byObjective[*t.ObjectiveID]; o != nil {
				o.Tasks = append(o.Tasks, t)
				o.TaskCount++
				o.TrackedSeconds += t.TrackedSeconds
				if t.Status == "done" {
					o.DoneCount++
				}
				continue
			}
			// objective_id que não está na lista do projeto não deveria existir
			// (FK + filtro por projeto), mas se acontecer cai no Geral por segurança.
		}
		b.General.Tasks = append(b.General.Tasks, t)
		b.General.TaskCount++
		b.General.TrackedSeconds += t.TrackedSeconds
		if t.Status == "done" {
			b.General.DoneCount++
		}
	}
	taskRows.Close()
	if err := taskRows.Err(); err != nil {
		return Board{}, err
	}

	for i := range b.Objectives {
		o := &b.Objectives[i]
		o.DonePct = donePct(o.DoneCount, o.TaskCount)
		// fase 29 (CA-10.1): progresso por modo, sem query nova.
		o.ProgressPct = progressByMode(o.CompletionMode, o.DonePct, o.TrackedSeconds, o.GoalHours)
	}
	return b, nil
}

// donePct = round(done/count*100); 0 quando count=0 (CA-16.1).
func donePct(done, count int) int {
	if count == 0 {
		return 0
	}
	return int(math.Round(float64(done) / float64(count) * 100))
}

// progressByMode (fase 29, função PURA — CA-10.1): progresso coerente com o
// modo de conclusão. by_tasks/by_date → done_pct; by_hours → % de horas vs meta
// (round(tracked/(goal*3600)*100)) clampado [0,100], 0 se goalHours nulo (sem
// div/0). Espelha exatamente o cálculo de progress_pct do contrato (§4.2).
func progressByMode(mode string, donePct int, trackedSeconds int64, goalHours *float64) int {
	if mode != "by_hours" {
		return donePct // by_tasks e by_date usam o progresso de tasks
	}
	if goalHours == nil || *goalHours <= 0 {
		return 0
	}
	pct := int(math.Round(float64(trackedSeconds) / (*goalHours * 3600) * 100))
	if pct < 0 {
		return 0
	}
	if pct > 100 {
		return 100
	}
	return pct
}
