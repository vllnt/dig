"use client";

import { ThemeProvider } from "@vllnt/ui";

import { Analytics } from "./analytics";

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <ThemeProvider
      attribute="class"
      defaultTheme="dark"
      disableTransitionOnChange
      enableSystem
      storageKey="theme"
    >
      <Analytics>{children}</Analytics>
    </ThemeProvider>
  );
}
