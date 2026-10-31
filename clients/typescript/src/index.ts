/**
 * @vllnt/dig — a thin, dependency-free TypeScript client for a local dig
 * daemon (`dig serve`). It drives the same surface as the CLI over HTTP:
 * search, drift, history, export (read) and organize / reconcile / undo
 * (mutations, preview-by-default). Local-first: it talks only to the loopback
 * daemon you run.
 */

/** A search hit from {@link DigClient.find}. */
export interface FindResult {
  /** KB-relative path. */
  Path: string;
  /** Content-addressed blob id. */
  Blob: string;
  /** Labels on the entry, or null. */
  Labels: string[] | null;
  /** Fusion/similarity score (vector/hybrid modes). */
  Score?: number;
}

/** Retrieval mode for {@link DigClient.find}. */
export type RetrievalMode = "fts" | "vector" | "hybrid";

/** One retrieved source in a {@link RecallPack}: a snippet plus provenance. */
export interface RecallItem {
  /** KB-relative path. */
  path: string;
  /** Content-addressed blob id. */
  blob: string;
  /** Fusion/similarity score (vector/hybrid modes). */
  score?: number;
  /** The query-relevant snippet of the source. */
  content: string;
}

/** A token-budgeted, provenance-tagged recall result from {@link DigClient.recall}. */
export interface RecallPack {
  query: string;
  /** KB root the pack was drawn from. */
  kb: string;
  /** Head manifest id — pins the pack to a point in history. */
  manifest: string;
  budgetTokens: number;
  usedTokens: number;
  items: RecallItem[];
}

/** Result of {@link DigClient.retain}. */
export interface RetainResult {
  /** The daemon's confirmation line, e.g. "Retained memory/… → manifest M2". */
  output: string;
}

/** Options shared by every call: which KB to target (omit to use the daemon's working-dir KB). */
export interface KbOptions {
  /** KB name or path. */
  kb?: string;
}

/** Constructor options. */
export interface DigClientOptions {
  /** Base URL of a running `dig serve` daemon. Defaults to the local default port. */
  baseUrl?: string;
  /** Optional fetch implementation (defaults to global fetch). */
  fetch?: typeof fetch;
}

/** Daemon health response. */
export interface Health {
  status: string;
  version: string;
}

/** Thrown when the daemon returns a non-2xx response. */
export class DigError extends Error {
  constructor(
    message: string,
    /** HTTP status code. */
    readonly status: number,
  ) {
    super(message);
    this.name = "DigError";
  }
}

const DEFAULT_BASE_URL = "http://127.0.0.1:3978";

const RETRIEVAL_MODES: readonly string[] = ["fts", "vector", "hybrid"];

/** Reject a present-but-empty (or non-string) value; undefined means "not set". */
function checkString(name: string, value: unknown): void {
  if (value === undefined) return;
  if (typeof value !== "string" || value.trim() === "") {
    throw new TypeError(`${name} must be a non-empty string`);
  }
}

/** Reject zero, negative, fractional, NaN or infinite counts; undefined means "not set". */
function checkCount(name: string, value: unknown): void {
  if (value === undefined) return;
  if (typeof value !== "number" || !Number.isInteger(value) || value <= 0) {
    throw new TypeError(`${name} must be a positive integer`);
  }
}

function checkMode(value: unknown): void {
  if (value === undefined) return;
  if (typeof value !== "string" || !RETRIEVAL_MODES.includes(value)) {
    throw new TypeError(`mode must be one of: ${RETRIEVAL_MODES.join(", ")}`);
  }
}

/** An empty kb would silently fall back to the daemon's working-directory KB. */
function checkKb(options: KbOptions): void {
  checkString("kb", options.kb);
}

function normalizeBaseUrl(raw: string): string {
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    throw new TypeError(`baseUrl is not a valid URL: ${raw}`);
  }
  if (url.protocol !== "http:" && url.protocol !== "https:") {
    throw new TypeError("baseUrl must use http or https");
  }
  if (url.search !== "" || url.hash !== "") {
    throw new TypeError("baseUrl must not contain a query string or fragment");
  }
  return trimTrailingSlashes(raw);
}

/** A loop, not `/\/+$/`: that regex backtracks quadratically on a long run of slashes. */
function trimTrailingSlashes(value: string): string {
  let end = value.length;
  while (end > 0 && value[end - 1] === "/") end--;
  return value.slice(0, end);
}

/**
 * A client for a local dig daemon.
 *
 * @example
 * const dig = new DigClient();
 * const hits = await dig.find("invoice acme", { mode: "hybrid", limit: 5 });
 */
export class DigClient {
  private readonly baseUrl: string;
  private readonly doFetch: typeof fetch;

  constructor(options: DigClientOptions = {}) {
    this.baseUrl = normalizeBaseUrl(options.baseUrl ?? DEFAULT_BASE_URL);
    // Never call fetch as a method of the client: browsers throw "Illegal
    // invocation" when fetch's `this` is not the window. The global is resolved
    // at call time so a later polyfill is honored.
    const custom = options.fetch;
    this.doFetch = (input, init) =>
      custom ? custom(input, init) : globalThis.fetch(input, init);
  }

