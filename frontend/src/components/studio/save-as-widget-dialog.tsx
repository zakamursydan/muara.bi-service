"use client";

import { useState } from "react";
import { Loader2, Save } from "lucide-react";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useSaveWidget } from "@/lib/mutations";
import { useDashboardsQuery } from "@/lib/queries";
import type * as T from "@/types/api";

const chartTypes: T.ChartType[] = [
  "bar",
  "line",
  "area",
  "pie",
  "scatter",
  "kpi",
  "table",
];

type SaveAsWidgetDialogProps = {
  workspaceId: string;
  projectId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  sql: string;
  defaultTitle: string;
  defaultChartType: T.ChartType;
};

export function SaveAsWidgetDialog({
  workspaceId,
  projectId,
  open,
  onOpenChange,
  sql,
  defaultTitle,
  defaultChartType,
}: SaveAsWidgetDialogProps) {
  const dashboards = useDashboardsQuery(workspaceId, projectId);
  const [dashboardId, setDashboardId] = useState("");
  const [title, setTitle] = useState(defaultTitle);
  const [chartType, setChartType] = useState<T.ChartType>(defaultChartType);

  const target = (dashboards.data ?? []).find(
    (item) => item.id === dashboardId,
  );

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Simpan sebagai widget</DialogTitle>
          <DialogDescription>
            Query disimpan apa adanya, termasuk placeholder `{"{{param}}"}` agar
            nilainya bisa diisi lewat filter dashboard.
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label>Judul widget</Label>
            <Input
              value={title}
              onChange={(event) => setTitle(event.target.value)}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label>Tipe chart</Label>
            <Select
              value={chartType}
              onValueChange={(next) => setChartType(next as T.ChartType)}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {chartTypes.map((type) => (
                  <SelectItem key={type} value={type}>
                    {type}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex flex-col gap-1.5">
            <Label>Dashboard tujuan</Label>
            {dashboards.isLoading ? (
              <p className="text-xs text-muted-foreground">Memuat dashboard…</p>
            ) : (dashboards.data ?? []).length === 0 ? (
              <p className="text-xs text-muted-foreground">
                Project ini belum punya dashboard. Buat satu dari Catalog lebih
                dulu.
              </p>
            ) : (
              <Select value={dashboardId} onValueChange={setDashboardId}>
                <SelectTrigger>
                  <SelectValue placeholder="Pilih dashboard" />
                </SelectTrigger>
                <SelectContent>
                  {(dashboards.data ?? []).map((dashboard) => (
                    <SelectItem key={dashboard.id} value={dashboard.id}>
                      {dashboard.title}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          </div>
        </div>

        {target ? (
          <SaveAction
            // Hook is created per target, so remount when the dashboard changes.
            key={target.id}
            workspaceId={workspaceId}
            dashboardId={target.id}
            title={title}
            chartType={chartType}
            sql={sql}
            onDone={() => onOpenChange(false)}
          />
        ) : (
          <div className="flex justify-end">
            <Button variant="ghost" onClick={() => onOpenChange(false)}>
              Batal
            </Button>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}

function SaveAction({
  workspaceId,
  dashboardId,
  title,
  chartType,
  sql,
  onDone,
}: {
  workspaceId: string;
  dashboardId: string;
  title: string;
  chartType: T.ChartType;
  sql: string;
  onDone: () => void;
}) {
  const save = useSaveWidget(workspaceId, dashboardId);

  return (
    <div className="flex justify-end gap-2">
      <Button variant="ghost" onClick={onDone} disabled={save.isPending}>
        Batal
      </Button>
      <Button
        disabled={save.isPending || !title.trim()}
        onClick={async () => {
          await save.mutateAsync({
            body: {
              title: title.trim(),
              chart_type: chartType,
              query_mode: "CUSTOM_SQL",
              raw_sql: sql,
              echarts_config: {},
            },
          });
          onDone();
        }}
      >
        {save.isPending ? (
          <Loader2 className="mr-1 size-4 animate-spin" />
        ) : (
          <Save className="mr-1 size-4" />
        )}
        Simpan ke dashboard
      </Button>
    </div>
  );
}
