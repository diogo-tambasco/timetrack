// useBoardTimerSync (fase 30, CA-13/30-H5): o tempo "GRUDA" na task. Observa o
// timer GLOBAL e invalida o board do projeto quando uma entry rodando DESTE
// projeto para (transição → ocioso), pro `tracked_seconds` das tasks convergir
// SEM reload (padrão de invalidação local da fase 12).
//
// Por que um hook no SHELL (ProjectDetail), e não dentro do TaskTimeControl:
// o TaskTimeControl DESMONTA ao fechar o popup → um efeito nele perderia o Stop
// disparado pela TimerBar/troca/auto-corte com o popup já FECHADO (corrida real
// do 30-H5). Este hook vive na página de projeto, que permanece montada enquanto
// o projeto está aberto (qualquer aba, popup aberto/fechado).
//
// Gatilho exato: observa a IDENTIDADE da entry rodando (entry.id) + o projeto dela.
// Invalida UMA vez quando a entry que estava rodando NESTE projeto deixa de ser a
// corrente — seja por PARAR (→ ocioso/pausa, entry=null) seja por TROCAR de task/
// projeto (Start atômico que fecha a anterior na mesma transação — CA-05/CA-12).
// Em AMBOS os casos a entry deste projeto fechou e o `tracked_seconds` mudou, então
// o board precisa convergir. `useRef` guarda (id, project_id) anteriores (sem loop
// de re-render). Como o `state` só atualiza no `apply` da resposta do servidor
// (anti-stale da fase 04), a invalidação ocorre APÓS a entry fechada já estar
// contabilizada no /board. NÃO invalida ao INICIAR do ocioso (prev não era deste
// projeto), NÃO pra entry que nunca foi deste projeto, NÃO a cada render.
//
// (A 1ª versão só cobria a transição → ocioso [entryProjectId === null]; a troca
// DIRETA de task/projeto sem passar por ocioso deixava o tracked stale — gap achado
// pela verificação adversarial do build, 4 finders convergentes. Observar a
// identidade da entry cobre os dois sem regredir stop/pause [entry=null nos dois].)
import { useEffect, useRef } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useTimerStore } from '../../lib/timerStore';
import { boardKey } from '../../lib/queries/objectives';

export function useBoardTimerSync(wsId: string, projectId: string): void {
  const qc = useQueryClient();
  const entryId = useTimerStore((s) => s.state?.entry?.id ?? null);
  const entryProjectId = useTimerStore((s) => s.state?.entry?.project_id ?? null);
  const prevId = useRef<string | null>(null);
  const prevProj = useRef<string | null>(null);
  useEffect(() => {
    // transição = a entry rodando mudou de identidade E a ANTERIOR era deste projeto.
    if (entryId !== prevId.current && prevProj.current === projectId) {
      qc.invalidateQueries({ queryKey: boardKey(wsId, projectId) });
    }
    prevId.current = entryId;
    prevProj.current = entryProjectId;
  }, [entryId, entryProjectId, projectId, wsId, qc]);
}
