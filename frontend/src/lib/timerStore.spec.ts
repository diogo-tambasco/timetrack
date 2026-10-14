// timerStore.spec — T-14: auto-transições no zero (CA-33/34/34.1/34.2) e
// re-sync + transição pendente no visibilitychange (CA-37.1).
// API mockada com STUB DE FETCH PURO — RNF-01 veta MSW até como dev-dep.
// Fase 04 (T-07): attachTimerSync ganhou o socket — o WebSocket é stubado
// no SETUP (exceção mecânica sancionada pelo CA-04 da 04; nenhuma asserção
// da 03 mudou) e os testes novos vivem no describe "socket (fase 04)".
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import type { StartTimerInput, TimerResult, TimerState } from './api';
import {
  useTimerStore,
  attachTimerSync,
  initialTimerState,
  setAutoStartPolicy,
  type AutoStartPolicy,
} from './timerStore';
import * as focusSound from './focusSound';
import { usePresenceStore, activeMembers } from './presenceStore';

// beep stubado pra contar disparos (CA-19/19.2) — sem WebAudio real em jsdom.
vi.mock('./focusSound', () => ({ playPhaseEndBeep: vi.fn() }));

// stub passivo de WebSocket: só registra instâncias; os testes da fase 04
// disparam open/message/close na mão. Os testes da 03 nunca o tocam.
class FakeWebSocket {
  static instances: FakeWebSocket[] = [];
  url: string;
  closed = false;
  onopen: ((ev: Event) => void) | null = null;
  onmessage: ((ev: MessageEvent) => void) | null = null;
  onclose: ((ev: CloseEvent) => void) | null = null;

  constructor(url: string) {
    this.url = url;
    FakeWebSocket.instances.push(this);
  }

  close() {
    this.closed = true;
    this.onclose?.(new CloseEvent('close'));
  }

  fireOpen() {
    this.onopen?.(new Event('open'));
  }
  fireMessage(data: string) {
    this.onmessage?.({ data } as MessageEvent);
  }
  fireClose() {
    this.onclose?.(new CloseEvent('close'));
  }
}

const lastWS = () => FakeWebSocket.instances[FakeWebSocket.instances.length - 1];
const wsConns = () => FakeWebSocket.instances.length;

const T0 = '2026-06-10T15:00:00.000Z';

type Call = { method: string; path: string };

let calls: Call[] = [];
let getResult: TimerResult;

// stub de fetch puro: GET /timer responde getResult; mutações respondem
// mutate(path) e ATUALIZAM getResult (o servidor mudou de estado).
let mutate: (path: string) => TimerResult;

function stubFetch() {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const path = String(input);
      const method = init?.method ?? 'GET';
      calls.push({ method, path });
      const body = method === 'GET' ? getResult : mutate(path);
      return new Response(JSON.stringify(body), { status: 200 });
    }),
  );
}

const posts = (suffix: string) =>
  calls.filter((c) => c.method === 'POST' && c.path.endsWith(suffix));
const gets = () => calls.filter((c) => c.method === 'GET');

const idle = (serverTime: string): TimerResult => ({ state: null, server_time: serverTime });

function running(mode: TimerState['mode'], opts: Partial<TimerState>, serverTime: string): TimerResult {
  return {
    state: {
      mode,
      entry: {
        id: 'e1',
        workspace_id: 'w1',
        project_id: null,
        task_id: null,
        description: 'foco',
        tags: [],
        started_at: serverTime,
        stopped_at: null,
        duration_seconds: null,
        source: 'timer',
      },
      elapsed_seconds: 0,
      target_seconds: null,
      remaining_seconds: null,
      ...opts,
    },
    server_time: serverTime,
  };
}

function emPausa(remaining: number, resumeMode: 'pomodoro_focus' | 'countup', serverTime: string): TimerResult {
  return {
    state: {
      mode: 'break',
      entry: null,
      elapsed_seconds: 300 - remaining,
      target_seconds: 300,
      remaining_seconds: remaining,
      resume: { entry_id: 'e1', mode: resumeMode, target_seconds: resumeMode === 'countup' ? null : 900 },
    },
    server_time: serverTime,
  };
}

let detach: () => void = () => {};

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date(T0));
  calls = [];
  FakeWebSocket.instances = [];
  vi.stubGlobal('WebSocket', FakeWebSocket);
  useTimerStore.setState(initialTimerState);
  vi.mocked(focusSound.playPhaseEndBeep).mockClear();
  setAutoStartPolicy(); // default true/true/false (preserva a fase 03)
  mutate = () => {
    getResult = idle(new Date().toISOString());
    return getResult;
  };
  stubFetch();
});

