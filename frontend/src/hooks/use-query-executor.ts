"use client";

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { queryKeys } from "@/lib/queries";
import { useRefreshStore } from "@/stores/refresh-store";
import type * as T from "@/types/api";

export type QueryInput = {
  workspaceId?: string;
  projectId?: string;
  envTag?: string;
  sql?: string;
  parameters?: Record<string, unknown>;
  cacheTtlSeconds?: number;
  maxRows?: number;
  enabled?: boolean;
};

/**
 * Query keys already contain the nonce, so a bumped key has never been fetched.
 * Only that first fetch bypasses the Redis cache; later refetches of the same
 * key go through it again.
 */
const fetchedKeys = new Set<string>();

export function useQueryExecutor(input: QueryInput): UseQueryResult<T.ExecuteQueryResponse> {
  const nonce = useRefreshStore((state) => state.nonce);
  const parameters = input.parameters ?? {};

  const ready =
    input.enabled !== false &&
    Boolean(input.workspaceId && input.projectId && input.envTag && input.sql);

  const key = queryKeys.query({
    wsId: input.workspaceId ?? "",
    envTag: input.envTag ?? "",
    sql: input.sql ?? "",
    parameters,
    nonce,
  });

  return useQuery({
    queryKey: key,
    queryFn: () => {
      const forceRefresh = !fetchedKeys.has(key.join("|"));
      fetchedKeys.add(key.join("|"));
      return api.query.execute(input.workspaceId as string, {
        project_id: input.projectId as string,
        env_tag: input.envTag as string,
        raw_sql: input.sql as string,
        parameters,
        cache_ttl: input.cacheTtlSeconds ?? 300,
        max_rows: input.maxRows ?? 10_000,
        force_refresh: forceRefresh,
      });
    },
    enabled: ready,
    staleTime: 30_000,
    retry: 0,
  });
}
