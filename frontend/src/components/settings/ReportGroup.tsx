// Grupo "Relatórios" dos Ajustes (fase 14 T-11, CA-29): edita a jornada útil
// esperada (work_hours_per_day 1–24) e o "só dias úteis" (workdays_only) —
// base do rastreado×não-rastreado. Salva via PUT (useSaveReportSettings);
// formulário inválido (hpd fora de 1–24) NÃO chama a API e sinaliza o erro.
// As preferências sincronizam entre devices. Espelha o CalendarGroup (11).
import { useState } from "react";
import Switch from "../Switch";
import {
  useReportSettings,
  useSaveReportSettings,
} from "../../lib/queries/reports";
import type { ReportSettings } from "../../lib/api";

const fieldStyle = {
  padding: "8px 10px",
  borderRadius: 9,
  border: "1px solid var(--border-2)",
  background: "var(--surface-2)",
  color: "var(--text-1)",
  fontFamily: "var(--font-ui)",
  fontSize: 12.5,
  outline: "none",
} as const;

export default function ReportGroup() {
  const { data } = useReportSettings();
  if (!data) return null;
  return <GroupForm initial={data} />;
}

function GroupForm({ initial }: { initial: ReportSettings }) {
  const salvar = useSaveReportSettings();
  const [hours, setHours] = useState(initial.work_hours_per_day);
  const [workdaysOnly, setWorkdaysOnly] = useState(initial.workdays_only);
  const [erro, setErro] = useState<string | null>(null);
  const [salvo, setSalvo] = useState(false);

  const submit = () => {
    setErro(null);
    setSalvo(false);
    // CA-29: validação no cliente — fora de (0,24] não chama a API
    if (!Number.isFinite(hours) || hours <= 0 || hours > 24) {
      setErro("A jornada deve estar entre 1 e 24 horas.");
      return;
    }
    salvar.mutate(
      { work_hours_per_day: hours, workdays_only: workdaysOnly },
      {
        onSuccess: () => setSalvo(true),
        onError: (err) => setErro(err.message),
      },
    );
  };

  return (
    <section
      className="tt-card"
      style={{ marginTop: "var(--gap)", padding: "var(--pad)" }}
    >
      <div style={{ marginBottom: 16 }}>
        <div style={{ fontSize: 14, fontWeight: 600 }}>Relatórios</div>
        <div className="faint" style={{ fontSize: 12, marginTop: 2 }}>
          Jornada útil esperada — base do rastreado × não-rastreado nos
          relatórios. Vale em todos os dispositivos.
        </div>
      </div>

      <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
        <label style={{ fontSize: 13, maxWidth: 220 }}>
          <span style={{ display: "block", marginBottom: 8 }}>
            Horas úteis por dia
          </span>
          <input
            aria-label="Horas úteis por dia"
            type="number"
            min={1}
            max={24}
            step="any"
            value={hours}
            onChange={(e) => setHours(Number(e.target.value))}
            style={{ ...fieldStyle, width: "100%" }}
          />
        </label>

        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            gap: "var(--gap)",
          }}
        >
          <div>
            <div style={{ fontSize: 13, fontWeight: 540 }}>Só dias úteis</div>
            <div className="muted" style={{ fontSize: 12 }}>
              Conta apenas segunda a sexta no esperado do período.
            </div>
          </div>
          <Switch
            on={workdaysOnly}
            onToggle={() => setWorkdaysOnly((v) => !v)}
            label="Só dias úteis"
          />
        </div>

        {erro && (
          <span role="alert" style={{ color: "var(--danger)", fontSize: 12.5 }}>
            {erro}
          </span>
        )}

        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <button
            type="button"
            className="tt-btn tt-btn--accent"
            onClick={submit}
          >
            Salvar
          </button>
          {salvo && (
            <span className="faint" style={{ fontSize: 12 }}>
              Salvo — vale em todos os dispositivos.
            </span>
          )}
        </div>
      </div>
    </section>
  );
}
