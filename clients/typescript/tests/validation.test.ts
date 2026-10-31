/**
 * Contract tests that need no daemon: argument validation, URL building and
 * response handling, driven through the client's documented `fetch` option.
 */
import { strict as assert } from "node:assert";
import { test } from "node:test";

import { DigClient, DigError } from "../src/index.ts";

interface Seen {
  method?: string;
  url: URL;
  body?: unknown;
}

/** A fetch that records every request and answers with the given response. */
function recorder(respond: () => Response = () => Response.json([])): {
  fetch: typeof fetch;
  seen: Seen[];
} {
  const seen: Seen[] = [];
  const fn = async (input: URL | RequestInfo, init?: RequestInit): Promise<Response> => {
    seen.push({ method: init?.method, url: new URL(String(input)), body: init?.body });
    return respond();
  };
  return { fetch: fn as typeof fetch, seen };
}

test("invalid arguments are rejected before any request is sent", async () => {
  const { fetch, seen } = recorder();
  const dig = new DigClient({ fetch });
  const bad: Array<[string, () => Promise<unknown>]> = [
    ["empty query", () => dig.find("")],
    ["blank query", () => dig.find("   ")],
    ["empty recall query", () => dig.recall("")],
    ["empty kb", () => dig.find("x", { kb: "" })],
    ["empty kb on undo", () => dig.undo({ kb: "" })],
    ["empty kb on org", () => dig.org({ kb: "", apply: true })],
    ["empty kb on reconcile", () => dig.reconcile({ kb: "", apply: true })],
    ["empty kb on drift", () => dig.drift({ kb: "" })],
    ["empty kb on log", () => dig.log({ kb: "" })],
    ["empty kb on export", () => dig.export({ kb: "" })],
    ["zero limit", () => dig.find("x", { limit: 0 })],
    ["negative limit", () => dig.find("x", { limit: -1 })],
    ["NaN limit", () => dig.find("x", { limit: Number.NaN })],
    ["fractional limit", () => dig.find("x", { limit: 1.5 })],
    ["infinite limit", () => dig.find("x", { limit: Infinity })],
    ["zero budget", () => dig.recall("x", { budget: 0 })],
    ["fractional budget", () => dig.recall("x", { budget: 0.5 })],
    ["unknown mode", () => dig.find("x", { mode: "fuzzy" as "fts" })],
    ["unknown recall mode", () => dig.recall("x", { mode: "" as "fts" })],
    ["empty content", () => dig.retain("")],
    ["empty as", () => dig.retain("note", { as: "" })],
    ["empty filter", () => dig.export({ filter: "" })],
    ["empty at", () => dig.export({ at: "" })],
    ["non-string query", () => dig.find(42 as unknown as string)],
  ];
  for (const [name, call] of bad) {
    await assert.rejects(call, TypeError, name);
  }
  assert.equal(seen.length, 0, "no request may reach the daemon");
});

test("baseUrl is validated when the client is built", () => {
  for (const baseUrl of ["", "not a url", "ftp://127.0.0.1:3978", "http://h/?x=1", "http://h/#frag"]) {
    assert.throws(() => new DigClient({ baseUrl }), TypeError, baseUrl);
  }
});

test("trailing slashes are trimmed in linear time (no polynomial ReDoS)", () => {
  const baseUrl = `http://127.0.0.1:9/${"/".repeat(100_000)}x`;
  const started = performance.now();
  new DigClient({ baseUrl, fetch: recorder().fetch });
  assert.ok(performance.now() - started < 500, "baseUrl normalization took too long");
});

test("baseUrl keeps a path prefix and drops every trailing slash", async () => {
  const { fetch, seen } = recorder(() => Response.json({ status: "ok", version: "1" }));
  await new DigClient({ baseUrl: "http://127.0.0.1:9/dig///", fetch }).health();
  assert.equal(seen[0]?.url.href, "http://127.0.0.1:9/dig/health");
});

