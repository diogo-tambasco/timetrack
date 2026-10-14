package notes

import (
	"context"

	"timetrack.live/backend/internal/domain"
)

// LinkResult é a resposta do POST /notes/{id}/links (CA-07/08): o link criado +
// a entidade (task OU sub-nota). Só um dos dois ponteiros vem preenchido.
type LinkResult struct {
	Link NoteLink       `json:"link"`
	Task *tasksTaskView `json:"task,omitempty"`
	Note *Note          `json:"note,omitempty"`
}

// tasksTaskView é a projeção mínima da task criada pelo slash (evita expor o
// objeto inteiro do pacote tasks no contrato de notes). Espelha os campos que o
// front usa (CA-21).
type tasksTaskView struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
}

// CreateLink cria a entidade (task via tasks.CreateTx OU sub-nota) + o note_link
// numa ÚNICA transação (D-04 — atômico, sem órfão). kind:
//   - "task": project_id da nota NULL → ROLLBACK + Validation (CA-10); senão
//     tasks.CreateTx no project_id da nota (status default 'todo' — coord. C).
//   - "note": cria sub-nota (mesmo project_id da mãe, ou NULL se mãe solta —
//     CA-10).
//   - outro → ROLLBACK + Validation (CA-08.1).
//
// position do link = max(position)+1. Falha em qualquer passo → rollback total.
func (s *Service) CreateLink(ctx context.Context, workspaceID, userID, role, noteID, kind, name string) (LinkResult, error) {
	if kind != "task" && kind != "note" {
		return LinkResult{}, domain.Validation("kind deve ser task ou note")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return LinkResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// carrega a nota-mãe pelo gate de ESCRITA (blockArchived=true).
	g, err := s.loadNoteGate(ctx, tx, workspaceID, userID, role, noteID, true)
	if err != nil {
		return LinkResult{}, err
	}

	var refID string
	var res LinkResult
	switch kind {
	case "task":
		if g.ProjectID == nil {
			// nota solta não tem projeto pra ancorar a task (CA-10).
			return LinkResult{}, domain.Validation("nota solta não tem projeto para a task")
		}
		task, err := s.tasks.CreateTx(ctx, tx, workspaceID, userID, role, *g.ProjectID, name)
		if err != nil {
			return LinkResult{}, err
		}
		refID = task.ID
		res.Task = &tasksTaskView{ID: task.ID, ProjectID: task.ProjectID, Name: task.Name, Status: task.Status}
	case "note":
		// sub-nota: mesmo project_id da mãe (NULL se mãe solta — CA-10).
		title, err := validTitle(name)
		if err != nil {
			return LinkResult{}, err
		}
		sub, err := scanNote(tx.QueryRow(ctx, `
			INSERT INTO notes (workspace_id, project_id, title, body_md, created_by)
			VALUES ($1, $2, $3, '', $4)
			RETURNING `+noteCols, workspaceID, g.ProjectID, title, userID))
		if err != nil {
			return LinkResult{}, err
		}
		refID = sub.ID
		res.Note = &sub
	}

	// position = max+1 dos links DESTA nota; INSERT do note_link.
	var pos int
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(max(position), -1) + 1 FROM note_links WHERE note_id = $1`, noteID).Scan(&pos); err != nil {
		return LinkResult{}, err
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO note_links (note_id, ref_type, ref_id, position, workspace_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, ref_type, ref_id, position`,
		noteID, kind, refID, pos, workspaceID).Scan(&res.Link.ID, &res.Link.RefType, &res.Link.RefID, &res.Link.Position); err != nil {
		return LinkResult{}, err
	}
	if res.Task != nil {
		res.Link.Label = res.Task.Name
		res.Link.Status = res.Task.Status
	} else if res.Note != nil {
		res.Link.Label = res.Note.Title
	}

	if err := tx.Commit(ctx); err != nil {
		return LinkResult{}, err
	}
	return res, nil
}

// DeleteLink remove SÓ o note_link pelo id do link (CA-09; a task/sub-nota
// sobrevive). Exige escrita na nota-mãe. Link inexistente / linkID malformado /
// link de outra nota → 404 (forma de rota ÚNICA por {linkId} — 26-H6).
func (s *Service) DeleteLink(ctx context.Context, workspaceID, userID, role, noteID, linkID string) error {
	if pgxValidUUID(linkID) != nil {
		return domain.NotFound("link não encontrado")
	}
	// escrita na nota-mãe (gate de Delete-de-conteúdo usa blockArchived=true
	// como Patch: remover um link É escrita de conteúdo da nota; espelha o
	// precedente — porém o link de uma nota arquivada some pela resolução).
	// Mantemos blockArchived=true por simetria com a escrita de conteúdo.
	if _, err := s.loadNoteGate(ctx, s.db, workspaceID, userID, role, noteID, true); err != nil {
		return err
	}
	ct, err := s.db.Exec(ctx, `
		DELETE FROM note_links WHERE id = $1 AND note_id = $2 AND workspace_id = $3`,
		linkID, noteID, workspaceID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return domain.NotFound("link não encontrado")
	}
	return nil
}

