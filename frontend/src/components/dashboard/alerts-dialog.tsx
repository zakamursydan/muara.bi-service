"use client";

import { useState } from "react";
import { Loader2, Plus, Send, Trash2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { useAlertsQuery } from "@/lib/queries";
import { useDeleteAlert, useSaveAlert, useTestAlert } from "@/lib/mutations";
import { useEnvStore } from "@/stores/env-store";
import type * as T from "@/types/api";

const channels: T.AlertChannel[] = ["TELEGRAM", "SLACK", "EMAIL", "WEBHOOK"];
const operators = [">", "<", ">=", "<=", "==", "!="];

/** Recipient keys the worker dispatcher reads per channel. */
function recipientPlaceholder(channel: T.AlertChannel): string {
  switch (channel) {
    case "TELEGRAM":
      return '{"bot_token":"123:ABC","chat_id":"-100123"}';
    case "SLACK":
    case "WEBHOOK":
      return '{"webhook_url":"https://…"}';
    case "EMAIL":
      return '{"email_list":["analyst@example.com"]}';
  }
}

export function DashboardAlertsDialog({
  open,
  onOpenChange,
  workspaceId,
  dashboardId,
  widgets,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaceId: string;
  dashboardId: string;
  widgets: T.Widget[];
}) {
  const envTag = useEnvStore((state) => state.envTag) ?? "";
  const alerts = useAlertsQuery(open ? workspaceId : undefined, dashboardId);
  const save = useSaveAlert(workspaceId, dashboardId);
  const remove = useDeleteAlert(workspaceId, dashboardId);
  const test = useTestAlert(workspaceId, dashboardId);

  const [editing, setEditing] = useState(false);
  const [form, setForm] = useState<T.CreateAlertRequest>({
    title: "",
    alert_type: "SCHEDULED_DIGEST",
    cron_expression: "0 8 * * *",
    channel: "TELEGRAM",
    recipient_config: {},
    env_tag: envTag,
  });

  const reset = () =>
    setForm({
      title: "",
      alert_type: "SCHEDULED_DIGEST",
      cron_expression: "0 8 * * *",
      channel: "TELEGRAM",
      recipient_config: {},
      env_tag: envTag,
    });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Alert &amp; scheduled digest</DialogTitle>
          <DialogDescription>
            Digest dikirim sesuai cron; threshold alert dievaluasi dispatcher terhadap nilai
            widget terpilih.
          </DialogDescription>
        </DialogHeader>

        <div className="flex max-h-48 flex-col gap-2 overflow-auto">
          {(alerts.data ?? []).map((alert) => (
            <div key={alert.id} className="flex items-center gap-2 rounded-md border border-border px-3 py-2 text-xs">
              <Badge variant="secondary">{alert.alert_type}</Badge>
              <span className="font-medium">{alert.title}</span>
              <span className="font-mono text-muted-foreground">{alert.cron_expression}</span>
              <span className="text-muted-foreground">{alert.channel}</span>
              {!alert.is_active && <Badge variant="outline">nonaktif</Badge>}
              <span className="ml-auto flex items-center gap-1">
                <Button
                  size="sm"
                  variant="ghost"
                  className="h-7 text-xs"
                  disabled={test.isPending}
                  onClick={() => test.mutate(alert.id)}
                >
                  <Send className="mr-1 size-3" />
                  Uji
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  className="h-7 text-xs"
                  onClick={() => {
                    setEditing(true);
                    setForm({
                      widget_id: alert.widget_id ?? undefined,
                      title: alert.title,
                      alert_type: alert.alert_type,
                      cron_expression: alert.cron_expression,
                      condition_operator: alert.condition_operator ?? undefined,
                      threshold_value: alert.threshold_value ?? undefined,
                      channel: alert.channel,
                      recipient_config: (alert.recipient_config ?? {}) as Record<string, unknown>,
                      env_tag: alert.env_tag,
                      is_active: alert.is_active,
                    });
                  }}
                >
                  Ubah
                </Button>
                <Button
                  size="icon"
                  variant="ghost"
                  className="size-7 text-destructive"
                  onClick={() => remove.mutate(alert.id)}
                >
                  <Trash2 className="size-3.5" />
                </Button>
              </span>
            </div>
          ))}
          {alerts.isLoading && <p className="text-xs text-muted-foreground">Memuat alert…</p>}
          {!alerts.isLoading && !(alerts.data ?? []).length && (
            <p className="text-xs text-muted-foreground">Belum ada alert untuk dashboard ini.</p>
          )}
        </div>

        {editing ? (
          <div className="grid gap-3 border-t border-border pt-3 sm:grid-cols-2">
            <div className="flex flex-col gap-1.5 sm:col-span-2">
              <Label htmlFor="alert-title">Judul</Label>
              <Input
                id="alert-title"
                className="h-9"
                value={form.title}
                onChange={(event) => setForm({ ...form, title: event.target.value })}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label>Tipe</Label>
              <Select
                value={form.alert_type}
                onValueChange={(next) => setForm({ ...form, alert_type: next as T.AlertType })}
              >
                <SelectTrigger className="h-9">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="SCHEDULED_DIGEST">Scheduled digest</SelectItem>
                  <SelectItem value="THRESHOLD_ALERT">Threshold alert</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="alert-cron">Cron (menit jam tanggal bulan hari)</Label>
              <Input
                id="alert-cron"
                className="h-9 font-mono"
                value={form.cron_expression}
                onChange={(event) => setForm({ ...form, cron_expression: event.target.value })}
              />
            </div>

            {form.alert_type === "THRESHOLD_ALERT" && (
              <>
                <div className="flex flex-col gap-1.5">
                  <Label>Widget pemantau</Label>
                  <Select
                    value={form.widget_id ?? undefined}
                    onValueChange={(next) => setForm({ ...form, widget_id: next })}
                  >
                    <SelectTrigger className="h-9">
                      <SelectValue placeholder="Pilih widget" />
                    </SelectTrigger>
                    <SelectContent>
                      {widgets.map((widget) => (
                        <SelectItem key={widget.id} value={widget.id}>
                          {widget.title}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="flex items-end gap-2">
                  <div className="flex flex-col gap-1.5">
                    <Label>Operator</Label>
                    <Select
                      value={form.condition_operator ?? ">"}
                      onValueChange={(next) => setForm({ ...form, condition_operator: next })}
                    >
                      <SelectTrigger className="h-9 w-20">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {operators.map((operator) => (
                          <SelectItem key={operator} value={operator}>
                            {operator}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="flex flex-1 flex-col gap-1.5">
                    <Label htmlFor="alert-threshold">Threshold</Label>
                    <Input
                      id="alert-threshold"
                      type="number"
                      className="h-9"
                      value={form.threshold_value ?? ""}
                      onChange={(event) =>
                        setForm({ ...form, threshold_value: Number(event.target.value) })
                      }
                    />
                  </div>
                </div>
              </>
            )}

            <div className="flex flex-col gap-1.5">
              <Label>Kanal</Label>
              <Select
                value={form.channel}
                onValueChange={(next) => setForm({ ...form, channel: next as T.AlertChannel })}
              >
                <SelectTrigger className="h-9">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {channels.map((channel) => (
                    <SelectItem key={channel} value={channel}>
                      {channel}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="alert-env">Environment</Label>
              <Input
                id="alert-env"
                className="h-9"
                value={form.env_tag}
                onChange={(event) => setForm({ ...form, env_tag: event.target.value })}
              />
            </div>
            <div className="flex flex-col gap-1.5 sm:col-span-2">
              <Label htmlFor="alert-recipient">Konfigurasi penerima (JSON)</Label>
              <Input
                id="alert-recipient"
                className="h-9 font-mono text-xs"
                placeholder={recipientPlaceholder(form.channel)}
                value={JSON.stringify(form.recipient_config)}
                onChange={(event) => {
                  try {
                    setForm({ ...form, recipient_config: JSON.parse(event.target.value) });
                  } catch {
                    // Keep typing an incomplete JSON object without breaking the form.
                  }
                }}
              />
            </div>
            <label className="flex items-center gap-2 text-xs">
              <Switch
                checked={form.is_active ?? true}
                onCheckedChange={(next) => setForm({ ...form, is_active: next })}
              />
              Aktif
            </label>

            <div className="flex justify-end gap-2 sm:col-span-2">
              <Button
                size="sm"
                variant="ghost"
                onClick={() => {
                  setEditing(false);
                  reset();
                }}
              >
                Batal
              </Button>
              <Button
                size="sm"
                disabled={save.isPending || !form.title.trim()}
                onClick={async () => {
                  await save.mutateAsync({ body: form });
                  setEditing(false);
                  reset();
                }}
              >
                {save.isPending && <Loader2 className="mr-2 size-4 animate-spin" />}
                Simpan alert
              </Button>
            </div>
          </div>
        ) : (
          <Button size="sm" className="w-fit" onClick={() => setEditing(true)}>
            <Plus className="mr-1 size-4" />
            Alert baru
          </Button>
        )}
      </DialogContent>
    </Dialog>
  );
}
