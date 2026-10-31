import { Suspense } from "react";
import { Loader2 } from "lucide-react";
import { PublicDashboardView } from "@/components/public/public-dashboard-view";

export const metadata = {
  title: "Shared dashboard · muara.bi",
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
async function ShareContent({ params }: { params: Promise<{ token: string }> }) {
  const { token } = await params;
  return <PublicDashboardView credential={token} mode="share" />;
}

export default function SharePage({ params }: { params: Promise<{ token: string }> }) {
  return <Suspense fallback={<Loading />}><ShareContent params={params} /></Suspense>;
}
