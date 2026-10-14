// Ícones do design (porte fiel de parts.jsx → window.TT.Icon).
import type { CSSProperties, ReactNode } from 'react';

interface IconProps {
  size?: number;
  sw?: number;
  style?: CSSProperties;
  className?: string;
}

const make =
  (path: ReactNode, vb = 24) =>
  ({ size = 18, sw = 1.7, style, className }: IconProps) => (
    <svg
      viewBox={`0 0 ${vb} ${vb}`}
      width={size}
      height={size}
      fill="none"
      stroke="currentColor"
      strokeWidth={sw}
      strokeLinecap="round"
      strokeLinejoin="round"
      style={style}
      className={className}
    >
      {path}
    </svg>
  );

export const Icon = {
  today: make(
    <>
      <rect x="3" y="4.5" width="18" height="16" rx="2.5" />
      <path d="M3 9h18M8 2.5v4M16 2.5v4" />
      <circle cx="12" cy="14.5" r="2" fill="currentColor" stroke="none" />
    </>,
  ),
  cal: make(
    <>
      <rect x="3" y="4.5" width="18" height="16" rx="2.5" />
      <path d="M3 9h18M8 2.5v4M16 2.5v4M7.5 13h2M11 13h2M14.5 13h2M7.5 16.5h2M11 16.5h2" />
    </>,
  ),
  list: make(<path d="M8 6h12M8 12h12M8 18h12M3.5 6h.01M3.5 12h.01M3.5 18h.01" />),
  // doc (nota .md) — porte fiel do mockup projeto-workspace.html (I.note)
  doc: make(
    <>
      <path d="M5 4h11l3 3v13H5z" />
      <path d="M9 9h6M9 13h6M9 17h3" />
    </>,
  ),
  folder: make(<path d="M3 7a2 2 0 0 1 2-2h4l2 2.5h8a2 2 0 0 1 2 2V18a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />),
  chart: make(<path d="M4 20V4M4 20h16M8 16v-4M12.5 16V9M17 16v-7" />),
  focus: make(
    <>
      <path d="M4 8V5.5A1.5 1.5 0 0 1 5.5 4H8M16 4h2.5A1.5 1.5 0 0 1 20 5.5V8M20 16v2.5a1.5 1.5 0 0 1-1.5 1.5H16M8 20H5.5A1.5 1.5 0 0 1 4 18.5V16" />
      <circle cx="12" cy="12" r="2.5" />
    </>,
  ),
  cog: make(
    <>
      <circle cx="12" cy="12" r="3" />
      <path d="M12 2.5v2.5M12 19v2.5M4.2 7l2.1 1.3M17.7 15.7l2.1 1.3M19.8 7l-2.1 1.3M6.3 15.7L4.2 17M2.5 12H5M19 12h2.5" />
    </>,
  ),
  play: make(<path d="M7 5.5v13l11-6.5z" fill="currentColor" stroke="none" />),
  stop: make(<rect x="6.5" y="6.5" width="11" height="11" rx="2" fill="currentColor" stroke="none" />),
  pause: make(
    <>
      <rect x="7" y="6" width="3.2" height="12" rx="1.2" fill="currentColor" stroke="none" />
      <rect x="13.8" y="6" width="3.2" height="12" rx="1.2" fill="currentColor" stroke="none" />
    </>,
  ),
  plus: make(<path d="M12 5v14M5 12h14" />),
  search: make(
    <>
      <circle cx="11" cy="11" r="6.5" />
      <path d="M20 20l-4-4" />
    </>,
  ),
  clock: make(
    <>
      <circle cx="12" cy="12" r="8.5" />
      <path d="M12 7.5V12l3 2" />
    </>,
  ),
  bolt: make(<path d="M13 2.5 4.5 13.5H11l-1 8 8.5-11H12z" />),
  arrow: make(<path d="M5 12h14M13 6l6 6-6 6" />),
  cliente: make(
    <>
      <circle cx="12" cy="8" r="3.5" />
      <path d="M5.5 20a6.5 6.5 0 0 1 13 0" />
    </>,
  ),
  briefcase: make(
    <>
      <rect x="3" y="7" width="18" height="13" rx="2.5" />
      <path d="M8 7V5.5A1.5 1.5 0 0 1 9.5 4h5A1.5 1.5 0 0 1 16 5.5V7M3 12h18" />
    </>,
  ),
  users: make(
    <>
      <circle cx="9" cy="8" r="3" />
      <path d="M3.5 19a5.5 5.5 0 0 1 11 0M16 5.5a3 3 0 0 1 0 5.8M17 19a5.5 5.5 0 0 0-2.2-4.4" />
    </>,
  ),
  mail: make(
    <>
      <rect x="3" y="5" width="18" height="14" rx="2.5" />
      <path d="M4 7l8 6 8-6" />
    </>,
  ),
  link: make(
    <path d="M9 15l6-6M10.5 6.5l1.2-1.2a3.7 3.7 0 0 1 5.2 5.2L15.7 11.7M8.3 12.3l-1.2 1.2a3.7 3.7 0 0 0 5.2 5.2l1.2-1.2" />,
  ),
  shield: make(
    <>
      <path d="M12 3l7 2.5v5c0 4.5-3 7.5-7 9-4-1.5-7-4.5-7-9v-5z" />
      <path d="M9 12l2 2 4-4" />
    </>,
  ),
  check: make(<path d="M4.5 12.5l5 5 10-11" />),
  x: make(<path d="M6 6l12 12M18 6L6 18" />),
  table: make(
    <>
      <rect x="3.5" y="4.5" width="17" height="15" rx="2" />
      <path d="M3.5 9.5h17M3.5 14.5h17M9.5 4.5v15" />
    </>,
  ),
  chevron: make(<path d="M6 9l6 6 6-6" />),
  dots: make(
    <>
      <circle cx="5" cy="12" r="1.4" fill="currentColor" stroke="none" />
      <circle cx="12" cy="12" r="1.4" fill="currentColor" stroke="none" />
      <circle cx="19" cy="12" r="1.4" fill="currentColor" stroke="none" />
    </>,
  ),
  trash: make(<path d="M4 7h16M9 7V5.5A1.5 1.5 0 0 1 10.5 4h3A1.5 1.5 0 0 1 15 5.5V7M6 7l1 13h10l1-13" />),
  globe: make(
    <>
      <circle cx="12" cy="12" r="8.5" />
      <path d="M3.5 12h17M12 3.5c2.5 2.5 2.5 14.5 0 17M12 3.5c-2.5 2.5-2.5 14.5 0 17" />
    </>,
  ),
  grid: make(
    <>
      <rect x="4" y="4" width="7" height="7" rx="1.5" />
      <rect x="13" y="4" width="7" height="7" rx="1.5" />
      <rect x="4" y="13" width="7" height="7" rx="1.5" />
      <rect x="13" y="13" width="7" height="7" rx="1.5" />
    </>,
  ),
  // fase 24: board (colunas kanban), expand/collapse do popup, link externo, ⭐.
  board: make(
    <>
      <rect x="3.5" y="4.5" width="5" height="15" rx="1.5" />
      <rect x="9.5" y="4.5" width="5" height="10" rx="1.5" />
      <rect x="15.5" y="4.5" width="5" height="13" rx="1.5" />
    </>,
  ),
  expand: make(<path d="M9 4.5H4.5V9M15 4.5h4.5V9M9 19.5H4.5V15M15 19.5h4.5V15" />),
  collapse: make(<path d="M4.5 9H9V4.5M19.5 9H15V4.5M4.5 15H9v4.5M19.5 15H15v4.5" />),
  ext: make(
    <>
      <path d="M14 4.5h5.5V10" />
      <path d="M19.5 4.5L11 13" />
      <path d="M18 13.5v5a1.5 1.5 0 0 1-1.5 1.5h-11A1.5 1.5 0 0 1 4 18.5v-11A1.5 1.5 0 0 1 5.5 6h5" />
    </>,
  ),
  star: make(
    <path
      d="M12 3.5l2.6 5.3 5.9.85-4.25 4.15 1 5.85L12 16.9l-5.25 2.75 1-5.85L3.5 9.65l5.9-.85z"
      fill="currentColor"
      stroke="none"
    />,
  ),
  starO: make(
    <path d="M12 3.5l2.6 5.3 5.9.85-4.25 4.15 1 5.85L12 16.9l-5.25 2.75 1-5.85L3.5 9.65l5.9-.85z" />,
  ),
  // fase 31: octocat do GitHub (fill) — embed de issue/PR no card de referência.
  github: make(
    <path
      fill="currentColor"
      stroke="none"
      d="M12 .5C5.37.5 0 5.78 0 12.29c0 5.21 3.44 9.63 8.21 11.19.6.11.82-.26.82-.57 0-.28-.01-1.02-.02-2-3.34.72-4.04-1.61-4.04-1.61-.55-1.39-1.34-1.76-1.34-1.76-1.09-.74.08-.73.08-.73 1.2.08 1.84 1.24 1.84 1.24 1.07 1.83 2.81 1.3 3.5 1 .11-.78.42-1.3.76-1.6-2.67-.3-5.47-1.33-5.47-5.93 0-1.31.47-2.38 1.24-3.22-.13-.3-.54-1.52.12-3.18 0 0 1.01-.32 3.3 1.23a11.5 11.5 0 0 1 6 0c2.29-1.55 3.3-1.23 3.3-1.23.66 1.66.25 2.88.12 3.18.77.84 1.23 1.91 1.23 3.22 0 4.61-2.81 5.62-5.49 5.92.43.37.81 1.1.81 2.22 0 1.6-.01 2.9-.01 3.29 0 .31.22.69.82.57A12.3 12.3 0 0 0 24 12.29C24 5.78 18.63.5 12 .5z"
    />,
  ),
};
