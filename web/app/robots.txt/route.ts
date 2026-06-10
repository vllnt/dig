import { SITE_URL } from "@/lib/site";

export const revalidate = 86_400;

function buildRobotsBody(baseUrl: string): string {
  const lines = [
    "User-agent: *",
    "Allow: /",
    "",
    `Sitemap: ${baseUrl}/sitemap.xml`,
    "",
  ];
  return lines.join("\n");
}

export function GET(): Response {
  return new Response(buildRobotsBody(SITE_URL), {
    headers: {
      "Cache-Control": "public, max-age=3600, s-maxage=86400",
      "Content-Type": "text/plain; charset=utf-8",
    },
  });
}
