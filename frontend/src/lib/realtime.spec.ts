// realtime.spec — T-06 da fase 04: cliente WS puro (connectTimerSocket).
// WebSocket STUBADO com classe fake + fake timers — zero dependência nova
// (RNF-01), mesmo padrão do stub de fetch puro da 03.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { connectTimerSocket } from './realtime';

const T0 = '2026-06-11T12:00:00.000Z';

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

  // helpers de teste — simulam o lado do servidor/rede
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

const last = () => FakeWebSocket.instances[FakeWebSocket.instances.length - 1];
const conns = () => FakeWebSocket.instances.length;

let dispose: () => void = () => {};

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date(T0));
  FakeWebSocket.instances = [];
  vi.stubGlobal('WebSocket', FakeWebSocket);
  vi.stubGlobal('fetch', vi.fn()); // sentinela do "sem polling" (CA-16)
});

afterEach(() => {
  dispose();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe('connectTimerSocket', () => {
  // CA-11 (parcial) — mensagem type:"timer" entrega o payload ao onResult.
  it('mensagem timer chama onResult com o payload', () => {
    const onResult = vi.fn();
    ({ dispose } = connectTimerSocket({ makeUrl: () => 'ws://x/api/v1/ws', onResult }));

    last().fireOpen();
    const payload = { state: null, server_time: T0 };
    last().fireMessage(JSON.stringify({ type: 'timer', payload }));

    expect(onResult).toHaveBeenCalledTimes(1);
    expect(onResult).toHaveBeenCalledWith(payload);
  });

  // CA-18 — JSON inválido e type desconhecido são ignorados: sem fechar o
  // socket, sem chamar onResult (forward-compat com tipos futuros).
  it('mensagem malformada ou de tipo desconhecido é ignorada sem quebrar', () => {
    const onResult = vi.fn();
    const onNotice = vi.fn();
    ({ dispose } = connectTimerSocket({ makeUrl: () => 'ws://x/api/v1/ws', onResult, onNotice }));

    last().fireOpen();
    last().fireMessage('isto não é json {');
    last().fireMessage(JSON.stringify({ type: 'entries', payload: { qualquer: 1 } }));
    last().fireMessage(JSON.stringify({ semType: true }));

    expect(onResult).not.toHaveBeenCalled();
    expect(onNotice).not.toHaveBeenCalled();
    expect(last().closed).toBe(false);
    expect(conns()).toBe(1); // nenhuma reconexão disparada
  });

  // CA-11 (fase 16) — mensagem type:"notice" chama onNotice com o payload, SEM
  // tocar o onResult (o ramo do timer fica intocado).
  it('mensagem notice chama onNotice e não toca o onResult', () => {
    const onResult = vi.fn();
    const onNotice = vi.fn();
    ({ dispose } = connectTimerSocket({ makeUrl: () => 'ws://x/api/v1/ws', onResult, onNotice }));

    last().fireOpen();
    const payload = { kind: 'timer_auto_cut', entry_id: 'e-1', stopped_at: T0 };
    last().fireMessage(JSON.stringify({ type: 'notice', payload }));

    expect(onNotice).toHaveBeenCalledTimes(1);
    expect(onNotice).toHaveBeenCalledWith(payload);
    expect(onResult).not.toHaveBeenCalled();
    expect(last().closed).toBe(false);
  });

  // CA-11 — depois de um notice, um type:"timer" SEGUE chamando onResult
  // (regressão da fase 04 preservada: o ramo timer não muda).
  it('timer continua chamando onResult mesmo após um notice', () => {
    const onResult = vi.fn();
    const onNotice = vi.fn();
    ({ dispose } = connectTimerSocket({ makeUrl: () => 'ws://x/api/v1/ws', onResult, onNotice }));

    last().fireOpen();
    last().fireMessage(JSON.stringify({ type: 'notice', payload: { kind: 'timer_auto_cut' } }));
    const result = { state: null, server_time: T0 };
    last().fireMessage(JSON.stringify({ type: 'timer', payload: result }));

    expect(onResult).toHaveBeenCalledTimes(1);
    expect(onResult).toHaveBeenCalledWith(result);
    expect(onNotice).toHaveBeenCalledTimes(1);
  });

  // notice sem onNotice registrado (ou type:"notice" desconhecido) não quebra.
  it('notice sem onNotice registrado não derruba a conexão', () => {
    const onResult = vi.fn();
    ({ dispose } = connectTimerSocket({ makeUrl: () => 'ws://x/api/v1/ws', onResult }));
    last().fireOpen();
    last().fireMessage(JSON.stringify({ type: 'notice', payload: { kind: 'x' } }));
    expect(onResult).not.toHaveBeenCalled();
    expect(last().closed).toBe(false);
    expect(conns()).toBe(1);
  });

  // CA-15 (fase 21) — mensagem type:"presence" chama onPresence com o payload,
  // SEM tocar onResult nem onNotice (ramos timer/notice intocados).
  it('mensagem presence chama onPresence e não toca timer/notice', () => {
    const onResult = vi.fn();
    const onNotice = vi.fn();
    const onPresence = vi.fn();
    ({ dispose } = connectTimerSocket({
      makeUrl: () => 'ws://x/api/v1/ws',
      onResult,
      onNotice,
      onPresence,
    }));

    last().fireOpen();
    const payload = {
      user_id: 'u-a',
      name: 'Ana',
      workspace_id: 'ws-1',
      running: true,
      project: 'Site',
      since: T0,
    };
    last().fireMessage(JSON.stringify({ type: 'presence', payload }));

    expect(onPresence).toHaveBeenCalledTimes(1);
    expect(onPresence).toHaveBeenCalledWith(payload);
    expect(onResult).not.toHaveBeenCalled();
    expect(onNotice).not.toHaveBeenCalled();
    expect(last().closed).toBe(false);
  });

  // CA-15/20 — depois de um presence, timer SEGUE chamando onResult e notice o
  // onNotice (regressão das fases 04/16 preservada: os ramos não mudam).
  it('timer e notice continuam funcionando após um presence', () => {
    const onResult = vi.fn();
    const onNotice = vi.fn();
    const onPresence = vi.fn();
    ({ dispose } = connectTimerSocket({
      makeUrl: () => 'ws://x/api/v1/ws',
      onResult,
      onNotice,
      onPresence,
    }));

    last().fireOpen();
    last().fireMessage(
      JSON.stringify({ type: 'presence', payload: { user_id: 'u', workspace_id: 'w', running: false } }),
    );
    const result = { state: null, server_time: T0 };
    last().fireMessage(JSON.stringify({ type: 'timer', payload: result }));
    last().fireMessage(JSON.stringify({ type: 'notice', payload: { kind: 'timer_auto_cut' } }));

    expect(onResult).toHaveBeenCalledTimes(1);
    expect(onResult).toHaveBeenCalledWith(result);
    expect(onNotice).toHaveBeenCalledTimes(1);
    expect(onPresence).toHaveBeenCalledTimes(1);
  });

  // presence sem onPresence registrado não derruba a conexão (forward-compat).
  it('presence sem onPresence registrado não derruba a conexão', () => {
    const onResult = vi.fn();
    ({ dispose } = connectTimerSocket({ makeUrl: () => 'ws://x/api/v1/ws', onResult }));
    last().fireOpen();
    last().fireMessage(
      JSON.stringify({ type: 'presence', payload: { user_id: 'u', workspace_id: 'w', running: true } }),
    );
    expect(onResult).not.toHaveBeenCalled();
    expect(last().closed).toBe(false);
    expect(conns()).toBe(1);
  });

  // CA-16 — reconexão com backoff exponencial 1s→2s→…→30s (teto), jitter
  // injetável (identidade aqui pra sequência ser determinística) e NENHUMA
  // requisição periódica nova (fetch nunca é chamado por este módulo).
  it('reconecta com atrasos crescentes até o teto, sem polling', async () => {
    ({ dispose } = connectTimerSocket({
      makeUrl: () => 'ws://x/api/v1/ws',
      onResult: vi.fn(),
      jitter: (ms) => ms,
    }));
    expect(conns()).toBe(1);

    const delays = [1000, 2000, 4000, 8000, 16000, 30000, 30000]; // 2^5=32s clampa em 30s
    for (const delay of delays) {
      const before = conns();
      last().fireClose();
      await vi.advanceTimersByTimeAsync(delay - 1);
      expect(conns()).toBe(before); // ainda esperando o backoff
      await vi.advanceTimersByTimeAsync(1);
      expect(conns()).toBe(before + 1); // reconectou exatamente no atraso
    }

    expect(vi.mocked(fetch)).not.toHaveBeenCalled();
  });

  // backoff REARMA após conexão bem-sucedida: open zera a escada.
  it('open bem-sucedido zera o backoff', async () => {
    ({ dispose } = connectTimerSocket({
      makeUrl: () => 'ws://x/api/v1/ws',
      onResult: vi.fn(),
      jitter: (ms) => ms,
    }));
    last().fireClose();
    await vi.advanceTimersByTimeAsync(1000);
    last().fireClose();
    await vi.advanceTimersByTimeAsync(2000);
    expect(conns()).toBe(3);

    last().fireOpen(); // conectou de verdade
    last().fireClose(); // caiu de novo: recomeça do 1s
    await vi.advanceTimersByTimeAsync(1000);
    expect(conns()).toBe(4);
  });

  // CA-19 — dispose() fecha o socket e cancela o reagendamento: avançar os
  // timers não cria conexão nova.
  it('dispose fecha o socket e cancela a reconexão agendada', async () => {
    const { dispose: d } = connectTimerSocket({ makeUrl: () => 'ws://x/api/v1/ws', onResult: vi.fn() });
    dispose = d;

    last().fireOpen();
    d();
    expect(last().closed).toBe(true);
    await vi.advanceTimersByTimeAsync(120_000);
    expect(conns()).toBe(1); // o close do próprio dispose não reagenda

    // e dispose com reconexão JÁ agendada também cancela
    FakeWebSocket.instances = [];
    const second = connectTimerSocket({ makeUrl: () => 'ws://x/api/v1/ws', onResult: vi.fn() });
    dispose = second.dispose;
    last().fireClose(); // agenda reconexão (~1s)
    second.dispose();
    await vi.advanceTimersByTimeAsync(120_000);
    expect(conns()).toBe(1);
  });

  // makeUrl é chamado A CADA tentativa (D-07) — a URL pode depender do
  // location atual quando a reconexão acontecer.
  it('makeUrl é reavaliado a cada tentativa', async () => {
    const makeUrl = vi.fn(() => 'ws://x/api/v1/ws');
    ({ dispose } = connectTimerSocket({ makeUrl, onResult: vi.fn(), jitter: (ms) => ms }));
    last().fireClose();
    await vi.advanceTimersByTimeAsync(1000);
    expect(makeUrl).toHaveBeenCalledTimes(2);
  });

  // CA-16 — o jitter DEFAULT (o único que roda em produção) mantém o atraso
  // em [50%, 100%] da base, nunca ~0 (verificação adversarial do build,
  // 11/06: todos os outros testes injetam jitter identidade — um default
  // degenerado pra 0ms recriaria a tempestade que o CA-16 proíbe).
  it('jitter default espaça a reconexão em [50%, 100%] da base', async () => {
    const rand = vi.spyOn(Math, 'random');

    // piso: random=0 → delay = 1000 × 0.5 = 500ms exatos
    rand.mockReturnValue(0);
    ({ dispose } = connectTimerSocket({ makeUrl: () => 'ws://x/api/v1/ws', onResult: vi.fn() }));
    last().fireClose();
    await vi.advanceTimersByTimeAsync(499);
    expect(conns()).toBe(1); // nada antes de 50% da base
    await vi.advanceTimersByTimeAsync(1);
    expect(conns()).toBe(2);

    // teto: random→1 → delay ≈ 100% da base (open zera a escada: base 1000ms)
    rand.mockReturnValue(0.999999);
    last().fireOpen();
    last().fireClose();
    await vi.advanceTimersByTimeAsync(998);
    expect(conns()).toBe(2); // ainda esperando perto do teto
    await vi.advanceTimersByTimeAsync(2);
    expect(conns()).toBe(3);

    rand.mockRestore();
  });
});
