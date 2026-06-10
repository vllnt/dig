export function Terminal({ code, title }: { code: string; title: string }) {
  return (
    <div className="w-full overflow-hidden rounded-lg border border-border bg-card text-left">
      <div className="flex items-center gap-2 border-b border-border px-4 py-2">
        <span
          aria-hidden
          className="h-2 w-2 rounded-full bg-muted-foreground/40"
        />
        <span
          aria-hidden
          className="h-2 w-2 rounded-full bg-muted-foreground/40"
        />
        <span
          aria-hidden
          className="h-2 w-2 rounded-full bg-muted-foreground/40"
        />
        <span className="ml-2 font-mono text-xs text-muted-foreground">
          {title}
        </span>
      </div>
      <pre className="overflow-x-auto p-4 font-mono text-sm leading-6 text-card-foreground/90">
        <code>{code}</code>
      </pre>
    </div>
  );
}
