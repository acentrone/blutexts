import { NextRequest, NextResponse } from "next/server";

/**
 * Hostname-based routing for the BluTexts Vercel project.
 *
 *   blutexts.com        → marketing site only ("/", /privacy, /terms)
 *   www.blutexts.com    → 301 redirect to apex (canonical marketing host)
 *   app.blutexts.com    → dashboard / app routes only ("/" → /login)
 *
 * One Next.js codebase serves both surfaces; we pick which routes are
 * reachable on which host so URLs are unambiguous and deep links don't
 * accidentally show the marketing page on the app subdomain (or vice versa).
 *
 * Add a new app route? Add it to APP_PATHS below.
 * Add a new marketing route? Add it to MARKETING_PATHS below.
 * Anything not in either list defaults to "app" — safer to route an unknown
 * path to the dashboard (where it'll 404) than to leak it onto the marketing
 * site.
 */

// Route prefixes that belong on app.blutexts.com.
const APP_PATHS = [
  "/dashboard",
  "/messages",
  "/contacts",
  "/settings",
  "/billing",
  "/login",
  "/signup",
  "/forgot-password",
  "/reset-password",
  "/accept-invite",
  "/onboarding",
  "/admin",
];

// Route paths that belong on bare blutexts.com.
// Plain entries match exactly. Entries ending with "/*" match the path or any
// nested path under it (used for /integrations/highlevel, /solutions/dtc, …).
const MARKETING_PATHS = [
  "/",
  "/privacy",
  "/terms",
  "/demo",
  "/pricing",
  "/integrations",
  "/integrations/*",
  "/solutions/*",
];

function isAppPath(pathname: string): boolean {
  return APP_PATHS.some((p) => pathname === p || pathname.startsWith(p + "/"));
}

function isMarketingPath(pathname: string): boolean {
  return MARKETING_PATHS.some((p) => {
    if (p.endsWith("/*")) {
      const base = p.slice(0, -2);
      return pathname === base || pathname.startsWith(base + "/");
    }
    return pathname === p;
  });
}

export function middleware(req: NextRequest) {
  const url = req.nextUrl;
  const host = (req.headers.get("host") || "").toLowerCase();
  const pathname = url.pathname;

  // www → apex (301). Marketing-canonical, doesn't affect app subdomain.
  if (host === "www.blutexts.com") {
    const target = new URL(req.url);
    target.host = "blutexts.com";
    return NextResponse.redirect(target, 301);
  }

  const isApp = host === "app.blutexts.com";
  const isMarketing = host === "blutexts.com";

  // App subdomain at root → straight to /login. Login already redirects to
  // /dashboard if the user is already authenticated, so we don't try to
  // read auth state at the edge.
  if (isApp && pathname === "/") {
    return NextResponse.redirect(new URL("/login", req.url));
  }

  // App subdomain trying to load a marketing-only page → bounce to apex.
  // (Hitting blutexts.com/privacy is canonical; app.blutexts.com/privacy
  // shouldn't render or 404.)
  if (isApp && isMarketingPath(pathname) && pathname !== "/") {
    const target = new URL(req.url);
    target.host = "blutexts.com";
    return NextResponse.redirect(target, 308);
  }

  // Marketing apex trying to load an app page → bounce to app subdomain.
  // Preserves path + query so deep links like /login?next=... work.
  if (isMarketing && isAppPath(pathname)) {
    const target = new URL(req.url);
    target.host = "app.blutexts.com";
    return NextResponse.redirect(target, 308);
  }

  return NextResponse.next();
}

// Run on every request EXCEPT Next.js internals and static assets.
export const config = {
  matcher: [
    /*
     * Match all request paths except:
     * - _next/static (build output)
     * - _next/image (image optimization)
     * - favicon.ico
     * - marketing/* (static brand assets)
     * - any file with an extension (.png, .svg, .jpg, .ico, .css, .js, etc.)
     */
    "/((?!_next/static|_next/image|favicon.ico|marketing/|.*\\.[a-z0-9]+$).*)",
  ],
};
