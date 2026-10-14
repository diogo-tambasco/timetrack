// ProjectBoard.canComment.spec — fase 19 (verificação do build, CA-19.1):
// exercita o wiring REAL do carve-out viewer-assignee no front (canCommentOn).
// Sem este teste, mutações em canCommentOn (===→!==, ||→&&, dropar o ramo do
// assignee, remover o guard de null) sobreviviam à suíte inteira — os specs do
// TaskThread/TaskModal passam canComment como prop literal e nunca tocam a função.
import { describe, it, expect } from 'vitest';
import { canCommentOn } from './canComment';

const ME = 'u-me';

describe('canCommentOn — carve-out viewer-assignee (CA-19.1)', () => {
  it('member/owner (can_write) sempre pode comentar, independente do assignee', () => {
    expect(canCommentOn(true, null, ME)).toBe(true);
    expect(canCommentOn(true, { assignee: null }, ME)).toBe(true);
    expect(canCommentOn(true, { assignee: 'outro' }, ME)).toBe(true);
  });

  it('viewer-assignee (assignee === eu) PODE comentar', () => {
    expect(canCommentOn(false, { assignee: ME }, ME)).toBe(true);
  });

  it('viewer NÃO-assignee (assignee de outro) NÃO pode', () => {
    expect(canCommentOn(false, { assignee: 'outro' }, ME)).toBe(false);
  });

  it('viewer numa task SEM assignee NÃO pode (sem falso match null === null)', () => {
    expect(canCommentOn(false, { assignee: null }, null)).toBe(false);
    expect(canCommentOn(false, { assignee: null }, ME)).toBe(false);
  });

  it('task null + viewer NÃO pode', () => {
    expect(canCommentOn(false, null, ME)).toBe(false);
  });
});
