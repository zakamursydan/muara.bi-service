"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Loader2, TriangleAlert } from "lucide-react";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { ChartRenderer } from "@/components/canvas/chart-renderer";
import { DataTable } from "@/components/canvas/data-table";
import { api } from "@/lib/api";
import { buildEchartsOption, inferFields, pickKpi } from "@/lib/echarts-options";
import { stableStringify } from "@/lib/queries";
import { cn } from "@/lib/utils";
import type * as T from "@/types/api";

type PublicCanvasProps = {
  dashboard: T.PublicDashboard;
  password?: string;
  /** Embed mode drops the header chrome because the host page supplies it. */
  compact?: boolean;
};

export function PublicCanvas({ dashboard, password, compact }: PublicCanvasProps) {
  const [values, setValues] = useState<Record<string, unknown>>({});

  const filters = useMemo(() => dashboard.global_filters ?? [], [dashboard.global_filters]);

  const parameters = useMemo(() => {
    const out: Record<string, unknown> = {};
    for (const filter of filters) {
      const value = values[filter.key];
      if (value !== undefined && value !== "") out[filter.key] = value;
      else if (filter.default_value !== undefined && filter.default_value !== null) {
        out[filter.key] = filter.default_value;
      }
    }
    return out;
  }, [filters, values]);

  const widgets = dashboard.widgets ?? [];
  const byId = new Map(widgets.map((widget) => [widget.id, widget]));
  // Widgets missing from layout_config still render, appended after the grid.
  const positions = new Map((dashboard.layout_config ?? []).map((item) => [item.i, item]));
  const ordered = [
    ...(dashboard.layout_config ?? []).map((item) => byId.get(item.i)).filter(Boolean),
    ...widgets.filter((widget) => !positions.has(widget.id)),
  ] as T.PublicWidget[];

  return (
    <div className="flex flex-col gap-4">
      {!compact && (dashboard.title || dashboard.description || dashboard.expires_at) && (
        <header className="flex flex-col gap-1">
          <h1 className="text-lg font-semibold text-foreground">{dashboard.title}</h1>
          {dashboard.description && (
            <p className="text-sm text-muted-foreground">{dashboard.description}</p>
          )}
          {dashboard.expires_at && (
            <p className="text-[11px] text-muted-foreground">
              Tautan berlaku sampai {new Date(dashboard.expires_at).toLocaleString("id-ID")}
            </p>
          )}
        </header>
      )}

      {filters.length > 0 && (
        <div className="flex flex-wrap items-end gap-3 rounded-lg border border-border bg-card p-3">
          {filters.map((filter) => (
            <div key={filter.key} className="flex min-w-40 flex-col gap-1">
              <Label className="text-[11px]">{filter.label || filter.key}</Label>
              <FilterControl
                filter={filter}
                value={values[filter.key]}
                onChange={(next) => setValues((state) => ({ ...state, [filter.key]: next }))}
              />
            </div>
          ))}
        </div>
      )}

      <div className="grid grid-cols-1 gap-3 lg:auto-rows-[16rem] lg:grid-cols-12">
        {ordered.map((widget) => {
          const position = positions.get(widget.id);
          return (
            <PublicWidgetCard
              key={widget.id}
              widget={widget}
              shareToken={dashboard.share_token}
              password={password}
              parameters={parameters}
              // Column span comes from layout_config; the CSS var keeps it
              // responsive (full width on mobile, N of 12 on desktop).
              className="col-span-full lg:[grid-column:span_var(--span)]"
              style={{ "--span": clampSpan(position?.w) } as React.CSSProperties}
            />
          );
        })}
      </div>

      {widgets.length === 0 && (
        <p className="py-16 text-center text-sm text-muted-foreground">
          Dashboard ini belum memiliki widget yang dibagikan.
        </p>
      )}
    </div>
  );
}

const UNSET = "__unset__";

/** Widgets without a stored position default to half the 12-column canvas. */
function clampSpan(columns?: number) {
  return Math.min(12, Math.max(1, columns ?? 6));
}

function FilterControl({
  filter,
  value,
  onChange,
}: {
  filter: T.GlobalFilter;
  value: unknown;
  onChange: (value: unknown) => void;
}) {
  if (filter.type === "select") {
    const text = value === undefined || value === null || value === "" ? UNSET : String(value);
    return (
      <Select
        value={text}
        onValueChange={(next) => onChange(next === UNSET ? undefined : next)}
      >
        <SelectTrigger className="h-8">
          <SelectValue placeholder="Semua" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={UNSET}>Semua</SelectItem>
          {(filter.options ?? []).map((option) => (
            <SelectItem key={option} value={option}>
              {option}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    );
  }

  return (
    <Input
      className="h-8"
      type={filter.type === "number" ? "number" : filter.type === "date" ? "date" : "text"}
      value={String(value ?? "")}
      onChange={(event) => {
        const raw = event.target.value;
        onChange(filter.type === "number" && raw !== "" ? Number(raw) : raw);
      }}
    />
  );
}

function PublicWidgetCard({
  widget,
  shareToken,
  password,
  parameters,
  className,
  style,
}: {
  widget: T.PublicWidget;
  shareToken: string;
  password?: string;
  parameters: Record<string, unknown>;
  className?: string;
  style?: React.CSSProperties;
}) {
  const query = useQuery({
    queryKey: ["public-widget", shareToken, widget.id, stableStringify(parameters)],
    queryFn: () => api.public.queryWidget(shareToken, widget.id, parameters, password),
    retry: 0,
    staleTime: 30_000,
  });

  const result = query.data;

  const option = useMemo(
    () =>
      result
        ? buildEchartsOption({ chartType: widget.chart_type, result, config: widget.echarts_config })
        : null,
    [result, widget.chart_type, widget.echarts_config],
  );

  const kpi = useMemo(
    () => (widget.chart_type === "kpi" && result ? pickKpi(result, widget.echarts_config) : null),
    [result, widget],
  );

  return (
    <Card className={cn("flex flex-col overflow-hidden bg-card shadow-sm", className)} style={style}>
      <header className="border-b border-border bg-secondary/60 px-3 py-1.5">
        <p className="truncate text-sm font-medium text-foreground">{widget.title}</p>
      </header>
      <div className="relative min-h-40 flex-1 p-2">
        {query.isLoading && (
          <div className="flex h-full items-center justify-center">
            <Loader2 className="size-5 animate-spin text-muted-foreground" />
          </div>
        )}
        {query.isError && (
          <div className="flex h-full flex-col items-center justify-center gap-2 px-3 text-center">
            <TriangleAlert className="size-5 text-destructive" />
            <p className="line-clamp-3 text-xs text-destructive">{query.error?.message}</p>
          </div>
        )}
        {kpi && (
          <div className="flex h-full flex-col items-center justify-center">
            <p className="text-3xl font-semibold tabular-nums text-foreground">{kpi.value}</p>
            <p className="text-xs text-muted-foreground">{kpi.label}</p>
          </div>
        )}
        {result && widget.chart_type === "table" && (
          <DataTable result={result} config={widget.echarts_config} limit={widget.max_rows} />
        )}
        {option && result && (
          <ChartRenderer
            option={option}
            categoryField={inferFields(result, widget.echarts_config).xField}
          />
        )}
        {result && !result.rows.length && (
          <p className="flex h-full items-center justify-center text-xs text-muted-foreground">
            Tidak ada data untuk filter ini.
          </p>
        )}
      </div>
    </Card>
  );
}
