/**
 * Integration test for the AI SDK helpers: the generated tools drive a REAL
 * `dig serve` daemon — no mocks.
 */
import { strict as assert } from "node:assert";
import { spawn, spawnSync, type ChildProcess } from "node:child_process";
import { mkdtempSync, writeFileSync, mkdirSync, existsSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { after, before, test } from "node:test";

import { DigClient } from "../src/index.ts";
import { digTools } from "../src/ai.ts";

const DIG = process.env.DIG_BIN ?? "dig";
const PORT = 3985;
const policy = `
[[rule]]
name  = "invoices"
match = { ext = ["pdf"], content_matches = "invoice" }
into  = "finance/invoices"
label = ["finance"]
`;

let daemon: ChildProcess;
let kb: string;

function dig(...args: string[]): void {
  const r = spawnSync(DIG, args, { encoding: "utf8" });
  if (r.status !== 0) throw new Error(`dig ${args.join(" ")}: ${r.stderr || r.stdout}`);
}

before(async () => {
  kb = mkdtempSync(join(tmpdir(), "dig-ai-"));
  mkdirSync(join(kb, "inbox"), { recursive: true });
  writeFileSync(join(kb, "inbox", "acme.pdf"), "ACME invoice #1007");
  dig("init", kb);
  writeFileSync(join(kb, ".dig", "policy.toml"), policy);
  dig("--kb", kb, "scan");

  daemon = spawn(DIG, ["serve", "--addr", `127.0.0.1:${PORT}`], { stdio: "ignore" });
  const client = new DigClient({ baseUrl: `http://127.0.0.1:${PORT}` });
  for (let i = 0; i < 50; i++) {
    try {
      await client.health();
      break;
    } catch {
      await new Promise((r) => setTimeout(r, 100));
    }
  }
});

after(() => daemon?.kill());

const tools = (): ReturnType<typeof digTools> =>
  digTools(new DigClient({ baseUrl: `http://127.0.0.1:${PORT}` }));

const opts = { toolCallId: "test", messages: [] };

test("digTools exposes the dig surface", () => {
  const names = Object.keys(tools()).sort();
  assert.deepEqual(names, [
    "dig_drift",
    "dig_export",
    "dig_find",
    "dig_log",
    "dig_org",
    "dig_reconcile",
    "dig_undo",
  ]);
});

test("dig_find tool drives a real daemon", async () => {
  const found = (await tools().dig_find.execute!(
    { query: "invoice", kb },
    opts,
  )) as Array<{ Path: string }>;
  assert.ok(found.some((h) => h.Path.endsWith("acme.pdf")));
});

test("dig_org tool previews, then applies, then dig_undo reverts", async () => {
  const t = tools();
  const moved = (): boolean => existsSync(join(kb, "finance", "invoices", "acme.pdf"));

  await t.dig_org.execute!({ kb }, opts); // preview
  assert.equal(moved(), false);

  await t.dig_org.execute!({ kb, apply: true }, opts);
  assert.equal(moved(), true);

  await t.dig_undo.execute!({ kb }, opts);
  assert.equal(moved(), false);
});
