"use client";

import { useEffect, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import { AppHeader } from "@/components/shell/header";
import { NavRail } from "@/components/shell/nav-rail";
import { Skeleton } from "@/components/ui/skeleton";
import { useHydrated } from "@/hooks/use-hydrated";
import { useAuthStore } from "@/stores/auth-store";

export default function AppLayout({ children }: { children: ReactNode }) {
  const router = useRouter();
  const hydrated = useHydrated();
  const status = useAuthStore((state) => state.status);

  useEffect(() => {
    if (hydrated && status === "anonymous") router.replace("/login");
  }, [hydrated, router, status]);

  if (!hydrated || status !== "authenticated") {
    return (
      <div className="flex min-h-screen gap-3 bg-background p-4">
        <Skeleton className="h-full w-[220px] rounded-xl" />
        <div className="flex flex-1 flex-col gap-3">
          <Skeleton className="h-14 w-full rounded-xl" />
          <Skeleton className="flex-1 rounded-xl" />
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-screen overflow-hidden bg-background">
      <NavRail />
      <div className="flex min-w-0 flex-1 flex-col">
        <AppHeader />
        <main className="min-w-0 flex-1 overflow-auto p-4">{children}</main>
      </div>
    </div>
  );
}
