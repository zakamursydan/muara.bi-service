"use client";

import { useMemo } from "react";
import { Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { SchemaTable } from "@/hooks/use-schema-tree";
import type * as T from "@/types/api";

const aggregations: T.BuilderMetric["aggregation"][] = [
  "SUM",
  "COUNT",
  "AVG",
  "MIN",
  "MAX",
  "COUNT_DISTINCT",
  "NONE",
];

const operators: T.BuilderFilter["operator"][] = [
  "=",
  "!=",
  ">",
  "<",
  ">=",
  "<=",
  "LIKE",
  "ILIKE",
];

type BuilderFormProps = {
  sources: SchemaTable[];
  state: T.WidgetBuilderState;
  onChange: (state: T.WidgetBuilderState) => void;
  disabled?: boolean;
};

/** Source key is the alias when present, otherwise the semantic model name. */
const sourceKey = (table: SchemaTable) =>
  table.alias ? `${table.alias}::${table.table}` : `model::${table.table}`;

const parseSourceKey = (key: string) => {
  const [alias, table] = key.split("::");
  return { alias: alias === "model" ? "" : alias, table };
};

export function BuilderForm({ sources, state, onChange, disabled }: BuilderFormProps) {
  const currentKey = state.datasource_alias
    ? `${state.datasource_alias}::${state.table}`
    : `model::${state.table}`;

  const selected = useMemo(
    () => sources.find((table) => sourceKey(table) === currentKey),
    [currentKey, sources],
  );

  // The backend only accepts fully qualified identifiers, so every column the
  // builder emits is prefixed with its datasource alias.
  const qualify = (column: string) =>
    [state.datasource_alias, state.table, column].filter(Boolean).join(".");

  const columns = selected?.columns ?? [];

  const pickSource = (key: string) => {
    const { alias, table } = parseSourceKey(key);
    onChange({ ...state, datasource_alias: alias, table, dimensions: [], metrics: [], filters: [] });
  };

  const toggleDimension = (column: string) => {
    const qualifiedName = qualify(column);
    const next = state.dimensions.includes(qualifiedName)
      ? state.dimensions.filter((item) => item !== qualifiedName)
      : [...state.dimensions, qualifiedName];
    onChange({ ...state, dimensions: next });
  };

  return (
    <div className={disabled ? "pointer-events-none opacity-60" : undefined}>
      <div className="flex flex-col gap-4 p-4">
        <div className="grid grid-cols-2 gap-3">
          <div className="flex flex-col gap-1.5">
            <Label>Sumber data</Label>
            <Select
              value={selected ? sourceKey(selected) : undefined}
              onValueChange={pickSource}
            >
              <SelectTrigger className="h-8">
                <SelectValue placeholder="Pilih tabel" />
              </SelectTrigger>
              <SelectContent>
                {sources.map((table) => (
                  <SelectItem key={sourceKey(table)} value={sourceKey(table)}>
                    {table.qualified}
                    {table.kind === "semantic-model" ? " (model)" : ""}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="builder-limit">Limit baris</Label>
            <Input
              id="builder-limit"
              type="number"
              className="h-8"
              value={state.limit ?? ""}
              placeholder="10000"
              onChange={(event) =>
                onChange({
                  ...state,
                  limit: event.target.value ? Number(event.target.value) : undefined,
                })
              }
            />
          </div>
        </div>

        <section className="flex flex-col gap-2">
          <Label>Dimensi (GROUP BY)</Label>
          {columns.length ? (
            <div className="flex max-h-32 flex-wrap gap-1.5 overflow-auto rounded-md border border-border p-2">
              {columns.map((column) => {
                const active = state.dimensions.includes(qualify(column.name));
                return (
                  <button
                    key={column.name}
                    type="button"
                    className={
                      "rounded-full border px-2 py-1 text-[11px] transition-colors " +
                      (active
                        ? "border-primary bg-primary/10 text-primary"
                        : "border-border text-muted-foreground hover:bg-secondary")
                    }
                    onClick={() => toggleDimension(column.name)}
                  >
                    {column.name}
                    <span className="ml-1 text-[10px] opacity-70">{column.type}</span>
                  </button>
                );
              })}
            </div>
          ) : (
            <p className="text-xs text-muted-foreground">
              Pilih sumber data; kolom muncul dari schema yang sudah disinkronkan.
            </p>
          )}
        </section>

        <section className="flex flex-col gap-2">
          <div className="flex items-center justify-between">
            <Label>Metrik</Label>
            <Button
              size="sm"
              variant="outline"
              className="h-7 text-xs"
              onClick={() =>
                onChange({
                  ...state,
                  metrics: [...state.metrics, { column: qualify(columns[0]?.name ?? ""), aggregation: "SUM" }],
                })
              }
              disabled={!columns.length}
            >
              <Plus className="mr-1 size-3" />
              Metrik
            </Button>
          </div>
          {state.metrics.map((metric, index) => (
            <div key={index} className="flex items-center gap-1.5">
              <Select
                value={metric.column}
                onValueChange={(next) => {
                  const metrics = state.metrics.slice();
                  metrics[index] = { ...metric, column: next };
                  onChange({ ...state, metrics });
                }}
              >
                <SelectTrigger className="h-8 flex-1">
                  <SelectValue placeholder="Kolom" />
                </SelectTrigger>
                <SelectContent>
                  {columns.map((column) => (
                    <SelectItem key={column.name} value={qualify(column.name)}>
                      {column.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Select
                value={metric.aggregation}
                onValueChange={(next) => {
                  const metrics = state.metrics.slice();
                  metrics[index] = { ...metric, aggregation: next as T.BuilderMetric["aggregation"] };
                  onChange({ ...state, metrics });
                }}
              >
                <SelectTrigger className="h-8 w-32">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {aggregations.map((aggregation) => (
                    <SelectItem key={aggregation} value={aggregation}>
                      {aggregation}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button
                size="icon"
                variant="ghost"
                className="size-8 text-destructive"
                onClick={() => onChange({ ...state, metrics: state.metrics.filter((_, i) => i !== index) })}
              >
                <Trash2 className="size-3.5" />
              </Button>
            </div>
          ))}
        </section>

        <section className="flex flex-col gap-2">
          <div className="flex items-center justify-between">
            <Label>{"Filter ({{param}} otomatis)"}</Label>
            <Button
              size="sm"
              variant="outline"
              className="h-7 text-xs"
              onClick={() =>
                onChange({
                  ...state,
                  filters: [...state.filters, { column: qualify(columns[0]?.name ?? ""), operator: "=" }],
                })
              }
              disabled={!columns.length}
            >
              <Plus className="mr-1 size-3" />
              Filter
            </Button>
          </div>
          {state.filters.map((filter, index) => (
            <div key={index} className="flex items-center gap-1.5">
              <Select
                value={filter.column}
                onValueChange={(next) => {
                  const filters = state.filters.slice();
                  filters[index] = { ...filter, column: next };
                  onChange({ ...state, filters });
                }}
              >
                <SelectTrigger className="h-8 flex-1">
                  <SelectValue placeholder="Kolom" />
                </SelectTrigger>
                <SelectContent>
                  {columns.map((column) => (
                    <SelectItem key={column.name} value={qualify(column.name)}>
                      {column.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Select
                value={filter.operator}
                onValueChange={(next) => {
                  const filters = state.filters.slice();
                  filters[index] = { ...filter, operator: next as T.BuilderFilter["operator"] };
                  onChange({ ...state, filters });
                }}
              >
                <SelectTrigger className="h-8 w-24">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {operators.map((operator) => (
                    <SelectItem key={operator} value={operator}>
                      {operator}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Input
                className="h-8 flex-1"
                placeholder="nilai default"
                value={filter.value === undefined || filter.value === null ? "" : String(filter.value)}
                onChange={(event) => {
                  const filters = state.filters.slice();
                  filters[index] = { ...filter, value: event.target.value };
                  onChange({ ...state, filters });
                }}
              />
              <Button
                size="icon"
                variant="ghost"
                className="size-8 text-destructive"
                onClick={() => onChange({ ...state, filters: state.filters.filter((_, i) => i !== index) })}
              >
                <Trash2 className="size-3.5" />
              </Button>
            </div>
          ))}
        </section>
      </div>
    </div>
  );
}