afterEach(() => {
  detach();
  setAutoStartPolicy();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe('useTimerStore — auto-transições no zero', () => {
  // CA-33 — countdown a zero dispara EXATAMENTE 1 POST /timer/stop.
  it('countdown zerado dispara exatamente 1 stop', async () => {
    getResult = running('countdown', { target_seconds: 5, remaining_seconds: 5 }, T0);
    detach = attachTimerSync();
    await vi.advanceTimersByTimeAsync(0); // load inicial

    await vi.advanceTimersByTimeAsync(4000);
    expect(posts('/timer/stop')).toHaveLength(0); // ainda não zerou

    await vi.advanceTimersByTimeAsync(2000);
    expect(posts('/timer/stop')).toHaveLength(1);

    await vi.advanceTimersByTimeAsync(10000); // não redispara
    expect(posts('/timer/stop')).toHaveLength(1);
  });

  // CA-34 — foco de Pomodoro a zero dispara EXATAMENTE 1 POST /timer/pause
  // (emenda na pausa configurada).
  it('foco pomodoro zerado dispara exatamente 1 pause', async () => {
    getResult = running('pomodoro', { target_seconds: 3, remaining_seconds: 3 }, T0);
    mutate = () => {
      getResult = emPausa(300, 'pomodoro_focus', new Date().toISOString());
      return getResult;
    };
    detach = attachTimerSync();
    await vi.advanceTimersByTimeAsync(0);

    await vi.advanceTimersByTimeAsync(4000);
    expect(posts('/timer/pause')).toHaveLength(1);
    expect(useTimerStore.getState().state?.mode).toBe('break');

    await vi.advanceTimersByTimeAsync(10000);
    expect(posts('/timer/pause')).toHaveLength(1);
  });

  // CA-34.1 — pausa zerada com contexto pomodoro_focus dispara EXATAMENTE
  // 1 POST /timer/resume (ciclo foco ↔ pausa sem intervenção).
  it('pausa de ciclo pomodoro zerada dispara exatamente 1 resume', async () => {
    getResult = emPausa(2, 'pomodoro_focus', T0);
    mutate = () => {
      getResult = running('pomodoro', { target_seconds: 900, remaining_seconds: 900 }, new Date().toISOString());
      return getResult;
    };
    detach = attachTimerSync();
    await vi.advanceTimersByTimeAsync(0);

    await vi.advanceTimersByTimeAsync(3000);
    expect(posts('/timer/resume')).toHaveLength(1);

    await vi.advanceTimersByTimeAsync(10000);
    expect(posts('/timer/resume')).toHaveLength(1);
  });

  // CA-34.2 — pausa zerada com contexto countup/countdown NÃO retoma
  // sozinha: nenhum POST automático.
  it('pausa de countup zerada NÃO dispara resume', async () => {
    getResult = emPausa(2, 'countup', T0);
    detach = attachTimerSync();
    await vi.advanceTimersByTimeAsync(0);

    await vi.advanceTimersByTimeAsync(15000);
    expect(calls.filter((c) => c.method === 'POST')).toHaveLength(0);
  });
});

describe('useTimerStore — respostas fora de ordem', () => {
  // F3 da verificação adversarial: um GET stale (enviado antes de uma
  // mutação, entregue depois) regredia o estado e rearmava firedKey —
  // a transição automática disparava uma SEGUNDA vez (e um stop duplicado
  // pode matar uma entry recém-criada no servidor).
  it('resposta stale não regride o estado nem rearma a transição', () => {
    const apply = useTimerStore.getState().apply;

    apply(idle('2026-06-10T15:10:00.000Z')); // estado mais novo: ocioso
    // resposta velha chega DEPOIS: countdown vencido de 15:00
    apply(running('countdown', { elapsed_seconds: 700, target_seconds: 600, remaining_seconds: 0 }, T0));

    expect(useTimerStore.getState().state).toBeNull(); // segue no mais novo
  });

  it('resposta com o mesmo server_time aplica normalmente', () => {
    const apply = useTimerStore.getState().apply;
    apply(idle(T0));
    apply(running('countup', {}, T0));
    expect(useTimerStore.getState().state?.mode).toBe('countup');
  });
});

describe('attachTimerSync — socket (fase 04)', () => {
  // CA-14 — a conexão abrir (primeira OU reconexão) dispara re-sync via
  // GET /timer E o estado retornado é APLICADO (verificação adversarial do
  // build, 11/06: só contar GETs deixava passar um load que descarta a
  // resposta — "eventos perdidos nunca deixam estado fantasma" exige o
  // estado no store).
  it('open e RE-open do socket re-sincronizam e APLICAM o estado', async () => {
    getResult = idle(T0);
    detach = attachTimerSync();
    await vi.advanceTimersByTimeAsync(0); // load inicial
    expect(wsConns()).toBe(1);
    const before = gets().length;

    // durante a "queda" o servidor mudou: o open tem que trazer o rodando
    getResult = running('countup', {}, '2026-06-10T15:01:00.000Z');
    lastWS().fireOpen();
    await vi.advanceTimersByTimeAsync(0);
    expect(gets().length).toBe(before + 1);
    expect(useTimerStore.getState().state?.mode).toBe('countup');

    // caiu e reconectou (1º backoff ≤ 1s): o RE-open re-sincroniza de novo
    // e aplica o estado NOVO (o stop que aconteceu enquanto o WS caiu)
    getResult = idle('2026-06-10T15:02:00.000Z');
    lastWS().fireClose();
    await vi.advanceTimersByTimeAsync(1000);
    expect(wsConns()).toBe(2);
    lastWS().fireOpen();
    await vi.advanceTimersByTimeAsync(0);
    expect(gets().length).toBe(before + 2);
    expect(useTimerStore.getState().state).toBeNull();
  });

  // CA-11 — evento do WS entra pelo apply(): estado novo aplica, evento com
  // server_time ANTERIOR ao âncora é descartado (ordem WS × REST nunca
  // regride o estado) — reuso do teste-padrão da 03 com fonte WS.
  it('evento WS aplica via apply com anti-stale', async () => {
    getResult = idle('2026-06-10T15:10:00.000Z'); // âncora: 15:10
    detach = attachTimerSync();
    await vi.advanceTimersByTimeAsync(0);
    lastWS().fireOpen();
    await vi.advanceTimersByTimeAsync(0);

    // evento VELHO (15:00, reordenado na rede): descartado
    lastWS().fireMessage(
      JSON.stringify({ type: 'timer', payload: running('countdown', { remaining_seconds: 0 }, T0) }),
    );
    expect(useTimerStore.getState().state).toBeNull();

    // evento mais novo que o âncora: aplica
    lastWS().fireMessage(
      JSON.stringify({ type: 'timer', payload: running('countup', {}, '2026-06-10T15:11:00.000Z') }),
    );
    expect(useTimerStore.getState().state?.mode).toBe('countup');
  });

  // CA-15 — WS caído ≠ app degradado: mutações do store e o re-sync
  // periódico da fase 03 seguem 100% funcionais via REST.
  it('com o socket caído, mutações e re-sync via REST seguem funcionando', async () => {
    getResult = idle(T0);
    detach = attachTimerSync();
    await vi.advanceTimersByTimeAsync(0);
    lastWS().fireClose(); // conexão nunca abriu: backoff agendado

    mutate = () => {
      getResult = running('countup', {}, new Date().toISOString());
      return getResult;
    };
    await useTimerStore.getState().start({ workspace_id: 'w1', description: 'sem ws' } as StartTimerInput);
    expect(posts('/timer/start')).toHaveLength(1);
    expect(useTimerStore.getState().state?.mode).toBe('countup');

    // re-sync periódico de 60s (RNF-03 da 03) intacto
    const getsAntes = gets().length;
    await vi.advanceTimersByTimeAsync(60_000);
    expect(gets().length).toBeGreaterThan(getsAntes);
  });

  // CA-19 — o teardown devolvido fecha TUDO: interval, listeners E socket;
  // avançar os timers não cria conexão nova (reconexão cancelada).
  it('teardown desliga o socket e cancela a reconexão', async () => {
    getResult = idle(T0);
    detach = attachTimerSync();
    await vi.advanceTimersByTimeAsync(0);
    const ws = lastWS();

    detach();
    expect(ws.closed).toBe(true);
    const n = wsConns();
    await vi.advanceTimersByTimeAsync(120_000);
    expect(wsConns()).toBe(n);
  });

  // CA-16 (fase 21) — o evento type:"presence" alimenta o presenceStore SEM
  // tocar o estado do timer (onResult/anti-stale intocados). Confirma que o
  // attachTimerSync ligou o onPresence.
  it('evento presence alimenta o presenceStore sem mexer no timer', async () => {
    usePresenceStore.setState({ byWorkspace: {} });
    getResult = idle(T0);
    detach = attachTimerSync();
    await vi.advanceTimersByTimeAsync(0);
    lastWS().fireOpen();
    await vi.advanceTimersByTimeAsync(0);

    const estadoTimerAntes = useTimerStore.getState().state;
    lastWS().fireMessage(
      JSON.stringify({
        type: 'presence',
        payload: { user_id: 'colega', name: 'Cadu', workspace_id: 'ws-1', running: true, project: 'Site' },
      }),
    );

    // o membro entrou na faixa…
    expect(activeMembers(usePresenceStore.getState().byWorkspace, 'ws-1').map((m) => m.user_id)).toEqual([
      'colega',
    ]);
    // …e o estado do timer não mudou (caminho lateral)
    expect(useTimerStore.getState().state).toBe(estadoTimerAntes);

    // running:false remove
    lastWS().fireMessage(
      JSON.stringify({
        type: 'presence',
        payload: { user_id: 'colega', name: 'Cadu', workspace_id: 'ws-1', running: false },
      }),
    );
    expect(activeMembers(usePresenceStore.getState().byWorkspace, 'ws-1')).toHaveLength(0);
  });
});

describe('useTimerStore — re-sync no visibilitychange (CA-37.1)', () => {
  it('aba visível com alvo vencido re-sincroniza e dispara a transição pendente 1 vez', async () => {
    getResult = running('countdown', { target_seconds: 600, remaining_seconds: 600 }, T0);
    detach = attachTimerSync();
    await vi.advanceTimersByTimeAsync(0);
    const getsAntes = gets().length;

    // a aba "dormiu": o relógio salta 700s sem nenhum tick de interval
    vi.setSystemTime(new Date(Date.parse(T0) + 700_000));
    // o servidor reporta o countdown VENCIDO ainda rodando (CA-13.1/22.1)
    getResult = running(
      'countdown',
      { elapsed_seconds: 700, target_seconds: 600, remaining_seconds: 0 },
      new Date().toISOString(),
    );

    document.dispatchEvent(new Event('visibilitychange'));
    await vi.advanceTimersByTimeAsync(0);

    expect(gets().length).toBeGreaterThan(getsAntes); // re-sync via GET /timer
    expect(posts('/timer/stop')).toHaveLength(1); // transição pendente, 1 vez

    // novo visibilitychange com o servidor já ocioso não redispara
    document.dispatchEvent(new Event('visibilitychange'));
    await vi.advanceTimersByTimeAsync(0);
    expect(posts('/timer/stop')).toHaveLength(1);
  });
});

describe('mutações manuais — convergência em falha (fase 05, CA-17/17.1)', () => {
  // CA-17 — mutação manual que perde a corrida (409: o estado mudou em
  // outro device com o WS caído) dispara UM GET /timer IMEDIATO e aplica a
  // resposta: a tela converge pra verdade sem esperar o re-sync de 60s.
  it('pause com 409 re-sincroniza imediatamente e aplica o estado do GET', async () => {
    const verdade = running('countup', {}, '2026-06-10T15:05:00.000Z');
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const path = String(input);
        const method = init?.method ?? 'GET';
        calls.push({ method, path });
        if (method === 'POST') {
          return new Response(
            JSON.stringify({ error: { code: 'no_active_timer', message: 'sem timer ativo' } }),
            { status: 409 },
          );
        }
        return new Response(JSON.stringify(verdade), { status: 200 });
      }),
    );

    // (b) a ação NÃO rejeita (fire-and-forget seguro — D-05)
    await useTimerStore.getState().pause();

    // (a) o GET de convergência saiu na hora (nenhum timer de 60s envolvido)
    expect(gets()).toHaveLength(1);
    // ...e o estado aplicado é o da resposta do GET (a verdade do servidor)
    expect(useTimerStore.getState().state?.mode).toBe('countup');
  });

  // CA-17.1 — a falha do próprio re-sync (rede fora) também é silêncio:
  // nenhuma exceção vaza; a rede de segurança segue sendo o re-sync
  // periódico/visibilitychange da fase 03.
  it('409 + GET falhando (rede) não vaza exceção', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const method = init?.method ?? 'GET';
        calls.push({ method, path: String(input) });
        if (method === 'POST') {
          return new Response(
            JSON.stringify({ error: { code: 'entry_running', message: 'conflito' } }),
            { status: 409 },
          );
        }
        throw new TypeError('network down');
      }),
    );

    await expect(useTimerStore.getState().stop()).resolves.toBeUndefined();
    expect(gets()).toHaveLength(1); // a convergência foi TENTADA
  });
});

