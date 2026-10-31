"use client";

import { useState } from "react";
import { CheckCircle2, Loader2, Plug, RefreshCw, ShieldCheck, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { useSyncSchema, useTestConnection, useUpsertEnvironment } from "@/lib/mutations";
import type * as T from "@/types/api";

const badgeColors: T.BadgeColor[] = ["blue", "green", "red", "yellow", "purple", "orange"];

const dotColor: Record<T.BadgeColor, string> = {
  blue: "bg-blue-500",
  green: "bg-emerald-500",
  red: "bg-red-500",
  yellow: "bg-amber-500",
  purple: "bg-violet-500",
  orange: "bg-orange-500",
};

type EnvironmentFormProps = {
  workspaceId: string;
  connection: T.Connection;
  /** Undefined while drafting a new environment. */
  environment?: T.EnvironmentInfo;
  onDone: (envTag: string) => void;
  onClose: () => void;
};

/**
 * One form for both create and edit. The parent keys it on the environment so a
 * switch always remounts with a fresh copy instead of copying props into state.
 */
export function EnvironmentForm({
  workspaceId,
  connection,
  environment,
  onDone,
  onClose,
}: EnvironmentFormProps) {
  const upsert = useUpsertEnvironment(workspaceId);
  const test = useTestConnection(workspaceId);
  const sync = useSyncSchema(workspaceId);

  const [envTag, setEnvTag] = useState(environment?.env_tag ?? "");
  const [displayName, setDisplayName] = useState(environment?.display_name ?? "");
  const [badgeColor, setBadgeColor] = useState<T.BadgeColor>(environment?.badge_color ?? "blue");
  const [isDefault, setIsDefault] = useState(environment?.is_default ?? false);
  const [host, setHost] = useState(environment?.host ?? "");
  const [port, setPort] = useState(String(environment?.port ?? 5432));
  const [database, setDatabase] = useState(environment?.database ?? "");
  const [user, setUser] = useState(environment?.user ?? "");
  const [password, setPassword] = useState("");
  const [sslMode, setSslMode] = useState("prefer");
  const [uri, setUri] = useState("");
  const [ssh, setSsh] = useState(environment?.ssh_tunnel_enabled ?? false);
  const [sshHost, setSshHost] = useState("");
  const [sshPort, setSshPort] = useState("22");
  const [sshUser, setSshUser] = useState("");
  const [sshPassword, setSshPassword] = useState("");
  const [sshKey, setSshKey] = useState("");

  // Only a saved environment has stored credentials for the API to dial.
  const [savedTag, setSavedTag] = useState(environment?.env_tag);
  const [lastTest, setLastTest] = useState<T.TestConnectionResponse | null>(null);

  const body: T.UpsertEnvironmentRequest = {
    env_tag: envTag.trim(),
    display_name: displayName.trim() || envTag.trim(),
    badge_color: badgeColor,
    is_default: isDefault,
    host: host.trim(),
    port: Number(port) || 0,
    database: database.trim(),
    user: user.trim(),
    password,
    ssl_mode: sslMode,
    uri: uri.trim(),
    ssh_tunnel_enabled: ssh,
    ssh_host: sshHost.trim(),
    ssh_port: Number(sshPort) || 22,
    ssh_user: sshUser.trim(),
    ssh_password: sshPassword,
    ssh_private_key: sshKey,
  };

  const canSave = body.env_tag.length > 0 && (body.uri.length > 0 || body.host.length > 0);

  async function save() {
    await upsert.mutateAsync({ connectionId: connection.id, body });
    setSavedTag(body.env_tag);
    onDone(body.env_tag);
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-start justify-between gap-2">
        <div>
          <h2 className="text-sm font-semibold text-foreground">
            {environment ? "Ubah environment" : "Environment baru"}
          </h2>
          <p className="text-xs text-muted-foreground">
            {connection.alias_name} · {connection.driver_type}
          </p>
        </div>
        <Button variant="ghost" size="icon" className="size-7" onClick={onClose}>
          <X className="size-4" />
        </Button>
      </div>

      <div className="grid gap-3 sm:grid-cols-2">
        <TextField
          id="env-tag"
          label="Tag environment"
          placeholder="production"
          value={envTag}
          onChange={setEnvTag}
          disabled={Boolean(environment)}
        />
        <TextField
          id="env-display"
          label="Nama tampilan"
          placeholder="Production"
          value={displayName}
          onChange={setDisplayName}
        />
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="env-badge">Warna badge</Label>
          <div className="flex items-center gap-2">
            <span className={dotClass(badgeColor)} />
            <Select value={badgeColor} onValueChange={(next) => setBadgeColor(next as T.BadgeColor)}>
              <SelectTrigger id="env-badge" className="h-8">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {badgeColors.map((color) => (
                  <SelectItem key={color} value={color}>
                    {color}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
        <div className="flex items-end gap-2 pb-1">
          <Switch id="env-default" checked={isDefault} onCheckedChange={setIsDefault} />
          <Label htmlFor="env-default" className="text-xs">
            Jadikan default
          </Label>
        </div>
      </div>

      <Separator />

      <div className="grid gap-3 sm:grid-cols-2">
        <TextField id="db-host" label="Host" placeholder="127.0.0.1" value={host} onChange={setHost} />
        <TextField id="db-port" label="Port" type="number" value={port} onChange={setPort} />
        <TextField
          id="db-name"
          label="Database"
          placeholder="analytics"
          value={database}
          onChange={setDatabase}
        />
        <TextField id="db-user" label="Pengguna" value={user} onChange={setUser} />
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="db-password">Kata sandi</Label>
          <Input
            id="db-password"
            type="password"
            autoComplete="new-password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
          {environment && (
            <p className="text-[11px] text-muted-foreground">
              Isi ulang kata sandi: penyimpanan kredensial menggantikan seluruh konfigurasi, jadi nilai
              kosong akan menghapus sandi lama.
            </p>
          )}
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="db-ssl">SSL mode</Label>
          <Select value={sslMode} onValueChange={setSslMode}>
            <SelectTrigger id="db-ssl" className="h-8">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {["disable", "allow", "prefer", "require", "verify-ca", "verify-full"].map((mode) => (
                <SelectItem key={mode} value={mode}>
                  {mode}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
      <TextField
        id="db-uri"
        label="URI (opsional, menggantikan host/port)"
        placeholder="mongodb://user:pass@host:27017"
        value={uri}
        onChange={setUri}
      />

      <details className="rounded-lg border border-border bg-secondary/40 px-3 py-2">
        <summary className="flex cursor-pointer items-center gap-2 text-sm font-medium text-foreground">
          <ShieldCheck className="size-4 text-muted-foreground" />
          SSH bastion tunnel
          <span className="text-[11px] font-normal text-muted-foreground">
            {ssh ? "aktif" : "mati — koneksi langsung"}
          </span>
        </summary>
        <div className="mt-3 flex flex-col gap-3">
          <div className="flex items-center gap-2">
            <Switch id="ssh-enabled" checked={ssh} onCheckedChange={setSsh} />
            <Label htmlFor="ssh-enabled" className="text-xs">
              Sambungkan lewat bastion
            </Label>
          </div>
          {ssh && (
            <div className="grid gap-3 sm:grid-cols-2">
              <TextField id="ssh-host" label="Bastion host" value={sshHost} onChange={setSshHost} />
              <TextField id="ssh-port" label="Bastion port" type="number" value={sshPort} onChange={setSshPort} />
              <TextField id="ssh-user" label="Bastion user" value={sshUser} onChange={setSshUser} />
              <TextField
                id="ssh-password"
                label="Bastion password"
                type="password"
                autoComplete="new-password"
                value={sshPassword}
                onChange={setSshPassword}
              />
              <div className="flex flex-col gap-1.5 sm:col-span-2">
                <Label htmlFor="ssh-key">Private key (PEM)</Label>
                <textarea
                  id="ssh-key"
                  rows={4}
                  spellCheck={false}
                  className="w-full resize-y rounded-md border border-input bg-transparent px-3 py-2 font-mono text-xs outline-none focus-visible:border-ring"
                  value={sshKey}
                  onChange={(event) => setSshKey(event.target.value)}
                />
              </div>
            </div>
          )}
        </div>
      </details>

      {lastTest && (
        <div
          className={`flex items-center gap-2 rounded-lg border px-3 py-2 text-xs ${
            lastTest.success
              ? "border-emerald-200 bg-emerald-50 text-emerald-700"
              : "border-destructive/40 bg-destructive/5 text-destructive"
          }`}
        >
          <CheckCircle2 className="size-3.5 shrink-0" />
          <span className="min-w-0 flex-1 truncate">{lastTest.message}</span>
          <span className="tabular-nums">{lastTest.latency_ms} ms</span>
        </div>
      )}

      <div className="flex flex-wrap items-center gap-2">
        <Button size="sm" onClick={save} disabled={!canSave || upsert.isPending}>
          {upsert.isPending && <Loader2 className="mr-1 size-4 animate-spin" />}
          Simpan
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={!savedTag || test.isPending}
          title={savedTag ? "Uji koneksi tersimpan" : "Simpan environment dulu sebelum menguji"}
          onClick={async () => {
            if (!savedTag) return;
            setLastTest(await test.mutateAsync({ connectionId: connection.id, envTag: savedTag }));
          }}
        >
          <Plug className="mr-1 size-4" />
          {test.isPending ? "Menguji…" : "Uji koneksi"}
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={!savedTag || sync.isPending}
          onClick={async () => {
            if (!savedTag) return;
            await sync.mutateAsync({ connectionId: connection.id, envTag: savedTag });
          }}
        >
          <RefreshCw className="mr-1 size-4" />
          {sync.isPending ? "Menyinkronkan…" : "Sinkron schema"}
        </Button>
      </div>
    </div>
  );
}

function TextField({
  id,
  label,
  type = "text",
  value,
  onChange,
  placeholder,
  disabled,
  autoComplete,
}: {
  id: string;
  label: string;
  type?: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  disabled?: boolean;
  autoComplete?: string;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      <Input
        id={id}
        type={type}
        value={value}
        placeholder={placeholder}
        disabled={disabled}
        autoComplete={autoComplete ?? (type === "password" ? "new-password" : "off")}
        onChange={(event) => onChange(event.target.value)}
      />
    </div>
  );
}

function dotClass(color: T.BadgeColor) {
  return `size-2.5 shrink-0 rounded-full ${dotColor[color]}`;
}
