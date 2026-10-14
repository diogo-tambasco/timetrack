// objective_link.go — fase 12 T-04: vínculo task↔objetivo + recálculo de
// status do objetivo POR TRANSIÇÃO (D-04). O recálculo é SQL INLINE no pacote
// tasks pra NÃO importar objectives (sem ciclo de import).
package tasks

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"timetrack.live/backend/internal/domain"
)

// rowQuerier abstrai pool e tx (ambos têm QueryRow) pra reuso da validação.
type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// validateObjectiveRef confere que objective_id (quando presente) existe e
// pertence ao MESMO project_id + workspace (CA-09.1). Inexistente, de outro
// projeto/workspace ou malformado respondem o MESMO invalid_reference
// (uuid.Validate antes do SQL → nunca 500).
func validateObjectiveRef(ctx context.Context, q rowQuerier, workspaceID, projectID string, objectiveID *string) error {
	if objectiveID == nil {
		return nil
	}
	if uuid.Validate(*objectiveID) != nil {
		return domain.InvalidReference("objective_id não pertence a este projeto")
	}
	var ok bool
	err := q.QueryRow(ctx, `
		SELECT true FROM objectives
		 WHERE id = $1 AND project_id = $2 AND workspace_id = $3`,
		*objectiveID, projectID, workspaceID).Scan(&ok)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.InvalidReference("objective_id não pertence a este projeto")
	}
	return err
}

// recalcObjectiveStatus aplica a regra de transição (D-04) ramificando por
// completion_mode do objetivo (fase 29 — D-02):
//   - by_tasks (fase 12, byte-equivalente — CA-05): ≥1 task e 100% done & open →
//     done; <100% & done → open; n=0 → no-op; senão no-op (preserva manual).
//   - by_hours (CA-06): goal_hours não-nulo & Σ horas (PER-USUÁRIO, decisão E) >=
//     goal_hours×3600 & open → done; < & done → open; goal_hours nulo → no-op
//     (CA-06.1). Independente do done-ness das tasks.
//   - by_date (CA-07): no-op SEMPRE (marco fecha só por PATCH status manual).
//
// userID = quem fez a mutação de task: o bucket de horas filtra e.user_id (MESMO
// per-usuário do /board — board.go:106). Chamado SÓ quando a mutação muda
// status/objective_id/cria/deleta (D-04); a edição benigna NÃO chama, então o
// fecho/reabertura manual persiste (e o by_hours converge no próximo gatilho de
// task — CA-06.2/29-H7; fechar entry NÃO recalcula).
//
// CUIDADO com o fan-out (plan §7): n/d em subquery sobre `tasks`; secs em
// subquery SEPARADA sobre time_entries ⋈ tasks do objetivo (per-usuário) — senão
// o count infla com N entries por task.
func recalcObjectiveStatus(ctx context.Context, tx pgx.Tx, objectiveID *string, userID string) error {
	if objectiveID == nil {
		return nil
	}
	// Serializa o recálculo por objetivo: trava a linha ANTES de contar as tasks.
	// Sob READ COMMITTED, sem esta trava, duas tasks marcadas done em paralelo
	// veem cada uma d=1,n=2 (snapshot pré-commit da irmã) e o objetivo fica preso
	// em open a 100% (CA-06 — verificação adversarial do build). Com o FOR UPDATE,
	// a 2ª transação espera a 1ª commitar e o UPDATE seguinte re-conta num snapshot
	// fresco. Quando há 2 objetivos (move A→B), os call sites travam em ordem de id
	// pra não criar deadlock (ver service.go::Patch).
	if _, err := tx.Exec(ctx, `SELECT 1 FROM objectives WHERE id = $1 FOR UPDATE`, *objectiveID); err != nil {
		return err
	}
	// CASE ramifica por completion_mode:
	//   by_tasks → d==n (≥1 task) abre/fecha como a fase 12 (byte-equivalente).
	//   by_hours → secs vs goal_hours×3600 (goal_hours não-nulo); secs PER-USUÁRIO.
	//   by_date  → mantém o status (no-op).
	// n/d numa subquery sobre tasks (sem o join de entries); secs numa subquery
	// SEPARADA filtrando e.user_id (per-usuário) — evita o fan-out do join.
	_, err := tx.Exec(ctx, `
		UPDATE objectives o
		   SET status = CASE
		         WHEN o.completion_mode = 'by_tasks' AND agg.n >= 1 AND agg.d = agg.n AND o.status = 'open' THEN 'done'::objective_status
		         WHEN o.completion_mode = 'by_tasks' AND agg.n >= 1 AND agg.d < agg.n AND o.status = 'done' THEN 'open'::objective_status
		         WHEN o.completion_mode = 'by_hours' AND o.goal_hours IS NOT NULL AND agg.secs >= o.goal_hours * 3600 AND o.status = 'open' THEN 'done'::objective_status
		         WHEN o.completion_mode = 'by_hours' AND o.goal_hours IS NOT NULL AND agg.secs <  o.goal_hours * 3600 AND o.status = 'done' THEN 'open'::objective_status
		         ELSE o.status
		       END
		  FROM (
		    SELECT
		      (SELECT count(*) FROM tasks WHERE objective_id = $1) AS n,
		      (SELECT count(*) FROM tasks WHERE objective_id = $1 AND status = 'done') AS d,
		      (SELECT COALESCE(SUM(e.duration_seconds), 0)
		         FROM time_entries e
		        WHERE e.user_id = $2 AND e.duration_seconds IS NOT NULL
		          AND e.task_id IN (SELECT id FROM tasks WHERE objective_id = $1)) AS secs
		  ) agg
		 WHERE o.id = $1`, *objectiveID, userID)
	return err
}

// ptrEqual compara dois *string (nil == nil).
func ptrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
