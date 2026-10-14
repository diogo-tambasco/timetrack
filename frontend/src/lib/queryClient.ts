// queryClient singleton — um client por app (D-12 da fase 02). Extraído pra um
// módulo (fase 21) pra que o attachTimerSync (fora de React) possa invalidar
// queries — ex.: re-buscar o snapshot de presence ao reconectar o socket
// (CA-18). O App.tsx importa este mesmo client no QueryClientProvider.
import { QueryClient } from '@tanstack/react-query';

// refetchOnWindowFocus DESLIGADO: o app já tem realtime por WebSocket (timer/
// presence) + invalidate explícito pós-mutação, então o refetch automático no foco
// da janela é redundante — e PERIGOSO no fluxo do popup: um refetch do board
// disparado por um evento de foco/visibilidade durante a edição podia LER o estado
// PRÉ-PATCH e, completando fora de ordem, sobrescrever o refetch fresco do
// invalidate → o board ficava preso no valor antigo ("New task") mesmo com o PATCH
// já commitado (CA-20). Desligar elimina essa corrida. As listas seguem frescas
// pelos invalidates das mutações.
export const queryClient = new QueryClient({
  defaultOptions: { queries: { refetchOnWindowFocus: false } },
});
