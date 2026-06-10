import { Card } from "@vllnt/ui";
import {
  FolderTree,
  GitCompare,
  GitMerge,
  History,
  Layers,
  Search,
} from "lucide-react";
import { useTranslations } from "next-intl";

const FEATURES = [
  { Icon: Search, key: "f1" },
  { Icon: FolderTree, key: "f2" },
  { Icon: Layers, key: "f3" },
  { Icon: History, key: "f4" },
  { Icon: GitCompare, key: "f5" },
  { Icon: GitMerge, key: "f6" },
] as const;

export function Features() {
  const t = useTranslations("features");

  return (
    <section className="border-t border-border bg-muted/30" id="features">
      <div className="mx-auto flex max-w-5xl flex-col gap-12 px-6 py-24">
        <div className="flex flex-col gap-4 text-center">
          <h2 className="text-balance text-3xl font-semibold tracking-tight sm:text-4xl">
            {t("title")}
          </h2>
          <p className="mx-auto max-w-2xl text-pretty text-muted-foreground">
            {t("subtitle")}
          </p>
        </div>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {FEATURES.map(({ Icon, key }) => (
            <Card className="flex flex-col gap-3 p-6" key={key}>
              <div className="flex h-8 w-8 items-center justify-center rounded-md bg-muted">
                <Icon aria-hidden className="h-4 w-4" />
              </div>
              <h3 className="font-semibold">{t(`${key}_title`)}</h3>
              <p className="text-sm leading-6 text-muted-foreground">
                {t(`${key}_body`)}
              </p>
            </Card>
          ))}
        </div>
      </div>
    </section>
  );
}
