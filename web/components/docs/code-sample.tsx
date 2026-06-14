/** A labelled, syntax-neutral code block for short inline examples. */
export function CodeSample({
  code,
  lang,
}: {
  /** The code to render, verbatim. */
  code: string;
  /** Optional language label shown above the block. */
  lang?: string;
}): React.JSX.Element {
  return (
    <div className="flex flex-col gap-1">
      {lang ? (
        <span className="font-mono text-xs uppercase tracking-wider text-muted-foreground">
          {lang}
        </span>
      ) : null}
      <pre className="overflow-x-auto rounded-lg border border-border bg-muted/50 p-4 font-mono text-sm leading-6">
        <code>{code}</code>
      </pre>
    </div>
  );
}
