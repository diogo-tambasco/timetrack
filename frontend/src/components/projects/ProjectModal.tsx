// Modal Novo/Editar projeto (T-18) — porte fiel do newproject.jsx:
// nome, cliente (select), paleta de cores, toggle Privado/Compartilhado,
// meta h/sem e membros do projeto (checkbox + Editar/Só vê, Owner fixo).
// Privado desabilita a seção de membros (CA-28); modo Editar abre
// pré-preenchido e o submit aplica um PATCH só com o que mudou — a lista
// reflete por invalidação, sem reload (CA-28.1/29). A paleta envia hex
// (#RRGGBB, CA-07) com os valores das cores --p-* do tokens.css.
import { useState, type FormEvent, type ReactNode } from 'react';
import type {
  Project,
  ProjectAccess,
  ProjectInput,
  ProjectMember,
  WorkspaceMember,
} from '../../lib/api';
import { Icon } from '../icons';
import { useAuthStore } from '../../lib/auth';
import { PROJECT_PALETTE } from '../../lib/palette';
import { useClients } from '../../lib/queries/clients';
import { useCreateProject, usePatchProject, useProject } from '../../lib/queries/projects';
import { useWorkspaceMembers } from '../../lib/queries/workspace';

type ProjectModalProps = {
  wsId: string;
  // presente = modo Editar (os members vêm do detalhe via useProject)
  project?: Project | null;
  onClose: () => void;
};

export default function ProjectModal({ wsId, project = null, onClose }: ProjectModalProps) {
  if (project) return <EditLoader wsId={wsId} project={project} onClose={onClose} />;
  return <ModalForm wsId={wsId} initial={null} onClose={onClose} />;
}

// modo Editar espera o detalhe (members embutidos) antes de semear o form
function EditLoader({
  wsId,
  project,
  onClose,
}: {
  wsId: string;
  project: Project;
  onClose: () => void;
}) {
  const { data: detail } = useProject(wsId, project.id);
  if (!detail) return null;
  return <ModalForm wsId={wsId} initial={detail} onClose={onClose} />;
}

const inputStyle = {
  background: 'var(--surface-2)',
  border: '1px solid var(--border-2)',
  borderRadius: 'var(--r-md)',
  padding: '11px 13px',
  fontSize: 14,
  color: 'var(--text-1)',
  fontFamily: 'var(--font-ui)',
  outline: 'none',
  width: '100%',
  boxSizing: 'border-box',
} as const;

function Field({
  id,
  label,
  hint,
  children,
}: {
  id?: string;
  label: string;
  hint?: string;
  children: ReactNode;
}) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 7 }}>
      <label htmlFor={id} style={{ fontSize: 12.5, fontWeight: 540, color: 'var(--text-2)' }}>
        {label}
      </label>
      {children}
      {hint && (
        <span className="faint" style={{ fontSize: 11.5 }}>
          {hint}
        </span>
      )}
    </div>
  );
}

