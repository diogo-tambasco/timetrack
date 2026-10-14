// Paleta de cor de PROJETO (T-18). Cor de projeto é DADO de domínio — a
// API valida e persiste hex literal (#RRGGBB, CA-07) — não tema de UI;
// por isso este arquivo é exceção sancionada (allowlist) do
// scripts/check-cores.sh. Os valores DEVEM espelhar os tokens --p-* e o
// --accent default do styles/tokens.css — lib/palette.spec.ts garante a
// integridade (divergiu, quebra).
export const PROJECT_PALETTE = [
  '#4f9dde', // --p-acme
  '#b07cd6', // --p-lauda
  '#e0a13c', // --p-arkan
  '#5cc98b', // --p-primor
  '#c9806a', // --p-freela
  '#2dd4bf', // --accent (default)
];
