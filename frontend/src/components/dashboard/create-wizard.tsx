"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Loader2, LayoutGrid, Sparkles, WandSparkles } from "lucide-react";
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
import { Textarea } from "@/components/ui/textarea";
import { useCreateDashboard, useGenerateDashboard } from "@/lib/mutations";
import { useEnvStore } from "@/stores/env-store";
import { useProjectStore } from "@/stores/project-store";
import { useWorkspaceStore } from "@/stores/workspace-store";
import { cn } from "@/lib/utils";
import type * as T from "@/types/api";

type Mode = "ai" | "template" | "blank";

/** Preset prompts standing in for the PLAN's project templates. */
const templates: { key: string; name: string; prompt: string }[] = [
  {
    key: "chatbot",
    name: "Apps Chatbot",
    prompt:
      "Dashboard performa chatbot: total sesi per hari, jumlah pesan, rasio escalation ke agen manusia, dan top intent.",
  },
  {
    key: "omnichannel",
    name: "Apps Omnichannel",
    prompt:
      "Dashboard omnichannel: volume percakapan per kanal, waktu respons rata-rata per kanal, dan backlog tiket yang belum selesai.",
  },
  {
    key: "retail",
    name: "E-Commerce & Retail",
    prompt:
      "Dashboard retail: pendapatan harian, jumlah pesanan, top produk berdasarkan pendapatan, dan penjualan per cabang.",
  },
];

export function CreateDashboardWizard({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const router = useRouter();
  const workspaceId = useWorkspaceStore((state) => state.workspaceId) ?? "";
  const project = useProjectStore((state) => state.project);
  const envTag = useEnvStore((state) => state.envTag) ?? "";

  const generate = useGenerateDashboard(workspaceId);
  const create = useCreateDashboard(workspaceId);

  const [mode, setMode] = useState<Mode>("ai");
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [prompt, setPrompt] = useState("");
  const [templateKey, setTemplateKey] = useState(templates[0].key);

  const busy = generate.isPending || create.isPending;

  const reset = () => {
    setMode("ai");
    setTitle("");
    setDescription("");
    setPrompt("");
    setTemplateKey(templates[0].key);
  };

  const submit = async () => {
    if (!project || !title.trim()) return;

    const body: T.SaveDashboardRequest = {
      title: title.trim(),
      description: description.trim(),
      layout_config: [],
      global_filters: [],
    };

    if (mode === "blank") {
      const dashboard = await create.mutateAsync({ projectId: project.id, body });
      reset();
      onOpenChange(false);
      router.push(`/dashboards/${dashboard.id}`);
      return;
    }

    if (!envTag) return;

    const generated = await generate.mutateAsync({
      project_id: project.id,
      env_tag: envTag,
      prompt: prompt.trim(),
      title: title.trim(),
    });

    const widgets: T.SaveWidgetRequest[] = generated.widgets.map((widget) => ({
      title: widget.title,
      chart_type: widget.chart_type,
      query_mode: "CUSTOM_SQL",
      raw_sql: widget.raw_sql,
      echarts_config: widget.echarts_config as T.EchartsConfig,
    }));

    const dashboard = await create.mutateAsync({
      projectId: project.id,
      // The mutation writes the real layout once the generated widgets have ids.
      body,
      widgets,
    });

    reset();
    onOpenChange(false);
    router.push(`/dashboards/${dashboard.id}`);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>Buat dashboard</DialogTitle>
          <DialogDescription>
            Pilih jalur pembuatan. AI Co-Pilot hanya membaca schema (DDL) dan tidak pernah
            membaca baris data.
          </DialogDescription>
        </DialogHeader>

        <div className="grid grid-cols-3 gap-2">
          {(
            [
              { key: "ai", label: "AI Co-Pilot", icon: Sparkles },
              { key: "template", label: "Template", icon: WandSparkles },
              { key: "blank", label: "Canvas kosong", icon: LayoutGrid },
            ] as const
          ).map((item) => (
            <button
              key={item.key}
              type="button"
              className={cn(
                "flex flex-col items-center gap-1.5 rounded-lg border px-3 py-3 text-xs font-medium transition-colors",
                mode === item.key
                  ? "border-primary bg-primary/5 text-primary"
                  : "border-border text-muted-foreground hover:bg-secondary",
              )}
              onClick={() => {
                setMode(item.key);
                if (item.key === "template") {
                  const preset = templates.find((t) => t.key === templateKey);
                  setPrompt(preset?.prompt ?? "");
                  if (preset && !title) setTitle(preset.name);
                }
              }}
            >
              <item.icon className="size-4" />
              {item.label}
            </button>
          ))}
        </div>

        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="dashboard-title">Judul</Label>
            <Input
              id="dashboard-title"
              value={title}
              placeholder="mis. Performa Penjualan Harian"
              onChange={(event) => setTitle(event.target.value)}
            />
          </div>

          {mode === "blank" && (
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="dashboard-description">Deskripsi</Label>
              <Input
                id="dashboard-description"
                value={description}
                placeholder="Opsional"
                onChange={(event) => setDescription(event.target.value)}
              />
            </div>
          )}

          {mode === "template" && (
            <div className="flex flex-col gap-1.5">
              <Label>Template project</Label>
              <div className="flex gap-2">
                {templates.map((template) => (
                  <button
                    key={template.key}
                    type="button"
                    className={cn(
                      "flex-1 rounded-lg border px-2 py-2 text-xs transition-colors",
                      templateKey === template.key
                        ? "border-primary bg-primary/5 text-primary"
                        : "border-border text-muted-foreground hover:bg-secondary",
                    )}
                    onClick={() => {
                      setTemplateKey(template.key);
                      setPrompt(template.prompt);
                      if (!title) setTitle(template.name);
                    }}
                  >
                    {template.name}
                  </button>
                ))}
              </div>
            </div>
          )}

          {mode !== "blank" && (
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="dashboard-prompt">Prompt natural language</Label>
              <Textarea
                id="dashboard-prompt"
                rows={4}
                value={prompt}
                placeholder="mis. Tampilkan pendapatan per bulan dan top 5 produk terlaris"
                onChange={(event) => setPrompt(event.target.value)}
              />
              <p className="text-[11px] text-muted-foreground">
                Environment aktif: <span className="font-medium">{envTag || "belum dipilih"}</span>
              </p>
            </div>
          )}
        </div>

        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={busy}>
            Batal
          </Button>
          <Button
            onClick={submit}
            disabled={
              busy ||
              !project ||
              !title.trim() ||
              (mode !== "blank" && (!prompt.trim() || !envTag))
            }
          >
            {busy && <Loader2 className="mr-2 size-4 animate-spin" />}
            {mode === "blank" ? "Buat canvas" : "Generate dashboard"}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
