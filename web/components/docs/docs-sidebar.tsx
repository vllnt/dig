/** A sticky in-page section nav for the docs, tech-doc style. */
export function DocsSidebar({
  items,
}: {
  /** Section anchors and their labels, in order. */
  items: readonly { href: string; label: string }[];
}): React.JSX.Element {
  return (
    <aside className="hidden w-44 shrink-0 lg:block">
      <nav className="sticky top-24 flex flex-col gap-2 border-l border-border pl-4 text-sm">
        <span className="mb-1 font-mono text-xs uppercase tracking-wider text-muted-foreground">
          On this page
        </span>
        {items.map((item) => (
          <a
            className="text-muted-foreground transition-colors hover:text-foreground"
            href={item.href}
            key={item.href}
          >
            {item.label}
          </a>
        ))}
      </nav>
    </aside>
  );
}
