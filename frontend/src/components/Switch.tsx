// Toggle switch do design (Switch de parts.jsx + .tt-switch de tokens.css).
interface SwitchProps {
  on: boolean;
  onToggle: () => void;
  label?: string;
}

export default function Switch({ on, onToggle, label }: SwitchProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={on}
      aria-label={label}
      onClick={onToggle}
      className="tt-switch"
      style={{ padding: 0 }}
    >
      <i />
    </button>
  );
}
