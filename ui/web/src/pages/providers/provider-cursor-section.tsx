import { useCallback, useEffect, useState } from "react";
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
import { Button } from "@/components/ui/button";
import { useHttp } from "@/hooks/use-ws";
import { AlertTriangle, CheckCircle2, Loader2, RefreshCw } from "lucide-react";

interface CursorAuthStatus {
  authenticated: boolean;
  email?: string;
  error?: string;
  in_docker?: boolean;
}

interface CursorCLISectionProps {
  open: boolean;
  mode: string;
  onModeChange: (v: string) => void;
  apiKey: string;
  onApiKeyChange: (v: string) => void;
}

export function CursorCLISection({
  open,
  mode,
  onModeChange,
  apiKey,
  onApiKeyChange,
}: CursorCLISectionProps) {
  const { t } = useTranslation("providers");
  const http = useHttp();
  const [authStatus, setAuthStatus] = useState<CursorAuthStatus | null>(null);
  const [loading, setLoading] = useState(false);

  const checkAuth = useCallback(() => {
    setLoading(true);
    http
      .get<CursorAuthStatus>("/v1/providers/cursor-cli/auth-status")
      .then(setAuthStatus)
      .catch(() => setAuthStatus({ authenticated: false, error: "Failed to check auth status" }))
      .finally(() => setLoading(false));
  }, [http]);

  useEffect(() => {
    if (open) {
      checkAuth();
      return;
    }
    setAuthStatus(null);
  }, [open, checkAuth]);

  const isAuthenticated = authStatus?.authenticated === true;

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

      {loading ? (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loader2 className="h-3.5 w-3.5 animate-spin" />
          {t("cursor.checkingAuth")}
        </div>
      ) : isAuthenticated ? (
        <div className="flex items-center justify-between rounded-md border border-green-200 bg-green-50 px-3 py-2 dark:border-green-800 dark:bg-green-950">
          <div className="flex items-center gap-2">
            <CheckCircle2 className="h-4 w-4 text-green-600 dark:text-green-400" />
            <p className="text-sm text-green-700 dark:text-green-300">
              {t("cursor.authenticatedAs")} <strong>{authStatus?.email || t("cursor.authenticated")}</strong>
            </p>
          </div>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-7 px-2 text-green-700 hover:text-green-800 dark:text-green-400 dark:hover:text-green-300"
            onClick={checkAuth}
          >
            <RefreshCw className="h-3.5 w-3.5" />
          </Button>
        </div>
      ) : (
        <div className="space-y-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 dark:border-amber-800 dark:bg-amber-950">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <AlertTriangle className="h-4 w-4 text-amber-600 dark:text-amber-400" />
              <p className="text-sm font-medium text-amber-700 dark:text-amber-300">
                {t("cursor.notAuthenticated")}
              </p>
            </div>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-7 px-2 text-amber-700 hover:text-amber-800 dark:text-amber-400 dark:hover:text-amber-300"
              onClick={checkAuth}
            >
              <RefreshCw className="h-3.5 w-3.5 mr-1" />
              <span className="text-xs">{t("cursor.recheckButton")}</span>
            </Button>
          </div>
          <p className="text-sm text-amber-600 dark:text-amber-400">{t("cursor.runOnServer")}</p>
          <code className="block rounded bg-amber-100 px-2 py-1 text-xs font-mono dark:bg-amber-900 dark:text-amber-300">
            {authStatus?.in_docker ? "docker compose exec goclaw agent login" : "agent login"}
          </code>
          {authStatus?.error && (
            <p className="text-xs text-amber-500">{authStatus.error}</p>
          )}
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
      )}
    </div>
  );
}
