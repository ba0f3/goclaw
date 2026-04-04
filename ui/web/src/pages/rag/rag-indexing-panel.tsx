import { useTranslation } from "react-i18next";
import { Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ConfigGroupHeader } from "@/components/shared/config-group-header";
import type { RAGDepsResponse } from "@/types/agent";

/** Optional RAG extractors — must match internal/rag/deps.go CheckDeps order/semantics */
export const RAG_BUILTIN_EXTS = [".md", ".txt", ".csv"] as const;
export const RAG_OPTIONAL_EXTS = [".pdf", ".docx", ".odt", ".epub", ".xlsx", ".pptx"] as const;

export function pkgsForRagUnsupported(unsupported: string[] | undefined): string[] {
  if (!unsupported?.length) return [];
  const u = new Set(unsupported);
  const out: string[] = [];
  if (u.has(".pdf")) out.push("pip:pypdf");
  if (u.has(".docx") || u.has(".odt") || u.has(".epub")) out.push("pandoc");
  if (u.has(".xlsx")) out.push("pip:openpyxl");
  if (u.has(".pptx")) out.push("pip:python-pptx");
  return [...new Set(out)];
}

interface RagIndexingPanelProps {
  ragLiveDeps: RAGDepsResponse | null;
  ragProbeLoading: boolean;
  ragProbeFailed: boolean;
  onRetryProbe: () => void;
  onInstallMissing: () => void;
  installingRagPkgs: boolean;
  /** When set, optional extractors table uses this for “supported” before live probe completes */
  ragCachedSupported?: string[] | undefined;
}

export function RagIndexingPanel({
  ragLiveDeps,
  ragProbeLoading,
  ragProbeFailed,
  onRetryProbe,
  onInstallMissing,
  installingRagPkgs,
  ragCachedSupported,
}: RagIndexingPanelProps) {
  const { t } = useTranslation("rag");

  const ragSupported = ragLiveDeps?.supported ?? ragCachedSupported;
  const ragUnsupported = ragLiveDeps?.unsupported;
  const ragWarnings = ragLiveDeps?.warnings;

  const supportedSet = new Set((ragSupported ?? []).map((e) => e.toLowerCase()));
  const unsupportedSet = new Set((ragUnsupported ?? []).map((e) => e.toLowerCase()));

  return (
    <div className="space-y-4">
      <ConfigGroupHeader title={t("title")} description={t("description")} />
      <div className="space-y-3 rounded-lg border p-4 text-sm">
        <p className="text-muted-foreground">{t("formatsIntro")}</p>
        <div className="space-y-1">
          <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            {t("builtInTitle")}
          </p>
          <p>
            <span className="text-muted-foreground">{t("alwaysAvailable")}: </span>
            {RAG_BUILTIN_EXTS.join(" ")}
          </p>
          <p className="text-muted-foreground">{t("builtInHint")}</p>
        </div>
        <div className="space-y-2">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
              {t("optionalTitle")}
            </p>
            {ragProbeLoading ? (
              <span className="inline-flex items-center gap-1 text-muted-foreground text-xs">
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
                {t("probeLoading")}
              </span>
            ) : null}
          </div>
          {ragProbeFailed ? (
            <div className="flex flex-wrap items-center gap-2">
              <p className="text-destructive text-sm">{t("probeFailed")}</p>
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={ragProbeLoading}
                onClick={() => void onRetryProbe()}
              >
                {t("retryProbe")}
              </Button>
            </div>
          ) : null}
          <div className="overflow-x-auto rounded-md border">
            <table className="w-full min-w-[520px] text-left text-sm">
              <thead>
                <tr className="border-b bg-muted/40">
                  <th className="px-3 py-2 font-medium">{t("colExtension")}</th>
                  <th className="px-3 py-2 font-medium">{t("colStatus")}</th>
                  <th className="px-3 py-2 font-medium">{t("colNotes")}</th>
                </tr>
              </thead>
              <tbody>
                {RAG_OPTIONAL_EXTS.map((ext) => {
                  const e = ext.toLowerCase();
                  const ok = supportedSet.has(e);
                  const missing = unsupportedSet.has(e);
                  const hintKey =
                    ext === ".pdf"
                      ? "hintPdf"
                      : ext === ".docx" || ext === ".odt" || ext === ".epub"
                        ? "hintOfficeDocs"
                        : ext === ".xlsx"
                          ? "hintXlsx"
                          : "hintPptx";
                  let statusLabel: string;
                  if (ragProbeLoading && !ragLiveDeps) {
                    statusLabel = t("statusPending");
                  } else if (ok) {
                    statusLabel = t("statusActive");
                  } else if (missing) {
                    statusLabel = t("statusMissing");
                  } else if (ragProbeFailed) {
                    statusLabel = t("statusUnknown");
                  } else {
                    statusLabel = t("statusUnknown");
                  }
                  return (
                    <tr key={ext} className="border-b last:border-0">
                      <td className="px-3 py-2 font-mono text-xs">{ext}</td>
                      <td className="px-3 py-2">
                        <span
                          className={
                            ok
                              ? "text-emerald-600 dark:text-emerald-400"
                              : missing
                                ? "text-amber-700 dark:text-amber-500"
                                : "text-muted-foreground"
                          }
                        >
                          {statusLabel}
                        </span>
                      </td>
                      <td className="px-3 py-2 text-muted-foreground">{t(hintKey)}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          <p className="text-muted-foreground">{t("installHowTo")}</p>
        </div>
        {ragWarnings && ragWarnings.length > 0 ? (
          <ul className="list-disc pl-5 text-amber-700 dark:text-amber-500">
            {ragWarnings.map((w) => (
              <li key={w}>{w}</li>
            ))}
          </ul>
        ) : null}
        {pkgsForRagUnsupported(ragUnsupported).length > 0 ? (
          <Button
            type="button"
            variant="secondary"
            size="sm"
            disabled={installingRagPkgs || ragProbeLoading}
            onClick={() => void onInstallMissing()}
          >
            {installingRagPkgs ? t("installing") : t("installMissing")}
          </Button>
        ) : null}
      </div>
    </div>
  );
}
