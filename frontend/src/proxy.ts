import { NextResponse, type NextRequest } from "next/server";

/**
 * Public share documents must carry the same `frame-ancestors` policy the Go API
 * applies to the JSON payload, otherwise an allowed origin could load the page
 * shell even though the data call is protected.
 */

const apiBaseUrl = process.env.API_BASE_URL ?? "http://127.0.0.1:8080";
const CACHE_TTL_MS = 60_000;

const cache = new Map<string, { ancestors: string[]; expires: number }>();

async function resolveFrameAncestors(url: string): Promise<string[] | null> {
  const hit = cache.get(url);
  if (hit && hit.expires > Date.now()) return hit.ancestors;

  try {
    const response = await fetch(url, { cache: "no-store" });
    if (!response.ok) return null;
    const body = (await response.json()) as { data?: { frame_ancestors?: string[] } };
    const ancestors = body.data?.frame_ancestors ?? [];
    cache.set(url, { ancestors, expires: Date.now() + CACHE_TTL_MS });
    return ancestors;
  } catch {
    return null;
  }
}

export async function proxy(request: NextRequest) {
  const { pathname, searchParams } = request.nextUrl;

  // A password-protected share still reports its allowlist before the password
  // check, so the policy can be resolved anonymously.
  const shareToken = pathname.startsWith("/share/") ? pathname.slice("/share/".length).split("/")[0] : "";
  const embedToken = pathname === "/embed" ? searchParams.get("token") ?? "" : "";

  const target = shareToken
    ? `${apiBaseUrl}/api/v1/public/dashboards/${encodeURIComponent(shareToken)}`
    : `${apiBaseUrl}/api/v1/public/embed?token=${encodeURIComponent(embedToken)}`;

  const ancestors = shareToken || embedToken ? await resolveFrameAncestors(target) : null;

  const response = NextResponse.next();
  if (!ancestors?.length) {
    // Unknown, revoked, or an allowlist-less share: refuse framing entirely.
    response.headers.set("Content-Security-Policy", "frame-ancestors 'none'");
    response.headers.set("X-Frame-Options", "DENY");
  } else {
    response.headers.set("Content-Security-Policy", `frame-ancestors ${ancestors.join(" ")}`);
  }
  return response;
}

export const config = {
  matcher: ["/share/:path*", "/embed"],
};
