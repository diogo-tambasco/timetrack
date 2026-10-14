// ProjectCalendarTab (fase 28 T-10, CA-13/18/21): vista de MÊS do projeto.
// Grade de 42 dias (monthGridDays da fase 11) com os ITENS DATADOS do projeto
// (useProjectCalendar via ?month=) na COR do projeto, SOBREPOSTOS aos eventos do
// Google Calendar (read-only, fase 06). 28-H4: o overlay GCal usa a janela da
// GRADE (monthRange(refDate) — 42 dias, cobre os dias de spillover), NÃO o mês
// estrito. Clicar item de task/objetivo → openTask (RF-23); evento → html_link.
// Degrada sem conexão GCal (CA-13). À prova de culpa: sem vermelho acusatório.
import { useState } from 'react';
import type { Board, ProjectCalendarItem, CalendarEvent } from '../../lib/api';
import { useProjectCalendar } from '../../lib/queries/projects';
import { useCalendarEvents, useCalendarStatus } from '../../lib/queries/calendar';
import {
  monthGridDays,
  monthRange,
  inMonth,
  isoDay,
  parseDay,
  periodTitle,
  addMonths,
} from '../../lib/calendar-layout';

const DOW = ['Seg', 'Ter', 'Qua', 'Qui', 'Sex', 'Sáb', 'Dom'];
const MAX = 4;

// localDay de um ISO com horário (bucket dos eventos timed por dia de início).
function eventDay(e: CalendarEvent): string {
  return isoDay(new Date(e.start));
}

