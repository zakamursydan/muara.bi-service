"use client";

import { Building2, Sparkles, Users } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { AISettingsForm } from "@/components/settings/ai-settings-form";
import { MembersPanel } from "@/components/settings/members-panel";
import { useAISettingsQuery } from "@/lib/queries";
import { useAuthStore } from "@/stores/auth-store";
import { useProjectStore } from "@/stores/project-store";
import { useCan, useWorkspaceStore } from "@/stores/workspace-store";

export default function SettingsPage() {
  const workspaceId = useWorkspaceStore((state) => state.workspaceId) ?? "";
  const access = useWorkspaceStore((state) => state.access);
  const workspaces = useAuthStore((state) => state.workspaces);
  const user = useAuthStore((state) => state.user);
  const project = useProjectStore((state) => state.project);

  const canWriteConnections = useCan("connection:write");
  const canManageMembers = useCan("member:manage");

  const workspace = workspaces.find((item) => item.id === workspaceId);
  const settingsQuery = useAISettingsQuery(workspaceId);

  return (
    <div className="flex flex-col gap-4">
      <div>
        <h1 className="text-lg font-semibold text-foreground">Pengaturan</h1>
        <p className="text-sm text-muted-foreground">
          Konfigurasi AI Co-Pilot dan keanggotaan project untuk workspace ini.
        </p>
      </div>

      <Card className="flex flex-wrap items-center gap-x-6 gap-y-2 px-4 py-3">
        <div className="flex items-center gap-2">
          <Building2 className="size-4 text-muted-foreground" />
          <span className="text-sm font-medium text-foreground">{workspace?.name ?? "Workspace"}</span>
          {workspace?.planTier && (
            <Badge variant="secondary" className="text-[10px] uppercase">
              {workspace.planTier}
            </Badge>
          )}
        </div>
        <div className="text-xs text-muted-foreground">
          {user?.fullName} · {user?.email}
        </div>
        {access?.workspace_role && (
          <Badge variant="outline" className="text-[10px]">
            {access.workspace_role}
          </Badge>
        )}
      </Card>

      <Tabs defaultValue="ai" className="flex flex-col gap-3">
        <TabsList className="w-fit">
          <TabsTrigger value="ai" className="gap-1.5">
            <Sparkles className="size-3.5" />
            AI Co-Pilot
          </TabsTrigger>
          <TabsTrigger value="members" className="gap-1.5">
            <Users className="size-3.5" />
            Keanggotaan &amp; RLS
          </TabsTrigger>
        </TabsList>

        <TabsContent value="ai">
          <Card className="p-4">
            {settingsQuery.isLoading ? (
              <div className="flex flex-col gap-3">
                {Array.from({ length: 4 }).map((_, index) => (
                  <Skeleton key={index} className="h-9 rounded-md" />
                ))}
              </div>
            ) : settingsQuery.data ? (
              <AISettingsForm
                key={settingsQuery.data.workspace_id}
                workspaceId={workspaceId}
                settings={settingsQuery.data}
                canWrite={canWriteConnections}
              />
            ) : (
              <p className="text-xs text-muted-foreground">
                Pengaturan AI belum dapat dimuat. Muat ulang halaman setelah memilih workspace.
              </p>
            )}
          </Card>
        </TabsContent>

        <TabsContent value="members">
          <Card className="p-4">
            {project ? (
              <div className="flex flex-col gap-3">
                <div>
                  <h2 className="text-sm font-semibold text-foreground">
                    Anggota {project.name}
                  </h2>
                  <p className="text-xs text-muted-foreground">
                    Aturan RLS dibinding sebagai parameter, bukan string yang digabungkan ke SQL.
                  </p>
                </div>
                <MembersPanel
                  key={project.id}
                  workspaceId={workspaceId}
                  projectId={project.id}
                  canManage={canManageMembers}
                />
              </div>
            ) : (
              <p className="text-xs text-muted-foreground">
                Pilih project di header untuk mengelola anggotanya.
              </p>
            )}
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
