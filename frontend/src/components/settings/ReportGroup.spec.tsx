// ReportGroup.spec — fase 14 T-11 (CA-29): carrega a config salva, valida
// work_hours_per_day em 1–24 no client (sem chamar a API) e salva via PUT com
// o corpo do contrato. Espelha o CalendarGroup.spec da 11.
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import ReportGroup from "./ReportGroup";
import * as api from "../../lib/api";

vi.mock("../../lib/api", async (importOriginal) => {
  const mod = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...mod,
    getReportSettings: vi.fn(),
    putReportSettings: vi.fn(),
  };
});

function renderGrupo() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={qc}>
      <ReportGroup />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.getReportSettings).mockResolvedValue({
    work_hours_per_day: 8,
    workdays_only: true,
  });
});

describe("ReportGroup (fase 14)", () => {
  it("carrega a config persistida (horas + dias úteis)", async () => {
    renderGrupo();
    expect(await screen.findByLabelText("Horas úteis por dia")).toHaveValue(8);
    expect(
      screen.getByRole("switch", { name: "Só dias úteis" }),
    ).toHaveAttribute("aria-checked", "true");
  });

  it("hpd <= 0 NÃO chama a API e mostra erro (CA-29)", async () => {
    renderGrupo();
    await screen.findByLabelText("Horas úteis por dia");

    fireEvent.change(screen.getByLabelText("Horas úteis por dia"), {
      target: { value: "0" },
    });
    fireEvent.click(screen.getByRole("button", { name: /salvar/i }));

    expect(api.putReportSettings).not.toHaveBeenCalled();
    expect(await screen.findByRole("alert")).toHaveTextContent(/entre 1 e 24/i);
  });

  it("hpd > 24 NÃO chama a API e mostra erro (CA-29)", async () => {
    renderGrupo();
    await screen.findByLabelText("Horas úteis por dia");

    fireEvent.change(screen.getByLabelText("Horas úteis por dia"), {
      target: { value: "25" },
    });
    fireEvent.click(screen.getByRole("button", { name: /salvar/i }));

    expect(api.putReportSettings).not.toHaveBeenCalled();
    expect(await screen.findByRole("alert")).toBeInTheDocument();
  });

  it("salvar válido envia PUT com o corpo do contrato (CA-29)", async () => {
    vi.mocked(api.putReportSettings).mockResolvedValue({
      work_hours_per_day: 6,
      workdays_only: false,
    });
    renderGrupo();
    await screen.findByLabelText("Horas úteis por dia");

    fireEvent.change(screen.getByLabelText("Horas úteis por dia"), {
      target: { value: "6" },
    });
    fireEvent.click(screen.getByRole("switch", { name: "Só dias úteis" })); // desliga
    fireEvent.click(screen.getByRole("button", { name: /salvar/i }));

    await waitFor(() =>
      expect(api.putReportSettings).toHaveBeenCalledWith({
        work_hours_per_day: 6,
        workdays_only: false,
      }),
    );
  });
});
