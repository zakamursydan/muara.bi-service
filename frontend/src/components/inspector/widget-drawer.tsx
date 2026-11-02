"use client";

import { useMemo, useState } from "react";
import { Loader2, Play, Sparkles, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { BuilderForm } from "@/components/inspector/builder-form";
import { ChartPaletteFields } from "@/components/inspector/chart-palette-fields";
import { SqlEditor } from "@/components/inspector/sql-editor";
import { TableStyleFields } from "@/components/inspector/table-style-fields";
import { ChartRenderer } from "@/components/canvas/chart-renderer";
import { DataTable } from "@/components/canvas/data-table";
import { TruncationNote } from "@/components/query/truncation-note";
import { useSchemaTree } from "@/hooks/use-schema-tree";
import { useQueryExecutor } from "@/hooks/use-query-executor";
import { buildEchartsOption, inferFields } from "@/lib/echarts-options";
import {
  useDeleteWidget,
  useGenerateSQL,
  useSaveWidget,
} from "@/lib/mutations";
import { useCanvasStore, type DrawerTab } from "@/stores/canvas-store";
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

const emptyBuilder: T.WidgetBuilderState = {
  datasource_alias: "",
  table: "",
  dimensions: [],
  metrics: [],
  filters: [],
};

type WidgetDrawerProps = {
  workspaceId: string;
  dashboardId: string;
  projectId: string;
  envTag?: string;
  canEdit: boolean;
};

export function WidgetDrawer({
  workspaceId,
  dashboardId,
  projectId,
  envTag,
  canEdit,
}: WidgetDrawerProps) {
  const widget = useCanvasStore((state) =>
    state.widgets.find((w) => w.id === state.selectedWidgetId),
  );
  const drawerOpen = useCanvasStore((state) => state.drawerOpen);
  const closeDrawer = useCanvasStore((state) => state.closeDrawer);

  return (
    <Sheet
      open={drawerOpen}
      onOpenChange={(open) => (open ? undefined : closeDrawer())}
    >
      <SheetContent
        side="right"
        className="flex w-[min(560px,100vw)] flex-col gap-0 p-0"
      >
        <SheetHeader className="border-b border-border px-4 py-3">
          <SheetTitle className="text-sm">Widget inspector</SheetTitle>
        </SheetHeader>
        {/* Remounting per selection makes the draft a fresh copy of the widget,
            so no effect has to copy props into state. */}
        {drawerOpen ? (
          <WidgetDraft
            key={widget?.id ?? "new"}
            widget={widget}
            workspaceId={workspaceId}
            dashboardId={dashboardId}
            projectId={projectId}
            envTag={envTag}
            canEdit={canEdit}
          />
        ) : null}
      </SheetContent>
    </Sheet>
  );
}

type WidgetDraftProps = WidgetDrawerProps & {
  widget: T.Widget | undefined;
};

function WidgetDraft({
  widget,
  workspaceId,
  dashboardId,
  projectId,
  envTag,
  canEdit,
}: WidgetDraftProps) {
  const drawerTab = useCanvasStore((state) => state.drawerTab);
  const openDrawer = useCanvasStore((state) => state.openDrawer);
  const closeDrawer = useCanvasStore((state) => state.closeDrawer);
  const upsertWidget = useCanvasStore((state) => state.upsertWidget);
  const selectWidget = useCanvasStore((state) => state.selectWidget);

  const [title, setTitle] = useState(widget?.title ?? "Untitled widget");
  const [chartType, setChartType] = useState<T.ChartType>(
    widget?.chart_type ?? "bar",
  );
  const [builder, setBuilder] = useState<T.WidgetBuilderState>(
    widget?.builder_state ?? emptyBuilder,
  );
  const [sql, setSql] = useState(widget?.raw_sql ?? "");
  const [config, setConfig] = useState<T.EchartsConfig>(
    widget?.echarts_config ?? {},
  );
  const [cacheTtl, setCacheTtl] = useState(widget?.cache_ttl_seconds ?? 300);
  const [maxRows, setMaxRows] = useState(widget?.max_rows ?? 10000);
  const [prompt, setPrompt] = useState("");
  /** SQL the last preview run used, so typing does not fire a query per keystroke. */
  const [previewSql, setPreviewSql] = useState("");

  const { tables } = useSchemaTree(workspaceId, envTag, projectId);
  const generate = useGenerateSQL(workspaceId);
  const save = useSaveWidget(workspaceId, dashboardId);
  const remove = useDeleteWidget(workspaceId, dashboardId);

  // The active tab is the query mode: the backend recompiles builder_state for
  // SYNCED widgets and takes the editor text verbatim for CUSTOM_SQL.
  const queryMode: T.QueryMode = drawerTab === "sql" ? "CUSTOM_SQL" : "SYNCED";

  const previewParameters = useMemo(
    () => (queryMode === "SYNCED" ? builderPreviewParameters(builder) : {}),
    [builder, queryMode],
  );

  const preview = useQueryExecutor({
    workspaceId,
    projectId,
    envTag,
    sql: previewSql,
    parameters: previewParameters,
    cacheTtlSeconds: cacheTtl,
    maxRows,
    // WidgetDraft only mounts while the drawer is open.
    enabled: Boolean(envTag) && previewSql.trim().length > 0,
  });

  const previewResult = preview.data;

  const previewOption = useMemo(
    () =>
      previewResult
        ? buildEchartsOption({ chartType, result: previewResult, config })
        : null,
    [chartType, config, previewResult],
  );

  const previewFields = useMemo(
    () =>
      previewResult
        ? [...new Set(previewResult.columns.map((column) => column.name))]
        : [],
    [previewResult],
  );

  const run = async () => {
    if (!envTag) {
      toast.error("Pilih environment data di header lebih dulu");
      return;
    }
    if (previewSql === sql && previewResult) {
      await preview.refetch();
      return;
    }
    setPreviewSql(sql);
  };

  const persist = async () => {
    const body: T.SaveWidgetRequest = {
      title,
      chart_type: chartType,
      query_mode: queryMode,
      builder_state: queryMode === "SYNCED" ? builder : undefined,
      raw_sql: sql,
      echarts_config: config,
      cache_ttl_seconds: cacheTtl,
      max_rows: maxRows,
    };
    const saved = await save.mutateAsync({ id: widget?.id, body });
    upsertWidget(saved);
    if (!widget) selectWidget(saved.id, drawerTab);
  };

  const applyGeneratedSQL = (generated: T.GeneratedSQL) => {
    setSql(generated.raw_sql);
    // The generated text is hand-written SQL, so the widget leaves the builder.
    openDrawer("sql");
    if (generated.explanation) toast.info(generated.explanation);
  };

  return (
    <>
      {/* Layer 1 — AI Co-Pilot prompt bar. Reads schema DDL only, never rows. */}
      <div className="flex flex-col gap-2 border-b border-border bg-secondary/40 px-4 py-3">
        <div className="flex items-center gap-2 text-xs font-medium text-foreground">
          <Sparkles className="size-3.5 text-primary" />
          AI Co-Pilot
        </div>
        <div className="flex gap-2">
          <Input
            className="h-8"
            placeholder="mis. pendapatan per bulan 12 bulan terakhir"
            value={prompt}
            onChange={(event) => setPrompt(event.target.value)}
          />
          <Button
            size="sm"
            className="h-8 shrink-0"
            disabled={!prompt.trim() || generate.isPending || !canEdit}
            onClick={async () => {
              const generated = await generate.mutateAsync({
                project_id: projectId,
                env_tag: envTag,
                prompt: prompt.trim(),
              });
              applyGeneratedSQL(generated);
            }}
          >
            {generate.isPending ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              "Generate SQL"
            )}
          </Button>
        </div>
        {generate.data?.relevant_tables.length ? (
          <p className="truncate text-[11px] text-muted-foreground">
            Tabel relevan: {generate.data.relevant_tables.join(", ")}
          </p>
        ) : null}
      </div>

      <Tabs
        value={drawerTab === "sql" ? "sql" : "builder"}
        onValueChange={(next) => openDrawer(next as DrawerTab)}
        className="flex min-h-0 flex-1 flex-col"
      >
        <TabsList className="mx-4 mt-3 grid w-auto grid-cols-2">
          <TabsTrigger value="builder">Visual builder</TabsTrigger>
          <TabsTrigger value="sql">SQL mentah</TabsTrigger>
        </TabsList>

        <div className="min-h-0 flex-1 overflow-auto">
          <TabsContent value="builder" className="mt-0">
            <BuilderForm
              sources={tables}
              state={builder}
              onChange={setBuilder}
              disabled={!canEdit}
            />
          </TabsContent>

          <TabsContent value="sql" className="mt-0 flex flex-col gap-2 p-4">
            <SqlEditor
              value={sql}
              onChange={setSql}
              workspaceId={workspaceId}
              projectId={projectId}
              envTag={envTag}
              readOnly={!canEdit}
            />
            <div className="flex items-center gap-2">
              <Button
                size="sm"
                variant="outline"
                className="h-7 text-xs"
                onClick={run}
                disabled={preview.isFetching}
              >
                {preview.isFetching ? (
                  <Loader2 className="mr-1 size-3 animate-spin" />
                ) : (
                  <Play className="mr-1 size-3" />
                )}
                Jalankan preview
              </Button>
              {preview.data && (
                <span className="text-[11px] text-muted-foreground">
                  {preview.data.row_count} baris · {preview.data.duration_ms}ms
                  {preview.data.cached ? " · cached" : ""}
                </span>
              )}
            </div>
          </TabsContent>
        </div>
      </Tabs>

      {/* Layer 3 — presentation + storage settings and the preview itself. */}
      <Separator />
      <div className="flex max-h-[45vh] flex-col gap-3 overflow-auto px-4 py-3">
        <div className="grid grid-cols-2 gap-2">
          <div className="col-span-2 flex flex-col gap-1.5">
            <Label htmlFor="widget-title">Judul</Label>
            <Input
              id="widget-title"
              className="h-8"
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
              <SelectTrigger className="h-8">
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
            <Label htmlFor="widget-ttl">Cache TTL (detik)</Label>
            <Input
              id="widget-ttl"
              type="number"
              className="h-8"
              value={cacheTtl}
              onChange={(event) => setCacheTtl(Number(event.target.value) || 0)}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="widget-max-rows">Baris maks</Label>
            <Input
              id="widget-max-rows"
              type="number"
              className="h-8"
              value={maxRows}
              onChange={(event) => setMaxRows(Number(event.target.value) || 0)}
            />
          </div>

          {previewFields.length > 0 && chartType !== "kpi" && chartType !== "table" && (
            <>
              <div className="flex flex-col gap-1.5">
                <Label>Kolom sumbu / kategori</Label>
                <Select
                  value={config.x_field ?? previewFields[0]}
                  onValueChange={(next) =>
                    setConfig({ ...config, x_field: next })
                  }
                >
                  <SelectTrigger className="h-8">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {previewFields.map((field) => (
                      <SelectItem key={field} value={field}>
                        {field}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="flex flex-col gap-1.5">
                <Label>Ukuran (Y)</Label>
                <Select
                  value={
                    config.y_fields?.[0] ??
                    previewFields.find(
                      (f) => f !== (config.x_field ?? previewFields[0]),
                    ) ??
                    previewFields[0]
                  }
                  onValueChange={(next) =>
                    setConfig({ ...config, y_fields: [next] })
                  }
                >
                  <SelectTrigger className="h-8">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {previewFields.map((field) => (
                      <SelectItem key={field} value={field}>
                        {field}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </>
          )}

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="widget-unit">Satuan</Label>
            <Input
              id="widget-unit"
              className="h-8"
              placeholder="Rp, %, USD"
              value={config.unit ?? ""}
              onChange={(event) =>
                setConfig({ ...config, unit: event.target.value || undefined })
              }
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="widget-decimals">Desimal</Label>
            <Input
              id="widget-decimals"
              type="number"
              className="h-8"
              value={config.decimals ?? 0}
              onChange={(event) =>
                setConfig({ ...config, decimals: Number(event.target.value) })
              }
            />
          </div>

          {chartType === "table" && (
            <TableStyleFields
              value={config.table}
              onChange={(table) => setConfig({ ...config, table })}
            />
          )}

          {!["table", "kpi"].includes(chartType) && (
            <ChartPaletteFields
              value={config.palette}
              onChange={(palette) => setConfig({ ...config, palette })}
              disabled={!canEdit}
            />
          )}
        </div>

        {!["table", "kpi"].includes(chartType) && (
          <div className="flex flex-wrap gap-3 text-xs text-muted-foreground">
          <label className="flex items-center gap-1.5">
            <input
              type="checkbox"
              className="size-3.5 accent-primary"
              checked={Boolean(config.stacked)}
              onChange={(event) =>
                setConfig({ ...config, stacked: event.target.checked })
              }
            />
            Stack
          </label>
          <label className="flex items-center gap-1.5">
            <input
              type="checkbox"
              className="size-3.5 accent-primary"
              checked={Boolean(config.smooth)}
              onChange={(event) =>
                setConfig({ ...config, smooth: event.target.checked })
              }
            />
            Smooth
          </label>
          <label className="flex items-center gap-1.5">
            <input
              type="checkbox"
              className="size-3.5 accent-primary"
              checked={Boolean(config.show_percent)}
              onChange={(event) =>
                setConfig({ ...config, show_percent: event.target.checked })
              }
            />
            Persen
          </label>
          </div>
        )}

        <TruncationNote flags={previewResult} />

        {chartType === "table" && previewResult ? (
          <div className="h-44">
            <DataTable result={previewResult} config={config} limit={50} />
          </div>
        ) : previewOption && previewResult ? (
          <div className="h-44 overflow-hidden rounded-md border border-border">
            <ChartRenderer
              option={previewOption}
              categoryField={inferFields(previewResult, config).xField}
            />
          </div>
        ) : preview.isError ? (
          <p className="rounded-md bg-destructive/10 px-3 py-2 text-xs text-destructive">
            {preview.error?.message}
          </p>
        ) : null}
      </div>

      <footer className="flex items-center gap-2 border-t border-border px-4 py-3">
        <Button
          size="sm"
          onClick={persist}
          disabled={!canEdit || save.isPending || !sql.trim()}
        >
          {save.isPending ? (
            <Loader2 className="mr-1 size-4 animate-spin" />
          ) : null}
          Simpan widget
        </Button>
        {widget && canEdit && (
          <Button
            size="sm"
            variant="ghost"
            className="text-destructive"
            disabled={remove.isPending}
            onClick={async () => {
              await remove.mutateAsync(widget.id);
              closeDrawer();
            }}
          >
            <Trash2 className="mr-1 size-4" />
            Hapus
          </Button>
        )}
        <span className="ml-auto text-[11px] text-muted-foreground">
          Mode{" "}
          {queryMode === "SYNCED"
            ? "SYNCED (dikompilasi server)"
            : "CUSTOM_SQL"}
        </span>
      </footer>
    </>
  );
}

/** Builder filters bind `flt_*` values; the preview uses the stored defaults. */
function builderPreviewParameters(
  state: T.WidgetBuilderState,
): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const filter of state.filters) {
    const param =
      filter.parameter?.trim() || `flt_${filter.column.replace(/\./g, "_")}`;
    out[param] = filter.value ?? null;
  }
  return out;
}
