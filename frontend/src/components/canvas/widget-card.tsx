"use client";

import { useMemo } from "react";
import { Loader2, Pencil, RefreshCw, Trash2, TriangleAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ChartRenderer } from "@/components/canvas/chart-renderer";
import { DataTable } from "@/components/canvas/data-table";
import { TruncationMarker, truncationMessage } from "@/components/query/truncation-note";
import { useCrossFilter } from "@/hooks/use-cross-filter";
import { useQueryExecutor } from "@/hooks/use-query-executor";
import { buildEchartsOption, inferFields, pickKpi } from "@/lib/echarts-options";
import { cn } from "@/lib/utils";
import { builderDefaults } from "@/hooks/use-variable-parser";
import { useFilterStore, widgetParameters } from "@/stores/filter-store";
import { useRefreshStore } from "@/stores/refresh-store";
import type * as T from "@/types/api";

type WidgetCardProps = {
  widget: T.Widget;
  workspaceId: string;
  projectId: string;
  envTag?: string;
  selected?: boolean;
  canEdit?: boolean;
  onSelect?: () => void;
  onEdit?: () => void;
  onRemove?: () => void;
  className?: string;
};

export function WidgetCard({
  widget,
  workspaceId,
  projectId,
  envTag,
  selected,
  canEdit,
  onSelect,
  onEdit,
  onRemove,
  className,
}: WidgetCardProps) {
  const values = useFilterStore((state) => state.values);
  const cross = useFilterStore((state) => state.cross);
  const bumpRefresh = useRefreshStore((state) => state.bump);
  const crossFilter = useCrossFilter(widget.id);

  const parameters = useMemo(
    () => widgetParameters(widget.id, builderDefaults(widget.builder_state), values, cross),
    [widget.id, widget.builder_state, values, cross],
  );

  const query = useQueryExecutor({
    workspaceId,
    projectId,
    envTag,
    sql: widget.raw_sql,
    parameters,
    cacheTtlSeconds: widget.cache_ttl_seconds,
    maxRows: widget.max_rows,
    enabled: Boolean(envTag),
  });

  const result = query.data;

  const option = useMemo(
    () =>
      result
        ? buildEchartsOption({
            chartType: widget.chart_type,
            result,
            config: widget.echarts_config,
            highlight:
              crossFilter.activeColumn && crossFilter.activeValue
                ? { field: crossFilter.activeColumn, value: crossFilter.activeValue }
                : undefined,
          })
        : null,
    [result, widget.chart_type, widget.echarts_config, crossFilter.activeColumn, crossFilter.activeValue],
  );

  const categoryField = useMemo(() => {
    if (!result) return undefined;
    return inferFields(result, widget.echarts_config).xField;
  }, [result, widget.echarts_config]);

  const kpi = useMemo(
    () => (widget.chart_type === "kpi" && result ? pickKpi(result, widget.echarts_config) : null),
    [widget, result],
  );

  const truncation = truncationMessage(result);

  return (
    <Card
      className={cn(
        "flex h-full flex-col overflow-hidden bg-card shadow-sm",
        selected && "ring-2 ring-primary",
        crossFilter.isFilteredByOther && "opacity-95",
        className,
      )}
      onClick={onSelect}
    >
      <header className="drag-handle flex cursor-grab items-center justify-between gap-2 border-b border-border bg-secondary/60 px-3 py-1.5">
        <div className="min-w-0">
          <p className="flex min-w-0 items-center gap-1.5 text-sm font-medium text-foreground">
            {truncation && (
              <span title={truncation.detail}>
                <TriangleAlert className="size-3.5 shrink-0 text-amber-600 dark:text-amber-400" />
              </span>
            )}
            <span className="truncate">{widget.title}</span>
          </p>
          <p className="text-[11px] text-muted-foreground">
            {widget.chart_type} · {widget.query_mode}
            {result ? ` · ${result.row_count} rows · ${result.duration_ms}ms` : ""}
            {result?.cached ? " · cached" : ""}
            {result && <TruncationMarker flags={result} />}
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-0.5">
          <Button
            variant="ghost"
            size="icon"
            className="size-7 no-drag"
            title="Refresh this widget"
            onClick={(event) => {
              event.stopPropagation();
              bumpRefresh();
            }}
          >
            <RefreshCw className="size-3.5" />
          </Button>
          {canEdit && (
            <>
              <Button
                variant="ghost"
                size="icon"
                className="size-7 no-drag"
                title="Edit widget"
                onClick={(event) => {
                  event.stopPropagation();
                  onEdit?.();
                }}
              >
                <Pencil className="size-3.5" />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                className="size-7 no-drag text-destructive"
                title="Delete widget"
                onClick={(event) => {
                  event.stopPropagation();
                  onRemove?.();
                }}
              >
                <Trash2 className="size-3.5" />
              </Button>
            </>
          )}
        </div>
      </header>

      <main className="relative min-h-0 flex-1 p-2">
        {!envTag && (
          <EmptyState text="Pilih environment data di header untuk memuat widget ini." />
        )}
        {envTag && query.isLoading && (
          <div className="flex h-full items-center justify-center">
            <Loader2 className="size-5 animate-spin text-muted-foreground" />
          </div>
        )}
        {envTag && query.isError && (
          <div className="flex h-full flex-col items-center justify-center gap-2 px-3 text-center">
            <TriangleAlert className="size-5 text-destructive" />
            <p className="line-clamp-4 text-xs text-destructive">{query.error?.message}</p>
          </div>
        )}
        {result && widget.chart_type === "kpi" && kpi && (
          <div className="flex h-full flex-col items-center justify-center">
            <p className="text-3xl font-semibold tabular-nums text-foreground">{kpi.value}</p>
            <p className="text-xs text-muted-foreground">{kpi.label}</p>
          </div>
        )}
        {result && widget.chart_type === "table" && (
          <DataTable
            result={result}
            config={widget.echarts_config}
            limit={widget.max_rows}
            highlight={
              crossFilter.activeColumn
                ? { field: crossFilter.activeColumn, value: crossFilter.activeValue ?? "" }
                : undefined
            }
            onCellClick={(column, value) => crossFilter.toggle(column, value)}
          />
        )}
        {option && (
          <ChartRenderer
            option={option}
            categoryField={categoryField}
            onCategoryClick={(field, value) => crossFilter.toggle(field, value)}
          />
        )}
        {result && !result.rows.length && (
          <EmptyState text="Query selesai tetapi tidak menghasilkan baris." />
        )}
      </main>
    </Card>
  );
}

function EmptyState({ text }: { text: string }) {
  return (
    <div className="flex h-full items-center justify-center px-4 text-center text-xs text-muted-foreground">
      {text}
    </div>
  );
}
