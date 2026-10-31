"use client";

import { useState } from "react";
import { Database, Plus, RefreshCw, Server } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
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
import { Skeleton } from "@/components/ui/skeleton";
import { EnvironmentForm } from "@/components/sources/environment-form";
import { useConnectionsQuery } from "@/lib/queries";
import { useCreateConnection } from "@/lib/mutations";
import { useCan, useWorkspaceStore } from "@/stores/workspace-store";
import { useEnvStore } from "@/stores/env-store";
import type * as T from "@/types/api";

const categories: T.ConnectionCategory[] = ["SQL", "NOSQL", "FILE", "CLOUD_DW"];
const drivers: T.ConnectionDriver[] = [
  "postgres",
  "mysql",
  "sqlite",
  "mongodb",
  "elasticsearch",
  "redis",
  "s3",
];

const dotColor: Record<T.BadgeColor, string> = {
  blue: "bg-blue-500",
  green: "bg-emerald-500",
  red: "bg-red-500",
  yellow: "bg-amber-500",
  purple: "bg-violet-500",
  orange: "bg-orange-500",
};

/** A connection plus the environment being edited; "" means the list is empty. */
type Selection = { connectionId: string; envTag: string } | null;

export default function SourcesPage() {
  const workspaceId = useWorkspaceStore((state) => state.workspaceId) ?? "";
  const canWrite = useCan("connection:write");
  const setEnvTag = useEnvStore((state) => state.setEnvTag);

  const connectionsQuery = useConnectionsQuery(workspaceId);
  const connections = connectionsQuery.data ?? [];

  const [createOpen, setCreateOpen] = useState(false);
  const [selection, setSelection] = useState<Selection>(null);

  const active =
    connections
      .find((connection) => connection.id === selection?.connectionId)
      ?.environments.find((env) => env.env_tag === selection?.envTag) ?? null;
  const activeConnection = connections.find(
    (connection) => connection.id === selection?.connectionId,
  );

  // The environment id is part of the key so a draft remounts once the saved row
  // arrives from the server, instead of staying an empty form in edit mode.
  const formKey = selection ? `${selection.connectionId}:${selection.envTag}:${active?.id ?? "draft"}` : "";

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-lg font-semibold text-foreground">Sumber data</h1>
          <p className="text-sm text-muted-foreground">
            {connections.length} koneksi · kredensial disimpan terenkripsi AES-256-GCM
          </p>
        </div>
        {canWrite && (
          <Button size="sm" className="h-8" onClick={() => setCreateOpen(true)}>
            <Plus className="mr-1 size-4" />
            Sumber data
          </Button>
        )}
      </div>

      {connectionsQuery.isLoading ? (
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 3 }).map((_, index) => (
            <Skeleton key={index} className="h-40 rounded-xl" />
          ))}
        </div>
      ) : connections.length === 0 ? (
        <Card className="flex flex-col items-center gap-3 border-dashed px-6 py-16 text-center">
          <Database className="size-6 text-muted-foreground" />
          <p className="text-sm text-muted-foreground">
            Belum ada sumber data. Buat koneksi lalu tambahkan environment
            (production, staging, sandbox) untuk mulai mengquery.
          </p>
          {canWrite && (
            <Button size="sm" onClick={() => setCreateOpen(true)}>
              Buat sumber data
            </Button>
          )}
        </Card>
      ) : (
        <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,460px)]">
          <div className="grid content-start gap-3 sm:grid-cols-2">
            {connections.map((connection) => (
              <ConnectionCard
                key={connection.id}
                connection={connection}
                selected={selection?.connectionId}
                canWrite={canWrite}
                onPick={(env) => setSelection({ connectionId: connection.id, envTag: env })}
              />
            ))}
          </div>

          <Card className="min-h-[24rem] overflow-auto p-4">
            {activeConnection && canWrite ? (
              <EnvironmentForm
                key={formKey}
                workspaceId={workspaceId}
                connection={activeConnection}
                environment={active ?? undefined}
                onDone={(envTag) => {
                  setSelection({ connectionId: activeConnection.id, envTag });
                  setEnvTag(envTag);
                }}
                onClose={() => setSelection(null)}
              />
            ) : (
              <div className="flex h-full flex-col items-center justify-center gap-3 text-center">
                <Server className="size-6 text-muted-foreground" />
                <p className="text-sm text-muted-foreground">
                  {canWrite
                    ? "Pilih environment pada kartu koneksi untuk mengubah kredensial, menguji koneksi, atau sinkron schema."
                    : "Hanya membaca. Anda tidak punya izin untuk mengubah sumber data."}
                </p>
              </div>
            )}
          </Card>
        </div>
      )}

      <CreateConnectionDialog
        workspaceId={workspaceId}
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreated={(connection) =>
          setSelection({ connectionId: connection.id, envTag: "production" })
        }
      />
    </div>
  );
}

