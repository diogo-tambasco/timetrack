// membros.spec — fase 17 T-11 (CA-35): gestão de membros & convites na página
// /workspace, logado como OWNER (stub diogo.fixture@example.com).
//
// LIMITAÇÃO DO STUB (documentada): o googlestub autentica SÓ 1 usuário
// (diogo.fixture@example.com, owner). Para o caminho "convite → 1º login do
// convidado → membership" usamos o helper SQL seedInvitedLogin (opção B do
// plan/tasks — NÃO acopla ao stub um 2º usuário): ele materializa o user do
// email convidado E roda a MESMA resolução do callback de auth. O caso
// "non-owner não vê controles" (CA-29) é coberto no nível de componente em
// Workspace.manage.spec.tsx (o stub não loga um member).
//
// Arquivo PARALELO (não muta timer). Emails únicos por run (db-test é
// PERSISTENTE entre runs). Liberar a :8080 antes (CLAUDE.md).
import { test, expect } from "@playwright/test";
import { loginViaStub, seedInvitedLogin, seedWorkspaceMember } from "./helpers";

test("CA-35: owner convida, o convidado entra no 1º login, e o owner gere o quadro", async ({
  page,
}) => {
  const suf = Date.now();
  const convidado = `cadu.${suf}@example.com`;

  await loginViaStub(page);
  await page.goto("/workspace");
  await expect(page.getByTestId("workspace-name")).toHaveText(
    "Workspace de Diogo",
  );

  // — Owner convida um email NOVO (sem conta) → 404 user_not_found → fallback
  //   de convite → o convite aparece nos pendentes.
  await page.getByLabel("Email da pessoa").fill(convidado);
  await page.getByRole("button", { name: "Adicionar", exact: true }).click();
  // a pessoa não tem conta → oferece o convite por email (CA-33)
  await page.getByRole("button", { name: "Enviar convite" }).click();

  const linhaConvite = page.locator('[data-testid^="invite-"]', {
    hasText: convidado,
  });
  await expect(linhaConvite).toBeVisible();
  await expect(linhaConvite).toContainText("Membro");

  // — Simula o 1º login do convidado (opção B): materializa user + resolução.
  seedInvitedLogin({
    googleId: `g-cadu-${suf}`,
    name: "Cadu Convidado",
    email: convidado,
  });
  await page.reload();

  // o convidado VIRA membro (aparece na lista) e o convite SOME dos pendentes.
  const linhaMembro = page.locator('[data-testid^="member-"]', {
    hasText: "Cadu Convidado",
  });
  await expect(linhaMembro).toBeVisible();
  await expect(
    page.locator('[data-testid^="invite-"]', { hasText: convidado }),
  ).toHaveCount(0);

  // — Owner muda a role do convidado (member → viewer) e o remove.
  const userIdConvidado = (await linhaMembro.getAttribute(
    "data-testid",
  ))!.replace("member-", "");
  await linhaMembro
    .getByLabel(`Role de ${userIdConvidado}`)
    .selectOption("viewer");
  // a role reflete após a invalidação (badge/select do membro)
  await expect(
    page.locator('[data-testid^="member-"]', { hasText: "Cadu Convidado" }),
  ).toBeVisible();

  // remover o convidado (confirma o diálogo nativo)
  page.once("dialog", (d) => d.accept());
  await page
    .locator('[data-testid^="member-"]', { hasText: "Cadu Convidado" })
    .getByLabel(`Remover ${userIdConvidado}`)
    .click();
  await expect(
    page.locator('[data-testid^="member-"]', { hasText: "Cadu Convidado" }),
  ).toHaveCount(0);

  // — O owner (único) não expõe ação que viole a invariante: o select dele é
  //   desabilitado e não há "Remover" pra ele (CA-32).
  const linhaDono = page.locator('[data-testid^="member-"]', {
    hasText: "Diogo Fixture",
  });
  const donoId = (await linhaDono.getAttribute("data-testid"))!.replace(
    "member-",
    "",
  );
  await expect(linhaDono.getByLabel(`Role de ${donoId}`)).toBeDisabled();
  await expect(linhaDono.getByLabel(`Remover ${donoId}`)).toHaveCount(0);
});

test("CA-35: owner adiciona um usuário existente por email → membership na hora", async ({
  page,
}) => {
  const suf = Date.now();
  const existente = `bob.${suf}@example.com`;

  await loginViaStub(page);

  // garante a EXISTÊNCIA da conta do "existente" semeando-o como membro;
  // remove pela UI (fica existindo, sem vínculo) e então o ADD pelo form
  // recria a membership na hora (sem cair no fallback de convite — CA-10).
  seedWorkspaceMember({
    googleId: `g-bob-${suf}`,
    name: "Bob Existente",
    email: existente,
    role: "member",
  });
  await page.goto("/workspace");
  const linhaBob = page.locator('[data-testid^="member-"]', {
    hasText: "Bob Existente",
  });
  await expect(linhaBob).toBeVisible();
  const bobId = (await linhaBob.getAttribute("data-testid"))!.replace(
    "member-",
    "",
  );

  // remove o Bob pela UI
  page.once("dialog", (d) => d.accept());
  await linhaBob.getByLabel(`Remover ${bobId}`).click();
  await expect(
    page.locator('[data-testid^="member-"]', { hasText: "Bob Existente" }),
  ).toHaveCount(0);

  // agora o user existe mas NÃO é membro → "Adicionar" cria a membership na
  // hora (sem cair no fallback de convite).
  await page.getByLabel("Email da pessoa").fill(existente);
  await page.getByRole("button", { name: "Adicionar", exact: true }).click();
  await expect(page.getByTestId("manage-msg")).toContainText(
    "Membro adicionado",
  );
  await expect(
    page.locator('[data-testid^="member-"]', { hasText: "Bob Existente" }),
  ).toBeVisible();

  // limpa pro retry do runner: remove o Bob de novo.
  const linhaBob2 = page.locator('[data-testid^="member-"]', {
    hasText: "Bob Existente",
  });
  const bobId2 = (await linhaBob2.getAttribute("data-testid"))!.replace(
    "member-",
    "",
  );
  page.once("dialog", (d) => d.accept());
  await linhaBob2.getByLabel(`Remover ${bobId2}`).click();
  await expect(
    page.locator('[data-testid^="member-"]', { hasText: "Bob Existente" }),
  ).toHaveCount(0);
});
