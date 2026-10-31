"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useHydrated } from "@/hooks/use-hydrated";
import { useAuthStore } from "@/stores/auth-store";

export default function RootPage() {
  const router = useRouter();
  const hydrated = useHydrated();
  const status = useAuthStore((state) => state.status);

  useEffect(() => {
    if (!hydrated || status === "unknown") return;
    router.replace(status === "authenticated" ? "/catalog" : "/login");
  }, [hydrated, router, status]);

  return <div className="min-h-screen bg-background" />;
}