function ModalForm({
  wsId,
  initial,
  onClose,
}: {
  wsId: string;
  initial: (Project & { members: ProjectMember[] }) | null;
  onClose: () => void;
}) {
  const meId = useAuthStore((s) => s.me?.user.id ?? null);
  const { data: clients = [] } = useClients(wsId, { archived: true });
  const { data: wsMembers = [] } = useWorkspaceMembers(wsId);
  const criar = useCreateProject(wsId);
  const editar = usePatchProject(wsId);

  const [nome, setNome] = useState(initial?.name ?? '');
  const [clienteId, setClienteId] = useState(initial?.client_id ?? '');
  const [cor, setCor] = useState(initial?.color ?? PROJECT_PALETTE[0]);
  const [vis, setVis] = useState<'private' | 'shared'>(initial?.visibility ?? 'shared');
  const [meta, setMeta] = useState(
    initial?.weekly_hours_goal !== null && initial?.weekly_hours_goal !== undefined
      ? String(initial.weekly_hours_goal)
      : '',
  );
  // user_id → acesso dos membros incluídos (o dono é implícito, fora daqui)
  const [acessos, setAcessos] = useState<Record<string, ProjectAccess>>(() =>
    Object.fromEntries((initial?.members ?? []).map((m) => [m.user_id, m.access])),
  );

  const isEdit = initial !== null;
  const ownerId = initial?.owner_user_id ?? meId;
  const shared = vis === 'shared';
  const swatches = PROJECT_PALETTE.includes(cor) ? PROJECT_PALETTE : [cor, ...PROJECT_PALETTE];
  // cliente arquivado some das opções, exceto se for o vínculo atual (CA-05)
  const opcoesCliente = clients.filter(
    (c) => c.archived_at === null || c.id === initial?.client_id,
  );

  const toggleIncluir = (userId: string) =>
    setAcessos((s) => {
      const next = { ...s };
      if (next[userId]) delete next[userId];
      else next[userId] = 'edit';
      return next;
    });

  const membrosSelecionados = (): ProjectMember[] =>
    Object.entries(acessos)
      .filter(([userId]) => userId !== ownerId)
      .map(([user_id, access]) => ({ user_id, access }));

  const submit = (e: FormEvent) => {
    e.preventDefault();
    const name = nome.trim();
    if (!name) return;
    const goal = meta === '' ? null : Number(meta);
    const members = shared ? membrosSelecionados() : [];

    if (!isEdit) {
      const input: ProjectInput = { name, color: cor, visibility: vis };
      if (clienteId) input.client_id = clienteId;
      if (goal !== null) input.weekly_hours_goal = goal;
      if (shared && members.length > 0) input.members = members;
      criar.mutate(input, { onSuccess: onClose });
      return;
    }

    // diff contra os valores atuais: PATCH só carrega o que mudou
    const patch: Partial<ProjectInput> = {};
    if (name !== initial.name) patch.name = name;
    if (cor !== initial.color) patch.color = cor;
    if (vis !== initial.visibility) patch.visibility = vis;
    if (clienteId !== (initial.client_id ?? '')) patch.client_id = clienteId || null;
    if (goal !== initial.weekly_hours_goal) patch.weekly_hours_goal = goal;
    if (shared) {
      const chave = (ms: ProjectMember[]) =>
        ms.map((m) => `${m.user_id}:${m.access}`).sort().join(',');
      if (chave(members) !== chave(initial.members)) patch.members = members;
    }
    if (Object.keys(patch).length === 0) {
      onClose();
      return;
    }
    editar.mutate({ id: initial.id, patch }, { onSuccess: onClose });
  };

  const titulo = isEdit ? 'Editar projeto' : 'Novo projeto';
  const nIncluidos = Object.keys(acessos).filter((id) => id !== ownerId).length;

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 60,
        display: 'grid',
        placeItems: 'center',
      }}
    >
      <div
        aria-hidden
        onClick={onClose}
        style={{
          position: 'absolute',
          inset: 0,
          background: 'rgba(0,0,0,.45)',
          backdropFilter: 'blur(2px)',
        }}
      />
      <form
        role="dialog"
        aria-modal="true"
        aria-label={titulo}
        onSubmit={submit}
        className="tt-card"
        style={{
          position: 'relative',
          width: 640,
          maxWidth: 'calc(100vw - 32px)',
          maxHeight: '92%',
          background: 'var(--surface-1)',
          boxShadow: 'var(--shadow)',
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
        }}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '18px 22px',
            borderBottom: '1px solid var(--border)',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 11 }}>
            <span
              aria-hidden
              style={{
                width: 30,
                height: 30,
                borderRadius: 9,
                background: cor,
                transition: 'background .2s',
              }}
            />
            <div style={{ fontSize: 17, fontWeight: 600 }}>{titulo}</div>
          </div>
          <button
            type="button"
            aria-label="Fechar"
            className="tt-iconbtn"
            onClick={onClose}
            style={{ width: 32, height: 32, background: 'transparent', color: 'var(--text-3)' }}
          >
            <Icon.x size={16} />
          </button>
        </div>

        <div
          style={{
            padding: 22,
            display: 'flex',
            flexDirection: 'column',
            gap: 18,
            overflowY: 'auto',
          }}
        >
          <Field id="pm-name" label="Nome do projeto">
            <input
              id="pm-name"
              value={nome}
              onChange={(e) => setNome(e.target.value)}
              placeholder="Ex.: Acme — App mobile"
              autoFocus
              style={inputStyle}
            />
          </Field>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr auto', gap: 16 }}>
            <Field id="pm-client" label="Cliente">
              <select
                id="pm-client"
                value={clienteId}
                onChange={(e) => setClienteId(e.target.value)}
                style={inputStyle}
              >
                <option value="">Sem cliente</option>
                {opcoesCliente.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                    {c.archived_at !== null ? ' (arquivado)' : ''}
                  </option>
                ))}
              </select>
            </Field>
            <Field label="Cor">
              <div style={{ display: 'flex', gap: 8, paddingTop: 2 }}>
                {swatches.map((c) => (
                  <button
                    key={c}
                    type="button"
                    aria-label={`Cor ${c}`}
                    aria-pressed={cor === c}
                    onClick={() => setCor(c)}
                    style={{
                      width: 26,
                      height: 26,
                      borderRadius: 8,
                      cursor: 'pointer',
                      background: c,
                      border: '2px solid',
                      borderColor: cor === c ? 'var(--text-1)' : 'transparent',
                    }}
                  />
                ))}
              </div>
            </Field>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
            <Field
              label="Visibilidade"
              hint={
                shared ? 'Membros do workspace podem ver e rastrear.' : 'Só você vê este projeto.'
              }
            >
              <div
                style={{
                  display: 'flex',
                  background: 'var(--surface-3)',
                  borderRadius: 9,
                  padding: 2,
                  gap: 2,
                }}
              >
                {(
                  [
                    ['private', 'Privado'],
                    ['shared', 'Compartilhado'],
                  ] as const
                ).map(([v, l]) => (
                  <button
                    key={v}
                    type="button"
                    aria-pressed={vis === v}
                    onClick={() => setVis(v)}
                    style={{
                      flex: 1,
                      border: 'none',
                      cursor: 'pointer',
                      fontFamily: 'var(--font-ui)',
                      fontSize: 12.5,
                      fontWeight: 540,
                      padding: '8px 0',
                      borderRadius: 6,
                      background: vis === v ? 'var(--surface-1)' : 'transparent',
                      color: vis === v ? 'var(--text-1)' : 'var(--text-2)',
                    }}
                  >
                    {l}
                  </button>
                ))}
              </div>
            </Field>
            <Field id="pm-goal" label="Meta de horas / semana">
              {/* domínio nativo = CA-07: (0,168] em horas decimais — step
                  fixo travava o Salvar pra metas válidas tipo 12.25 */}
              <input
                id="pm-goal"
                type="number"
                min={0.01}
                max={168}
                step="any"
                value={meta}
                onChange={(e) => setMeta(e.target.value)}
                placeholder="—"
                style={inputStyle}
              />
            </Field>
          </div>

          <Field
            label="Membros do projeto"
            hint={
              shared
                ? 'Escolha quem participa e o que cada um pode fazer.'
                : 'Disponível em projetos compartilhados.'
            }
          >
            <div
              style={{
                border: '1px solid var(--border)',
                borderRadius: 'var(--r-md)',
                overflow: 'hidden',
                opacity: shared ? 1 : 0.5,
                pointerEvents: shared ? 'auto' : 'none',
              }}
            >
              {wsMembers.map((m, i) => (
                <MemberRow
                  key={m.user_id}
                  member={m}
                  first={i === 0}
                  isOwner={m.user_id === ownerId}
                  access={m.user_id === ownerId ? null : (acessos[m.user_id] ?? null)}
                  disabled={!shared}
                  onToggle={() => toggleIncluir(m.user_id)}
                  onAccess={(a) => setAcessos((s) => ({ ...s, [m.user_id]: a }))}
                />
              ))}
              {wsMembers.length === 0 && (
                <span className="faint" style={{ display: 'block', padding: '10px 13px', fontSize: 12 }}>
                  Carregando membros…
                </span>
              )}
            </div>
          </Field>
        </div>

        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '16px 22px',
            borderTop: '1px solid var(--border)',
          }}
        >
          <span className="faint" style={{ fontSize: 12 }}>
            {shared ? `${1 + nIncluidos} membro${nIncluidos === 0 ? '' : 's'}` : 'Privado'}
          </span>
          <div style={{ display: 'flex', gap: 10 }}>
            <button type="button" className="tt-btn tt-btn--ghost" onClick={onClose}>
              Cancelar
            </button>
            <button type="submit" className="tt-btn tt-btn--accent">
              {isEdit ? (
                'Salvar alterações'
              ) : (
                <>
                  <Icon.plus size={15} sw={2.2} />
                  Criar projeto
                </>
              )}
            </button>
          </div>
        </div>
      </form>
    </div>
  );
}

