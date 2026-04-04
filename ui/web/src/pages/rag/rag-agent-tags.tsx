import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { ChevronDown, X } from "lucide-react";
import { Popover } from "radix-ui";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type { AgentData } from "@/types/agent";
import { cn } from "@/lib/utils";

interface RagAgentTagsProps {
  agents: AgentData[];
  disabled?: boolean;
  selectedIds: string[];
  onSelectedIdsChange: (ids: string[]) => void;
}

export function RagAgentTags({
  agents,
  disabled,
  selectedIds,
  onSelectedIdsChange,
}: RagAgentTagsProps) {
  const { t } = useTranslation("rag");
  const [pickerOpen, setPickerOpen] = useState(false);

  const selectedSet = useMemo(() => new Set(selectedIds), [selectedIds]);
  const selectedAgents = useMemo(
    () => selectedIds.map((id) => agents.find((a) => a.id === id)).filter(Boolean) as AgentData[],
    [agents, selectedIds],
  );

  const availableToAdd = useMemo(
    () => agents.filter((a) => !selectedSet.has(a.id)),
    [agents, selectedSet],
  );

  const addAgent = (id: string) => {
    if (selectedSet.has(id)) return;
    onSelectedIdsChange([...selectedIds, id]);
    setPickerOpen(false);
  };

  const removeAgent = (id: string) => {
    onSelectedIdsChange(selectedIds.filter((x) => x !== id));
  };

  const clearToAll = () => {
    onSelectedIdsChange([]);
  };

  return (
    <div className="space-y-2">
      <div
        className={cn(
          "flex min-h-10 flex-wrap items-center gap-2 rounded-md border bg-background p-2",
          disabled && "pointer-events-none opacity-50",
        )}
      >
        {selectedIds.length === 0 ? (
          <Badge variant="secondary" className="font-normal">
            {t("allAgentsBadge")}
          </Badge>
        ) : (
          selectedAgents.map((a) => (
            <Badge
              key={a.id}
              variant="secondary"
              className="max-w-full gap-0.5 pr-0.5 font-normal"
            >
              <span className="truncate">{a.display_name || a.agent_key}</span>
              <button
                type="button"
                className="rounded-full p-0.5 hover:bg-muted-foreground/20 focus:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                aria-label={t("removeAgentAria", { name: a.display_name || a.agent_key })}
                onClick={() => removeAgent(a.id)}
              >
                <X className="h-3.5 w-3.5 shrink-0 opacity-70" />
              </button>
            </Badge>
          ))
        )}

        {agents.length > 0 && availableToAdd.length > 0 ? (
          <Popover.Root open={pickerOpen} onOpenChange={setPickerOpen}>
            <Popover.Trigger asChild>
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="h-7 gap-1 text-xs"
                disabled={disabled}
              >
                {t("addAgent")}
                <ChevronDown className="h-3.5 w-3.5 opacity-60" />
              </Button>
            </Popover.Trigger>
            <Popover.Portal>
              <Popover.Content
                align="start"
                sideOffset={6}
                className="z-50 max-h-60 w-[min(100vw-2rem,280px)] overflow-y-auto rounded-md border bg-popover p-1 text-popover-foreground shadow-md pointer-events-auto animate-in fade-in-0 zoom-in-95"
              >
                {availableToAdd.map((a) => (
                  <button
                    key={a.id}
                    type="button"
                    className="flex w-full cursor-pointer items-center rounded-sm px-2 py-1.5 text-left text-sm hover:bg-accent"
                    onClick={() => addAgent(a.id)}
                  >
                    <span className="truncate">{a.display_name || a.agent_key}</span>
                  </button>
                ))}
              </Popover.Content>
            </Popover.Portal>
          </Popover.Root>
        ) : null}

        {selectedIds.length > 0 ? (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-7 text-xs text-muted-foreground"
            disabled={disabled}
            onClick={clearToAll}
          >
            {t("useAllAgents")}
          </Button>
        ) : null}
      </div>
      <p className="text-xs text-muted-foreground">{t("agentTagsHint")}</p>
    </div>
  );
}
