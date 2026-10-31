"use client";

import { useEffect, useMemo } from "react";
import { FilterX, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Label } from "@/components/ui/label";
import { builderDefaults, parseVariables } from "@/hooks/use-variable-parser";
import { useFilterStore } from "@/stores/filter-store";
import type * as T from "@/types/api";

type FilterBarProps = {
  filters: T.GlobalFilter[];
  widgets: T.Widget[];
  className?: string;
};

/** `__unset__` because Radix Select reserves the empty string for no value. */
const UNSET = "__unset__";

type Control = {
  param: string;
  label: string;
  type: T.GlobalFilter["type"];
  options?: string[];
};

const prettyLabel = (param: string) =>
  param
    .replace(/^flt_/, "")
    .split("_")
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ");

export function FilterBar({ filters, widgets, className }: FilterBarProps) {
  const values = useFilterStore((state) => state.values);
  const cross = useFilterStore((state) => state.cross);
  const setValue = useFilterStore((state) => state.setValue);
  const clearValue = useFilterStore((state) => state.clearValue);
  const seedValues = useFilterStore((state) => state.seedValues);
  const clearCrossFilter = useFilterStore((state) => state.clearCrossFilter);

  const declared = useMemo(() => new Map(filters.map((filter) => [filter.key, filter])), [filters]);

  const defaults = useMemo(() => {
    const out: Record<string, unknown> = {};
    for (const filter of filters) {
      if (filter.default_value !== undefined && filter.default_value !== null) {
        out[filter.key] = filter.default_value;
      }
    }
    for (const widget of widgets) Object.assign(out, builderDefaults(widget.builder_state));
    return out;
  }, [filters, widgets]);

  useEffect(() => {
    seedValues(defaults);
  }, [defaults, seedValues]);

  const controls = useMemo<Control[]>(() => {
    const params = new Set<string>(filters.map((filter) => filter.key));
    for (const widget of widgets) {
      for (const name of parseVariables(widget.raw_sql)) {
        // `xf_` tokens are injected by RLS and are never user-editable.
        if (!name.startsWith("xf_")) params.add(name);
      }
    }
    if (cross) params.delete(cross.param);

    return [...params]
      .sort((a, b) => a.localeCompare(b))
      .map((param) => {
        const filter = declared.get(param);
        return {
          param,
          label: filter?.label ?? prettyLabel(param),
          type: filter?.type ?? "text",
          options: filter?.options,
        };
      });
  }, [cross, declared, filters, widgets]);

  if (!controls.length && !cross) return null;

  return (
    <div
      className={
        "flex flex-wrap items-end gap-3 rounded-xl border border-border bg-card px-3 py-2 shadow-sm " +
        (className ?? "")
      }
    >
      {controls.map((control) => {
        const value = values[control.param];
        const text = value === undefined || value === null ? "" : String(value);
        return (
          <div key={control.param} className="flex w-40 flex-col gap-1">
            <Label className="text-[11px] font-medium text-muted-foreground" htmlFor={control.param}>
              {control.label}
            </Label>
            {control.options?.length ? (
              <Select
                value={text || UNSET}
                onValueChange={(next) =>
                  next === UNSET ? clearValue(control.param) : setValue(control.param, next)
                }
              >
                <SelectTrigger id={control.param} className="w-full">
                  <SelectValue placeholder="(semua)" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={UNSET}>(semua)</SelectItem>
                  {control.options.map((option) => (
                    <SelectItem key={option} value={option}>
                      {option}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            ) : (
              <Input
                id={control.param}
                type={control.type}
                value={text}
                placeholder="(kosong)"
                onChange={(event) => {
                  const next = event.target.value;
                  if (next === "") clearValue(control.param);
                  else if (control.type === "number") {
                    const parsed = Number(next);
                    setValue(control.param, Number.isNaN(parsed) ? next : parsed);
                  } else setValue(control.param, next);
                }}
              />
            )}
          </div>
        );
      })}

      {cross && (
        <div className="flex items-center gap-1.5 self-center rounded-full bg-primary/10 px-2.5 py-1 text-xs text-primary">
          <FilterX className="size-3" />
          <span>
            {cross.label}
          </span>
          <button
            type="button"
            className="rounded-full p-0.5 hover:bg-primary/20"
            title="Hapus cross-filter"
            onClick={clearCrossFilter}
          >
            <X className="size-3" />
          </button>
        </div>
      )}

      {controls.length > 0 && (
        <Button
          variant="ghost"
          size="sm"
          className="self-center text-xs"
          onClick={() => {
            for (const control of controls) clearValue(control.param);
            seedValues(defaults);
          }}
        >
          Reset filter
        </Button>
      )}
    </div>
  );
}
