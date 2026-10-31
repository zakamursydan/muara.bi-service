"use client";

import { useState } from "react";
import { Check, Copy, Loader2, Trash2 } from "lucide-react";
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
import { useEnvStore } from "@/stores/env-store";
import { useCreateShare, useRevokeShare } from "@/lib/mutations";
import { useSharesQuery } from "@/lib/queries";
import type * as T from "@/types/api";

export function DashboardShareDialog({
  open,
  onOpenChange,
  workspaceId,
  dashboardId,
  title,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaceId: string;
  dashboardId: string;
  title: string;
}) {
  const headerEnv = useEnvStore((state) => state.envTag) ?? "";
  const shares = useSharesQuery(open ? workspaceId : undefined, dashboardId);
  const create = useCreateShare(workspaceId, dashboardId);
  const revoke = useRevokeShare(workspaceId, dashboardId);

  const [mode, setMode] = useState<T.AccessMode>("PUBLIC_LINK");
  const [envTag, setEnvTag] = useState(headerEnv);
  const [password, setPassword] = useState("");
  const [usePassword, setUsePassword] = useState(false);
  const [domains, setDomains] = useState("");
  const [expires, setExpires] = useState("");

  const created = create.data;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Bagikan “{title}”</DialogTitle>
          <DialogDescription>
            Tautan publik hanya menampilkan hasil render. SQL mentah dan builder state tidak
            pernah dikirim ke penonton publik.
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-3 sm:grid-cols-2">
          <div className="flex flex-col gap-1.5">
            <Label>Mode akses</Label>
            <Select value={mode} onValueChange={(next) => setMode(next as T.AccessMode)}>
              <SelectTrigger className="h-9">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="PUBLIC_LINK">Tautan publik</SelectItem>
                <SelectItem value="EMBED_IFRAME">Embed iframe (JWT)</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="share-env">Environment data</Label>
            <Input
              id="share-env"
              className="h-9"
              value={envTag}
              onChange={(event) => setEnvTag(event.target.value)}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="share-password">Kata sandi penonton</Label>
            <div className="flex items-center gap-2">
              <Switch checked={usePassword} onCheckedChange={setUsePassword} aria-label="Gunakan kata sandi" />
              <Input
                id="share-password"
                type="password"
                className="h-9"
                disabled={!usePassword}
                value={password}
                onChange={(event) => setPassword(event.target.value)}
              />
            </div>
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="share-expires">Berakhir pada (opsional)</Label>
            <Input
              id="share-expires"
              type="date"
              className="h-9"
              value={expires}
              onChange={(event) => setExpires(event.target.value)}
            />
          </div>
          <div className="flex flex-col gap-1.5 sm:col-span-2">
            <Label htmlFor="share-domains">Domain yang diizinkan (pisahkan dengan koma)</Label>
            <Input
              id="share-domains"
              className="h-9"
              placeholder="https://partner.example.com, https://app.example.com"
              value={domains}
              onChange={(event) => setDomains(event.target.value)}
            />
          </div>
        </div>

        <Button
          size="sm"
          className="w-fit"
          disabled={create.isPending || !envTag}
          onClick={() =>
            create.mutate({
              access_mode: mode,
              env_tag: envTag,
              password: usePassword ? password : undefined,
              allowed_domains: domains
                .split(",")
                .map((domain) => domain.trim())
                .filter(Boolean)
                // The API only accepts absolute origins, so a bare host is assumed https.
                .map((domain) => (domain.includes("://") ? domain : `https://${domain}`)),
              expires_at: expires ? new Date(`${expires}T23:59:59Z`).toISOString() : undefined,
            })
          }
        >
          {create.isPending && <Loader2 className="mr-2 size-4 animate-spin" />}
          Buat tautan
        </Button>

        {created?.share_url && <CopyField label="URL publik" value={created.share_url} />}
        {created?.embed_token && (
          <CopyField
            label="Embed token (hanya tampil sekali)"
            value={`${window.location.origin}/embed?token=${created.embed_token}`}
          />
        )}

        <div className="flex max-h-52 flex-col gap-2 overflow-auto">
          {(shares.data ?? []).map((share) => (
            <div
              key={share.id}
              className="flex items-center gap-2 rounded-md border border-border px-3 py-2 text-xs"
            >
              <Badge variant="secondary">{share.access_mode}</Badge>
              <span className="font-mono text-muted-foreground">
                {share.share_token.slice(0, 12)}…
              </span>
              <span className="text-muted-foreground">env {share.env_tag}</span>
              {share.password_protected && <Badge variant="outline">password</Badge>}
              {!share.is_active && <Badge variant="outline">nonaktif</Badge>}
              <span className="ml-auto flex items-center gap-1">
                <CopyFieldCompact value={`${window.location.origin}/share/${share.share_token}`} />
                {share.is_active && (
                  <Button
                    size="icon"
                    variant="ghost"
                    className="size-7 text-destructive"
                    onClick={() => revoke.mutate(share.id)}
                  >
                    <Trash2 className="size-3.5" />
                  </Button>
                )}
              </span>
            </div>
          ))}
        </div>
      </DialogContent>
    </Dialog>
  );
}

function CopyField({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-1">
      <Label className="text-[11px] text-muted-foreground">{label}</Label>
      <div className="flex gap-1.5">
        <Input readOnly value={value} className="h-8 font-mono text-xs" onFocus={(event) => event.target.select()} />
        <CopyButton value={value} />
      </div>
    </div>
  );
}

function CopyFieldCompact({ value }: { value: string }) {
  return <CopyButton value={value} compact />;
}

function CopyButton({ value, compact }: { value: string; compact?: boolean }) {
  const [copied, setCopied] = useState(false);
  return (
    <Button
      size="icon"
      variant={compact ? "ghost" : "outline"}
      className={compact ? "size-7" : "size-8 shrink-0"}
      onClick={async () => {
        await navigator.clipboard.writeText(value);
        setCopied(true);
        window.setTimeout(() => setCopied(false), 1500);
      }}
      aria-label="Salin"
    >
      {copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
    </Button>
  );
}
