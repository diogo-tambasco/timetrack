import { describe, it, expect, vi, afterEach } from "vitest";
import {
  listProperties,
  createProperty,
  patchProperty,
  deleteProperty,
  setPropertyValue,
  clearPropertyValue,
} from "./api";

afterEach(() => {
  vi.unstubAllGlobals();
});

// fase 25 T-08: shape das chamadas do client de propriedades (paths + métodos).
describe("lib/api properties", () => {
  it("listProperties → GET .../properties, desembrulha", async () => {
    const props = [{ id: "d1", name: "Cliente", type: "select", options: [], position: 0 }];
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({ properties: props }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    const out = await listProperties("w1", "p1");
    expect(out).toHaveLength(1);
    expect(fetchMock.mock.calls[0][0]).toBe("/api/v1/workspaces/w1/projects/p1/properties");
  });

  it("createProperty → POST .../properties com body", async () => {
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({ id: "d1" }), { status: 201 }));
    vi.stubGlobal("fetch", fetchMock);
    await createProperty("w1", "p1", { name: "Cliente", type: "select", options: [{ label: "Lauda" }] });
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/v1/workspaces/w1/projects/p1/properties");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body as string)).toEqual({
      name: "Cliente",
      type: "select",
      options: [{ label: "Lauda" }],
    });
  });

  it("patchProperty → PATCH .../properties/{defId}", async () => {
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({ id: "d1" }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    await patchProperty("w1", "p1", "d1", { name: "Conta" });
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/v1/workspaces/w1/projects/p1/properties/d1");
    expect(init.method).toBe("PATCH");
  });

  it("deleteProperty → DELETE .../properties/{defId}", async () => {
    const fetchMock = vi.fn(async () => new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);
    await deleteProperty("w1", "p1", "d1");
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/v1/workspaces/w1/projects/p1/properties/d1");
    expect(init.method).toBe("DELETE");
  });

  it("setPropertyValue → PUT .../tasks/{taskId}/properties/{defId} com {value}", async () => {
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({ def_id: "d1", value: "op_a" }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    await setPropertyValue("w1", "p1", "t1", "d1", "op_a");
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/v1/workspaces/w1/projects/p1/tasks/t1/properties/d1");
    expect(init.method).toBe("PUT");
    expect(JSON.parse(init.body as string)).toEqual({ value: "op_a" });
  });

  it("setPropertyValue com null (clear) → PUT, 204 vira undefined", async () => {
    const fetchMock = vi.fn(async () => new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);
    const out = await setPropertyValue("w1", "p1", "t1", "d1", null);
    expect(out).toBeUndefined();
    expect(JSON.parse(fetchMock.mock.calls[0][1].body as string)).toEqual({ value: null });
  });

  it("clearPropertyValue → DELETE .../tasks/{taskId}/properties/{defId}", async () => {
    const fetchMock = vi.fn(async () => new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);
    await clearPropertyValue("w1", "p1", "t1", "d1");
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/v1/workspaces/w1/projects/p1/tasks/t1/properties/d1");
    expect(init.method).toBe("DELETE");
  });
});
