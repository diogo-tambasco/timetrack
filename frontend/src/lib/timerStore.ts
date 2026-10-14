// Store do timer (T-14): o estado É a resposta do GET /timer + a âncora de
// relógio. O cliente NUNCA acumula tempo como verdade (RNF-02): elapsed e
// remaining são derivados de server_time + offset a cada render; 1 único
// setInterval de 1s (RNF-03) re-renderiza e dispara as auto-transições no
// zero (CA-33/34/34.1); visibilitychange/focus re-sincronizam (CA-37.1).
import { useMemo } from 'react';
import { create } from 'zustand';
import {
  editTimer,
  getTimer,
  pauseTimer,
  resumeTimer,
  skipBreak,
  startTimer,
  stopTimer,
  type EditTimerInput,
  type StartTimerInput,
  type TimerResult,
  type TimerState,
} from './api';
import { playPhaseEndBeep } from './focusSound';
import { pushNotice } from './noticeStore';
import { pushPresence } from './presenceStore';
import { queryClient } from './queryClient';
import { connectTimerSocket } from './realtime';

// Política de auto-start + som (fase 15, CA-17/18/19). Lida do
// pomodoro_settings cacheado; o store é puro (fora de React) e não usa
// hooks, então o getter é INJETADO no boot (attachTimerSync). Default
// true/true/false PRESERVA o ciclo automático foco↔pausa da fase 03 (D-05)
// — os testes da fase 03 que não setam política seguem vendo auto-transição.
export type AutoStartPolicy = { break: boolean; focus: boolean; sound: boolean };

const DEFAULT_POLICY: AutoStartPolicy = { break: true, focus: true, sound: false };

let policyGetter: () => AutoStartPolicy = () => DEFAULT_POLICY;

// setAutoStartPolicy injeta o getter (cache do pomodoro_settings). Passar
// undefined volta ao default (preserva a fase 03).
export function setAutoStartPolicy(getter?: () => AutoStartPolicy): void {
  policyGetter = getter ?? (() => DEFAULT_POLICY);
}

type TimerStore = {
  state: TimerState | null;
  /** server_time da última resposta, em ms epoch — âncora do tick local */
  anchorMs: number;
  /** serverNow − Date.now() no instante da resposta (skew do device) */
  offsetMs: number;
  /** bump por segundo: dependência dos componentes pra re-derivar o clock */
  tick: number;
  /** chave do estado pro qual a transição automática JÁ disparou */
  firedKey: string | null;
  /**
   * chave do estado pro qual o beep de fim-de-fase JÁ tocou (CA-19.2). AO
   * CONTRÁRIO do firedKey, NÃO é zerada pelo apply() — só muda quando o
   * stateKey muda. Senão, com auto-start OFF a fase fica parada em
   * remaining=0 e cada re-sync (60s)/evento WS re-armaria o beep.
   */
  soundedKey: string | null;
  /** mutação automática em voo — não disparar outra */
  transitioning: boolean;

  load: () => Promise<void>;
  start: (input: StartTimerInput) => Promise<void>;
  /** fase 27: edita desc/projeto/tags do timer RODANDO (allowance HITL 23/06) */
  editCurrent: (input: EditTimerInput) => Promise<void>;
  stop: () => Promise<void>;
  pause: (breakMinutes?: number) => Promise<void>;
  resume: () => Promise<void>;
  skip: () => Promise<void>;

  apply: (r: TimerResult) => void;
  checkAutoTransition: () => void;
};

export const initialTimerState = {
  state: null,
  anchorMs: 0,
  offsetMs: 0,
  tick: 0,
  firedKey: null,
  soundedKey: null,
  transitioning: false,
} as const;

// deriveClock: segundos exibidos AGORA, derivados da âncora — funciona pra
// trabalho e pra pausa (que não tem entry) sem requisição por segundo.
export function deriveClock(s: Pick<TimerStore, 'state' | 'anchorMs' | 'offsetMs'>): {
  elapsed: number;
  remaining: number | null;
} {
  if (!s.state) return { elapsed: 0, remaining: null };
  const serverNowMs = Date.now() + s.offsetMs;
  const deltaSec = Math.max(0, Math.floor((serverNowMs - s.anchorMs) / 1000));
  const elapsed = s.state.elapsed_seconds + deltaSec;
  const remaining =
    s.state.remaining_seconds === null || s.state.remaining_seconds === undefined
      ? null
      : Math.max(0, s.state.remaining_seconds - deltaSec);
  return { elapsed, remaining };
}

