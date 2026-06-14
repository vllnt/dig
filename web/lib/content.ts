import fs from "node:fs";
import path from "node:path";

import matter from "gray-matter";

/** The SEO content clusters. Each maps to `content/<cluster>/*.mdx`. */
export const CLUSTERS = ["compare", "use-cases", "learn"] as const;

/** A content cluster slug. */
export type Cluster = (typeof CLUSTERS)[number];

/** Frontmatter every content page must declare. */
export type ContentMeta = {
  /** Meta description + page subtitle. */
  description: string;
  /** Page `<h1>` and `<title>`. */
  title: string;
  /** Optional ISO date, used for sitemap `lastModified`. */
  updated?: string;
};

/** A resolved content entry: its metadata plus where it lives. */
export type ContentEntry = ContentMeta & {
  cluster: Cluster;
  slug: string;
};

/** A parsed file: validated metadata and the MDX body (frontmatter stripped). */
export type ParsedContent = {
  body: string;
  meta: ContentEntry;
};

const CONTENT_DIR = path.join(process.cwd(), "content");

/** Type guard: is `value` one of the known clusters? */
export function isCluster(value: string): value is Cluster {
  return (CLUSTERS as readonly string[]).includes(value);
}

/**
 * Parse a raw `.mdx` file (frontmatter + body) into a validated entry. Pure —
 * no filesystem — so the validation branches stay unit-testable.
 *
 * @returns the parsed content, or `undefined` when required frontmatter is missing
 */
export function parseContent(
  raw: string,
  cluster: Cluster,
  slug: string,
): ParsedContent | undefined {
  const { content, data } = matter(raw);
  const title = typeof data.title === "string" ? data.title.trim() : "";
  const description =
    typeof data.description === "string" ? data.description.trim() : "";
  if (!title || !description) return undefined;
  const updated = typeof data.updated === "string" ? data.updated : undefined;
  return {
    body: content,
    meta: {
      cluster,
      description,
      slug,
      title,
      ...(updated ? { updated } : {}),
    },
  };
}

/** Slugs (no extension) for every `.mdx` file in a cluster, sorted. */
export function listSlugs(cluster: Cluster): string[] {
  const directory = path.join(CONTENT_DIR, cluster);
  if (!fs.existsSync(directory)) return [];
  return fs
    .readdirSync(directory)
    .filter((file) => file.endsWith(".mdx"))
    .map((file) => file.replace(/\.mdx$/, ""))
    .sort();
}

/** Read and parse one entry from disk; `undefined` when missing or invalid. */
export function readContent(
  cluster: Cluster,
  slug: string,
): ParsedContent | undefined {
  const file = path.join(CONTENT_DIR, cluster, `${slug}.mdx`);
  if (!fs.existsSync(file)) return undefined;
  return parseContent(fs.readFileSync(file, "utf8"), cluster, slug);
}

/** Metadata for every valid entry across all clusters (sitemap + params). */
export function listAllContent(): ContentEntry[] {
  return CLUSTERS.flatMap((cluster) =>
    listSlugs(cluster)
      .map((slug) => readContent(cluster, slug)?.meta)
      .filter((meta): meta is ContentEntry => meta !== undefined),
  );
}
