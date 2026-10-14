package main

import (
	"testing"
	"time"

	"timetrack.live/backend/internal/timer"
)

// fase 21 T-06 — composeOnTimer encadeia BroadcastTimer (fase 04) + presence
// (fase 21): AMBOS são chamados, com os MESMOS argumentos, e o `first`
// (BroadcastTimer) roda ANTES do `then` (presence). Esse hook composto é o que
// vai no SetOnChange E no guard.WithOnTimer — uma mutação que deixe de chamar
// um dos dois (ou inverta a ordem) deve quebrar o teste.
func TestComposeOnTimerChamaOsDoisEmOrdem(t *testing.T) {
	var order []string
	var gotFirst, gotThen struct {
		user string
		st   *timer.State
		at   time.Time
	}

	first := func(user string, st *timer.State, at time.Time) {
		order = append(order, "timer")
		gotFirst.user, gotFirst.st, gotFirst.at = user, st, at
	}
	then := func(user string, st *timer.State, at time.Time) {
		order = append(order, "presence")
		gotThen.user, gotThen.st, gotThen.at = user, st, at
	}

	hook := composeOnTimer(first, then)

	at := time.Now().UTC()
	state := &timer.State{Mode: "countup"}
	hook("user-a", state, at)

	if len(order) != 2 || order[0] != "timer" || order[1] != "presence" {
		t.Fatalf("BroadcastTimer deve rodar antes do presence: %v", order)
	}
	// mesmos argumentos pros dois
	if gotFirst.user != "user-a" || gotThen.user != "user-a" {
		t.Errorf("userID divergente: first=%q then=%q", gotFirst.user, gotThen.user)
	}
	if gotFirst.st != state || gotThen.st != state {
		t.Errorf("state não repassado igual aos dois hooks")
	}
	if !gotFirst.at.Equal(at) || !gotThen.at.Equal(at) {
		t.Errorf("at não repassado igual aos dois hooks")
	}
}

// state==nil (stop/corte) também é encadeado pros dois (o corte da 16 derruba a
// presence — CA-09/D-04).
func TestComposeOnTimerEncadeiaEstadoNil(t *testing.T) {
	calls := 0
	hook := composeOnTimer(
		func(string, *timer.State, time.Time) { calls++ },
		func(string, *timer.State, time.Time) { calls++ },
	)
	hook("u", nil, time.Now())
	if calls != 2 {
		t.Errorf("os dois hooks deveriam ser chamados no estado nil, veio %d", calls)
	}
}
