import { create } from "zustand";
import { persist, createJSONStorage } from "zustand/middleware";
import type * as T from "@/types/api";

type ProjectState = {
  project: T.Project | null;
  setProject: (project: T.Project | null) => void;
};

/**
 * Holds only the project identity; its dashboards and members are React Query
 * data, so the store must not duplicate them.
 */
export const useProjectStore = create<ProjectState>()(
  persist(
    (set) => ({
      project: null,
      setProject: (project) => set({ project }),
    }),
    {
      name: "muara.bi.project",
      storage: createJSONStorage(() => localStorage),
    },
  ),
);
