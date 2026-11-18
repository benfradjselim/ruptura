// A side panel over the page, closed with its button, Escape or a click on
// the backdrop. It opens below the console masthead, measured when it opens,
// because the console keeps the masthead above plugin content.
import * as React from 'react';
import './plugin.css';

const MASTHEAD = '.pf-v6-c-masthead, .pf-v5-c-masthead, .pf-c-masthead';

function mastheadBottom(): number {
  const el = typeof document === 'undefined' ? null : document.querySelector(MASTHEAD);
  return el ? Math.max(0, Math.round(el.getBoundingClientRect().bottom)) : 0;
}

export default function Drawer({ onClose, label, children }: { onClose: () => void; label: string; children: React.ReactNode }): JSX.Element {
  const closeRef = React.useRef<HTMLButtonElement>(null);
  const [top] = React.useState(mastheadBottom);
  React.useEffect(() => {
    closeRef.current?.focus();
    const onKey = (e: KeyboardEvent): void => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose]);
  return (
    <div className="ruptura-drawer" role="presentation" style={{ top }}>
      <div className="ruptura-drawer__backdrop" onClick={onClose} />
      <aside className="ruptura-drawer__panel" role="dialog" aria-modal="true" aria-label={label}>
        <button ref={closeRef} type="button" className="ruptura-drawer__close" onClick={onClose} aria-label="Close details">
          ×
        </button>
        {children}
      </aside>
    </div>
  );
}
