import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQueryClient } from "@tanstack/react-query";
import { Save, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { PageHeader } from "@/components/shared/page-header";
import { useAgents } from "@/pages/agents/hooks/use-agents";
import { useHttp } from "@/hooks/use-ws";
import { queryKeys } from "@/lib/query-keys";
import { toast } from "@/stores/use-toast-store";
import { userFriendlyError } from "@/lib/error-utils";
import { LOCAL_STORAGE_KEYS } from "@/lib/constants";
import type { AgentData, RAGDepsResponse, RAGIndexingConfig, AgentUpdateResponse } from "@/types/agent";
import { RagIndexingPanel, pkgsForRagUnsupported } from "./rag-indexing-panel";
import { RagAgentTags } from "./rag-agent-tags";

function ragEnabledFromAgent(a: AgentData): boolean {
  const ri = ((a.other_config ?? {}) as Record<string, unknown>).rag_indexing;
  if (!ri || typeof ri !== "object" || Array.isArray(ri)) return false;
  return (ri as RAGIndexingConfig).enabled === true;
}

function buildOtherConfigWithRag(agent: AgentData, enabled: boolean): Record<string, unknown> {
  const raw = agent.other_config;
  const base =
    raw && typeof raw === "object" && !Array.isArray(raw)
      ? ({ ...raw } as Record<string, unknown>)
      : {};
  base.rag_indexing = { enabled };
  return base;
}

export function RagPage() {
  const { t } = useTranslation("rag");
  const http = useHttp();
  const queryClient = useQueryClient();
  const { agents, loading: agentsLoading } = useAgents();

  /** Empty = all agents (default). Non-empty = only listed agent IDs. Persisted in localStorage. */
  const [selectedAgentIds, setSelectedAgentIds] = useState<string[]>([]);
  const [ragScopeHydrated, setRagScopeHydrated] = useState(false);
  const [ragIndexingEnabled, setRagIndexingEnabled] = useState(false);
  const [ragLiveDeps, setRagLiveDeps] = useState<RAGDepsResponse | null>(null);
  const [ragProbeLoading, setRagProbeLoading] = useState(false);
  const [ragProbeFailed, setRagProbeFailed] = useState(false);
  const [installingRagPkgs, setInstallingRagPkgs] = useState(false);
  const [saving, setSaving] = useState(false);
  const [scopeMixed, setScopeMixed] = useState(false);

  const scopedAgents = useMemo(() => {
    if (selectedAgentIds.length === 0) return agents;
    const set = new Set(selectedAgentIds);
    return agents.filter((a) => set.has(a.id));
  }, [agents, selectedAgentIds]);

  /** Single agent in scope (for cached supported_types preview). */
  const singleScopedAgent = useMemo(
    () => (scopedAgents.length === 1 ? scopedAgents[0] : undefined),
    [scopedAgents],
  );

  // Restore agent scope from localStorage once the agent list is available (empty stored = all agents).
  useEffect(() => {
    if (agents.length === 0 || ragScopeHydrated) return;
    try {
      const raw = localStorage.getItem(LOCAL_STORAGE_KEYS.RAG_SELECTED_AGENT_IDS);
      if (raw != null) {
        const parsed: unknown = JSON.parse(raw);
        if (Array.isArray(parsed) && parsed.every((x) => typeof x === "string")) {
          const valid = new Set(agents.map((a) => a.id));
          setSelectedAgentIds(parsed.filter((id) => valid.has(id)));
        }
      }
    } catch {
      // ignore corrupt storage
    }
    setRagScopeHydrated(true);
  }, [agents, ragScopeHydrated]);

  // Drop stale IDs when agent list changes. Wait until scope is hydrated so we do not overwrite localStorage restore.
  useEffect(() => {
    if (agents.length === 0 || !ragScopeHydrated) return;
    const valid = new Set(agents.map((a) => a.id));
    setSelectedAgentIds((prev) => {
      const filtered = prev.filter((id) => valid.has(id));
      return filtered.length === prev.length ? prev : filtered;
    });
  }, [agents, ragScopeHydrated]);

  useEffect(() => {
    if (!ragScopeHydrated) return;
    try {
      localStorage.setItem(LOCAL_STORAGE_KEYS.RAG_SELECTED_AGENT_IDS, JSON.stringify(selectedAgentIds));
    } catch {
      // ignore quota / private mode
    }
  }, [selectedAgentIds, ragScopeHydrated]);

  const syncEnabledFromScope = useCallback(() => {
    if (scopedAgents.length === 0) {
      setRagIndexingEnabled(false);
      setScopeMixed(false);
      return;
    }
    const vals = scopedAgents.map(ragEnabledFromAgent);
    const allOn = vals.every(Boolean);
    const allOff = vals.every((v) => !v);
    setScopeMixed(!allOn && !allOff);
    setRagIndexingEnabled(allOn);
  }, [scopedAgents]);

  useEffect(() => {
    syncEnabledFromScope();
  }, [syncEnabledFromScope]);

  useEffect(() => {
    let cancelled = false;
    setRagProbeLoading(true);
    setRagProbeFailed(false);
    void (async () => {
      try {
        const data = await http.get<RAGDepsResponse>("/v1/agents/rag-deps");
        if (!cancelled) setRagLiveDeps(data);
      } catch {
        if (!cancelled) setRagProbeFailed(true);
      } finally {
        if (!cancelled) setRagProbeLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [http]);

  const refreshRagDeps = async () => {
    setRagProbeLoading(true);
    setRagProbeFailed(false);
    try {
      const data = await http.get<RAGDepsResponse>("/v1/agents/rag-deps");
      setRagLiveDeps(data);
    } catch {
      setRagProbeFailed(true);
    } finally {
      setRagProbeLoading(false);
    }
  };

  const handleInstallRagMissing = async () => {
    const pkgs = pkgsForRagUnsupported(ragLiveDeps?.unsupported);
    if (pkgs.length === 0) return;
    setInstallingRagPkgs(true);
    try {
      for (const pkg of pkgs) {
        const r = await http.post<{ ok: boolean; error?: string }>("/v1/packages/install", { package: pkg });
        if (!r.ok) {
          toast.error(pkg + (r.error ? `: ${r.error}` : ""));
          return;
        }
      }
      await refreshRagDeps();
      toast.success(t("installHint"));
    } catch (err) {
      toast.error(t("installMissing"), userFriendlyError(err));
    } finally {
      setInstallingRagPkgs(false);
    }
  };

  const ragCachedSupported = singleScopedAgent
    ? (singleScopedAgent.other_config?.rag_indexing as RAGIndexingConfig | undefined)?.supported_types
    : undefined;

  const handleSave = async () => {
    const targets = scopedAgents;
    if (targets.length === 0) {
      toast.error(t("noAgents"));
      return;
    }
    setSaving(true);
    try {
      const settled = await Promise.allSettled(
        targets.map(async (a) => {
          const res = await http.put<AgentUpdateResponse>(`/v1/agents/${a.id}`, {
            other_config: buildOtherConfigWithRag(a, ragIndexingEnabled),
          });
          return { agent: a, res };
        }),
      );

      const successes: Array<{ agent: AgentData; res: AgentUpdateResponse }> = [];
      const failures: Array<{ agent: AgentData; error: unknown }> = [];
      settled.forEach((s, i) => {
        const agent = targets[i];
        if (!agent) return;
        if (s.status === "fulfilled") {
          successes.push(s.value);
        } else {
          failures.push({ agent, error: s.reason });
        }
      });

      // Best-effort refresh.
      const ragDepsList = successes.map((s) => s.res?.rag_deps).filter(Boolean) as RAGDepsResponse[];
      const lastRagDeps = ragDepsList.length > 0 ? ragDepsList[ragDepsList.length - 1] : undefined;
      if (lastRagDeps) setRagLiveDeps(lastRagDeps);

      await queryClient.invalidateQueries({ queryKey: queryKeys.agents.all });

      if (failures.length > 0) {
        const names = failures
          .map((f) => f.agent.display_name || f.agent.agent_key || f.agent.id)
          .filter(Boolean)
          .slice(0, 3)
          .join(", ");
        const more = failures.length > 3 ? ` +${failures.length - 3}` : "";
        toast.error(
          t("saveFailed"),
          `${successes.length}/${targets.length} updated. Failed: ${names}${more}. ${userFriendlyError(failures[0]!.error)}`,
        );
        return;
      }

      if (selectedAgentIds.length === 0) {
        toast.success(t("savedAll", { count: targets.length }));
      } else if (targets.length === 1 && targets[0]) {
        const one = targets[0];
        toast.success(t("savedOne", { name: one.display_name || one.agent_key || "" }));
      } else {
        toast.success(t("savedMany", { count: targets.length }));
      }

      setScopeMixed(false);
    } catch (err) {
      await queryClient.invalidateQueries({ queryKey: queryKeys.agents.all });
      toast.error(t("saveFailed"), userFriendlyError(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="p-4 sm:p-6 pb-10">
      <PageHeader title={t("pageTitle")} description={t("pageDescription")} />

      <div className="mt-6 max-w-3xl space-y-6">
        <div className="flex items-center justify-between gap-4 rounded-lg border p-4">
          <div className="space-y-1 min-w-0">
            <Label htmlFor="rag-indexing-enabled" className="text-base font-medium">
              {t("enabled")}
            </Label>
            <p className="text-sm text-muted-foreground">{t("toggleHint")}</p>
          </div>
          <Switch
            id="rag-indexing-enabled"
            className="shrink-0"
            checked={ragIndexingEnabled}
            onCheckedChange={setRagIndexingEnabled}
          />
        </div>

        <div className="grid gap-1.5">
          <Label className="text-xs">{t("agentScope")}</Label>
          <RagAgentTags
            agents={agents}
            disabled={agentsLoading || agents.length === 0}
            selectedIds={selectedAgentIds}
            onSelectedIdsChange={setSelectedAgentIds}
          />
          {scopeMixed ? (
            <p className="text-sm text-amber-700 dark:text-amber-500">{t("mixedAgentsHint")}</p>
          ) : null}
        </div>

        <RagIndexingPanel
          ragLiveDeps={ragLiveDeps}
          ragProbeLoading={ragProbeLoading}
          ragProbeFailed={ragProbeFailed}
          onRetryProbe={refreshRagDeps}
          onInstallMissing={handleInstallRagMissing}
          installingRagPkgs={installingRagPkgs}
          ragCachedSupported={ragCachedSupported}
        />

        <div className="flex justify-end">
          <Button onClick={() => void handleSave()} disabled={saving || agents.length === 0}>
            {saving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
            {saving ? t("saving") : t("save")}
          </Button>
        </div>
      </div>
    </div>
  );
}
