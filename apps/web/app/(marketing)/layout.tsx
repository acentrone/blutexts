import Script from "next/script";

/**
 * Marketing-route-group layout.
 *
 * Lives at app/(marketing)/layout.tsx so it ONLY wraps the public marketing
 * pages (homepage, /pricing, /integrations/*, /solutions/*, /demo, /privacy,
 * /terms). Everything outside the (marketing) group — the dashboard, auth
 * pages, admin — does NOT inherit this layout.
 *
 * The single responsibility right now: load the iClosed widget script for
 * Book-a-demo popup CTAs. It used to live in the root layout, which meant
 * it loaded on the dashboard too. The widget injects its own CSS at
 * runtime and was probably interfering with the dashboard's grid/flex
 * layouts (we tracked a bizarre threads-list breakage to it).
 *
 * `afterInteractive` ensures the script runs after React hydrates so the
 * loader sees React-rendered buttons with the data-iclosed-link attribute.
 */
export default function MarketingLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <>
      {children}
      <Script
        src="https://app.iclosed.io/assets/widget.js"
        strategy="afterInteractive"
      />
    </>
  );
}
