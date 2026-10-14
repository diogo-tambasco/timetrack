// due_test.go — TDD do helper de horário (fase 20 T-05): 09:00 LOCAL do dia da
// due_date, tz-correto (CA-11 base). O teste da CRIAÇÃO do due na escrita da task
// (CA-11/12/12.1/13/14/15/28) mora no harness de tasks (api/tasks_due_test.go),
// que liga o Hook real ao Service.
package reminders

import (
	"testing"
	"time"
)

func TestDueRemindAt(t *testing.T) {
	// America/Sao_Paulo (UTC-03 em junho): 09:00 local = 12:00Z.
	got, err := dueRemindAt("2026-06-25", "America/Sao_Paulo")
	if err != nil {
		t.Fatalf("dueRemindAt SP: %v", err)
	}
	want := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("09:00 em SP deveria ser %s, veio %s", want, got.UTC())
	}
	// confirma que é 09:00 na PAREDE local (não 09:00Z).
	if h := got.Hour(); got.Location().String() != "America/Sao_Paulo" || h != 9 {
		t.Errorf("deveria ser 09:00 na parede de SP, veio %dh em %s", h, got.Location())
	}

	// tz vazio → UTC.
	got, err = dueRemindAt("2026-06-25", "")
	if err != nil {
		t.Fatalf("dueRemindAt UTC: %v", err)
	}
	if !got.Equal(time.Date(2026, 6, 25, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("tz vazio deveria dar 09:00Z, veio %s", got.UTC())
	}

	// tz inválido → erro.
	if _, err := dueRemindAt("2026-06-25", "Marte/Olympus"); err == nil {
		t.Errorf("tz IANA inválido deveria dar erro")
	}
	// due_date malformada → erro.
	if _, err := dueRemindAt("25/06/2026", "UTC"); err == nil {
		t.Errorf("due_date malformada deveria dar erro")
	}
}