test("values are percent-encoded into the query, never into the path", async () => {
  const { fetch, seen } = recorder();
  const query = "a&kb=evil #x/../y?z";
  await new DigClient({ fetch }).find(query, { kb: "my kb/../other", limit: 3, mode: "hybrid" });
  const url = seen[0]?.url;
  assert.equal(url?.pathname, "/find");
  assert.equal(url?.searchParams.get("query"), query);
  assert.equal(url?.searchParams.get("kb"), "my kb/../other");
  assert.equal(url?.searchParams.get("limit"), "3");
  assert.equal(url?.searchParams.get("mode"), "hybrid");
  assert.equal([...(url?.searchParams.keys() ?? [])].length, 4);
});

test("mutations preview unless apply is set, and retain posts the body", async () => {
  const { fetch, seen } = recorder(() => Response.json({ output: "ok" }));
  const dig = new DigClient({ fetch });
  await dig.org();
  await dig.reconcile({ apply: true });
  await dig.undo({ kb: "k" });
  await dig.retain("note", { as: "memory/a.md", kb: "k" });
  assert.deepEqual(
    seen.map((s) => [s.method, s.url.pathname, s.url.searchParams.get("apply")]),
    [
      ["POST", "/org", null],
      ["POST", "/reconcile", "true"],
      ["POST", "/undo", null],
      ["POST", "/retain", null],
    ],
  );
  assert.equal(seen[3]?.body, "note");
  assert.equal(seen[3]?.url.searchParams.get("as"), "memory/a.md");
});

test("a custom fetch is not called as a method of the client", async () => {
  // Browsers throw "Illegal invocation" when fetch runs with another `this`.
  let receiver: unknown = "unset";
  const custom = function (this: unknown): Promise<Response> {
    receiver = this;
    return Promise.resolve(Response.json({ status: "ok", version: "1" }));
  } as unknown as typeof fetch;
  await new DigClient({ fetch: custom }).health();
  assert.equal(receiver, undefined);
});

test("the global fetch is called with the global receiver", async () => {
  const original = globalThis.fetch;
  let receiver: unknown = "unset";
  globalThis.fetch = function (this: unknown): Promise<Response> {
    receiver = this;
    return Promise.resolve(Response.json({ status: "ok", version: "1" }));
  } as unknown as typeof fetch;
  try {
    await new DigClient().health();
  } finally {
    globalThis.fetch = original;
  }
  assert.equal(receiver, globalThis);
});

test("an error response with no JSON body still has a useful message", async () => {
  // HTTP/2 responses carry no statusText, and proxies answer in HTML or plain text.
  const { fetch } = recorder(() => new Response("", { status: 502 }));
  await assert.rejects(
    () => new DigClient({ fetch }).health(),
    (err: unknown) =>
      err instanceof DigError && err.status === 502 && /502/.test(err.message),
  );
});

test("a daemon error message is passed through", async () => {
  const { fetch } = recorder(() => Response.json({ error: "no such kb" }, { status: 400 }));
  await assert.rejects(
    () => new DigClient({ fetch }).drift(),
    (err: unknown) => err instanceof DigError && err.status === 400 && err.message === "no such kb",
  );
});

test("a 2xx response that is not JSON surfaces as a DigError", async () => {
  const { fetch } = recorder(() => new Response("<html>proxy</html>", { status: 200 }));
  await assert.rejects(
    () => new DigClient({ fetch }).health(),
    (err: unknown) => err instanceof DigError && err.status === 200,
  );
});

test("export returns the output text, or empty for an empty body", async () => {
  const withOutput = recorder(() => Response.json({ output: "{}\n" }));
  assert.equal(await new DigClient({ fetch: withOutput.fetch }).export({ filter: "label:a", at: "M1" }), "{}\n");
  assert.equal(withOutput.seen[0]?.url.searchParams.get("filter"), "label:a");
  assert.equal(withOutput.seen[0]?.url.searchParams.get("at"), "M1");

  const empty = recorder(() => new Response("", { status: 200 }));
  assert.equal(await new DigClient({ fetch: empty.fetch }).export(), "");
});
