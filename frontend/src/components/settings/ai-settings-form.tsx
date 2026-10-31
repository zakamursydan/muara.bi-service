"use client";

import { useState } from "react";
import { KeyRound, Loader2, Save, ShieldOff } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { useUpdateAISettings } from "@/lib/mutations";
import type * as T from "@/types/api";

const providers: T.AIProvider[] = ["anthropic", "openai", "gemini", "azure_openai", "ollama"];

export function AISettingsForm({
  workspaceId,
  settings,
  canWrite,
}: {
  workspaceId: string;
  settings: T.AISettings;
  canWrite: boolean;
}) {
  const update = useUpdateAISettings(workspaceId);

  const [provider, setProvider] = useState<T.AIProvider>(settings.provider);
  const [apiKey, setApiKey] = useState("");
  const [baseUrl, setBaseUrl] = useState(settings.base_url ?? "");
  const [defaultModel, setDefaultModel] = useState(settings.default_model);
  const [fastModel, setFastModel] = useState(settings.fast_model);
  const [temperature, setTemperature] = useState(String(settings.temperature));
  const [budget, setBudget] = useState(String(settings.max_tokens_budget));
  const [instructions, setInstructions] = useState(settings.custom_instructions ?? "");
  const [masking, setMasking] = useState(settings.schema_masking.join(", "));

  const maskList = masking
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);

  async function save() {
    await update.mutateAsync({
      provider,
      api_key: apiKey.trim() || undefined,
      base_url: baseUrl.trim(),
      default_model: defaultModel.trim(),
      fast_model: fastModel.trim(),
      temperature: Number(temperature) || 0,
      max_tokens_budget: Number(budget) || 0,
      custom_instructions: instructions.trim(),
      schema_masking: maskList,
    });
    setApiKey("");
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <Badge variant={settings.has_api_key ? "secondary" : "destructive"} className="gap-1 text-[11px]">
            {settings.has_api_key ? <KeyRound className="size-3" /> : <ShieldOff className="size-3" />}
            {settings.has_api_key ? "API key tersimpan" : "Belum ada API key"}
          </Badge>
          <span className="text-[11px] text-muted-foreground">
            Co-Pilot hanya membaca metadata schema, bukan baris data.
          </span>
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-2">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="ai-provider">Provider</Label>
          <Select value={provider} onValueChange={(next) => setProvider(next as T.AIProvider)}>
            <SelectTrigger id="ai-provider" className="h-8">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {providers.map((item) => (
                <SelectItem key={item} value={item}>
                  {item}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="ai-key">API key</Label>
          <Input
            id="ai-key"
            type="password"
            autoComplete="new-password"
            disabled={!canWrite}
            placeholder={settings.has_api_key ? "Kosong berarti tidak mengubah key" : "sk-…"}
            value={apiKey}
            onChange={(event) => setApiKey(event.target.value)}
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="ai-base">Base URL</Label>
          <Input
            id="ai-base"
            disabled={!canWrite}
            placeholder="https://api.openai.com/v1"
            value={baseUrl}
            onChange={(event) => setBaseUrl(event.target.value)}
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="ai-budget">Bujet token / bulan</Label>
          <Input
            id="ai-budget"
            type="number"
            disabled={!canWrite}
            value={budget}
            onChange={(event) => setBudget(event.target.value)}
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="ai-model">Model utama</Label>
          <Input
            id="ai-model"
            disabled={!canWrite}
            value={defaultModel}
            onChange={(event) => setDefaultModel(event.target.value)}
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="ai-fast">Model cepat</Label>
          <Input
            id="ai-fast"
            disabled={!canWrite}
            value={fastModel}
            onChange={(event) => setFastModel(event.target.value)}
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="ai-temp">Temperature (0–1)</Label>
          <Input
            id="ai-temp"
            type="number"
            step="0.1"
            min="0"
            max="1"
            disabled={!canWrite}
            value={temperature}
            onChange={(event) => setTemperature(event.target.value)}
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="ai-mask">Kolom yang disamarkan</Label>
          <Input
            id="ai-mask"
            disabled={!canWrite}
            placeholder="email, phone, nik"
            value={masking}
            onChange={(event) => setMasking(event.target.value)}
          />
        </div>
      </div>

      <div className="flex flex-col gap-1.5">
        <Label htmlFor="ai-instr">Instruksi khusus</Label>
        <Textarea
          id="ai-instr"
          rows={3}
          disabled={!canWrite}
          placeholder="Selalu batasi hasil pada 30 hari terakhir dan gunakan format tanggal ISO."
          value={instructions}
          onChange={(event) => setInstructions(event.target.value)}
        />
      </div>

      {canWrite ? (
        <Button size="sm" className="w-fit" disabled={update.isPending} onClick={save}>
          {update.isPending ? (
            <Loader2 className="mr-1 size-4 animate-spin" />
          ) : (
            <Save className="mr-1 size-4" />
          )}
          Simpan pengaturan AI
        </Button>
      ) : (
        <p className="text-xs text-muted-foreground">
          Pengaturan AI hanya dapat diubah oleh anggota dengan izin menulis sumber data.
        </p>
      )}
    </div>
  );
}
