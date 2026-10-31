"use client";

import { useState } from "react";
import { Boxes, Plus, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { ModelEditor } from "@/components/semantic/model-editor";
import { useSemanticModelsQuery } from "@/lib/queries";
import { useCan, useWorkspaceStore } from "@/stores/workspace-store";
import { useProjectStore } from "@/stores/project-store";
import { useEnvStore } from "@/stores/env-store";
import type * as T from "@/types/api";

export default function SemanticModelsPage() {
  const workspaceId = useWorkspaceStore((state) => state.workspaceId) ?? "";
  const project = useProjectStore((state) => state.project);
  const envTag = useEnvStore((state) => state.envTag) ?? undefined;
  const canWrite = useCan("semantic:write");

  const modelsQuery = useSemanticModelsQuery(workspaceId, project?.id);
  const models = modelsQuery.data ?? [];

  // "new" is a draft; anything else is a saved model id. The editor is keyed on
  // this so switching selection always remounts with a fresh copy.
  const [selection, setSelection] = useState<string>("");
  const [search, setSearch] = useState("");

  const visible = models.filter((model) =>
    `${model.name} ${model.description}`
      .toLowerCase()
      .includes(search.toLowerCase()),
  );
  const active: T.SemanticModel | undefined = models.find(
    (model) => model.id === selection,
  );

  if (!project) {
    return (
      <Card className="flex flex-col items-center gap-3 border-dashed px-6 py-16 text-center">
        <Boxes className="size-6 text-muted-foreground" />
        <p className="text-sm text-muted-foreground">
          Pilih project dan environment data di header untuk melihat semantic
          model.
        </p>
      </Card>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-lg font-semibold text-foreground">
            Semantic models
          </h1>
          <p className="text-sm text-muted-foreground">
            {project.name} · {envTag ?? "belum ada environment"} ·{" "}
            {models.length} model
          </p>
        </div>
        <div className="flex items-center gap-2">
          <div className="relative">
            <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input
              className="h-8 w-52 pl-8"
              placeholder="Cari model…"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
          </div>
          {canWrite && (
            <Button
              size="sm"
              className="h-8"
              onClick={() => setSelection("new")}
            >
              <Plus className="mr-1 size-4" />
              Model
            </Button>
          )}
        </div>
      </div>

      <div className="grid min-h-0 flex-1 gap-4 lg:grid-cols-[280px_minmax(0,1fr)]">
        <Card className="flex min-h-0 flex-col overflow-hidden">
          {modelsQuery.isLoading ? (
            <div className="flex flex-col gap-2 p-3">
              {Array.from({ length: 5 }).map((_, index) => (
                <Skeleton key={index} className="h-12 rounded-lg" />
              ))}
            </div>
          ) : visible.length === 0 ? (
            <p className="px-3 py-6 text-center text-xs text-muted-foreground">
              {search
                ? `Tidak ada model dengan kata "${search}".`
                : "Belum ada semantic model di project ini."}
            </p>
          ) : (
            <ul className="min-h-0 flex-1 overflow-auto">
              {visible.map((model) => (
                <li key={model.id}>
                  <button
                    type="button"
                    className={`flex w-full flex-col gap-0.5 border-b border-border px-3 py-2.5 text-left transition-colors hover:bg-accent/60 ${
                      selection === model.id ? "bg-accent" : ""
                    }`}
                    onClick={() => setSelection(model.id)}
                  >
                    <span className="truncate text-sm font-medium text-foreground">
                      {model.name}
                    </span>
                    <span className="line-clamp-2 text-[11px] text-muted-foreground">
                      {model.description || "Tanpa deskripsi"}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </Card>

        <Card className="flex min-h-0 flex-col overflow-auto p-4">
          {selection === "new" && canWrite ? (
            <ModelEditor
              key="new"
              workspaceId={workspaceId}
              projectId={project.id}
              envTag={envTag}
              canEdit
              onSaved={(saved) => setSelection(saved.id)}
              onDeleted={() => setSelection("")}
            />
          ) : active ? (
            <ModelEditor
              key={active.id}
              workspaceId={workspaceId}
              projectId={project.id}
              envTag={envTag}
              model={active}
              canEdit={canWrite}
              onSaved={(saved) => setSelection(saved.id)}
              onDeleted={() => setSelection("")}
            />
          ) : (
            <div className="flex flex-1 flex-col items-center justify-center gap-3 text-center">
              <Boxes className="size-6 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">
                Pilih model di daftar kiri, atau buat model baru untuk menyusun
                view reusable.
              </p>
            </div>
          )}
        </Card>
      </div>
    </div>
  );
}
