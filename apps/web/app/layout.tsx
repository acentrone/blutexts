import type { Metadata } from "next";
import Script from "next/script";
import "./globals.css";
import "./app-brand.css";

export const metadata: Metadata = {
  title: {
    template: "%s | BluTexts",
    default: "BluTexts — iMessage for Business",
  },
  description:
    "Send iMessages through your CRM. Higher response rates, personal delivery, and seamless Go High Level integration.",
  openGraph: {
    title: "BluTexts — iMessage for Business",
    description: "The CRM that speaks iMessage.",
    siteName: "BluTexts",
    type: "website",
  },
  // Icons are picked up automatically by Next.js's file convention:
  //   app/icon.svg       → favicon (all browsers)
  //   app/apple-icon.svg → home-screen / iOS bookmark
  // No need to declare them here unless we add resolution-specific PNGs.
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        {/* Marketing-page typefaces. Loaded site-wide so the (marketing) route
            renders crisply on first paint without a layout shift. The dashboard
            ignores them — it uses the system stack via Tailwind's font-sans. */}
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link
          rel="preconnect"
          href="https://fonts.gstatic.com"
          crossOrigin="anonymous"
        />
        <link
          href="https://fonts.googleapis.com/css2?family=Instrument+Serif:ital@0;1&family=JetBrains+Mono:wght@400;500&family=Manrope:wght@400;500;600;700;800&display=swap"
          rel="stylesheet"
        />
      </head>
      <body className="min-h-screen bg-background font-sans antialiased">
        {children}
        {/*
          iClosed widget loader.
          - `afterInteractive` ensures it runs AFTER React hydrates, so the
            loader sees React-rendered buttons with data-iclosed-link.
          - Loaded site-wide so the popup works on every marketing route
            and the inline embed on /demo picks it up too.
        */}
        <Script
          src="https://app.iclosed.io/assets/widget.js"
          strategy="afterInteractive"
        />
      </body>
    </html>
  );
}
