import { evaluate } from "@mdx-js/mdx";
import * as runtime from "react/jsx-runtime";

import { baseMdxComponents } from "@/mdx-components";

/**
 * Renders an MDX body string (frontmatter already stripped by the content
 * loader) to React. Compilation happens at build time via `generateStaticParams`,
 * so there is no per-request cost. Server component.
 */
export async function Mdx({
  source,
}: {
  /** MDX body without frontmatter. */
  source: string;
}): Promise<React.JSX.Element> {
  const { default: Content } = await evaluate(source, {
    ...runtime,
    baseUrl: import.meta.url,
  });
  return <Content components={baseMdxComponents} />;
}
