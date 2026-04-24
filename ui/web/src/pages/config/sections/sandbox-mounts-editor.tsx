import { useTranslation } from "react-i18next";
import { Plus, Trash2 } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { InfoLabel } from "@/components/shared/info-label";
import type { SandboxConfig, ExtraMount } from "@/types/agent";

interface Props {
  value: SandboxConfig;
  onChange: (patch: Partial<SandboxConfig>) => void;
  disabled?: boolean;
}

export function SandboxMountsEditor({ value, onChange, disabled }: Props) {
  const { t } = useTranslation(["config", "agents"]);
  const s = "agents.sandbox"; // Fallback to config.json's agents.sandbox prefix for simplicity
  
  const readOnlyPaths = value.read_only_host_paths ?? [];
  const extraMounts = value.extra_mounts ?? [];

  const handleReadOnlyPathsChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const val = e.target.value;
    const paths = val.split("\n").map((p) => p.trim()).filter(Boolean);
    onChange({ read_only_host_paths: paths });
  };

  const addMount = () => {
    onChange({
      extra_mounts: [...extraMounts, { host_path: "", container_path: "", access: "ro" }],
    });
  };

  const updateMount = (
    index: number,
    patch: Partial<Pick<ExtraMount, "host_path" | "container_path" | "access">>,
  ) => {
    const updated = [...extraMounts];
    const current = updated[index];
    if (!current) return;
    updated[index] = { ...current, ...patch };
    onChange({ extra_mounts: updated });
  };

  const removeMount = (index: number) => {
    const updated = [...extraMounts];
    updated.splice(index, 1);
    onChange({ extra_mounts: updated });
  };

  return (
    <div className="col-span-1 sm:col-span-2 space-y-6 pt-2 border-t mt-2">
      <div className="space-y-2">
        <InfoLabel tip={t(`${s}.readOnlyHostPathsTip`, { defaultValue: "Directories from the host to mirror into the sandbox as read-only. Useful for ~/.aws or ~/.kube." })}>
          {t(`${s}.readOnlyHostPaths`, { defaultValue: "Read-Only Host Paths" })}
        </InfoLabel>
        <Textarea
          placeholder="/home/user/.aws&#10;/home/user/.kube"
          value={readOnlyPaths.join("\n")}
          onChange={handleReadOnlyPathsChange}
          disabled={disabled}
          className={`min-h-[80px] font-mono text-xs ${disabled ? "opacity-60" : ""}`}
        />
      </div>

      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <InfoLabel tip={t(`${s}.extraMountsTip`, { defaultValue: "Additional custom bind mounts to expose host directories inside the sandbox." })}>
            {t(`${s}.extraMounts`, { defaultValue: "Extra Mounts" })}
          </InfoLabel>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={addMount}
            disabled={disabled}
            className="h-7 text-xs"
          >
            <Plus className="mr-1 h-3 w-3" />
            {t("add", { ns: "common", defaultValue: "Add Mount" })}
          </Button>
        </div>

        {extraMounts.length === 0 ? (
          <div className="text-sm text-muted-foreground italic px-2">
            {t(`${s}.noExtraMounts`, { defaultValue: "No extra mounts configured." })}
          </div>
        ) : (
          <div className="space-y-2">
            {extraMounts.map((mount, idx) => (
              <div key={idx} className="flex items-center gap-2">
                <Input
                  placeholder="Host Path"
                  value={mount.host_path}
                  onChange={(e) => updateMount(idx, { host_path: e.target.value })}
                  disabled={disabled}
                  className="font-mono text-xs"
                />
                <span className="text-muted-foreground text-sm">→</span>
                <Input
                  placeholder="Container Path"
                  value={mount.container_path}
                  onChange={(e) => updateMount(idx, { container_path: e.target.value })}
                  disabled={disabled}
                  className="font-mono text-xs"
                />
                <Select
                  value={mount.access}
                  onValueChange={(val: "ro" | "rw" | "none") => updateMount(idx, { access: val })}
                  disabled={disabled}
                >
                  <SelectTrigger className="w-[100px]">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="ro">RO</SelectItem>
                    <SelectItem value="rw">RW</SelectItem>
                    <SelectItem value="none">None</SelectItem>
                  </SelectContent>
                </Select>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="h-9 w-9 text-destructive hover:bg-destructive/10 hover:text-destructive shrink-0"
                  onClick={() => removeMount(idx)}
                  disabled={disabled}
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
