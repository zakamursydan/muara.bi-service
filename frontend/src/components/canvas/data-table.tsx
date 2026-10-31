"use client";

import { useMemo, useState } from "react";
import { ArrowDown, ArrowUp, ChevronLeft, ChevronRight, ChevronsUpDown } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { tableCssVars } from "@/lib/table-theme";
import { isNumericColumn, temporalKind, formatTemporal, toNumber } from "@/lib/echarts-options";
import type * as T from "@/types/api";

type DataTableProps = {
  result: { columns: T.ColumnMeta[]; rows: Record<string, unknown>[] };
  /** Cap rendered rows; the guardrail already caps the payload at max_rows. */
  limit?: number;
  onCellClick?: (column: string, value: unknown) => void;
  highlight?: { field: string; value: string };
  /** Widget presentation config; only `table` is read here. */
  config?: T.EchartsConfig;
  className?: string;
};

type Sort = { column: string; dir: "asc" | "desc" };

const PAGE_SIZES = [10, 25, 50, 100];

function stringify(value: unknown): string {
  if (value === null || value === undefined) return "";
  if (value instanceof Date) return value.toISOString();
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

const groupFormat = new Intl.NumberFormat("en-US", { maximumFractionDigits: 2 });
const preciseFormat = new Intl.NumberFormat("en-US", { maximumFractionDigits: 6 });

/**
 * toPrecision(15) first because float noise (0.1 + 0.2) would otherwise print
 * every guard digit. Sub-cent values keep more decimals so a real small measure
 * never rounds away to zero.
 */
function formatNumeric(num: number): string {
  const clean = Number(num.toPrecision(15));
  const tiny = clean !== 0 && Math.abs(clean) < 0.01;
  return (tiny ? preciseFormat : groupFormat).format(clean);
}


function compare(a: unknown, b: unknown, numeric: boolean): number {
  if (numeric) {
    const left = toNumber(a);
    const right = toNumber(b);
    if (left === null && right === null) return 0;
    if (left === null) return 1;
    if (right === null) return -1;
    return left - right;
  }
  return stringify(a).localeCompare(stringify(b), undefined, { numeric: true });
}

export function DataTable({
  result,
  limit = 100,
  onCellClick,
  highlight,
  config,
  className,
}: DataTableProps) {
  const style = config?.table;
  const [sort, setSort] = useState<Sort | null>(null);
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState(PAGE_SIZES[1]);

  const isCozy = style?.density === "cozy";
  const cellPad = isCozy ? "px-3 py-2" : "px-2 py-1";
  const sticky = style?.sticky_header !== false;
  const zebra = Boolean(style?.zebra);

  const capped = useMemo(() => result.rows.slice(0, limit), [result.rows, limit]);

  const sorted = useMemo(() => {
    if (!sort) return capped;
    const numeric = result.columns.some(
      (column) => column.name === sort.column && isNumericColumn(column.type),
    );
    const factor = sort.dir === "asc" ? 1 : -1;
    return [...capped].sort(
      (a, b) => factor * compare(a[sort.column], b[sort.column], numeric),
    );
  }, [capped, sort, result.columns]);

  const pageCount = Math.max(1, Math.ceil(sorted.length / pageSize));
  // Clamped rather than reset in an effect: a background refresh keeps the
  // reader on the page they were looking at instead of jumping back.
  const current = Math.min(page, pageCount - 1);
  const visible = sorted.slice(current * pageSize, current * pageSize + pageSize);

  const setOrder = (next: Sort | null) => {
    setPage(0);
    setSort(next);
  };

  const numberOffset = style?.row_numbers ? current * pageSize : 0;

  return (
    <div
      style={tableCssVars(style)}
      className={cn(
        "flex h-full w-full flex-col overflow-hidden rounded-lg border border-border bg-[var(--dt-body-bg)]",
        className,
      )}
    >
      <div className="min-h-0 flex-1 overflow-auto">
        <table
          className={cn(
            "w-full caption-bottom border-separate border-spacing-0",
            isCozy ? "text-xs" : "text-[11px]",
          )}
        >
          <thead>
            <tr>
              {style?.row_numbers && (
                <th
                  scope="col"
                  className={cn(
                    "sticky left-0 z-10 border-b border-border bg-[var(--dt-header-bg)] px-2 text-right font-medium text-[var(--dt-header-fg)]",
                    sticky && "top-0",
                    isCozy ? "py-2" : "py-1",
                  )}
                >
                  #
                </th>
              )}
              {result.columns.map((column) => {
                const active = sort?.column === column.name;
                const Icon = active
                  ? sort.dir === "asc"
                    ? ArrowUp
                    : ArrowDown
                  : ChevronsUpDown;
                return (
                  <th
                    key={column.name}
                    scope="col"
                    className={cn(
                      "sticky left-0 z-10 border-b border-border bg-[var(--dt-header-bg)] p-0 font-medium text-[var(--dt-header-fg)]",
                      sticky && "top-0",
                    )}
                  >
                    <button
                      type="button"
                      title={`${column.name} · ${column.type}`}
                      onClick={() =>
                        setOrder(
                          sort?.column !== column.name
                            ? { column: column.name, dir: "asc" }
                            : sort.dir === "asc"
                              ? { column: column.name, dir: "desc" }
                              : null,
                        )
                      }
                      className={cn(
                        "flex w-full items-center gap-1 text-left hover:bg-[var(--dt-hover)]",
                        cellPad,
                        isNumericColumn(column.type) && "justify-end text-right",
                      )}
                    >
                      <span className="truncate">{column.name}</span>
                      <Icon
                        className={cn(
                          "size-3 shrink-0",
                          active ? "opacity-100" : "opacity-35",
                        )}
                      />
                    </button>
                  </th>
                );
              })}
            </tr>
          </thead>
          <tbody>
            {visible.map((row, rowIndex) => (
              <tr key={rowIndex} className="group/row">
                {style?.row_numbers && (
                  <td
                    className={cn(
                      "sticky left-0 z-[1] border-b border-border/60 bg-[var(--dt-body-bg)] px-2 text-right tabular-nums text-[var(--dt-text-fg)]/60 group-hover/row:bg-[var(--dt-hover)]",
                      cellPad,
                    )}
                  >
                    {numberOffset + rowIndex + 1}
                  </td>
                )}
                {result.columns.map((column) => {
                  const value = row[column.name];
                  const numeric = isNumericColumn(column.type);
                  const temporal = temporalKind(column.type);
                  const raw = stringify(value);
                  const text = numeric
                    ? formatNumericCell(value)
                    : temporal
                      ? formatTemporal(raw, temporal)
                      : raw;
                  const isHighlight =
                    highlight?.field === column.name && highlight.value === stringify(value);
                  return (
                    <td
                      key={column.name}
                      title={text || undefined}
                      className={cn(
                        "max-w-[280px] truncate border-b border-border/60 px-2 text-[var(--dt-text-fg)]",
                        cellPad,
                        numeric && "text-right tabular-nums",
                        zebra && rowIndex % 2 === 1 && "bg-[var(--dt-stripe-bg)]",
                        "group-hover/row:bg-[var(--dt-hover)]",
                        isHighlight &&
                          "bg-[var(--dt-accent)] font-medium text-[var(--dt-text-fg)] ring-1 ring-inset ring-[var(--dt-header-fg)]/20",
                      )}
                    >
                      {text ? (
                        onCellClick ? (
                          <button
                            type="button"
                            className={cn(
                              "max-w-full truncate hover:underline",
                              !isHighlight && "text-[color:var(--dt-text-fg)]",
                            )}
                            onClick={() => onCellClick(column.name, value)}
                          >
                            {text}
                          </button>
                        ) : (
                          text
                        )
                      ) : (
                        <span className="opacity-40">—</span>
                      )}
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
        {!sorted.length && (
          <p className="flex h-full items-center justify-center py-6 text-xs text-muted-foreground">
            Tidak ada baris untuk filter ini.
          </p>
        )}
      </div>

      {(sorted.length > pageSize || capped.length < result.rows.length) && (
        <footer className="flex shrink-0 flex-wrap items-center gap-2 border-t border-border bg-[var(--dt-header-bg)] px-2 py-1 text-[11px] text-[var(--dt-header-fg)]">
          <span className="tabular-nums">
            {sorted.length
              ? `${current * pageSize + 1}–${current * pageSize + visible.length} dari ${sorted.length}`
              : "0 baris"}
            {capped.length < result.rows.length ? ` (dibatasi ${limit})` : ""}
          </span>
          {pageCount > 1 && (
            <div className="ml-auto flex items-center gap-1">
              <Button
                variant="ghost"
                size="icon-xs"
                aria-label="Baris sebelumnya"
                disabled={current === 0}
                onClick={() => setPage(current - 1)}
              >
                <ChevronLeft className="size-3" />
              </Button>
              <span className="tabular-nums">
                {current + 1}/{pageCount}
              </span>
              <Button
                variant="ghost"
                size="icon-xs"
                aria-label="Baris berikutnya"
                disabled={current >= pageCount - 1}
                onClick={() => setPage(current + 1)}
              >
                <ChevronRight className="size-3" />
              </Button>
            </div>
          )}
          <label className={cn("flex items-center gap-1", pageCount <= 1 && "ml-auto")}>
            Baris
            <select
              value={pageSize}
              onChange={(event) => {
                setPageSize(Number(event.target.value));
                setPage(0);
              }}
              className="h-6 rounded border border-border bg-transparent px-1 text-[11px] outline-none"
            >
              {PAGE_SIZES.map((size) => (
                <option key={size} value={size}>
                  {size}
                </option>
              ))}
            </select>
          </label>
        </footer>
      )}
    </div>
  );
}

/** A numeric column that receives a non-numeric value falls back to text. */
function formatNumericCell(value: unknown): string {
  const num = toNumber(value);
  return num === null ? stringify(value) : formatNumeric(num);
}