export default function ProjectCalendarTab({
  wsId,
  board,
  onOpenTask,
  initialRef,
}: {
  wsId: string;
  board: Board;
  onOpenTask: (taskId: string) => void;
  initialRef?: Date;
}) {
  const projectId = board.project.id;
  const color = board.project.color;
  const [refDate, setRefDate] = useState(() => initialRef ?? new Date());

  const month = `${refDate.getFullYear()}-${String(refDate.getMonth() + 1).padStart(2, '0')}`;
  const days = monthGridDays(refDate);
  const todayISO = isoDay(new Date());

  // itens do projeto (?month=) — caem só nos dias do próprio mês.
  const items = useProjectCalendar(wsId, projectId, month).data?.items ?? [];
  const itemsByDay = new Map<string, ProjectCalendarItem[]>();
  for (const it of items) {
    const list = itemsByDay.get(it.date) ?? [];
    list.push(it);
    itemsByDay.set(it.date, list);
  }

  // overlay GCal: janela da GRADE de 42 dias (28-H4), só quando conectado.
  const connected = useCalendarStatus().data?.connected ?? false;
  const { from, to } = monthRange(refDate);
  const events = useCalendarEvents(from, to, connected).data ?? [];
  const eventsByDay = new Map<string, CalendarEvent[]>();
  for (const e of events) {
    const k = eventDay(e);
    const list = eventsByDay.get(k) ?? [];
    list.push(e);
    eventsByDay.set(k, list);
  }

  const onItem = (it: ProjectCalendarItem) => {
    // task/objetivo abrem a entidade (RF-23); project_deadline não abre nada.
    if (it.kind === 'task' || it.kind === 'objective') onOpenTask(it.id);
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <button
          type="button"
          className="tt-btn tt-btn--ghost"
          aria-label="Anterior"
          onClick={() => setRefDate((d) => addMonths(d, -1))}
        >
          ‹
        </button>
        <button type="button" className="tt-btn" onClick={() => setRefDate(new Date())}>
          Hoje
        </button>
        <button
          type="button"
          className="tt-btn tt-btn--ghost"
          aria-label="Próximo"
          onClick={() => setRefDate((d) => addMonths(d, 1))}
        >
          ›
        </button>
        <strong style={{ fontSize: 15, marginLeft: 6 }}>{periodTitle('month', refDate)}</strong>
      </div>

      <div data-testid="project-month-grid" className="tt-card" style={{ padding: 0, overflow: 'hidden' }}>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(7, 1fr)', borderBottom: '1px solid var(--border)' }}>
          {DOW.map((d) => (
            <div
              key={d}
              className="faint"
              style={{ textAlign: 'center', padding: '6px 0', fontSize: 11, textTransform: 'uppercase', letterSpacing: '.04em' }}
            >
              {d}
            </div>
          ))}
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(7, 1fr)', gridAutoRows: 'minmax(92px, 1fr)' }}>
          {days.map((d) => {
            const isToday = d === todayISO;
            const outside = !inMonth(d, refDate);
            const dayItems = itemsByDay.get(d) ?? [];
            const dayEvents = (eventsByDay.get(d) ?? []).slice().sort((a, b) => a.start.localeCompare(b.start));
            const total = dayItems.length + dayEvents.length;
            const shownItems = dayItems.slice(0, MAX);
            const shownEvents = dayEvents.slice(0, Math.max(0, MAX - shownItems.length));
            const overflow = total - shownItems.length - shownEvents.length;
            return (
              <div
                key={d}
                data-testid="cal-cell"
                style={{
                  borderLeft: '1px solid var(--border)',
                  borderTop: '1px solid var(--border)',
                  padding: 4,
                  display: 'grid',
                  gridTemplateRows: 'auto 1fr',
                  gap: 3,
                  background: outside ? 'var(--surface-1)' : 'transparent',
                  opacity: outside ? 0.5 : 1,
                }}
              >
                <div
                  data-testid={isToday ? 'cal-today' : undefined}
                  style={{
                    fontSize: 12,
                    fontWeight: isToday ? 700 : 500,
                    textAlign: 'right',
                    color: isToday ? 'var(--accent-ink)' : 'var(--text-2)',
                    background: isToday ? 'var(--accent)' : 'transparent',
                    borderRadius: 6,
                    width: isToday ? 22 : 'auto',
                    height: isToday ? 22 : 'auto',
                    lineHeight: isToday ? '22px' : undefined,
                    justifySelf: 'end',
                    padding: isToday ? 0 : '0 2px',
                  }}
                >
                  {parseDay(d).getDate()}
                </div>
                <div style={{ display: 'grid', gap: 2, alignContent: 'start' }}>
                  {shownItems.map((it) => (
                    <button
                      key={`${it.kind}-${it.id}`}
                      type="button"
                      data-testid="cal-item"
                      onClick={() => onItem(it)}
                      title={it.name}
                      style={{
                        textAlign: 'left',
                        border: 'none',
                        borderLeft: `3px solid ${color}`,
                        background: 'var(--surface-3)',
                        borderRadius: 4,
                        padding: '2px 6px',
                        fontSize: 11.5,
                        fontFamily: 'var(--font-ui)',
                        color: 'var(--text-1)',
                        cursor: it.kind === 'project_deadline' ? 'default' : 'pointer',
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap',
                        textDecoration: it.status === 'done' ? 'line-through' : 'none',
                      }}
                    >
                      {it.name}
                    </button>
                  ))}
                  {shownEvents.map((e) => (
                    <a
                      key={e.id}
                      href={e.html_link || undefined}
                      target="_blank"
                      rel="noreferrer"
                      data-testid="cal-event"
                      title={e.summary}
                      style={{
                        borderLeft: '3px solid var(--accent)',
                        background: 'var(--accent-soft)',
                        borderRadius: 4,
                        padding: '2px 6px',
                        fontSize: 11.5,
                        color: 'var(--text-1)',
                        textDecoration: 'none',
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap',
                      }}
                    >
                      {e.summary}
                    </a>
                  ))}
                  {overflow > 0 && (
                    <span className="faint" style={{ fontSize: 11, padding: '0 6px' }}>
                      +{overflow} mais
                    </span>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