// resolveLinks preenche o array Links de cada nota em `list`, resolvendo cada
// note_link com a visibilidade do user POR TIPO de alvo (CA-12/D-07/26-H2),
// sem N+1: 1 query batch dos task-links + 1 dos note-links de toda a página.
// Link cujo alvo sumiu (deletado) ou ficou invisível é OMITIDO (não vaza o
// nome). Implementação completa em T-05 (este arquivo).
func (s *Service) resolveLinks(ctx context.Context, workspaceID, userID string, list []Note) error {
	if len(list) == 0 {
		return nil
	}
	noteIDs := make([]string, 0, len(list))
	idx := make(map[string]int, len(list))
	for i := range list {
		if list[i].Links == nil {
			list[i].Links = []NoteLink{}
		}
		noteIDs = append(noteIDs, list[i].ID)
		idx[list[i].ID] = i
	}

	// task-links: resolve label/status pela regra de TASK (project shared OR
	// owner_user_id=$u). Task private de outro → linha ausente no JOIN →
	// omitido. Mutação removendo a cláusula de visibilidade vaza o nome.
	taskRows, err := s.db.Query(ctx, `
		SELECT nl.note_id, nl.id, nl.ref_type, nl.ref_id, nl.position, t.name, t.status
		  FROM note_links nl
		  JOIN tasks t    ON t.id = nl.ref_id
		  JOIN projects p ON p.id = t.project_id
		   AND (p.visibility = 'shared' OR p.owner_user_id = $2)
		 WHERE nl.workspace_id = $1 AND nl.ref_type = 'task'
		   AND nl.note_id = ANY($3)
		 ORDER BY nl.note_id, nl.position, nl.id`,
		workspaceID, userID, noteIDs)
	if err != nil {
		return err
	}
	for taskRows.Next() {
		var noteID string
		var l NoteLink
		if err := taskRows.Scan(&noteID, &l.ID, &l.RefType, &l.RefID, &l.Position, &l.Label, &l.Status); err != nil {
			taskRows.Close()
			return err
		}
		if i, ok := idx[noteID]; ok {
			list[i].Links = append(list[i].Links, l)
		}
	}
	taskRows.Close()
	if err := taskRows.Err(); err != nil {
		return err
	}

	// note-links: resolve label pela regra de NOTA (a mesma do gate). LEFT JOIN
	// projects + (projeto shared OR owner_user_id=$u) OR (sub-nota project_id IS
	// NULL AND created_by=$u). 26-H2: NÃO usar INNER JOIN projects cego — isso
	// descartaria sub-nota SOLTA do criador (criada por /note numa nota solta).
	noteRows, err := s.db.Query(ctx, `
		SELECT nl.note_id, nl.id, nl.ref_type, nl.ref_id, nl.position, sub.title
		  FROM note_links nl
		  JOIN notes sub  ON sub.id = nl.ref_id AND sub.workspace_id = $1
		  LEFT JOIN projects p ON p.id = sub.project_id
		 WHERE nl.workspace_id = $1 AND nl.ref_type = 'note'
		   AND nl.note_id = ANY($3)
		   AND (
		         (p.id IS NOT NULL AND (p.visibility = 'shared' OR p.owner_user_id = $2))
		      OR (sub.project_id IS NULL AND sub.created_by = $2)
		       )
		 ORDER BY nl.note_id, nl.position, nl.id`,
		workspaceID, userID, noteIDs)
	if err != nil {
		return err
	}
	for noteRows.Next() {
		var noteID string
		var l NoteLink
		if err := noteRows.Scan(&noteID, &l.ID, &l.RefType, &l.RefID, &l.Position, &l.Label); err != nil {
			noteRows.Close()
			return err
		}
		if i, ok := idx[noteID]; ok {
			list[i].Links = append(list[i].Links, l)
		}
	}
	noteRows.Close()
	if err := noteRows.Err(); err != nil {
		return err
	}

	// ordena cada array por position (os dois batches misturam tipos).
	for i := range list {
		links := list[i].Links
		for a := 1; a < len(links); a++ {
			for b := a; b > 0 && links[b].Position < links[b-1].Position; b-- {
				links[b], links[b-1] = links[b-1], links[b]
			}
		}
	}
	return nil
}
