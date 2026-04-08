import { useState, useEffect, useMemo } from "react";
import { Plus, Save, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { InfoLabel } from "@/components/shared/info-label";

/** Matches internal/config.ModelPricing JSON shape (telemetry.model_pricing). */
export interface ModelPricingRow {
  input_per_million: number;
  output_per_million: number;
  cache_read_per_million?: number;
  cache_create_per_million?: number;
}

interface TelemetryWithPricing {
  enabled?: boolean;
  endpoint?: string;
  protocol?: string;
  insecure?: boolean;
  service_name?: string;
  headers?: Record<string, string>;
  model_pricing?: Record<string, ModelPricingRow>;
}

interface DraftRow {
  modelKey: string;
  inputPerMillion: string;
  outputPerMillion: string;
  cacheReadPerMillion: string;
  cacheCreatePerMillion: string;
}

function emptyDraftRow(): DraftRow {
  return {
    modelKey: "",
    inputPerMillion: "",
    outputPerMillion: "",
    cacheReadPerMillion: "",
    cacheCreatePerMillion: "",
  };
}

function fromConfig(data: TelemetryWithPricing | undefined): DraftRow[] {
  const mp = data?.model_pricing;
  if (!mp || Object.keys(mp).length === 0) {
    return [emptyDraftRow()];
  }
  return Object.entries(mp).map(([modelKey, row]) => ({
    modelKey,
    inputPerMillion: row.input_per_million !== undefined ? String(row.input_per_million) : "",
    outputPerMillion: row.output_per_million !== undefined ? String(row.output_per_million) : "",
    cacheReadPerMillion:
      row.cache_read_per_million !== undefined ? String(row.cache_read_per_million) : "",
    cacheCreatePerMillion:
      row.cache_create_per_million !== undefined ? String(row.cache_create_per_million) : "",
  }));
}

function parseNonNeg(s: string): number | undefined {
  const t = s.trim();
  if (t === "") return undefined;
  const n = Number(t);
  if (!Number.isFinite(n) || n < 0) return undefined;
  return n;
}

function buildPricingMap(rows: DraftRow[]): Record<string, ModelPricingRow> | undefined {
  const out: Record<string, ModelPricingRow> = {};
  for (const r of rows) {
    const key = r.modelKey.trim();
    if (!key) continue;
    const input = parseNonNeg(r.inputPerMillion);
    const output = parseNonNeg(r.outputPerMillion);
    if (input === undefined || output === undefined) continue;
    const row: ModelPricingRow = {
      input_per_million: input,
      output_per_million: output,
    };
    const cr = parseNonNeg(r.cacheReadPerMillion);
    const cc = parseNonNeg(r.cacheCreatePerMillion);
    if (cr !== undefined) row.cache_read_per_million = cr;
    if (cc !== undefined) row.cache_create_per_million = cc;
    out[key] = row;
  }
  return Object.keys(out).length > 0 ? out : undefined;
}

interface Props {
  data: TelemetryWithPricing | undefined;
  onSave: (value: TelemetryWithPricing) => Promise<void>;
  saving: boolean;
}

export function ModelPricingSection({ data, onSave, saving }: Props) {
  const { t } = useTranslation("config");
  const base = data ?? {};
  const [rows, setRows] = useState<DraftRow[]>(() => fromConfig(base));
  const [dirty, setDirty] = useState(false);

  useEffect(() => {
    setRows(fromConfig(data ?? {}));
    setDirty(false);
  }, [data]);

  const canSave = useMemo(() => {
    for (const r of rows) {
      const key = r.modelKey.trim();
      if (!key) continue;
      const input = parseNonNeg(r.inputPerMillion);
      const output = parseNonNeg(r.outputPerMillion);
      if (input === undefined || output === undefined) return false;
      const cr = r.cacheReadPerMillion.trim();
      const cc = r.cacheCreatePerMillion.trim();
      if (cr !== "" && parseNonNeg(cr) === undefined) return false;
      if (cc !== "" && parseNonNeg(cc) === undefined) return false;
    }
    return true;
  }, [rows]);

  const updateRow = (idx: number, patch: Partial<DraftRow>) => {
    setRows((prev) => prev.map((row, i) => (i === idx ? { ...row, ...patch } : row)));
    setDirty(true);
  };

  const addRow = () => {
    setRows((prev) => [...prev, emptyDraftRow()]);
    setDirty(true);
  };

  const removeRow = (idx: number) => {
    setRows((prev) => {
      const next = prev.filter((_, i) => i !== idx);
      return next.length > 0 ? next : [emptyDraftRow()];
    });
    setDirty(true);
  };

  const handleSave = async () => {
    const map = buildPricingMap(rows);
    await onSave({ ...base, model_pricing: map });
    setDirty(false);
  };

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-base">{t("modelPricing.title")}</CardTitle>
        <CardDescription>{t("modelPricing.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <p className="text-sm text-muted-foreground">{t("modelPricing.keyHint")}</p>

        <div className="overflow-x-auto rounded-md border">
          <table className="min-w-[640px] w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/40 text-left">
                <th className="px-3 py-2 font-medium">{t("modelPricing.modelKey")}</th>
                <th className="px-3 py-2 font-medium">
                  <InfoLabel tip={t("modelPricing.inputTip")}>{t("modelPricing.inputPerMillion")}</InfoLabel>
                </th>
                <th className="px-3 py-2 font-medium">
                  <InfoLabel tip={t("modelPricing.outputTip")}>{t("modelPricing.outputPerMillion")}</InfoLabel>
                </th>
                <th className="px-3 py-2 font-medium">
                  <InfoLabel tip={t("modelPricing.cacheReadTip")}>{t("modelPricing.cacheReadPerMillion")}</InfoLabel>
                </th>
                <th className="px-3 py-2 font-medium">
                  <InfoLabel tip={t("modelPricing.cacheCreateTip")}>{t("modelPricing.cacheCreatePerMillion")}</InfoLabel>
                </th>
                <th className="w-10 px-2 py-2" aria-hidden />
              </tr>
            </thead>
            <tbody>
              {rows.map((row, idx) => (
                <tr key={idx} className="border-b border-border/60 last:border-0">
                  <td className="px-2 py-2 align-middle">
                    <Input
                      className="text-base md:text-sm font-mono"
                      value={row.modelKey}
                      onChange={(e) => updateRow(idx, { modelKey: e.target.value })}
                      placeholder={t("modelPricing.modelKeyPlaceholder")}
                    />
                  </td>
                  <td className="px-2 py-2 align-middle">
                    <Input
                      className="text-base md:text-sm"
                      inputMode="decimal"
                      value={row.inputPerMillion}
                      onChange={(e) => updateRow(idx, { inputPerMillion: e.target.value })}
                      placeholder="0"
                    />
                  </td>
                  <td className="px-2 py-2 align-middle">
                    <Input
                      className="text-base md:text-sm"
                      inputMode="decimal"
                      value={row.outputPerMillion}
                      onChange={(e) => updateRow(idx, { outputPerMillion: e.target.value })}
                      placeholder="0"
                    />
                  </td>
                  <td className="px-2 py-2 align-middle">
                    <Input
                      className="text-base md:text-sm"
                      inputMode="decimal"
                      value={row.cacheReadPerMillion}
                      onChange={(e) => updateRow(idx, { cacheReadPerMillion: e.target.value })}
                      placeholder="—"
                    />
                  </td>
                  <td className="px-2 py-2 align-middle">
                    <Input
                      className="text-base md:text-sm"
                      inputMode="decimal"
                      value={row.cacheCreatePerMillion}
                      onChange={(e) => updateRow(idx, { cacheCreatePerMillion: e.target.value })}
                      placeholder="—"
                    />
                  </td>
                  <td className="px-1 py-2 align-middle">
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="h-9 w-9 text-muted-foreground hover:text-destructive"
                      onClick={() => removeRow(idx)}
                      aria-label={t("modelPricing.removeRow")}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <div className="flex flex-wrap items-center justify-between gap-2">
          <Button type="button" variant="outline" size="sm" onClick={addRow} className="gap-1">
            <Plus className="h-3.5 w-3.5" /> {t("modelPricing.addModel")}
          </Button>
          {dirty && (
            <Button type="button" size="sm" onClick={handleSave} disabled={saving || !canSave} className="gap-1.5">
              <Save className="h-3.5 w-3.5" /> {saving ? t("saving") : t("save")}
            </Button>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
