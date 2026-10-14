// focusSound.ts (fase 15, CA-19/19.1) — alerta sonoro no fim de uma fase
// do Pomodoro. Beep NATIVO via WebAudio (oscillator curto, ~150ms): zero
// arquivo de áudio empacotado, zero dependência (RNF-01). Falha SILENCIOSA
// se o WebAudio não existe ou o browser bloqueia áudio sem gesto do usuário
// (CA-19.1) — o Foco segue funcionando, sem erro visível nem loop.

// AudioCtor resolve AudioContext (ou o webkit legado) sem quebrar tipos.
type AudioCtor = typeof AudioContext;

function resolveAudioContext(): AudioCtor | null {
  if (typeof window === 'undefined') return null;
  const w = window as unknown as {
    AudioContext?: AudioCtor;
    webkitAudioContext?: AudioCtor;
  };
  return w.AudioContext ?? w.webkitAudioContext ?? null;
}

// playPhaseEndBeep emite um beep curto. Nunca lança: qualquer falha (sem
// WebAudio, contexto suspenso por falta de gesto, etc.) é engolida (CA-19.1).
export function playPhaseEndBeep(): void {
  try {
    const Ctor = resolveAudioContext();
    if (!Ctor) return; // sem suporte → no-op silencioso

    const ctx = new Ctor();
    const osc = ctx.createOscillator();
    const gain = ctx.createGain();
    osc.type = 'sine';
    osc.frequency.value = 880; // lá agudo, discreto
    // envelope curto pra não estalar: sobe rápido, decai em ~150ms
    const now = ctx.currentTime;
    gain.gain.setValueAtTime(0.0001, now);
    gain.gain.exponentialRampToValueAtTime(0.18, now + 0.01);
    gain.gain.exponentialRampToValueAtTime(0.0001, now + 0.15);
    osc.connect(gain);
    gain.connect(ctx.destination);
    osc.start(now);
    osc.stop(now + 0.16);
    // libera o contexto quando o beep termina (sem vazar AudioContext)
    osc.onended = () => {
      void ctx.close().catch(() => undefined);
    };
  } catch {
    // CA-19.1: bloqueio/erro de áudio é silencioso
  }
}
