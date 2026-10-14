-- 0015_pomodoro_cycles — fase 15 (plan §2): Pomodoro 2.0. Estende
-- pomodoro_settings com ciclos/pausa-longa/auto-start/som e timer_states
-- com o contador de focos do ciclo + o kind da pausa PERSISTIDO. Defaults
-- PRESERVAM o comportamento da fase 03 (ciclo automático foco↔pausa segue
-- ligado — D-05; auto-start DEFAULT true).
ALTER TABLE pomodoro_settings
  ADD COLUMN cycles_before_long integer NOT NULL DEFAULT 4   CHECK (cycles_before_long BETWEEN 1 AND 12),
  ADD COLUMN long_break_minutes integer NOT NULL DEFAULT 15  CHECK (long_break_minutes BETWEEN 1 AND 120),
  ADD COLUMN auto_start_break   boolean NOT NULL DEFAULT true,
  ADD COLUMN auto_start_focus   boolean NOT NULL DEFAULT true,
  ADD COLUMN sound_enabled      boolean NOT NULL DEFAULT false;

ALTER TABLE timer_states
  ADD COLUMN focus_count   integer NOT NULL DEFAULT 0,    -- focos completos desde a última pausa longa/reset
  ADD COLUMN break_is_long boolean NOT NULL DEFAULT false; -- kind da pausa atual, PERSISTIDO (CA-06.1) — não recomputar
