/**
 * Integration tests: the SDK drives a REAL `dig serve` daemon over HTTP against
 * a real temp KB — no mocks. The dig binary comes from $DIG_BIN (the CI builds
 * it) or `dig` on PATH.
 */
import { strict as assert } from "node:assert";
import { spawn, spawnSync, type ChildProcess } from "node:child_process";
import { mkdtempSync, writeFileSync, mkdirSync, existsSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { after, before, test } from "node:test";

import { DigClient, DigError } from "../src/index.ts";

const DIG = process.env.DIG_BIN ?? "dig";
const PORT = 3987;

let daemon: ChildProcess;
let kb: string;

const policy = `
[[rule]]
name  = "invoices"
match = { ext = ["pdf"], content_matches = "invoice" }
into  = "finance/invoices"
label = ["finance"]
`;

function dig(...args: string[]): void {
  const result = spawnSync(DIG, args, { encoding: "utf8" });
  if (result.status !== 0) {
    throw new Error(`dig ${args.join(" ")} failed: ${result.stderr || result.stdout}`);
  }
}

async function waitForHealth(client: DigClient): Promise<void> {
  for (let i = 0; i < 50; i++) {
    try {
      await client.health();
      return;
    } catch {
      await new Promise((r) => setTimeout(r, 100));
    }
  }
  throw new Error("daemon never became healthy");
}

before(async () => {
  kb = mkdtempSync(join(tmpdir(), "dig-sdk-"));
  mkdirSync(join(kb, "inbox"), { recursive: true });
  writeFileSync(join(kb, "inbox", "acme.pdf"), "ACME invoice #1007");
  writeFileSync(join(kb, "inbox", "todo.md"), "- [ ] things");
  dig("init", kb);
  writeFileSync(join(kb, ".dig", "policy.toml"), policy);
  dig("--kb", kb, "scan");

  daemon = spawn(DIG, ["serve", "--addr", `127.0.0.1:${PORT}`], {
    stdio: "ignore",
  });
  await waitForHealth(new DigClient({ baseUrl: `http://127.0.0.1:${PORT}` }));
});

after(() => {
  daemon?.kill();
});

const client = (): DigClient =>
  new DigClient({ baseUrl: `http://127.0.0.1:${PORT}` });

test("health reports the daemon version", async () => {
  const h = await client().health();
  assert.equal(h.status, "ok");
  assert.ok(h.version);
});

test("find returns the indexed document", async () => {
  const hits = await client().find("invoice", { kb });
  assert.ok(hits.some((hit) => hit.Path.endsWith("acme.pdf")));
});

test("org previews by default, then applies, then undo reverts", async () => {
  const moved = (): boolean => existsSync(join(kb, "finance", "invoices", "acme.pdf"));

  await client().org({ kb }); // preview
  assert.equal(moved(), false, "preview must not move files");

  await client().org({ kb, apply: true });
  assert.equal(moved(), true, "apply must move the file");

  await client().undo({ kb });
  assert.equal(moved(), false, "undo must revert");
});

test("log and drift return data", async () => {
  assert.ok(await client().log({ kb }));
  assert.ok(await client().drift({ kb }));
});

test("a bad request surfaces a DigError", async () => {
  await assert.rejects(
    () => client().find("anything", { kb: "/no/such/kb" }),
    (err: unknown) => err instanceof DigError && err.status >= 400,
  );
});