describe('auto-start condicional + som (fase 15, CA-17/18/18.1/19/19.2)', () => {
  const policy = (p: Partial<AutoStartPolicy>): (() => AutoStartPolicy) => () => ({
    break: false,
    focus: false,
    sound: false,
    ...p,
  });

  // CA-17 — foco zerado com auto_start_break OFF NÃO auto-pausa.
  it('foco pomodoro zerado NÃO auto-pausa com auto_start_break OFF (CA-17)', async () => {
    getResult = running('pomodoro', { target_seconds: 3, remaining_seconds: 3 }, T0);
    detach = attachTimerSync(policy({ break: false }));
    await vi.advanceTimersByTimeAsync(0);

    await vi.advanceTimersByTimeAsync(5000); // passa do zero
    expect(posts('/timer/pause')).toHaveLength(0); // espera o usuário
  });

  // CA-18 — pausa de ciclo zerada com auto_start_focus OFF NÃO auto-retoma.
  it('pausa de ciclo zerada NÃO auto-retoma com auto_start_focus OFF (CA-18)', async () => {
    getResult = emPausa(2, 'pomodoro_focus', T0);
    detach = attachTimerSync(policy({ focus: false }));
    await vi.advanceTimersByTimeAsync(0);

    await vi.advanceTimersByTimeAsync(5000);
    expect(posts('/timer/resume')).toHaveLength(0);
  });

  // CA-17/18 (binding independente das flags — furo do build): cada ramo lê a
  // SUA flag, não a do outro. Sem estes dois casos cruzados, uma mutação que
  // TROCA break↔focus entre o auto-pause e o auto-resume sobrevive à suíte.
  it('foco zerado NÃO auto-pausa com break OFF mesmo se focus ON (o pause lê break, não focus)', async () => {
    getResult = running('pomodoro', { target_seconds: 3, remaining_seconds: 3 }, T0);
    detach = attachTimerSync(policy({ break: false, focus: true }));
    await vi.advanceTimersByTimeAsync(0);

    await vi.advanceTimersByTimeAsync(5000);
    expect(posts('/timer/pause')).toHaveLength(0); // break OFF manda; focus ON não pausa
  });
  it('pausa de ciclo zerada NÃO auto-retoma com focus OFF mesmo se break ON (o resume lê focus, não break)', async () => {
    getResult = emPausa(2, 'pomodoro_focus', T0);
    detach = attachTimerSync(policy({ break: true, focus: false }));
    await vi.advanceTimersByTimeAsync(0);

    await vi.advanceTimersByTimeAsync(5000);
    expect(posts('/timer/resume')).toHaveLength(0); // focus OFF manda; break ON não retoma
  });

  // CA-18.1 — política default (true/true) preserva o ciclo da fase 03:
  // auto-pause e auto-resume disparam (exatamente 1×).
  it('política default (true/true) preserva auto-pause/resume da fase 03 (CA-18.1)', async () => {
    getResult = running('pomodoro', { target_seconds: 3, remaining_seconds: 3 }, T0);
    mutate = () => {
      getResult = emPausa(300, 'pomodoro_focus', new Date().toISOString());
      return getResult;
    };
    detach = attachTimerSync(); // sem política → default true/true/false
    await vi.advanceTimersByTimeAsync(0);
    await vi.advanceTimersByTimeAsync(5000);
    expect(posts('/timer/pause')).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(10000);
    expect(posts('/timer/pause')).toHaveLength(1); // não redispara
  });

  // CA-19 — som ON: o beep toca 1× quando a fase termina.
  it('beep toca 1× no fim do foco com sound ON (CA-19)', async () => {
    getResult = running('pomodoro', { target_seconds: 3, remaining_seconds: 3 }, T0);
    // auto-start OFF pra a fase ficar parada em 0 e o teste isolar o som
    detach = attachTimerSync(policy({ sound: true }));
    await vi.advanceTimersByTimeAsync(0);

    await vi.advanceTimersByTimeAsync(5000);
    expect(focusSound.playPhaseEndBeep).toHaveBeenCalledTimes(1);
  });

  // CA-19 — som OFF: silêncio.
  it('NÃO toca beep com sound OFF', async () => {
    getResult = running('pomodoro', { target_seconds: 3, remaining_seconds: 3 }, T0);
    detach = attachTimerSync(policy({ sound: false }));
    await vi.advanceTimersByTimeAsync(0);
    await vi.advanceTimersByTimeAsync(5000);
    expect(focusSound.playPhaseEndBeep).not.toHaveBeenCalled();
  });

  // CA-19.2 (FURO) — com auto-start OFF e som ON, a fase fica parada em
  // remaining=0; vários ticks E vários re-syncs (apply do MESMO estado) NÃO
  // podem re-disparar o beep — a soundedKey é por stateKey, não resetada
  // pelo apply().
  it('beep toca EXATAMENTE 1× mesmo com vários ticks/re-syncs no estado parado em 0 (CA-19.2)', async () => {
    getResult = running('pomodoro', { target_seconds: 2, remaining_seconds: 2 }, T0);
    detach = attachTimerSync(policy({ sound: true })); // auto-start OFF, som ON
    await vi.advanceTimersByTimeAsync(0);

    // passa do zero: 1 beep
    await vi.advanceTimersByTimeAsync(5000);
    expect(focusSound.playPhaseEndBeep).toHaveBeenCalledTimes(1);

    // vários re-syncs do MESMO estado parado em 0 (o servidor reporta o foco
    // vencido ainda rodando — CA-22.1): o apply NÃO re-arma o beep
    for (let i = 0; i < 3; i++) {
      getResult = running(
        'pomodoro',
        { elapsed_seconds: 10 + i, target_seconds: 2, remaining_seconds: 0 },
        T0, // MESMO server_time/stateKey (mesma entry e1)
      );
      useTimerStore.getState().apply(getResult);
      useTimerStore.getState().checkAutoTransition();
    }
    // e mais ticks do interval no estado parado
    await vi.advanceTimersByTimeAsync(120_000);

    expect(focusSound.playPhaseEndBeep).toHaveBeenCalledTimes(1);
  });

  // sem regressão: store sem política setada (setAutoStartPolicy()) — os
  // testes da fase 03 NÃO setam política, então o default true preserva a
  // auto-transição. Pinado aqui também.
  it('store sem política → default true → auto-transição segue (sem regressão)', async () => {
    setAutoStartPolicy(); // garante o default
    getResult = running('pomodoro', { target_seconds: 3, remaining_seconds: 3 }, T0);
    mutate = () => {
      getResult = emPausa(300, 'pomodoro_focus', new Date().toISOString());
      return getResult;
    };
    detach = attachTimerSync();
    await vi.advanceTimersByTimeAsync(0);
    await vi.advanceTimersByTimeAsync(5000);
    expect(posts('/timer/pause')).toHaveLength(1);
  });
});

