// canComment.ts (fase 19, re-hospedado na fase 24 T-11/CA-28.1): pode comentar
// nesta task? can_write (member/owner) OU ser o assignee dela (carve-out
// viewer-assignee da fase 19). PURA e exportada pra teste — o guard
// `assignee != null` evita o falso match null===null (um viewer sem assignee NÃO
// comenta). Load-bearing pro thread no TaskPanel (a fase 24 substituiu o
// ProjectBoard pelo popup, mas o carve-out NÃO pode ser perdido). Mutação
// (===→!==, ||→&&, dropar o ramo do assignee, ou remover o guard de null) deve
// quebrar canComment.spec.
export function canCommentOn(
  canWrite: boolean,
  task: { assignee: string | null } | null,
  currentUserId: string | null,
): boolean {
  return (
    canWrite ||
    (task != null && task.assignee != null && task.assignee === currentUserId)
  );
}
