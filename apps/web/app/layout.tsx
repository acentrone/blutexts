import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: {
    template: "%s | BlueSend",
    default: "BlueSend — iMessage for Business",
  },
  description:
    "Send iMessages through your CRM. Higher response rates, personal delivery, and seamless Go High Level integration.",
  openGraph: {
    title: "BlueSend — iMessage for Business",
    description: "The CRM that speaks iMessage.",
    siteName: "BlueSend",
    type: "website",
  },
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body className="min-h-screen bg-background font-sans antialiased">
        {children}
      </body>
    </html>
  );
}
