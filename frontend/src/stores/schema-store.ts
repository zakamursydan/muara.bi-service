import { create } from "zustand";
import type * as T from "@/types/api";

type SchemaEntry = {
  alias: string;
  envTag: string;
  tables: T.SchemaMap;
  lastSyncedAt?: string;
};

type SchemaState = {
  entries: Record<string, SchemaEntry>;
  /** Keyed lookup is `${alias}::${envTag}` because tags are per connection. */
  ingest: (connections: T.Connection[]) => void;
  forEnv: (envTag: string) => SchemaEntry[];
  columnsOf: (alias: string, envTag: string, table: string) => T.ColumnMeta[];
  search: (envTag: string, query: string) => { alias: string; table: string; column?: T.ColumnMeta }[];
  reset: () => void;
};

const keyOf = (alias: string, envTag: string) => `${alias}::${envTag}`;

export const useSchemaStore = create<SchemaState>((set, get) => ({
  entries: {},

  ingest: (connections) =>
    set((state) => {
      const next = { ...state.entries };
      for (const connection of connections) {
        for (const env of connection.environments ?? []) {
          if (!env.cached_schema) continue;
          next[keyOf(connection.alias_name, env.env_tag)] = {
            alias: connection.alias_name,
            envTag: env.env_tag,
            tables: env.cached_schema,
            lastSyncedAt: env.last_synced_at,
          };
        }
      }
      return { entries: next };
    }),

  forEnv: (envTag) =>
    Object.values(get().entries).filter((entry) => entry.envTag === envTag),

  columnsOf: (alias, envTag, table) =>
    get().entries[keyOf(alias, envTag)]?.tables[table] ?? [],

  search: (envTag, query) => {
    const needle = query.trim().toLowerCase();
    const hits: { alias: string; table: string; column?: T.ColumnMeta }[] = [];

    for (const entry of get().forEnv(envTag)) {
      for (const [table, columns] of Object.entries(entry.tables)) {
        if (!needle || table.toLowerCase().includes(needle)) {
          hits.push({ alias: entry.alias, table });
          continue;
        }
        for (const column of columns) {
          if (column.name.toLowerCase().includes(needle)) {
            hits.push({ alias: entry.alias, table, column });
            break;
          }
        }
      }
    }

    return hits;
  },

  reset: () => set({ entries: {} }),
}));
