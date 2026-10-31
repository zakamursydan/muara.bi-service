"use client";

import { useEffect, useRef } from "react";
import { useSaveDashboard } from "@/lib/mutations";
import { useCanvasStore } from "@/stores/canvas-store";
import type * as T from "@/types/api";

const AUTOSAVE_DELAY_MS = 800;

/**
 * Persists drag/resize changes 800ms after the last move. The dashboard PUT
 * replaces the whole record, so the untouched fields are replayed from the
 * server copy that is currently loaded.
 */
export function useAutosaveLayout(
  workspaceId: string,
  dashboardId: string,
  base: T.DashboardDetail | undefined,
) {
  const layout = useCanvasStore((state) => state.layout);
  const dirty = useCanvasStore((state) => state.layoutDirty);
  const markSaved = useCanvasStore((state) => state.markLayoutSaved);
  const save = useSaveDashboard(workspaceId, dashboardId);
  const timer = useRef<number | undefined>(undefined);

  useEffect(() => {
    if (!dirty || !base || save.isPending) return;
    timer.current = window.setTimeout(() => {
      void save
        .mutateAsync({
          title: base.title,
          description: base.description,
          layout_config: layout,
          global_filters: base.global_filters,
          cache_ttl_seconds: base.cache_ttl_seconds,
        })
        .then(() => markSaved())
        .catch(() => {
          // Keep the layout dirty so the next interaction retries the write.
        });
    }, AUTOSAVE_DELAY_MS);

    return () => window.clearTimeout(timer.current);
  }, [base, dirty, layout, markSaved, save]);
}
