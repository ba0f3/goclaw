import { useState, useEffect } from "react";
import { useTranslation } from "react-i18next";
import {
  Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select";
import { KeyValueEditor } from "@/components/shared/key-value-editor";
import type { SecureCLIBinary, CLICredentialInput, CLIPreset } from "./hooks/use-cli-credentials";

/** Env var keys whose values should be masked (type="password"). */
const SENSITIVE_ENV_RE = /^.*(key|secret|token|password|credential).*$/i;
const isSensitiveEnvKey = (key: string) => SENSITIVE_ENV_RE.test(key.trim());

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  credential?: SecureCLIBinary | null;
  presets: Record<string, CLIPreset>;
  onSubmit: (data: CLICredentialInput) => Promise<unknown>;
}

const NONE_PRESET = "__none__";

function nonEmptyEnvPayload(envValues: Record<string, string>): Record<string, string> {
  const out: Record<string, string> = {};
  for (const [k, v] of Object.entries(envValues)) {
    const kt = k.trim();
    if (!kt) continue;
    const vt = v.trim();
    if (vt) out[kt] = vt;
  }
  return out;
}

export function CliCredentialFormDialog({ open, onOpenChange, credential, presets, onSubmit }: Props) {
  const { t } = useTranslation("cli-credentials");
  const { t: tc } = useTranslation("common");

  const [selectedPreset, setSelectedPreset] = useState(NONE_PRESET);
  const [binaryName, setBinaryName] = useState("");
  const [binaryPath, setBinaryPath] = useState("");
  const [description, setDescription] = useState("");
  const [denyArgs, setDenyArgs] = useState("");
  const [denyVerbose, setDenyVerbose] = useState("");
  const [timeout, setTimeout] = useState(30);
  const [tips, setTips] = useState("");
  const [agentId, setAgentId] = useState("");
  const [enabled, setEnabled] = useState(true);
  const [envValues, setEnvValues] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const isEdit = !!credential;
  // Build typed entry list to avoid noUncheckedIndexedAccess issues
  const presetEntries: Array<[string, CLIPreset]> = Object.entries(presets).filter(
    (e): e is [string, CLIPreset] => e[1] !== undefined,
  );

  // Current preset definition (for env var fields)
  const activePreset: CLIPreset | null =
    selectedPreset !== NONE_PRESET ? (presets[selectedPreset] ?? null) : null;
  const showPresetEnvFields = activePreset !== null && activePreset.env_vars.length > 0;
  const showCustomEnvEditor = !showPresetEnvFields;

  useEffect(() => {
    if (!open) return;
    setSelectedPreset(NONE_PRESET);
    setBinaryName(credential?.binary_name ?? "");
    setBinaryPath(credential?.binary_path ?? "");
    setDescription(credential?.description ?? "");
    setDenyArgs((credential?.deny_args ?? []).join(", "));
    setDenyVerbose((credential?.deny_verbose ?? []).join(", "));
    setTimeout(credential?.timeout_seconds ?? 30);
    setTips(credential?.tips ?? "");
    setAgentId(credential?.agent_id ?? "");
    setEnabled(credential?.enabled ?? true);
    const ek = credential?.env_keys;
    if (ek && ek.length > 0) {
      setEnvValues(Object.fromEntries(ek.map((k) => [k, ""])));
    } else {
      setEnvValues({});
    }
    setError("");
  }, [open, credential]);

  const applyPreset = (key: string) => {
    setSelectedPreset(key);
    if (key === NONE_PRESET) {
      setEnvValues({});
      return;
    }
    const p = presets[key];
    if (!p) return;
    setBinaryName(p.binary_name);
    setDescription(p.description);
    setDenyArgs(p.deny_args.join(", "));
    setDenyVerbose(p.deny_verbose.join(", "));
    setTimeout(p.timeout);
    setTips(p.tips);
    setEnvValues({});
  };

  const splitCommaList = (v: string): string[] =>
    v.split(",").map((s) => s.trim()).filter(Boolean);

  const handleSubmit = async () => {
    if (!binaryName.trim()) {
      setError(t("form.binaryNameRequired"));
      return;
    }
    const envPayload = nonEmptyEnvPayload(envValues);
    if (!isEdit && Object.keys(envPayload).length === 0) {
      setError(t("form.envRequired"));
      return;
    }
    setLoading(true);
    setError("");
    try {
      const payload: CLICredentialInput = {
        binary_name: binaryName.trim(),
        binary_path: binaryPath.trim() || undefined,
        description: description.trim(),
        deny_args: splitCommaList(denyArgs),
        deny_verbose: splitCommaList(denyVerbose),
        timeout_seconds: timeout,
        tips: tips.trim(),
        agent_id: agentId.trim() || undefined,
        enabled,
      };
      if (selectedPreset !== NONE_PRESET) payload.preset = selectedPreset;
      if (isEdit && credential) {
        if (Object.keys(envPayload).length > 0) {
          payload.env = envPayload;
        }
        const initialKeys = new Set(credential.env_keys ?? []);
        const currentKeys = new Set(
          Object.keys(envValues)
            .map((k) => k.trim())
            .filter(Boolean),
        );
        const removed = [...initialKeys].filter((k) => !currentKeys.has(k));
        const removeFiltered = removed.filter((k) => !(k in envPayload));
        if (removeFiltered.length > 0) {
          payload.env_remove = removeFiltered;
        }
      } else {
        payload.env = envPayload;
      }
      await onSubmit(payload);
      onOpenChange(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("form.failedToSave"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(v) => !loading && onOpenChange(v)}>
      <DialogContent className="max-h-[85vh] flex flex-col sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{isEdit ? t("form.editTitle") : t("form.createTitle")}</DialogTitle>
        </DialogHeader>

        <div className="grid gap-4 py-2 -mx-4 px-4 sm:-mx-6 sm:px-6 overflow-y-auto min-h-0">
          {/* Preset selector — only on create */}
          {!isEdit && presetEntries.length > 0 && (
            <div className="grid gap-1.5">
              <Label>{t("form.preset")}</Label>
              <Select value={selectedPreset} onValueChange={applyPreset}>
                <SelectTrigger className="text-base md:text-sm">
                  <SelectValue placeholder={t("form.presetPlaceholder")} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={NONE_PRESET}>{t("form.noPreset")}</SelectItem>
                  {presetEntries.map(([k, p]) => (
                    <SelectItem key={k} value={k}>
                      {p.binary_name} — {p.description}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">
                {t("form.presetHint")}
              </p>
            </div>
          )}

          {/* Existing credentials indicator (edit mode) */}
          {isEdit && (
            <p className="text-xs text-muted-foreground rounded-md border border-dashed p-2">
              {t("form.encryptedHint")}
            </p>
          )}

          {/* Env vars: preset-defined fields, or free-form key/value for manual / edit */}
          {showPresetEnvFields && activePreset && (
            <div className="grid gap-3 rounded-md border p-3">
              <p className="text-sm font-medium">{t("form.envVars")}</p>
              {activePreset.env_vars.map((ev) => (
                <div key={ev.name} className="grid gap-1.5">
                  <Label htmlFor={`env-${ev.name}`}>
                    {ev.name}
                    {ev.optional && <span className="ml-1 text-xs text-muted-foreground">({tc("optional")})</span>}
                  </Label>
                  <Input
                    id={`env-${ev.name}`}
                    type="password"
                    autoComplete="off"
                    placeholder={ev.desc}
                    value={envValues[ev.name] ?? ""}
                    onChange={(e) => setEnvValues((prev) => ({ ...prev, [ev.name]: e.target.value }))}
                    className="text-base md:text-sm"
                  />
                  {ev.desc && (
                    <p className="text-xs text-muted-foreground">{ev.desc}</p>
                  )}
                </div>
              ))}
            </div>
          )}
          {showCustomEnvEditor && (
            <div className="grid gap-2 rounded-md border p-3">
              <p className="text-sm font-medium">{t("form.envVars")}</p>
              <KeyValueEditor
                value={envValues}
                onChange={setEnvValues}
                keyPlaceholder={t("placeholders.envKey")}
                valuePlaceholder={t("placeholders.envValue")}
                valuePlaceholderExisting={isEdit ? t("placeholders.envValueUnchanged") : undefined}
                existingKeys={isEdit ? credential?.env_keys : undefined}
                addLabel={t("form.addEnvVar")}
                maskValue={isSensitiveEnvKey}
              />
            </div>
          )}

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div className="grid gap-1.5">
              <Label htmlFor="cc-name">{t("form.binaryName")}</Label>
              <Input
                id="cc-name"
                value={binaryName}
                onChange={(e) => setBinaryName(e.target.value)}
                placeholder={t("placeholders.binaryName")}
                className="text-base md:text-sm"
              />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="cc-path">{t("form.binaryPath")} <span className="text-xs text-muted-foreground">({tc("optional")})</span></Label>
              <Input
                id="cc-path"
                value={binaryPath}
                onChange={(e) => setBinaryPath(e.target.value)}
                placeholder={t("placeholders.binaryPath")}
                className="text-base md:text-sm"
              />
            </div>
          </div>

          <div className="grid gap-1.5">
            <Label htmlFor="cc-desc">{tc("description")}</Label>
            <Textarea
              id="cc-desc"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder={t("placeholders.description")}
              rows={2}
              className="text-base md:text-sm"
            />
          </div>

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div className="grid gap-1.5">
              <Label htmlFor="cc-deny-args">{t("form.denyArgs")} <span className="text-xs text-muted-foreground">({t("form.commaSeparated")})</span></Label>
              <Input
                id="cc-deny-args"
                value={denyArgs}
                onChange={(e) => setDenyArgs(e.target.value)}
                placeholder={t("placeholders.denyArgs")}
                className="text-base md:text-sm"
              />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="cc-timeout">{t("form.timeout")}</Label>
              <Input
                id="cc-timeout"
                type="number"
                min={1}
                value={timeout}
                onChange={(e) => setTimeout(Number(e.target.value))}
                className="text-base md:text-sm"
              />
            </div>
          </div>

          <div className="grid gap-1.5">
            <Label htmlFor="cc-deny-verbose">{t("form.denyVerbose")} <span className="text-xs text-muted-foreground">({t("form.commaSeparated")})</span></Label>
            <Input
              id="cc-deny-verbose"
              value={denyVerbose}
              onChange={(e) => setDenyVerbose(e.target.value)}
              placeholder={t("placeholders.denyVerbose")}
              className="text-base md:text-sm"
            />
          </div>

          <div className="grid gap-1.5">
            <Label htmlFor="cc-tips">{t("form.tips")}</Label>
            <Textarea
              id="cc-tips"
              value={tips}
              onChange={(e) => setTips(e.target.value)}
              placeholder={t("placeholders.tips")}
              rows={2}
              className="text-base md:text-sm"
            />
          </div>

          <div className="grid gap-1.5">
            <Label htmlFor="cc-agent">{t("form.agentId")} <span className="text-xs text-muted-foreground">({t("form.agentIdHint")})</span></Label>
            <Input
              id="cc-agent"
              value={agentId}
              onChange={(e) => setAgentId(e.target.value)}
              placeholder={t("placeholders.agentId")}
              className="text-base md:text-sm"
            />
          </div>

          <div className="flex items-center gap-2">
            <Switch id="cc-enabled" checked={enabled} onCheckedChange={setEnabled} />
            <Label htmlFor="cc-enabled">{tc("enabled")}</Label>
          </div>

          {error && <p className="text-sm text-destructive">{error}</p>}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={loading}>{tc("cancel")}</Button>
          <Button onClick={handleSubmit} disabled={loading}>
            {loading ? tc("saving") : isEdit ? tc("update") : tc("create")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
