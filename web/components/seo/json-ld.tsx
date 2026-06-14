/**
 * Renders a JSON-LD structured-data block. Each page builds its schema object
 * inline (kept out of `lib/` to skip a coverage burden).
 */
export function JsonLd({
  data,
}: {
  /** A schema.org object graph. */
  data: Record<string, unknown>;
}): React.JSX.Element {
  return (
    <script
      // JSON.stringify output is safe to inline; this is the canonical JSON-LD pattern.
      dangerouslySetInnerHTML={{ __html: JSON.stringify(data) }}
      type="application/ld+json"
    />
  );
}
