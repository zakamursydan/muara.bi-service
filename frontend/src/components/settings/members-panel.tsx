"use client";

import { useState } from "react";
import { Loader2, UserPlus, X } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { useAssignMember } from "@/lib/mutations";
import { useProjectMembersQuery } from "@/lib/queries";
import type * as T from "@/types/api";

const roles: T.ProjectRole[] = ["ADMIN", "EDITOR", "VIEWER"];
const operators = ["=", "!=", "in", "not in", "<", ">"];

/** A rule row is edited as strings and typed only when it is sent. */
type RuleDraft = { column: string; operator: string; value: string };

export function MembersPanel({
  workspaceId,
  projectId,
  canManage,
}: {
  workspaceId: string;
  projectId: string;
  canManage: boolean;
}) {
  const membersQuery = useProjectMembersQuery(workspaceId, projectId);
  const assign = useAssignMember(workspaceId, projectId);

  const [userId, setUserId] = useState("");
  const [role, setRole] = useState<T.ProjectRole>("VIEWER");
  const [rules, setRules] = useState<RuleDraft[]>([]);

  const members = membersQuery.data ?? [];

  const body: T.AssignProjectMemberRequest = {
    user_id: userId.trim(),
    role,
    rls_rules: rules
      .filter((rule) => rule.column.trim().length > 0)
      .map((rule) => ({
        column: rule.column.trim(),
        operator: rule.operator,
        value: coerce(rule.value),
      })),
  };

  async function submit() {
    await assign.mutateAsync(body);
    setUserId("");
    setRules([]);
    setRole("VIEWER");
  }

  return (
    <div className="flex flex-col gap-4">
      {membersQuery.isLoading ? (
        <p className="text-xs text-muted-foreground">Memuat anggota…</p>
      ) : members.length === 0 ? (
        <p className="text-xs text-muted-foreground">
          Belum ada anggota project selain pemilik. Assign user workspace lewat form di bawah.
        </p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>User</TableHead>
              <TableHead>Peran</TableHead>
              <TableHead>Baris filter (RLS)</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {members.map((member) => (
              <TableRow key={member.id}>
                <TableCell className="max-w-[16rem] truncate">
                  <span className="text-sm text-foreground">{member.user_full_name}</span>
                  <span className="block text-[11px] text-muted-foreground">{member.user_email}</span>
                </TableCell>
                <TableCell>
                  <Badge variant="outline" className="text-[10px]">
                    {member.role}
                  </Badge>
                </TableCell>
                <TableCell className="text-[11px] text-muted-foreground">
                  {member.rls_rules.length
                    ? member.rls_rules
                        .map((rule) => `${rule.column} ${rule.operator} ${String(rule.value ?? "")}`)
                        .join(" · ")
                    : "Tanpa filter baris"}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      {canManage && (
        <div className="flex flex-col gap-3 rounded-lg border border-border bg-secondary/40 p-3">
          <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_140px]">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="member-id">User ID workspace</Label>
              <Input
                id="member-id"
                value={userId}
                placeholder="uuid anggota"
                onChange={(event) => setUserId(event.target.value)}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="member-role">Peran</Label>
              <Select value={role} onValueChange={(next) => setRole(next as T.ProjectRole)}>
                <SelectTrigger id="member-role" className="h-8">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {roles.map((item) => (
                    <SelectItem key={item} value={item}>
                      {item}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className="flex flex-col gap-2">
            <div className="flex items-center justify-between">
              <Label className="text-xs">Aturan row-level security</Label>
              <Button
                size="sm"
                variant="ghost"
                className="h-7 gap-1 text-[11px]"
                onClick={() => setRules((state) => [...state, { column: "", operator: "=", value: "" }])}
              >
                <UserPlus className="size-3" />
                Tambah aturan
              </Button>
            </div>
            {rules.map((rule, index) => (
              <div key={index} className="flex items-center gap-2">
                <Input
                  className="h-8"
                  placeholder="branch_code"
                  value={rule.column}
                  onChange={(event) =>
                    setRules((state) =>
                      state.map((item, i) => (i === index ? { ...item, column: event.target.value } : item)),
                    )
                  }
                />
                <Select
                  value={rule.operator}
                  onValueChange={(next) =>
                    setRules((state) =>
                      state.map((item, i) => (i === index ? { ...item, operator: next } : item)),
                    )
                  }
                >
                  <SelectTrigger className="h-8 w-[110px]">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {operators.map((item) => (
                      <SelectItem key={item} value={item}>
                        {item}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <Input
                  className="h-8"
                  placeholder="BRG-01"
                  value={rule.value}
                  onChange={(event) =>
                    setRules((state) =>
                      state.map((item, i) => (i === index ? { ...item, value: event.target.value } : item)),
                    )
                  }
                />
                <Button
                  size="icon"
                  variant="ghost"
                  className="size-8 shrink-0 text-destructive"
                  onClick={() => setRules((state) => state.filter((_, i) => i !== index))}
                >
                  <X className="size-4" />
                </Button>
              </div>
            ))}
          </div>

          <Button
            size="sm"
            className="w-fit"
            disabled={!body.user_id || assign.isPending}
            onClick={submit}
          >
            {assign.isPending && <Loader2 className="mr-1 size-4 animate-spin" />}
            Assign anggota
          </Button>
        </div>
      )}
    </div>
  );
}

/** Numeric and list values are bound as typed parameters by the query engine. */
function coerce(value: string): unknown {
  const trimmed = value.trim();
  if (!trimmed) return null;
  if (/^-?\d+(\.\d+)?$/.test(trimmed)) return Number(trimmed);
  if (trimmed.includes(",")) return trimmed.split(",").map((item) => item.trim());
  return trimmed;
}
