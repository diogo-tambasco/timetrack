// linkPreview (fase 31): deriva um preview do link colado no campo de referência
// da task (client-side, sem API). Detecta issue/PR do GitHub e página do Notion;
// senão, link genérico (hostname). Espelha como o Notion embeda issues do GitHub.
import { notionTitle } from './notion';

export type LinkKind = 'github' | 'notion' | 'generic';

export interface LinkInfo {
  kind: LinkKind;
  /** nome do serviço pra rótulos ("Notion", "GitHub", "link") */
  service: string;
  title: string;
  subtitle: string;
}

// Vários links de referência por task cabem no MESMO campo `notion_url` (texto
// livre), separados por quebra de linha — os outros consumidores (board/list/
// table) só checam `!= null` pra mostrar um ícone, então é seguro (sem migração).
export function parseLinks(raw: string | null | undefined): string[] {
  if (!raw) return [];
  return raw
    .split(/[\r\n]+/)
    .map((s) => s.trim())
    .filter(Boolean);
}

export function joinLinks(urls: string[]): string | null {
  const v = urls.map((u) => u.trim()).filter(Boolean);
  return v.length ? v.join('\n') : null;
}

export function linkInfo(url: string): LinkInfo {
  try {
    const u = new URL(url);
    const host = u.hostname.replace(/^www\./, '');

    // GitHub issue / pull request → "Issue n.806" · "owner/repo"
    if (host === 'github.com') {
      const m = u.pathname.match(/^\/([^/]+)\/([^/]+)\/(issues|pull)\/(\d+)/);
      if (m) {
        const [, owner, repo, kind, num] = m;
        const label = kind === 'pull' ? 'Pull request' : 'Issue';
        return { kind: 'github', service: 'GitHub', title: `${label} #${num}`, subtitle: `${owner}/${repo}` };
      }
      // repo/genérico do github
      return { kind: 'github', service: 'GitHub', title: u.pathname.replace(/^\//, '') || 'GitHub', subtitle: 'github.com' };
    }

    if (host.endsWith('notion.so') || host.endsWith('notion.site') || host.endsWith('notion.com')) {
      return { kind: 'notion', service: 'Notion', title: notionTitle(url), subtitle: host };
    }

    return { kind: 'generic', service: 'link', title: notionTitle(url), subtitle: host };
  } catch {
    // URL inválida → trata como Notion (comportamento herdado da fase 24)
    return { kind: 'notion', service: 'Notion', title: notionTitle(url), subtitle: 'notion.so' };
  }
}
