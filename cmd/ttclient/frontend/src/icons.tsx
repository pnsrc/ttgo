type P = { className?: string };

const stroke = {
  fill: "none",
  stroke: "currentColor",
  strokeWidth: "1.75",
  strokeLinecap: "round" as const,
  strokeLinejoin: "round" as const,
};

export const Logo = (p: P) => (
  <svg viewBox="0 0 24 24" {...stroke} className={p.className}>
    <path d="M12 2L3 7v6c0 5 3.8 9.4 9 11 5.2-1.6 9-6 9-11V7l-9-5z" />
    <path d="M9 12l2 2 4-4" />
  </svg>
);

export const ArrowLeft = (p: P) => (
  <svg viewBox="0 0 24 24" {...stroke} className={p.className}>
    <line x1="19" y1="12" x2="5" y2="12" />
    <polyline points="12 19 5 12 12 5" />
  </svg>
);

export const Power = (p: P) => (
  <svg viewBox="0 0 24 24" {...stroke} className={p.className}>
    <path d="M18.36 6.64a9 9 0 1 1-12.73 0" />
    <line x1="12" y1="2" x2="12" y2="12" />
  </svg>
);

export const Alert = (p: P) => (
  <svg viewBox="0 0 24 24" {...stroke} className={p.className}>
    <circle cx="12" cy="12" r="10" />
    <line x1="12" y1="8" x2="12" y2="12" />
    <line x1="12" y1="16" x2="12.01" y2="16" />
  </svg>
);

export const Plus = (p: P) => (
  <svg viewBox="0 0 24 24" {...stroke} className={p.className} strokeWidth="2">
    <line x1="12" y1="5" x2="12" y2="19" />
    <line x1="5" y1="12" x2="19" y2="12" />
  </svg>
);

export const Trash = (p: P) => (
  <svg viewBox="0 0 24 24" {...stroke} className={p.className}>
    <polyline points="3 6 5 6 21 6" />
    <path d="M19 6l-2 14a2 2 0 0 1-2 2H9a2 2 0 0 1-2-2L5 6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
  </svg>
);

export const Folder = (p: P) => (
  <svg viewBox="0 0 24 24" {...stroke} className={p.className}>
    <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
  </svg>
);

export const Refresh = (p: P) => (
  <svg viewBox="0 0 24 24" {...stroke} className={p.className}>
    <polyline points="23 4 23 10 17 10" />
    <polyline points="1 20 1 14 7 14" />
    <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15" />
  </svg>
);

export const ChevronRight = (p: P) => (
  <svg viewBox="0 0 24 24" {...stroke} className={p.className}>
    <polyline points="9 18 15 12 9 6" />
  </svg>
);

export const Empty = (p: P) => (
  <svg viewBox="0 0 24 24" {...stroke} className={p.className}>
    <rect x="3" y="3" width="18" height="18" rx="2" />
    <line x1="9" y1="3" x2="9" y2="21" />
  </svg>
);
