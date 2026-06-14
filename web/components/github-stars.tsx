import { GITHUB_URL } from "@/lib/site";

const REPO_API = "https://api.github.com/repos/vllnt/dig";

function formatStars(count: number): string {
  return count >= 1000 ? `${(count / 1000).toFixed(1)}k` : String(count);
}

async function fetchStarCount(): Promise<number | undefined> {
  const response = await fetch(REPO_API, { next: { revalidate: 3600 } });
  if (!response.ok) return undefined;
  const data: unknown = await response.json();
  if (
    data &&
    typeof data === "object" &&
    "stargazers_count" in data &&
    typeof data.stargazers_count === "number"
  ) {
    return data.stargazers_count;
  }
  return undefined;
}

/**
 * Resolve the star count server-side (revalidated every hour), bounded by a
 * timeout so a slow API never blocks the render. Returns `undefined` when the
 * count is unavailable — e.g. while the repo is private (the API 404s) — so the
 * link degrades to a plain label with no browser console noise.
 */
async function getStarCount(): Promise<number | undefined> {
  const timeout = new Promise<void>((resolve) => {
    setTimeout(resolve, 2000);
  });
  try {
    const result = await Promise.race([fetchStarCount(), timeout]);
    return typeof result === "number" ? result : undefined;
  } catch {
    return undefined;
  }
}

/**
 * GitHub repo link with a live star count. The count lights up once the repo is
 * public; until then the link renders the label alone.
 */
export async function GithubStars({
  className,
  label = "GitHub",
}: {
  /** Class applied to the anchor (style as a link or a button). */
  className?: string;
  /** Visible text before the count. */
  label?: string;
}): Promise<React.JSX.Element> {
  const stars = await getStarCount();
  return (
    <a className={className} href={GITHUB_URL} rel="noreferrer" target="_blank">
      <span>{label}</span>
      {stars === undefined ? null : (
        <span aria-label={`${stars} stars`}>{` ★ ${formatStars(stars)}`}</span>
      )}
    </a>
  );
}