function ConnectionCard({
  connection,
  selected,
  canWrite,
  onPick,
}: {
  connection: T.Connection;
  selected?: string;
  canWrite: boolean;
  onPick: (envTag: string) => void;
}) {
  return (
    <Card className="flex flex-col gap-2 p-4">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="truncate text-sm font-semibold text-foreground">
            {connection.alias_name}
          </p>
          <p className="text-[11px] text-muted-foreground">
            {connection.category} · {connection.driver_type}
          </p>
        </div>
        <Badge variant="secondary" className="shrink-0 text-[10px] capitalize">
          {connection.environments.length} env
        </Badge>
      </div>

      {connection.environments.length === 0 ? (
        <p className="rounded-md border border-dashed border-border px-2 py-3 text-center text-[11px] text-muted-foreground">
          {canWrite ? "Belum ada environment — tambahkan satu." : "Tidak ada environment."}
        </p>
      ) : (
        <ul className="flex flex-col gap-1">
          {connection.environments.map((env) => (
            <li key={env.id}>
              <button
                type="button"
                disabled={!canWrite}
                className={`flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs transition-colors hover:bg-accent/60 disabled:hover:bg-transparent ${
                  selected === env.env_tag ? "bg-accent" : ""
                }`}
                onClick={() => onPick(env.env_tag)}
              >
                <span className={`size-2 shrink-0 rounded-full ${dotColor[env.badge_color]}`} />
                <span className="min-w-0 flex-1 truncate font-medium text-foreground">
                  {env.display_name || env.env_tag}
                </span>
                {env.is_default && (
                  <Badge variant="outline" className="text-[10px]">
                    default
                  </Badge>
                )}
                <span className="shrink-0 text-[10px] text-muted-foreground">
                  {Object.keys(env.cached_schema ?? {}).length} tabel
                </span>
              </button>
              {env.last_synced_at && (
                <p className="pl-4 text-[10px] text-muted-foreground">
                  Sinkron terakhir{" "}
                  {new Date(env.last_synced_at).toLocaleString("id-ID", {
                    dateStyle: "medium",
                    timeStyle: "short",
                  })}
                </p>
              )}
            </li>
          ))}
        </ul>
      )}

      {canWrite && (
        <Button
          size="sm"
          variant="outline"
          className="mt-auto h-7 w-fit gap-1 text-[11px]"
          onClick={() => onPick("__new__")}
        >
          <Plus className="size-3" />
          Environment
        </Button>
      )}
    </Card>
  );
}

function CreateConnectionDialog({
  workspaceId,
  open,
  onOpenChange,
  onCreated,
}: {
  workspaceId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: (connection: T.Connection) => void;
}) {
  const [alias, setAlias] = useState("");
  const [category, setCategory] = useState<T.ConnectionCategory>("SQL");
  const [driver, setDriver] = useState<T.ConnectionDriver>("postgres");
  const create = useCreateConnection(workspaceId);

  async function submit() {
    const created = await create.mutateAsync({
      alias_name: alias.trim(),
      category,
      driver_type: driver,
    });
    setAlias("");
    onOpenChange(false);
    onCreated(created);
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Sumber data baru</DialogTitle>
          <DialogDescription>
            Nama alias dan driver bersifat tetap; kredensial koneksi ditambahkan
            per environment setelahnya.
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="conn-alias">Nama alias</Label>
            <Input
              id="conn-alias"
              value={alias}
              placeholder="dwh-core"
              onChange={(event) => setAlias(event.target.value)}
            />
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="conn-category">Kategori</Label>
              <Select
                value={category}
                onValueChange={(next) => setCategory(next as T.ConnectionCategory)}
              >
                <SelectTrigger id="conn-category" className="h-8">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {categories.map((item) => (
                    <SelectItem key={item} value={item}>
                      {item}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="conn-driver">Driver</Label>
              <Select value={driver} onValueChange={(next) => setDriver(next as T.ConnectionDriver)}>
                <SelectTrigger id="conn-driver" className="h-8">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {drivers.map((item) => (
                    <SelectItem key={item} value={item}>
                      {item}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
        </div>
        <div className="flex justify-end gap-2">
          <Button variant="ghost" disabled={create.isPending} onClick={() => onOpenChange(false)}>
            Batal
          </Button>
          <Button disabled={!alias.trim() || create.isPending} onClick={submit}>
            {create.isPending && <RefreshCw className="mr-1 size-4 animate-spin" />}
            Buat
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
