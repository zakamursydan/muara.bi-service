"use client";

import { useMemo, useState } from "react";
import {
  FlaskConical,
  Loader2,
  Play,
  Sparkles,
  Table2,
  TrendingUp,
} from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ChartRenderer } from "@/components/canvas/chart-renderer";
import { DataTable } from "@/components/canvas/data-table";
import { TruncationNote } from "@/components/query/truncation-note";
import { SqlEditor } from "@/components/inspector/sql-editor";
import { SaveAsWidgetDialog } from "@/components/studio/save-as-widget-dialog";
import { useQueryExecutor } from "@/hooks/use-query-executor";
import { useSchemaTree } from "@/hooks/use-schema-tree";
import { parseVariables } from "@/hooks/use-variable-parser";
import { buildEchartsOption } from "@/lib/echarts-options";
import { useGenerateSQL } from "@/lib/mutations";
import { useCan, useWorkspaceStore } from "@/stores/workspace-store";
import { useProjectStore } from "@/stores/project-store";
import { useEnvStore } from "@/stores/env-store";
import type * as T from "@/types/api";

export default function StudioPage() {
  const workspaceId = useWorkspaceStore((state) => state.workspaceId) ?? "";
  const project = useProjectStore((state) => state.project);
  const envTag = useEnvStore((state) => state.envTag) ?? undefined;
  const canQuery = useCan("query:execute");
  const canWrite = useCan("dashboard:write");

  const { tables } = useSchemaTree(workspaceId, envTag, project?.id);
  const generate = useGenerateSQL(workspaceId);

  const [sql, setSql] = useState("");
  const [prompt, setPrompt] = useState("");
  const [values, setValues] = useState<Record<string, unknown>>({});
  /** Query text is snapshotted on Run so typing cannot fire a request per keystroke. */
  const [runSql, setRunSql] = useState("");
  const [view, setView] = useState<"table" | T.ChartType>("table");
  const [saveOpen, setSaveOpen] = useState(false);

  const variables = useMemo(() => parseVariables(sql), [sql]);

  const parameters = useMemo(() => {
    const out: Record<string, unknown> = {};
    for (const name of variables)
      if (values[name] !== undefined && values[name] !== "")
        out[name] = values[name];
    return out;
  }, [values, variables]);

  const result = useQueryExecutor({
    workspaceId,
    projectId: project?.id,
    envTag,
    sql: runSql,
    parameters,
    enabled: canQuery && Boolean(project && envTag) && runSql.trim().length > 0,
  });

  const option = useMemo(
    () =>
      result.data && view !== "table"
        ? buildEchartsOption({
            chartType: view,
            result: result.data,
            config: {},
          })
        : null,
    [view, result.data],
  );

  const run = async () => {
    if (!project || !envTag) {
      toast.error("Pilih project dan environment data di header lebih dulu");
      return;
    }
    if (!sql.trim()) {
      toast.error("Tulis query lebih dulu");
      return;
    }
    if (runSql === sql && result.data) {
      await result.refetch();
      return;
    }
    setRunSql(sql);
  };

  if (!project || !envTag) {
    return (
      <Card className="flex flex-col items-center gap-3 border-dashed px-6 py-16 text-center">
        <FlaskConical className="size-6 text-muted-foreground" />
        <p className="text-sm text-muted-foreground">
          Pilih project dan environment data di header untuk membuka Data
          Studio.
        </p>
      </Card>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-lg font-semibold text-foreground">Data Studio</h1>
          <p className="text-sm text-muted-foreground">
            {project.name} · {envTag} · query federated lintas connection dan
            semantic model
          </p>
        </div>
        <div className="flex items-center gap-2">
          {result.data && canWrite && (
            <Button
              size="sm"
              variant="outline"
              className="h-8"
              onClick={() => setSaveOpen(true)}
            >
              Simpan sebagai widget
            </Button>
          )}
          <Button
            size="sm"
            className="h-8"
            onClick={run}
            disabled={result.isFetching || !canQuery}
          >
            {result.isFetching ? (
              <Loader2 className="mr-1 size-4 animate-spin" />
            ) : (
              <Play className="mr-1 size-4" />
            )}
            Jalankan
          </Button>
        </div>
      </div>

      <div className="grid min-h-0 flex-1 gap-4 xl:grid-cols-[260px_minmax(0,1fr)]">
        <Card className="flex min-h-0 flex-col overflow-hidden">
          <p className="border-b border-border px-3 py-2 text-xs font-medium text-muted-foreground">
            {tables.length} tabel / view
          </p>
          <div className="min-h-0 flex-1 overflow-auto">
            {tables.length === 0 ? (
              <p className="px-3 py-6 text-center text-xs text-muted-foreground">
                Belum ada schema. Jalankan Sync schema dari halaman Data
                Sources.
              </p>
            ) : (
              tables.map((table) => (
                <details
                  key={table.qualified}
                  className="border-b border-border/60 px-3 py-1.5"
                >
                  <summary className="flex cursor-pointer items-center gap-1.5 text-xs font-medium text-foreground">
                    {table.kind === "semantic-model" ? (
                      <Sparkles className="size-3 shrink-0 text-primary" />
                    ) : (
                      <Table2 className="size-3 shrink-0 text-muted-foreground" />
                    )}
                    <span className="truncate">{table.qualified}</span>
                  </summary>
                  <ul className="mt-1 space-y-0.5 pl-4">
                    {table.columns.length === 0 ? (
                      <li className="text-[11px] text-muted-foreground">
                        Kolom belum tersinkron
                      </li>
                    ) : (
                      table.columns.map((column) => (
                        <li
                          key={column.name}
                          className="text-[11px] text-muted-foreground"
                        >
                          <button
                            type="button"
                            className="hover:text-foreground"
                            onClick={() =>
                              setSql(
                                `${sql}${sql.endsWith(" ") || sql === "" ? "" : " "}${column.name}`,
                              )
                            }
                          >
                            {column.name}
                          </button>
                          <span className="ml-1 opacity-70">{column.type}</span>
                        </li>
                      ))
                    )}
                  </ul>
                </details>
              ))
            )}
          </div>
        </Card>

        <div className="flex min-h-0 flex-col gap-3">
          <div className="flex flex-col gap-2 rounded-lg border border-border bg-secondary/40 p-3">
            <div className="flex items-center gap-2 text-xs font-medium">
              <Sparkles className="size-3.5 text-primary" />
              AI Co-Pilot
            </div>
            <div className="flex gap-2">
              <Input
                className="h-8"
                placeholder="mis. rata-rata nilai transaksi per cabang 30 hari terakhir"
                value={prompt}
                onChange={(event) => setPrompt(event.target.value)}
              />
              <Button
                size="sm"
                className="h-8 shrink-0"
                disabled={!prompt.trim() || generate.isPending}
                onClick={async () => {
                  const generated = await generate.mutateAsync({
                    project_id: project.id,
                    env_tag: envTag,
                    prompt: prompt.trim(),
                  });
                  setSql(generated.raw_sql);
                  if (generated.explanation) toast.info(generated.explanation);
                }}
              >
                {generate.isPending ? (
                  <Loader2 className="size-4 animate-spin" />
                ) : (
                  "Generate SQL"
                )}
              </Button>
            </div>
          </div>

          <SqlEditor
            value={sql}
            onChange={setSql}
            workspaceId={workspaceId}
            projectId={project.id}
            envTag={envTag}
            readOnly={!canQuery}
          />

          {variables.length > 0 && (
            <div className="flex flex-wrap gap-2">
              {variables.map((name) => (
                <div key={name} className="flex flex-col gap-1">
                  <Label className="text-[11px]">{`{{${name}}}`}</Label>
                  <Input
                    className="h-7 w-40 text-xs"
                    value={String(values[name] ?? "")}
                    onChange={(event) =>
                      setValues((state) => ({
                        ...state,
                        [name]: event.target.value,
                      }))
                    }
                  />
                </div>
              ))}
            </div>
          )}

          <Card className="flex min-h-0 flex-1 flex-col gap-2 p-3">
            {result.isFetching && !result.data ? (
              <Skeleton className="h-full min-h-40 rounded-lg" />
            ) : result.isError ? (
              <p className="rounded-md bg-destructive/10 px-3 py-2 text-xs text-destructive">
                {result.error?.message}
              </p>
            ) : !result.data ? (
              <div className="flex flex-1 flex-col items-center justify-center gap-2 text-center">
                <FlaskConical className="size-6 text-muted-foreground" />
                <p className="text-sm text-muted-foreground">
                  Belum ada hasil. Tulis query lalu tekan Jalankan.
                </p>
                <p className="text-[11px] text-muted-foreground">
                  Guardrail server membatasi 10.000 baris dan 30 detik per
                  query.
                </p>
              </div>
            ) : (
              <>
                <TruncationNote flags={result.data} />
                <div className="flex items-center gap-2 text-[11px] text-muted-foreground">
                  <span>
                    {result.data.row_count} baris · {result.data.duration_ms}ms
                    {result.data.cached ? " · cached" : ""}
                  </span>
                  {result.data.columns.length > 1 && (
                    <Tabs
                      value={view}
                      className="ml-auto"
                      onValueChange={(next) =>
                        setView(next as "table" | T.ChartType)
                      }
                    >
                      <TabsList className="h-7">
                        <TabsTrigger value="table" className="text-xs">
                          <Table2 className="mr-1 size-3" />
                          Tabel
                        </TabsTrigger>
                        <TabsTrigger value="bar" className="text-xs">
                          <TrendingUp className="mr-1 size-3" />
                          Bar
                        </TabsTrigger>
                        <TabsTrigger value="line" className="text-xs">
                          <TrendingUp className="mr-1 size-3" />
                          Line
                        </TabsTrigger>
                        <TabsTrigger value="pie" className="text-xs">
                          <TrendingUp className="mr-1 size-3" />
                          Pie
                        </TabsTrigger>
                      </TabsList>
                    </Tabs>
                  )}
                </div>
                {option ? (
                  <div className="min-h-0 flex-1">
                    <ChartRenderer option={option} className="min-h-64" />
                  </div>
                ) : (
                  <DataTable
                    result={result.data}
                    limit={200}
                    className="min-h-0 flex-1"
                  />
                )}
              </>
            )}
          </Card>
        </div>
      </div>

      <SaveAsWidgetDialog
        workspaceId={workspaceId}
        projectId={project.id}
        open={saveOpen}
        onOpenChange={setSaveOpen}
        sql={runSql}
        defaultTitle={sql.split("\n")[0]?.slice(0, 40) || "Query scratchpad"}
        defaultChartType={view === "table" ? "bar" : view}
      />
    </div>
  );
}
