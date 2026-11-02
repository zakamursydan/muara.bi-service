"use client";

import { TriangleAlert } from "lucide-react";
import { cn } from "@/lib/utils";
import type * as T from "@/types/api";

type TruncationFlags = Pick<
  T.ExecuteQueryResponse,
  "truncated" | "truncated_by" | "row_limit" | "capped_sources"
>;

type Message = { label: string; detail: string };

/**
 * The engine decides whether an answer is complete; these components only say in
 * Indonesian what it reported, so a marker here always has a `truncated` behind
 * it and an unmarked result is a full answer.
 */
export function truncationMessage(flags?: TruncationFlags | null): Message | null {
  if (!flags?.truncated) return null;

  const sources = (flags.capped_sources ?? []).filter(Boolean);
  if (flags.truncated_by === "source_cap") {
    return {
      label: "Data sumber belum terbaca seluruhnya",
      detail: sources.length
        ? `Sumber ${sources.join(", ")} hanya dibaca sampai batas pengambilannya, jadi angka di sini dihitung dari data yang belum lengkap.`
        : "Sebagian sumber hanya dibaca sampai batasnya, jadi angka di sini dihitung dari data yang belum lengkap.",
    };
  }

  return {
    label: "Hasil dipotong guardrail",
    detail: sources.length
      ? `Hanya ${flags.row_limit} baris pertama yang diambil, dan sumber ${sources.join(", ")} juga hanya terbaca sebagian. Angka serta agregat di sini bukan seluruh jawaban.`
      : `Hanya ${flags.row_limit} baris pertama yang diambil. Angka serta agregat di sini bukan seluruh jawaban.`,
  };
}

/** Warning, not an error: the query succeeded, only its coverage is partial. */
const amber = "text-amber-700 dark:text-amber-400";

export function TruncationNote({
  flags,
  className,
}: {
  flags?: TruncationFlags | null;
  className?: string;
}) {
  const message = truncationMessage(flags);
  if (!message) return null;

  return (
    <p
      role="status"
      className={cn(
        "flex items-start gap-2 rounded-md bg-amber-500/10 px-3 py-2 text-xs",
        amber,
        className,
      )}
    >
      <TriangleAlert className="mt-0.5 size-3.5 shrink-0" />
      <span>
        <span className="font-medium">{message.label}.</span> {message.detail}
      </span>
    </p>
  );
}

/** Compact form for a metadata line, where the full sentence would not fit. The
  canvas cards pair it with an icon on the widget title, so this stays text. */
export function TruncationMarker({ flags }: { flags?: TruncationFlags | null }) {
  const message = truncationMessage(flags);
  if (!message) return null;

  return (
    <span role="status" title={message.detail} className={cn("ml-1.5 font-medium", amber)}>
      {message.label}
    </span>
  );
}
