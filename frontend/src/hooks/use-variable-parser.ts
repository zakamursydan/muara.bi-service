import { useMemo } from "react";
import type * as T from "@/types/api";

const VARIABLE_PATTERN = /\{\{\s*([A-Za-z0-9_]+)\s*\}\}/g;

/** `{{param}}` tokens the backend binds as positional `$n` parameters. */
export function parseVariables(sql: string): string[] {
  const names = new Set<string>();
  for (const match of sql.matchAll(VARIABLE_PATTERN)) names.add(match[1]);
  return [...names];
}

/** Builder filters compile to `{{flt_<column>}}`, so their default value seeds the bar. */
export function builderDefaults(state?: T.WidgetBuilderState): Record<string, unknown> {
  const defaults: Record<string, unknown> = {};
  for (const filter of state?.filters ?? []) {
    const param = filter.parameter?.trim() || `flt_${filter.column.replace(/\./g, "_")}`;
    if (filter.value !== undefined && filter.value !== null) defaults[param] = filter.value;
  }
  return defaults;
}

export function useVariableParser(sql?: string, builderState?: T.WidgetBuilderState) {
  return useMemo(() => {
    const variables = sql ? parseVariables(sql) : [];
    const defaults = builderDefaults(builderState);
    const missing = variables.filter(
      (name) => defaults[name] === undefined && !name.startsWith("xf_"),
    );
    return { variables, defaults, missing };
  }, [sql, builderState]);
}