// chave única da "instância" do estado: o mesmo estado nunca dispara a
// mesma transição duas vezes (CA-33/34/34.1 — "exatamente 1").
function stateKey(st: TimerState): string {
  return `${st.mode}:${st.entry?.id ?? st.resume?.entry_id ?? ''}`;
}

export const useTimerStore = create<TimerStore>((set, get) => {
  // mutate (fase 05, CA-17/17.1 — plan D-05): toda mutação MANUAL converge.
  // Sucesso → apply(result). Falha (ex.: 409 porque o estado mudou em outro
  // device com o WS caído) → load() IMEDIATO — a tela converge pra verdade
  // sem esperar o re-sync de 60s. O load falhando (offline) também é
  // engolido: o silêncio é o feedback (CA-17.1) e a rede de segurança segue
  // sendo o re-sync periódico/visibilitychange da fase 03. As ações nunca
  // rejeitam (fire-and-forget seguro — os componentes chamam `void ação()`).
  const mutate = async (fn: () => Promise<TimerResult>) => {
    try {
      get().apply(await fn());
    } catch {
      await get().load().catch(() => undefined);
    }
  };

  return {
  ...initialTimerState,

  apply: (r) => {
    const anchorMs = Date.parse(r.server_time);
    // resposta fora de ordem (GET stale entregue depois de uma mutação)
    // não pode regredir o estado nem rearmar a transição automática — o
    // relógio do servidor é monotônico entre respostas do mesmo usuário
    if (anchorMs < get().anchorMs) return;
    set({
      state: r.state,
      anchorMs,
      offsetMs: anchorMs - Date.now(),
      firedKey: null, // estado novo da fonte da verdade: transição rearmada
      tick: get().tick + 1,
    });
  },

  load: async () => {
    get().apply(await getTimer());
    // CA-37.1: re-sync com alvo já vencido dispara a pendente NA HORA
    get().checkAutoTransition();
  },

  start: (input) => mutate(() => startTimer(input)),
  // fase 27 (allowance HITL 23/06): converge pelo MESMO mutate (CA-17 —
  // nunca rejeita; sucesso → apply; falha → load). Timing intocado no backend.
  editCurrent: (input) => mutate(() => editTimer(input)),
  stop: () => mutate(stopTimer),
  pause: (breakMinutes) => mutate(() => pauseTimer(breakMinutes)),
  resume: () => mutate(resumeTimer),
  skip: () => mutate(skipBreak),

  checkAutoTransition: () => {
    const { state: st, firedKey, soundedKey, transitioning } = get();
    if (!st) return;
    const { remaining } = deriveClock(get());
    if (remaining === null || remaining > 0) return;

    const key = stateKey(st);
    const policy = policyGetter();

    // som (CA-19/19.2): uma fase de Pomodoro que termina (foco→fim ou
    // pausa→fim) toca 1 beep, atrelado ao stateKey (NÃO ao firedKey — não
    // re-arma no apply/re-sync). Independe de auto-start: com auto-start OFF
    // a fase fica parada em 0 e o beep ainda deve tocar UMA vez.
    const isPhaseEnd = st.mode === 'pomodoro' || st.mode === 'break';
    if (policy.sound && isPhaseEnd && soundedKey !== key) {
      set({ soundedKey: key });
      playPhaseEndBeep();
    }

    if (transitioning) return;
    if (firedKey === key) return;

    let action: (() => Promise<void>) | null = null;
    if (st.mode === 'countdown') {
      action = get().stop; // CA-33
    } else if (st.mode === 'pomodoro') {
      // CA-17 (fase 15): foco zerado só auto-pausa se auto_start_break ON;
      // OFF → espera o toque do usuário (servidor segue reportando o foco
      // vencido com remaining 0).
      if (policy.break) action = () => get().pause();
    } else if (st.mode === 'break' && st.resume?.mode === 'pomodoro_focus') {
      // CA-18 (fase 15): pausa de ciclo só auto-retoma se auto_start_focus ON.
      if (policy.focus) action = get().resume;
    }
    // CA-34.2/CA-18: pausa de countup/countdown vencida espera o usuário —
    // o servidor segue reportando break com remaining 0 (CA-22.1)
    if (!action) return;

    set({ firedKey: key, transitioning: true });
    // a convergência em falha já mora nas próprias ações (mutate, CA-17);
    // o catch fica como redundância inofensiva de lógica provada (D-05)
    void action()
      .catch(() => get().load())
      .finally(() => set({ transitioning: false }));
  },
  };
});

