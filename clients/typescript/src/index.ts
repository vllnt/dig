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
    this.baseUrl = (options.baseUrl ?? DEFAULT_BASE_URL).replace(/\/$/, "");
    this.doFetch = options.fetch ?? globalThis.fetch;
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
    return this.request<FindResult[]>("GET", "/find", {
      kb: options.kb,
      query,
      mode: options.mode,
      limit: options.limit,
    });
  }

  /** Report how the KB diverges from its policy. Read-only. */
  async drift(options: KbOptions = {}): Promise<unknown> {
    return this.request("GET", "/drift", { kb: options.kb });
  }

  /** Browse change history, newest first. Read-only. */
  async log(options: KbOptions = {}): Promise<unknown> {
    return this.request("GET", "/log", { kb: options.kb });
  }

  /** Export a reproducible, provenance-tagged dataset (JSONL text). Read-only. */
  async export(
    options: KbOptions & { filter?: string; at?: string } = {},
  ): Promise<string> {
    const body = await this.request<{ output?: string }>("GET", "/export", {
      kb: options.kb,
      filter: options.filter,
      at: options.at,
    });
    return body.output ?? "";
  }

  /** Apply organization policy. Previews unless apply is true (reversible with undo). */
  async org(options: KbOptions & { apply?: boolean } = {}): Promise<unknown> {
    return this.request("POST", "/org", { kb: options.kb, apply: options.apply });
  }

  /** Converge the KB to policy. Previews unless apply is true (reversible with undo). */
  async reconcile(
    options: KbOptions & { apply?: boolean } = {},
  ): Promise<unknown> {
    return this.request("POST", "/reconcile", {
      kb: options.kb,
      apply: options.apply,
    });
  }

  /** Revert the last changeset. */
  async undo(options: KbOptions = {}): Promise<unknown> {
    return this.request("POST", "/undo", { kb: options.kb });
  }

  private async request<T>(
    method: "GET" | "POST",
    path: string,
    params: Record<string, string | number | boolean | undefined>,
  ): Promise<T> {
    const url = new URL(this.baseUrl + path);
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined) url.searchParams.set(key, String(value));
    }
    const response = await this.doFetch(url, { method });
    const text = await response.text();
    if (!response.ok) {
      const message = parseError(text) ?? response.statusText;
      throw new DigError(message, response.status);
    }
    return parseBody<T>(text);
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
