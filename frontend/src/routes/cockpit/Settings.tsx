// Página Ajustes. Fase 00: toggle dark/light (pipeline de tema, CA-12);
// fase 03 (T-18): grupo Pomodoro (foco 15–90, pausa 5/15/20 + custom).
// Accent/densidade ficam pra fase de polish (09).
import Switch from "../../components/Switch";
import PomodoroGroup from "../../components/settings/Pomodoro";
import CalendarGroup from "../../components/settings/CalendarGroup";
import ReportGroup from "../../components/settings/ReportGroup";
import NotificationsGroup from "../../components/settings/NotificationsGroup";
import RemindersGroup from "../../components/settings/RemindersGroup";
import { useSkinStore } from "../../lib/skin";

export default function Settings() {
  const theme = useSkinStore((s) => s.theme);
  const setTheme = useSkinStore((s) => s.setTheme);

  return (
    <section>
      <p className="eyebrow">Ajustes</p>
      <div
        className="tt-card"
        style={{ marginTop: "var(--gap)", padding: "var(--pad)" }}
      >
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            gap: "var(--gap)",
          }}
        >
          <div>
            <div style={{ fontWeight: 540, fontSize: 14 }}>Tema claro</div>
            <div className="muted" style={{ fontSize: 12.5 }}>
              O padrão é o tema escuro; a escolha fica salva neste navegador.
            </div>
          </div>
          <Switch
            on={theme === "light"}
            onToggle={() => setTheme(theme === "light" ? "dark" : "light")}
            label="Tema claro"
          />
        </div>
      </div>
      <PomodoroGroup />
      <CalendarGroup />
      <ReportGroup />
      <NotificationsGroup />
      <RemindersGroup />
    </section>
  );
}