const RESYNC_MS = 60_000; // re-sync periódico ≤ 1/min (RNF-03)

// attachTimerSync liga o runtime do timer (1 vez, no boot do shell):
// load inicial, o ÚNICO interval de 1s, os listeners de re-sync e — desde a
// fase 04 — o socket de eventos. Devolve o teardown único (CA-19).
//
// getPolicy (fase 15): getter da política de auto-start/som lido do
// pomodoro_settings cacheado (sem request por tick — RNF). Ausente → default
// true/true/false (preserva a fase 03). Injetado aqui porque o store é puro
// (fora de React) e não lê hooks (plan D-04).
export function attachTimerSync(getPolicy?: () => AutoStartPolicy): () => void {
  setAutoStartPolicy(getPolicy);
  let lastSyncMs = Date.now();
  void useTimerStore.getState().load();

  const socket = connectTimerSocket({
    makeUrl: () => {
      const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
      return `${proto}//${location.host}/api/v1/ws`;
    },
    // CA-14: toda abertura (primeira conexão OU reconexão) re-sincroniza —
    // eventos perdidos durante a queda nunca deixam estado fantasma
    onOpen: () => {
      lastSyncMs = Date.now();
      void useTimerStore.getState().load();
      // fase 21 (CA-18): reconectou → re-busca o snapshot de presence (deltas
      // perdidos durante a queda não deixam a faixa com estado fantasma). O
      // PresenceStrip re-semeia o store com o resultado do refetch.
      void queryClient.invalidateQueries({ queryKey: ['presence'] });
    },
    // CA-11: o apply() segue sendo o ÚNICO ponto de entrada de estado — o
    // anti-stale ordena WS × REST sem caminho novo de mutação; transições
    // automáticas seguem sendo SÓ do tick de 1s e do load (plan D-07)
    onResult: (r) => useTimerStore.getState().apply(r),
    // CA-12 (fase 16): o aviso de corte (type:"notice") alimenta o
    // NoticeBanner — caminho LATERAL, não toca o estado do timer (que já
    // converge pra ocioso pelo type:"timer" que o corte também emite)
    onNotice: (n) => pushNotice(n),
    // CA-16 (fase 21): o presence (type:"presence") alimenta o presenceStore —
    // caminho LATERAL, não toca o estado do timer (onResult/anti-stale intocados)
    onPresence: (p) => pushPresence(p),
  });

  const interval = setInterval(() => {
    const s = useTimerStore.getState();
    s.checkAutoTransition();
    useTimerStore.setState({ tick: s.tick + 1 });
    if (Date.now() - lastSyncMs >= RESYNC_MS) {
      lastSyncMs = Date.now();
      void s.load();
    }
  }, 1000);

  const onWake = () => {
    if (document.visibilityState !== 'visible') return;
    lastSyncMs = Date.now();
    void useTimerStore.getState().load();
  };
  document.addEventListener('visibilitychange', onWake);
  window.addEventListener('focus', onWake);

  return () => {
    clearInterval(interval);
    document.removeEventListener('visibilitychange', onWake);
    window.removeEventListener('focus', onWake);
    socket.dispose(); // CA-19: sem socket nem reconexão zumbi
    setAutoStartPolicy(); // volta ao default (preserva a fase 03)
  };
}

// useTimerClock: clock derivado pros componentes — seleciona primitivas
// estáveis (objeto novo no selector = loop de render) e re-deriva a cada
// tick do interval via useMemo.
export function useTimerClock(): { elapsed: number; remaining: number | null } {
  const state = useTimerStore((s) => s.state);
  const anchorMs = useTimerStore((s) => s.anchorMs);
  const offsetMs = useTimerStore((s) => s.offsetMs);
  const tick = useTimerStore((s) => s.tick);
  // eslint-disable-next-line react-hooks/exhaustive-deps -- tick é o relógio
  return useMemo(() => deriveClock({ state, anchorMs, offsetMs }), [state, anchorMs, offsetMs, tick]);
}
