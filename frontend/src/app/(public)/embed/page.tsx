import { Suspense } from "react";
import { Loader2 } from "lucide-react";
import { PublicDashboardView } from "@/components/public/public-dashboard-view";

export const metadata = {
  title: "Embedded dashboard · muara.bi",
  robots: { index: false, follow: false },
};

function Loading() {
  return (
    <div className="flex min-h-64 items-center justify-center">
      <Loader2 className="size-6 animate-spin text-muted-foreground" />
    </div>
  );
}

/** The token is runtime data, so reading it has to stay inside the Suspense boundary. */
async function EmbedContent({ searchParams }: { searchParams: Promise<{ token?: string | string[] }> }) {
  const { token } = await searchParams;
  // A list comes from a repeated query param; the first signed token wins.
  const credential = Array.isArray(token) ? token[0] ?? "" : token ?? "";
  return <PublicDashboardView credential={credential} mode="embed" />;
}

export default function EmbedPage({
  searchParams,
}: {
  searchParams: Promise<{ token?: string | string[] }>;
}) {
  return <Suspense fallback={<Loading />}><EmbedContent searchParams={searchParams} /></Suspense>;
}
