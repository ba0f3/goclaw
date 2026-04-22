import { useTranslation } from "react-i18next";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";

interface CursorCLISectionProps {
  mode: string;
  onModeChange: (v: string) => void;
  apiKey: string;
  onApiKeyChange: (v: string) => void;
}

export function CursorCLISection({ mode, onModeChange, apiKey, onApiKeyChange }: CursorCLISectionProps) {
  const { t } = useTranslation("providers");

  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">
        {t("cursor.description")} <code className="rounded bg-muted px-1 py-0.5">agent</code> {t("cursor.descriptionSuffix")}
      </p>

      <div className="space-y-2">
        <Label>{t("cursor.mode")}</Label>
        <Select value={mode} onValueChange={onModeChange}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="agent">{t("cursor.modeAgent")}</SelectItem>
            <SelectItem value="plan">{t("cursor.modePlan")}</SelectItem>
            <SelectItem value="ask">{t("cursor.modeAsk")}</SelectItem>
          </SelectContent>
        </Select>
        <p className="text-xs text-muted-foreground">{t("cursor.modeHint")}</p>
      </div>

      <div className="space-y-2">
        <Label htmlFor="cursorApiKey">{t("cursor.apiKey")}</Label>
        <Input
          id="cursorApiKey"
          type="password"
          value={apiKey}
          onChange={(e) => onApiKeyChange(e.target.value)}
          placeholder={t("cursor.apiKeyPlaceholder")}
          className="text-base md:text-sm"
        />
        <p className="text-xs text-muted-foreground">{t("cursor.apiKeyHint")}</p>
      </div>
    </div>
  );
}
