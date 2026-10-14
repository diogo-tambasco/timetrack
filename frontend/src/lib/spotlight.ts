// Núcleo PURO do Spotlight (fase 27, D-01): parse/filtro/título dos comandos
// `/task` `/note` `/timer` `/goal`. SEM React, SEM DOM, SEM fetch — só strings.
// Espelha 1:1 SPOT/spotItems/spotTitle do mockup (l.625-636). Testado por
// Vitest unit direto (igual lib/calendar-layout.ts da fase 11).

export type SpotAct = 'task' | 'note' | 'timer' | 'goal';

export interface SpotCommand {
  act: SpotAct;
  label: string;
  desc: string;
  icon: string;
}

// 4 comandos fixos do PRD (espelho do SPOT do mockup l.625-630). O `icon` é o
// nome de uma chave do Icon (resolvido na UI — o núcleo não conhece o React).
export const SPOT: SpotCommand[] = [
  { act: 'task', label: 'New task', desc: '/task — cria uma task neste projeto', icon: 'board' },
  { act: 'note', label: 'New note', desc: '/note — cria uma nota neste projeto', icon: 'list' },
  { act: 'timer', label: 'Start timer', desc: '/timer — começa a cronometrar', icon: 'play' },
  { act: 'goal', label: 'New goal', desc: '/goal — cria uma meta', icon: 'star' },
];

// spotToken: 1º token sem a barra inicial, lowercase (espelha o mockup l.633).
export function spotToken(input: string): string {
  return (input || '')
    .replace(/^\//, '')
    .toLowerCase()
    .split(/\s+/)
    .filter(Boolean)[0] ?? '';
}

// spotItems: filtra a lista pelo 1º token — act.startsWith(token) OU alguma
// PALAVRA do label começa com o token (CA-07). Token vazio = lista completa.
// (O mockup usa `label.includes(q)` cru, mas isso casaria "Start timer" pra
// "/ta" — a CA-07 manda `/ta ⇒ só /task`; o casamento por início-de-palavra
// honra o exemplo binding da spec sem perder o "label contém o token".)
export function spotItems(input: string): SpotCommand[] {
  const q = spotToken(input);
  if (!q) return SPOT;
  return SPOT.filter(
    (s) =>
      s.act.startsWith(q) ||
      s.label.toLowerCase().split(/\s+/).some((w) => w.startsWith(q)),
  );
}

// spotTitle: o input SEM o comando (espelha o mockup l.636). Trim.
//   "/task Comprar domínio" → "Comprar domínio"; "/timer" → "".
export function spotTitle(input: string): string {
  return (input || '').replace(/^\/?\s*(task|note|timer|goal)\b/i, '').trim();
}

// spotParse: act = comando explícito do 1º token (null = "sem comando" — RF-19).
// O 1º token só vira `act` se for EXATAMENTE um dos 4 comandos (com ou sem `/`),
// seguido de espaço ou fim — "/x" e "task qualquer outra coisa" que não casa
// exato caem em null (sem comando). title = spotTitle(input).
export function spotParse(input: string): { act: SpotAct | null; title: string } {
  const m = (input || '').match(/^\/?\s*(task|note|timer|goal)\b/i);
  const act = m ? (m[1].toLowerCase() as SpotAct) : null;
  return { act, title: spotTitle(input) };
}
