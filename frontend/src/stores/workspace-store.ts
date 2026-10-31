import { create } from "zustand";
import { persist, createJSONStorage } from "zustand/middleware";
import { api } from "@/lib/api";
import type * as T from "@/types/api";

type WorkspaceState = {
  workspaceId: string | null;
  access: T.AccessResponse | null;
  loading: boolean;
  error: string | null;
  setActiveWorkspace: (workspaceId: string) => void;
  /** Pulls the caller's merged role + grants, optionally scoped to one project. */
  loadAccess: (workspaceId: string, projectId?: string) => Promise<void>;
  clearAccess: () => void;
  can: (permission: T.Permission) => boolean;
};

export const useWorkspaceStore = create<WorkspaceState>()(
  persist(
    (set, get) => ({
      workspaceId: null,
      access: null,
      loading: false,
      error: null,

      setActiveWorkspace: (workspaceId) => {
        if (get().workspaceId !== workspaceId) set({ workspaceId, access: null });
      },

      loadAccess: async (workspaceId, projectId) => {
        set({ loading: true, error: null });
        try {
          const access = await api.access.get(workspaceId, projectId);
          set({ access, loading: false });
        } catch (err) {
          set({
            access: null,
            loading: false,
            error: err instanceof Error ? err.message : "Failed to load access",
          });
        }
      },

      clearAccess: () => set({ access: null }),

      can: (permission) => get().access?.permissions.includes(permission) ?? false,
    }),
    {
      name: "muara.bi.workspace",
      storage: createJSONStorage(() => localStorage),
      // Grants come from the server on every load; only the tenant choice sticks.
      partialize: (state) => ({ workspaceId: state.workspaceId }),
    },
  ),
);

/**
 * Boolean selector, so callers never build a fresh Set inside a selector
 * (a new object identity per render would re-render forever).
 */
export const useCan = (permission: T.Permission) =>
  useWorkspaceStore((s) => s.access?.permissions.includes(permission) ?? false);
