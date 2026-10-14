import { mkdirSync, rmSync, statSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';

// Mutex cross-ARQUIVO pro TIMER GLOBAL do E2E. O googlestub só autentica 1
// usuário e o timer é per-usuário, então os specs que MUTAM o timer
// (cronometro.spec e remoto.spec) não podem rodar concorrentes sob
// `fullyParallel` — o `ensureIdle`/`Parar`/`Iniciar` de um clobbera o timer do
// outro (foi o que derrubou o gate da fase 23: o stop do remoto.spec apagava o
// timer que o cronometro.spec acabara de iniciar pra testar o reload — CA-37).
// `mkdir` é ATÔMICO: o 1º worker cria o lock e os outros esperam. Cada arquivo
// adquire no `beforeAll` e libera no `afterAll`, serializando os DOIS arquivos
// entre si (o resto da suíte segue paralelo). Espelha o `ensureIdle`/serial que
// os dois specs já usam internamente — só estende a serialização pra cross-file.
const LOCK = join(tmpdir(), 'tt-e2e-timer.lock');
const STALE_MS = 90_000; // worker morto sem liberar → rouba o lock obsoleto
const TIMEOUT_MS = 180_000;

export async function acquireTimerLock(): Promise<void> {
  const deadline = Date.now() + TIMEOUT_MS;
  for (;;) {
    try {
      mkdirSync(LOCK);
      return;
    } catch {
      // lock ocupado: se for obsoleto (worker que morreu), rouba.
      try {
        if (Date.now() - statSync(LOCK).mtimeMs > STALE_MS) {
          rmSync(LOCK, { recursive: true, force: true });
          continue;
        }
      } catch {
        // o lock sumiu entre o mkdir e o stat — tenta de novo na hora.
        continue;
      }
      if (Date.now() > deadline) throw new Error('timer lock: timeout esperando o mutex');
      await new Promise((r) => setTimeout(r, 100));
    }
  }
}

export function releaseTimerLock(): void {
  try {
    rmSync(LOCK, { recursive: true, force: true });
  } catch {
    // já liberado / nunca adquirido — no-op.
  }
}
