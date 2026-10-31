"use client";

import { useCallback } from "react";
import ReactGridLayout, { useContainerWidth } from "react-grid-layout";
import "react-grid-layout/css/styles.css";
import "react-resizable/css/styles.css";
import { WidgetCard } from "@/components/canvas/widget-card";
import { useCanvasStore } from "@/stores/canvas-store";
import type * as T from "@/types/api";

type CanvasGridProps = {
  workspaceId: string;
  projectId: string;
  envTag?: string;
  canEdit: boolean;
  onAddWidget?: () => void;
};

const ROW_HEIGHT = 64;
const COLS = 12;

export function CanvasGrid({
  workspaceId,
  projectId,
  envTag,
  canEdit,
  onAddWidget,
}: CanvasGridProps) {
  const { width, containerRef, mounted } = useContainerWidth();
  const layout = useCanvasStore((state) => state.layout);
  const widgets = useCanvasStore((state) => state.widgets);
  const selectedWidgetId = useCanvasStore((state) => state.selectedWidgetId);
  const setLayout = useCanvasStore((state) => state.setLayout);
  const selectWidget = useCanvasStore((state) => state.selectWidget);

  const handleLayoutChange = useCallback(
    (next: readonly T.LayoutItem[]) => {
      // react-grid-layout reports every child on each commit; ignore no-ops so a
      // click on the canvas does not mark the dashboard dirty and fire autosave.
      const unchanged =
        next.length === layout.length &&
        next.every((item, index) => {
          const current = layout[index];
          return (
            current &&
            current.i === item.i &&
            current.x === item.x &&
            current.y === item.y &&
            current.w === item.w &&
            current.h === item.h
          );
        });
      if (!unchanged) setLayout([...next]);
    },
    [layout, setLayout],
  );

  const byId = new Map(widgets.map((widget) => [widget.id, widget]));
  // The inspector edits in the mode the widget was stored in; opening a
  // CUSTOM_SQL card on the builder tab would save it back with an empty
  // builder_state and the server rejects that combination.
  const drawerTabFor = (widget: T.Widget) =>
    widget.query_mode === "CUSTOM_SQL" ? ("sql" as const) : ("builder" as const);
  // Layout can lag behind the widgets a wizard just added; render those last so
  // a new widget is never invisible.
  const ordered = [...layout.map((item) => byId.get(item.i)), ...widgets.filter((w) => !layout.some((i) => i.i === w.id))];

  return (
    <div
      ref={containerRef}
      className="relative min-h-[320px] w-full"
      onClick={() => selectWidget(null)}
    >
      {mounted && ordered.length > 0 && (
        <ReactGridLayout
          width={width}
          layout={layout}
          gridConfig={{ cols: COLS, rowHeight: ROW_HEIGHT, margin: [12, 12] }}
          dragConfig={
            canEdit
              ? { enabled: true, handle: ".drag-handle", cancel: ".no-drag", bounded: true }
              : { enabled: false }
          }
          resizeConfig={canEdit ? { enabled: true, handles: ["se"] } : { enabled: false }}
          onLayoutChange={handleLayoutChange}
        >
          {ordered.map((widget) =>
            widget ? (
              <div key={widget.id}>
                <WidgetCard
                  widget={widget}
                  workspaceId={workspaceId}
                  projectId={projectId}
                  envTag={envTag}
                  selected={selectedWidgetId === widget.id}
                  canEdit={canEdit}
                  onSelect={() => selectWidget(widget.id, drawerTabFor(widget))}
                  onEdit={() => selectWidget(widget.id, drawerTabFor(widget))}
                />
              </div>
            ) : null,
          )}
        </ReactGridLayout>
      )}
      {mounted && ordered.length === 0 && (
        <div className="flex min-h-[320px] flex-col items-center justify-center gap-3 rounded-xl border border-dashed border-border bg-card/50 text-center">
          <p className="text-sm text-muted-foreground">
            Dashboard ini belum punya widget.
          </p>
          {canEdit && (
            <button
              type="button"
              className="rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90"
              onClick={() => onAddWidget?.()}
            >
              Tambah widget
            </button>
          )}
        </div>
      )}
    </div>
  );
}