function MemberRow({
  member,
  first,
  isOwner,
  access,
  disabled,
  onToggle,
  onAccess,
}: {
  member: WorkspaceMember;
  first: boolean;
  isOwner: boolean;
  // null = não incluído (pro dono, o acesso é implícito e total)
  access: ProjectAccess | null;
  disabled: boolean;
  onToggle: () => void;
  onAccess: (a: ProjectAccess) => void;
}) {
  const incluido = isOwner || access !== null;
  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 12,
        padding: '10px 13px',
        borderTop: first ? 'none' : '1px solid var(--border)',
        background: incluido ? 'transparent' : 'var(--surface-1)',
      }}
    >
      <button
        type="button"
        aria-label={`Incluir ${member.name}`}
        aria-pressed={incluido}
        disabled={isOwner || disabled}
        onClick={onToggle}
        style={{
          width: 20,
          height: 20,
          borderRadius: 6,
          flex: '0 0 auto',
          cursor: isOwner || disabled ? 'default' : 'pointer',
          display: 'grid',
          placeItems: 'center',
          border: '1.5px solid',
          borderColor: incluido ? 'var(--accent)' : 'var(--border-2)',
          background: incluido ? 'var(--accent)' : 'transparent',
          color: 'var(--accent-ink)',
        }}
      >
        {incluido && <Icon.check size={13} sw={2.4} />}
      </button>
      <span
        aria-hidden
        style={{
          width: 30,
          height: 30,
          borderRadius: 9,
          background: 'var(--accent-soft)',
          color: 'var(--accent)',
          display: 'grid',
          placeItems: 'center',
          fontWeight: 700,
          fontSize: 13,
          flex: '0 0 auto',
        }}
      >
        {member.name[0] ?? '?'}
      </span>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ fontSize: 13.5, fontWeight: 540 }}>{member.name}</div>
        <div className="faint" style={{ fontSize: 11.5 }}>
          {member.email}
        </div>
      </div>
      {isOwner ? (
        <span className="tt-role tt-role--owner">
          <Icon.shield size={12} />
          Owner
        </span>
      ) : access !== null ? (
        <div
          style={{
            display: 'flex',
            background: 'var(--surface-3)',
            borderRadius: 8,
            padding: 2,
            gap: 2,
          }}
        >
          {(
            [
              ['edit', 'Editar'],
              ['view', 'Só vê'],
            ] as const
          ).map(([v, l]) => (
            <button
              key={v}
              type="button"
              aria-pressed={access === v}
              disabled={disabled}
              onClick={() => onAccess(v)}
              style={{
                border: 'none',
                cursor: disabled ? 'default' : 'pointer',
                fontFamily: 'var(--font-ui)',
                fontSize: 11.5,
                fontWeight: 540,
                whiteSpace: 'nowrap',
                padding: '4px 10px',
                borderRadius: 6,
                background: access === v ? 'var(--surface-1)' : 'transparent',
                color: access === v ? 'var(--text-1)' : 'var(--text-3)',
              }}
            >
              {l}
            </button>
          ))}
        </div>
      ) : (
        <span className="faint" style={{ fontSize: 12 }}>
          não incluído
        </span>
      )}
    </div>
  );
}
