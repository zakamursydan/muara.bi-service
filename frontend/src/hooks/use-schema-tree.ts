"use client";

import { useEffect, useMemo } from "react";
import { useConnectionsQuery, useSemanticModelsQuery } from "@/lib/queries";
import { useSchemaStore } from "@/stores/schema-store";
import type * as T from "@/types/api";

export type SchemaTable = {
  /** Qualified name used in SQL, e.g. `db_transaksi.orders` or a virtual view. */
  qualified: string;
  alias: string;
  table: string;
  kind: "connection" | "semantic-model";
  columns: T.ColumnMeta[];
};

/**
 * Materialises the tenant schema for one environment: physical tables from each
 * connection's discovered schema plus the project's semantic models, which the
 * backend mounts as DuckDB temporary views.
 */
export function useSchemaTree(workspaceId?: string, envTag?: string, projectId?: string) {
  const connections = useConnectionsQuery(workspaceId);
  const semanticModels = useSemanticModelsQuery(workspaceId, projectId);
  const ingest = useSchemaStore((state) => state.ingest);

  useEffect(() => {
    if (connections.data) ingest(connections.data);
  }, [connections.data, ingest]);

  const tables = useMemo<SchemaTable[]>(() => {
    if (!envTag) return [];
    const list: SchemaTable[] = [];

    for (const connection of connections.data ?? []) {
      const env = connection.environments?.find((e) => e.env_tag === envTag);
      for (const [table, columns] of Object.entries(env?.cached_schema ?? {})) {
        list.push({
          qualified: `${connection.alias_name}.${table}`,
          alias: connection.alias_name,
          table,
          kind: "connection",
          columns,
        });
      }
    }

    for (const model of semanticModels.data ?? []) {
      list.push({
        qualified: model.name,
        alias: "",
        table: model.name,
        kind: "semantic-model",
        columns: Object.values(model.cached_schema ?? {}).flat(),
      });
    }

    return list.sort((a, b) => a.qualified.localeCompare(b.qualified));
  }, [connections.data, semanticModels.data, envTag]);

  const aliases = useMemo(() => {
    const set = new Map<string, T.Connection>();
    for (const connection of connections.data ?? []) set.set(connection.alias_name, connection);
    return set;
  }, [connections.data]);

  return {
    tables,
    aliases,
    isLoading: connections.isLoading,
    connections: connections.data ?? [],
  };
}
