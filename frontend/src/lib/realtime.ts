// Cliente WebSocket do timer (T-06 da fase 04, D-07): downstream-only — o
// socket só ENTREGA o mesmo Result canônico do GET /timer; toda mutação
// continua na REST. Reconexão com backoff exponencial 1s→30s com jitter
// (CA-16, sem tempestade) e dispose() total (CA-19). Zero dependência: o
// WebSocket é o nativo do browser (RNF-01).
import type { TimerResult } from './api';

// Notice é o payload do envelope type:"notice" (fase 16, CA-10/11): um aviso
// lateral (ex.: timer cortado em 2h) — NÃO é estado de timer. O kind
// identifica o aviso; o front decide a UI por ele.
export type Notice = {
  kind?: string;
  entry_id?: string;
  stopped_at?: string;
  // fase 20: lembrete (kind:"reminder") — leva a task e o título
  task_id?: string;
  title?: string;
  [key: string]: unknown;
};

// Presence é o payload do envelope type:"presence" (fase 21, CA-15): o timer
// ATIVO de um membro do workspace, ao vivo. running:false remove o membro da
// faixa; running:true adiciona/atualiza. project só vem se o observador pode ver
// (shared); since é o started_at da entry rodando.
export type Presence = {
  user_id: string;
  name: string;
  workspace_id: string;
  running: boolean;
  project?: string;
  since?: string;
};

export type TimerSocketOptions = {
  /** chamado a cada tentativa — a URL deriva do location do momento */
  makeUrl: () => string;
  /** payload do envelope type:"timer" — mesmo shape da resposta REST */
  onResult: (r: TimerResult) => void;
  /** payload do envelope type:"notice" (fase 16) — aviso lateral, não estado */
  onNotice?: (n: Notice) => void;
  /** payload do envelope type:"presence" (fase 21) — timer ativo de um membro */
  onPresence?: (p: Presence) => void;
  /** conexão aberta (primeira OU reconexão) — o re-sync do CA-14 pluga aqui */
  onOpen?: () => void;
  /** injetável nos testes; default aleatório em [50%, 100%] do atraso */
  jitter?: (delayMs: number) => number;
};

const BASE_MS = 1_000;
const CAP_MS = 30_000;

export function connectTimerSocket(opts: TimerSocketOptions): { dispose: () => void } {
  const jitter = opts.jitter ?? ((ms) => ms * (0.5 + Math.random() * 0.5));
  let ws: WebSocket | null = null;
  let attempt = 0;
  let reconnect: ReturnType<typeof setTimeout> | null = null;
  let disposed = false;

  const open = () => {
    if (disposed) return;
    const sock = new WebSocket(opts.makeUrl());
    ws = sock;
    sock.onopen = () => {
      attempt = 0; // conexão de verdade: a escada de backoff rearma
      opts.onOpen?.();
    };
    sock.onmessage = (ev) => {
      // envelope versionável (RF-03): JSON inválido e type desconhecido são
      // ignorados sem fechar o socket nem tocar o estado (CA-18)
      let env: { type?: string; payload?: unknown };
      try {
        env = JSON.parse(String(ev.data)) as { type?: string; payload?: unknown };
      } catch {
        return;
      }
      // ramo type:"timer" (fase 04) — INTOCADO (CA-11): o estado do timer só
      // converge por aqui; o anti-stale do timerStore segue dono da ordenação
      if (env?.type === 'timer' && env.payload !== undefined) {
        opts.onResult(env.payload as TimerResult);
        return;
      }
      // ramo type:"notice" (fase 16, CA-11) — caminho LATERAL: avisa o banner,
      // não toca o estado do timer. Adicionado DEPOIS do ramo timer.
      if (env?.type === 'notice' && env.payload !== undefined) {
        opts.onNotice?.(env.payload as Notice);
        return;
      }
      // ramo type:"presence" (fase 21, CA-15) — caminho LATERAL: alimenta o
      // presenceStore (faixa de membros ativos), NÃO toca o estado do timer.
      // Adicionado DEPOIS dos ramos timer e notice (ambos INTOCADOS).
      if (env?.type === 'presence' && env.payload !== undefined) {
        opts.onPresence?.(env.payload as Presence);
        return;
      }
      // qualquer outro type/forma desconhecida segue ignorado (CA-18)
    };
    sock.onclose = () => {
      if (disposed) return;
      const delay = jitter(Math.min(BASE_MS * 2 ** attempt, CAP_MS));
      attempt += 1;
      reconnect = setTimeout(open, delay);
    };
  };

  open();

  return {
    dispose: () => {
      disposed = true;
      if (reconnect !== null) clearTimeout(reconnect);
      ws?.close();
    },
  };
}
