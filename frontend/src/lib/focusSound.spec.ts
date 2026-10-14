// focusSound.spec.ts (fase 15 T-05) — CA-19/19.1: o beep toca via WebAudio
// quando há suporte (1 oscillator iniciado por chamada) e é no-op SILENCIOSO
// sem suporte ou quando o AudioContext lança (browser bloqueia sem gesto).
import { afterEach, describe, expect, it, vi } from 'vitest';
import { playPhaseEndBeep } from './focusSound';

type Win = typeof window & {
  AudioContext?: unknown;
  webkitAudioContext?: unknown;
};

function setAudioContext(value: unknown) {
  Object.defineProperty(window, 'AudioContext', {
    value,
    configurable: true,
    writable: true,
  });
}

// fábrica de um AudioContext fake que registra o que foi criado/iniciado.
function makeFakeAudio() {
  const started: number[] = [];
  const closed = { n: 0 };
  const osc = {
    type: '',
    frequency: { value: 0 },
    connect: vi.fn(),
    start: vi.fn((t: number) => started.push(t)),
    stop: vi.fn(),
    onended: null as null | (() => void),
  };
  const gain = {
    gain: {
      setValueAtTime: vi.fn(),
      exponentialRampToValueAtTime: vi.fn(),
    },
    connect: vi.fn(),
  };
  class FakeCtx {
    currentTime = 0;
    destination = {};
    createOscillator() {
      return osc;
    }
    createGain() {
      return gain;
    }
    close() {
      closed.n += 1;
      return Promise.resolve();
    }
  }
  return { FakeCtx, osc, started, closed };
}

afterEach(() => {
  const w = window as Win;
  delete w.AudioContext;
  delete w.webkitAudioContext;
});

describe('playPhaseEndBeep', () => {
  it('toca 1 beep (oscillator iniciado) quando há WebAudio — CA-19', () => {
    const { FakeCtx, osc, started } = makeFakeAudio();
    setAudioContext(FakeCtx);

    playPhaseEndBeep();

    expect(started).toHaveLength(1);
    expect(osc.start).toHaveBeenCalledTimes(1);
    expect(osc.stop).toHaveBeenCalledTimes(1);
  });

  it('é no-op SILENCIOSO sem suporte a WebAudio — CA-19.1', () => {
    // sem AudioContext nem webkitAudioContext no window
    const w = window as Win;
    delete w.AudioContext;
    delete w.webkitAudioContext;
    expect(() => playPhaseEndBeep()).not.toThrow();
  });

  it('engole o erro quando o AudioContext lança (bloqueio sem gesto) — CA-19.1', () => {
    class Throwing {
      constructor() {
        throw new Error('NotAllowedError: gesture required');
      }
    }
    setAudioContext(Throwing);
    expect(() => playPhaseEndBeep()).not.toThrow();
  });
});
