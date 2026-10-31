"use client";

import { useEffect, useMemo } from "react";
import { useRouter } from "next/navigation";
import { useTheme } from "next-themes";
import { LogOut, Monitor, Moon, RefreshCw, Sun, UserRound } from "lucide-react";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { collectEnvTags, useConnectionsQuery, useProjectsQuery } from "@/lib/queries";
import { useAuthStore } from "@/stores/auth-store";
import { useEnvStore } from "@/stores/env-store";
import { useFilterStore } from "@/stores/filter-store";
import { useProjectStore } from "@/stores/project-store";
import { useRefreshStore } from "@/stores/refresh-store";
import { useWorkspaceStore } from "@/stores/workspace-store";
import type * as T from "@/types/api";

/** Matches the PLAN header spec: production red, staging amber, development green. */
function envBadgeClass(tag: string): string {
  const normalized = tag.toLowerCase();
  if (normalized.includes("prod")) return "bg-red-500/15 text-red-600 dark:text-red-400";
  if (normalized.includes("staging") || normalized.includes("uat"))
    return "bg-amber-500/15 text-amber-600 dark:text-amber-400";
  if (normalized.includes("dev") || normalized.includes("test"))
    return "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400";
  return "bg-secondary text-secondary-foreground";
}

export function AppHeader() {
  const router = useRouter();
  const { resolvedTheme, setTheme } = useTheme();

  const user = useAuthStore((state) => state.user);
  const workspaces = useAuthStore((state) => state.workspaces);
  const logout = useAuthStore((state) => state.logout);

  const workspaceId = useWorkspaceStore((state) => state.workspaceId);
  const setActiveWorkspace = useWorkspaceStore((state) => state.setActiveWorkspace);
  const loadAccess = useWorkspaceStore((state) => state.loadAccess);

  const project = useProjectStore((state) => state.project);
  const setProject = useProjectStore((state) => state.setProject);

  const envTag = useEnvStore((state) => state.envTag);
  const setEnvTag = useEnvStore((state) => state.setEnvTag);

  const bumpRefresh = useRefreshStore((state) => state.bump);
  const resetFilters = useFilterStore((state) => state.reset);

  const projectsQuery = useProjectsQuery(workspaceId ?? undefined);
  const connectionsQuery = useConnectionsQuery(workspaceId ?? undefined);
  const envs = useMemo(() => collectEnvTags(connectionsQuery.data), [connectionsQuery.data]);
  // Stable identities: the auto-selection effects below depend on these arrays.
  const projects: T.Project[] = useMemo(() => projectsQuery.data ?? [], [projectsQuery.data]);

  // First tenant/project selection is remembered locally; fall back to the
  // first available so a fresh login always has a working scope.
  useEffect(() => {
    if (!workspaceId && workspaces.length) setActiveWorkspace(workspaces[0].id);
  }, [setActiveWorkspace, workspaces, workspaceId]);

  useEffect(() => {
    if (!workspaceId) return;
    if (!projects.length) {
      if (project) setProject(null);
      return;
    }
    if (!project || !projects.some((item) => item.id === project.id)) setProject(projects[0]);
  }, [project, projects, setProject, workspaceId]);

  useEffect(() => {
    if (!envTag && envs.length) {
      const preferred = envs.find((env) => env.is_default) ?? envs[0];
      setEnvTag(preferred.env_tag);
    }
  }, [envs, envTag, setEnvTag]);

  useEffect(() => {
    if (workspaceId) void loadAccess(workspaceId, project?.id);
  }, [loadAccess, project?.id, workspaceId]);

  return (
    <header className="flex h-14 shrink-0 items-center gap-2 border-b border-border bg-card px-3">
      <Select
        value={workspaceId ?? undefined}
        onValueChange={(next) => {
          setActiveWorkspace(next);
          setProject(null);
        }}
      >
        <SelectTrigger className="h-8 w-[190px]" aria-label="Workspace">
          <SelectValue placeholder="Pilih workspace" />
        </SelectTrigger>
        <SelectContent>
          {workspaces.map((workspace) => (
            <SelectItem key={workspace.id} value={workspace.id}>
              {workspace.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <Select
        value={project?.id ?? undefined}
        onValueChange={(next) => {
          const found = projects.find((item) => item.id === next);
          if (found) setProject(found);
        }}
        disabled={!projects.length}
      >
        <SelectTrigger className="h-8 w-[190px]" aria-label="Project">
          <SelectValue placeholder={projects.length ? "Pilih project" : "Belum ada project"} />
        </SelectTrigger>
        <SelectContent>
          {projects.map((item) => (
            <SelectItem key={item.id} value={item.id}>
              {item.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <div className="flex items-center gap-1.5">
        <Select
          value={envTag ?? undefined}
          onValueChange={setEnvTag}
          disabled={!envs.length}
        >
          <SelectTrigger className="h-8 w-[160px]" aria-label="Environment">
            <SelectValue placeholder={envs.length ? "Pilih environment" : "Tidak ada environment"} />
          </SelectTrigger>
          <SelectContent>
            {envs.map((env) => (
              <SelectItem key={env.env_tag} value={env.env_tag}>
                <span className="flex items-center gap-2">
                  {env.env_tag}
                  <Badge variant="secondary" className={envBadgeClass(env.env_tag)}>
                    env
                  </Badge>
                </span>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {envTag && (
          <Badge variant="secondary" className={envBadgeClass(envTag)}>
            {envTag}
          </Badge>
        )}
      </div>

      <div className="ml-auto flex items-center gap-1">
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="outline"
              size="icon"
              className="size-8"
              onClick={() => {
                resetFilters();
                bumpRefresh();
              }}
            >
              <RefreshCw className="size-4" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>Reload data semua widget (lewati cache)</TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="size-8"
              onClick={() => setTheme(resolvedTheme === "dark" ? "light" : "dark")}
            >
              {resolvedTheme === "dark" ? (
                <Sun className="size-4" />
              ) : (
                <Moon className="size-4" />
              )}
            </Button>
          </TooltipTrigger>
          <TooltipContent>Ganti tema terang / gelap</TooltipContent>
        </Tooltip>

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="size-8">
              <Avatar className="size-7">
                <AvatarFallback className="text-[11px]">
                  {user?.fullName?.slice(0, 2).toUpperCase() ?? <UserRound className="size-3.5" />}
                </AvatarFallback>
              </Avatar>
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-56">
            <DropdownMenuLabel className="flex flex-col">
              <span className="text-sm font-medium">{user?.fullName}</span>
              <span className="text-xs font-normal text-muted-foreground">{user?.email}</span>
            </DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={() => router.push("/settings")}>
              <Monitor className="mr-2 size-4" />
              Pengaturan workspace
            </DropdownMenuItem>
            <DropdownMenuItem
              variant="destructive"
              onClick={() => {
                logout();
                router.replace("/login");
              }}
            >
              <LogOut className="mr-2 size-4" />
              Keluar
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </header>
  );
}