// fase 27 (allowance HITL 23/06): editCurrent → PATCH /timer; sucesso → apply,
// falha → load, nunca rejeita (CA-17). Timing intocado é garantido no backend.
describe('useTimerStore.editCurrent — allowance fase 27', () => {
  const patches = () => calls.filter((c) => c.method === 'PATCH' && c.path.endsWith('/timer'));

  it('editCurrent chama PATCH /timer e o apply reflete o State da resposta', async () => {
    useTimerStore.setState({ ...initialTimerState, anchorMs: Date.parse(T0) });
    mutate = () => running('countup', { entry: { ...running('countup', {}, T0).state!.entry!, description: 'editado' } }, '2026-06-10T15:01:00.000Z');

    await useTimerStore.getState().editCurrent({ description: 'editado', project_id: 'p1' });

    expect(patches()).toHaveLength(1);
    expect(useTimerStore.getState().state?.entry?.description).toBe('editado');
  });

  it('editCurrent que falha → load(), não rejeita (CA-17)', async () => {
    useTimerStore.setState({ ...initialTimerState });
    getResult = idle(T0);
    // PATCH rejeita; o mutate engole e cai no load() (GET /timer)
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const path = String(input);
        const method = init?.method ?? 'GET';
        calls.push({ method, path });
        if (method === 'PATCH') return new Response('{"error":{"code":"no_active_timer"}}', { status: 409 });
        return new Response(JSON.stringify(getResult), { status: 200 });
      }),
    );

    await expect(useTimerStore.getState().editCurrent({ description: 'x' })).resolves.toBeUndefined();
    expect(patches()).toHaveLength(1);
    expect(gets()).toHaveLength(1); // load() de recuperação
  });
});
