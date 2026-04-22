import type { Metadata } from "next";
import HighLevelDetail from "./HighLevelDetail";

/**
 * /integrations/highlevel — SEO-optimized landing page for the
 * "Send an iMessage from Go High Level" search query.
 *
 * Server component owns metadata (title, description, OG, canonical) so
 * the page is fully indexable. The interactive bits (rise-on-scroll)
 * live in the client component below.
 */
export const metadata: Metadata = {
  title: "Send an iMessage from Go High Level — BluText",
  description:
    "Trigger real iMessages from any HighLevel workflow. Real numbers, blue bubbles, two-way sync into the contact record. Live integration — no Zapier glue.",
  alternates: { canonical: "https://blutexts.com/integrations/highlevel" },
  openGraph: {
    title: "Send an iMessage from Go High Level",
    description:
      "Trigger real iMessages from HighLevel workflows. Blue bubbles, real numbers, two-way sync.",
    type: "article",
    url: "https://blutexts.com/integrations/highlevel",
    siteName: "BluText",
  },
  twitter: {
    card: "summary_large_image",
    title: "Send an iMessage from Go High Level",
    description:
      "Trigger real iMessages from HighLevel workflows. Blue bubbles, real numbers, two-way sync.",
  },
};

export default function Page() {
  return <HighLevelDetail />;
}
