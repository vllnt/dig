"use client";

import { useEffect, useState } from "react";

import { AnalyticsProvider, useAnalytics } from "@vllnt/analytics/react";
import { CookieConsent } from "@vllnt/ui";
import { useTranslations } from "next-intl";

const ANALYTICS_CONFIG = {
  app: "dig-web",
  debug: process.env.NODE_ENV !== "production",
  version: 1,
};

function ConsentBanner() {
  const t = useTranslations("consent");
  const { acceptAll, declineAnalytics, hasResponded } = useAnalytics();
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  if (!mounted || hasResponded) return null;

  return (
    <CookieConsent
      acceptText={t("accept")}
      declineText={t("decline")}
      message={t("message")}
      onAccept={acceptAll}
      onDecline={declineAnalytics}
    />
  );
}

export function Analytics({ children }: { children: React.ReactNode }) {
  return (
    <AnalyticsProvider config={ANALYTICS_CONFIG}>
      {children}
      <ConsentBanner />
    </AnalyticsProvider>
  );
}
