// notion.ts (fase 24, CA-24): deriva o título de uma página do Notion a partir do
// link colado (preview client-side; sem API/OAuth do Notion). Espelha o
// notionTitle() do mockup (projeto-workspace.html linha 261): pega o último
// segmento do path, remove o hash hex final CASE-INSENSITIVE (-[0-9a-fA-F]{6,}$ —
// IDs do Notion vêm em maiúsculas/mistas), troca '-' por espaço.

// notionTitle: URL inválida → "Página do Notion"; sem hash → slug inteiro.
export function notionTitle(url: string): string {
  try {
    let s = url
      .split("?")[0]
      .split("#")[0]
      .replace(/\/$/, "")
      .split("/")
      .pop();
    if (s == null) return "Página do Notion";
    s = decodeURIComponent(s)
      .replace(/-[0-9a-fA-F]{6,}$/, "")
      .replace(/-/g, " ")
      .trim();
    return s || "Página do Notion";
  } catch {
    return "Página do Notion";
  }
}
