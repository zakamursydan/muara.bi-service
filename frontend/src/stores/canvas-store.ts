import { create } from "zustand";
import type * as T from "@/types/api";

export type DrawerTab = "ai" | "builder" | "sql";

type CanvasState = {
  layout: T.LayoutItem[];
  widgets: T.Widget[];
  selectedWidgetId: string | null;
  drawerOpen: boolean;
  drawerTab: DrawerTab;
  /**
   * True while the local layout is ahead of the server copy. Widgets are saved
   * explicitly from the drawer, so only drag/resize needs debounced autosave.
   */
  layoutDirty: boolean;

  load: (dashboard: T.DashboardDetail) => void;
  setLayout: (layout: T.LayoutItem[]) => void;
  upsertWidget: (widget: T.Widget) => void;
  removeWidget: (widgetId: string) => void;
  selectWidget: (widgetId: string | null, drawerTab?: DrawerTab) => void;
  /** Opens the drawer with an unsaved draft instead of an existing widget. */
  startNewWidget: (tab?: DrawerTab) => void;
  openDrawer: (tab: DrawerTab) => void;
  closeDrawer: () => void;
  markLayoutSaved: () => void;
  reset: () => void;
};

const emptyLayout: T.LayoutItem[] = [];

/** Widgets can reach the canvas before their position is saved, and
 *  react-grid-layout falls back to a 1×1 cell, so unplaced widgets get the same
 *  6×6 slot the AI wizard uses, stacked below what is already on the grid. */
function withDefaults(layout: T.LayoutItem[], widgets: T.Widget[]): T.LayoutItem[] {
  const placed = new Set(layout.map((item) => item.i));
  const missing = widgets.filter((widget) => !placed.has(widget.id));
  if (missing.length === 0) return layout;

  const bottom = layout.reduce((max, item) => Math.max(max, item.y + item.h), 0);
  return [
    ...layout,
    ...missing.map((widget, index) => ({
      i: widget.id,
      x: (index % 2) * 6,
      y: bottom + Math.floor(index / 2) * 6,
      w: 6,
      h: 6,
    })),
  ];
}

export const useCanvasStore = create<CanvasState>((set, get) => ({
  layout: emptyLayout,
  widgets: [],
  selectedWidgetId: null,
  drawerOpen: false,
  drawerTab: "builder",
  layoutDirty: false,

  // A refetch after a widget save must not discard an unsaved drag, and must
  // not close the drawer the user is still editing in.
  load: (dashboard) =>
    set((state) => {
      const widgets = dashboard.widgets ?? [];
      const current = state.layoutDirty ? state.layout : (dashboard.layout_config ?? emptyLayout);
      const layout = withDefaults(current, widgets);
      return {
        widgets,
        layout,
        // A freshly added widget has no stored position yet, so its default slot
        // has to be persisted by the autosave.
        layoutDirty: state.layoutDirty || layout.length > current.length,
        drawerOpen:
          state.selectedWidgetId &&
          widgets.some((widget) => widget.id === state.selectedWidgetId)
            ? state.drawerOpen
            : false,
      };
    }),

  setLayout: (layout) => set({ layout, layoutDirty: true }),

  upsertWidget: (widget) =>
    set((state) => {
      const index = state.widgets.findIndex((w) => w.id === widget.id);
      if (index === -1) return { widgets: [...state.widgets, widget] };
      const next = state.widgets.slice();
      next[index] = widget;
      return { widgets: next };
    }),

  removeWidget: (widgetId) =>
    set((state) => ({
      widgets: state.widgets.filter((w) => w.id !== widgetId),
      layout: state.layout.filter((item) => item.i !== widgetId),
      selectedWidgetId:
        state.selectedWidgetId === widgetId ? null : state.selectedWidgetId,
      drawerOpen: state.selectedWidgetId === widgetId ? false : state.drawerOpen,
      layoutDirty: true,
    })),

  selectWidget: (widgetId, drawerTab) =>
    set(
      widgetId
        ? { selectedWidgetId: widgetId, drawerOpen: true, drawerTab: drawerTab ?? get().drawerTab }
        : { selectedWidgetId: null, drawerOpen: false },
    ),

  startNewWidget: (tab) =>
    set({ selectedWidgetId: null, drawerOpen: true, drawerTab: tab ?? "builder" }),

  openDrawer: (tab) => set({ drawerOpen: true, drawerTab: tab }),
  closeDrawer: () => set({ drawerOpen: false }),
  markLayoutSaved: () => set({ layoutDirty: false }),
  reset: () =>
    set({
      layout: emptyLayout,
      widgets: [],
      selectedWidgetId: null,
      drawerOpen: false,
      layoutDirty: false,
    }),
}));
