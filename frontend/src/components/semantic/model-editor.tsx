"use client";

import { useState } from "react";
import { Loader2, Play, Save, Sparkles, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { DataTable } from "@/components/canvas/data-table";
import { SqlEditor } from "@/components/inspector/sql-editor";
import { useQueryExecutor } from "@/hooks/use-query-executor";
import {
  useDeleteSemanticModel,
  useGenerateSQL,
  useSaveSemanticModel,
} from "@/lib/mutations";
import type * as T from "@/types/api";

/** The backend mounts each model as `CREATE TEMP VIEW <name>`, so the name must be a plain identifier. */
const MODEL_NAME = /^[A-Za-z_][A-Za-z0-9_]*$/;

type ModelEditorProps = {
  workspaceId: string;
  projectId: string;
  envTag?: string;
  model?: T.SemanticModel;
  canEdit: boolean;
  onSaved: (model: T.SemanticModel) => void;
  onDeleted: () => void;
};

export function ModelEditor({
  workspaceId,
  projectId,
  envTag,
  model,
  canEdit,
  onSaved,
  onDeleted,
}: ModelEditorProps) {
  const [name, setName] = useState(model?.name ?? "");
  const [description, setDescription] = useState(model?.description ?? "");
  const [sql, setSql] = useState(model?.raw_sql ?? "");
  const [prompt, setPrompt] = useState("");
  const [previewSql, setPreviewSql] = useState("");

  const generate = useGenerateSQL(workspaceId);
  const save = useSaveSemanticModel(workspaceId, projectId);
  const remove = useDeleteSemanticModel(workspaceId, projectId);

  const preview = useQueryExecutor({
    workspaceId,
    projectId,
    envTag,
    sql: previewSql,
    // A model is a view definition; the preview reads a slice of it.
    maxRows: 100,
    enabled: Boolean(envTag) && previewSql.trim().length > 0,
  });

  const nameValid = MODEL_NAME.test(name.trim());
  const dirty =
    !model ||
    model.name !== name ||
    model.description !== description ||
    model.raw_sql !== sql;

  const run = async () => {
    if (!envTag) {
      toast.error("Pilih environment data di header lebih dulu");
      return;
    }
    if (!sql.trim()) {
      toast.error("Tulis SQL model lebih dulu");
      return;
    }
    if (previewSql === sql && preview.data) {
      await preview.refetch();
      return;
    }
    setPreviewSql(sql);
  };

  const persist = async () => {
    if (!nameValid) {
      toast.error("Nama model hanya boleh huruf, angka, dan underscore");
      return;
    }
    const saved = await save.mutateAsync({
      id: model?.id,
      body: { name: name.trim(), description, raw_sql: sql },
    });
    onSaved(saved);
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      <div className="grid gap-2 sm:grid-cols-2">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="model-name">Nama view</Label>
          <Input
            id="model-name"
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder="orders_enriched"
            disabled={!canEdit}
            aria-invalid={name.length > 0 && !nameValid}
          />
          <p className="text-[11px] text-muted-foreground">
            {nameValid
              ? `Dipakai di SQL sebagai FROM ${name.trim() || "…"}`
              : "Huruf/angka/underscore, tidak boleh diawali angka."}
          </p>
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="model-description">Deskripsi</Label>
          <Textarea
            id="model-description"
            rows={3}
            className="min-h-0"
            value={description}
            onChange={(event) => setDescription(event.target.value)}
            placeholder="Dipakai AI Co-Pilot sebagai konteks saat memilih tabel."
            disabled={!canEdit}
          />
        </div>
      </div>

      {/* AI Co-Pilot reads DDL only, never transaction rows. */}
      {canEdit && (
        <div className="flex flex-col gap-2 rounded-lg border border-border bg-secondary/40 p-3">
          <div className="flex items-center gap-2 text-xs font-medium">
            <Sparkles className="size-3.5 text-primary" />
            AI Co-Pilot
          </div>
          <div className="flex gap-2">
            <Input
              className="h-8"
              placeholder="mis. gabungkan orders dengan customers per branch"
              value={prompt}
              onChange={(event) => setPrompt(event.target.value)}
            />
            <Button
              size="sm"
              className="h-8 shrink-0"
              disabled={!prompt.trim() || generate.isPending}
              onClick={async () => {
                const generated = await generate.mutateAsync({
                  project_id: projectId,
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
          {generate.data?.relevant_tables.length ? (
            <p className="truncate text-[11px] text-muted-foreground">
              Tabel relevan: {generate.data.relevant_tables.join(", ")}
            </p>
          ) : null}
        </div>
      )}

      <div className="flex min-h-0 flex-1 flex-col gap-2">
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
          <span className="ml-auto text-[11px] text-muted-foreground">
            {model
              ? `Diperbarui ${new Date(model.updated_at).toLocaleDateString("id-ID")}`
              : "Model baru"}
          </span>
        </div>
      </div>

      {preview.data ? (
        <DataTable result={preview.data} limit={100} className="max-h-64" />
      ) : preview.isError ? (
        <p className="rounded-md bg-destructive/10 px-3 py-2 text-xs text-destructive">
          {preview.error?.message}
        </p>
      ) : null}

      <footer className="flex items-center gap-2 border-t border-border pt-3">
        <Button
          size="sm"
          onClick={persist}
          disabled={
            !canEdit || save.isPending || !nameValid || !sql.trim() || !dirty
          }
        >
          {save.isPending ? (
            <Loader2 className="mr-1 size-4 animate-spin" />
          ) : (
            <Save className="mr-1 size-4" />
          )}
          Simpan model
        </Button>
        {model && canEdit && (
          <Button
            size="sm"
            variant="ghost"
            className="text-destructive"
            disabled={remove.isPending}
            onClick={async () => {
              await remove.mutateAsync(model.id);
              onDeleted();
            }}
          >
            <Trash2 className="mr-1 size-4" />
            Hapus
          </Button>
        )}
        <span className="ml-auto text-[11px] text-muted-foreground">
          Model menjadi DuckDB temporary view pada setiap query project ini.
        </span>
      </footer>
    </div>
  );
}