  /** Liveness + daemon version. */
  async health(): Promise<Health> {
    return this.request<Health>("GET", "/health", {});
  }

  /** Search the KB, ranked. Add mode to opt into semantic/hybrid retrieval. */
  async find(
    query: string,
    options: KbOptions & { mode?: RetrievalMode; limit?: number } = {},
  ): Promise<FindResult[]> {
    checkKb(options);
    checkString("query", query);
    checkMode(options.mode);
    checkCount("limit", options.limit);
    return this.request<FindResult[]>("GET", "/find", {
      kb: options.kb,
      query,
      mode: options.mode,
      limit: options.limit,
    });
  }

  /**
   * Load a token-budgeted, provenance-tagged context pack for a query — the
   * agent-memory recall primitive. Snippets land on the matching passage.
   *
   * @example
   * const pack = await dig.recall("billing ledger decision", { budget: 1000 });
   * for (const item of pack.items) console.log(item.path, item.content);
   */
  async recall(
    query: string,
    options: KbOptions & { mode?: RetrievalMode; budget?: number } = {},
  ): Promise<RecallPack> {
    checkKb(options);
    checkString("query", query);
    checkMode(options.mode);
    checkCount("budget", options.budget);
    return this.request<RecallPack>("GET", "/recall", {
      kb: options.kb,
      query,
      mode: options.mode,
      budget: options.budget,
    });
  }

  /**
   * Capture content (an agent session, a note, a document) into the KB and
   * index it — the agent-memory capture primitive. Writes to a dated memory/
   * path by default; pass `as` to choose the path. Reversible with undo.
   *
   * @example
   * await dig.retain(sessionMarkdown, { as: "memory/sessions/today.md" });
   */
  async retain(
    content: string,
    options: KbOptions & { as?: string } = {},
  ): Promise<RetainResult> {
    checkKb(options);
    checkString("content", content);
    checkString("as", options.as);
    return this.request<RetainResult>(
      "POST",
      "/retain",
      { kb: options.kb, as: options.as },
      content,
    );
  }

  /** Report how the KB diverges from its policy. Read-only. */
  async drift(options: KbOptions = {}): Promise<unknown> {
    checkKb(options);
    return this.request("GET", "/drift", { kb: options.kb });
  }

  /** Browse change history, newest first. Read-only. */
  async log(options: KbOptions = {}): Promise<unknown> {
    checkKb(options);
    return this.request("GET", "/log", { kb: options.kb });
  }

  /** Export a reproducible, provenance-tagged dataset (JSONL text). Read-only. */
  async export(
    options: KbOptions & { filter?: string; at?: string } = {},
  ): Promise<string> {
    checkKb(options);
    checkString("filter", options.filter);
    checkString("at", options.at);
    const body = await this.request<{ output?: string } | undefined>("GET", "/export", {
      kb: options.kb,
      filter: options.filter,
      at: options.at,
    });
    return body?.output ?? "";
  }

  /** Apply organization policy. Previews unless apply is true (reversible with undo). */
  async org(options: KbOptions & { apply?: boolean } = {}): Promise<unknown> {
    checkKb(options);
    return this.request("POST", "/org", { kb: options.kb, apply: options.apply });
  }

  /** Converge the KB to policy. Previews unless apply is true (reversible with undo). */
  async reconcile(
    options: KbOptions & { apply?: boolean } = {},
  ): Promise<unknown> {
    checkKb(options);
    return this.request("POST", "/reconcile", {
      kb: options.kb,
      apply: options.apply,
    });
  }

  /** Revert the last changeset. */
  async undo(options: KbOptions = {}): Promise<unknown> {
    checkKb(options);
    return this.request("POST", "/undo", { kb: options.kb });
  }

  private async request<T>(
    method: "GET" | "POST",
    path: string,
    params: Record<string, string | number | boolean | undefined>,
    body?: string,
  ): Promise<T> {
    const url = new URL(this.baseUrl + path);
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined) url.searchParams.set(key, String(value));
    }
    const init: RequestInit = { method };
    if (body !== undefined) init.body = body;
    const response = await this.doFetch(url, init);
    const text = await response.text();
    if (!response.ok) {
      const message =
        parseError(text) ||
        response.statusText ||
        `request failed with status ${response.status}`;
      throw new DigError(message, response.status);
    }
    try {
      return parseBody<T>(text);
    } catch {
      throw new DigError(`invalid JSON in response to ${method} ${path}`, response.status);
    }
  }
}

function parseError(text: string): string | undefined {
  try {
    const body: unknown = JSON.parse(text);
    if (body && typeof body === "object" && "error" in body) {
      return String((body as { error: unknown }).error);
    }
  } catch {
    // fall through
  }
  return undefined;
}

function parseBody<T>(text: string): T {
  if (text === "") return undefined as T;
  return JSON.parse(text) as T;
}
