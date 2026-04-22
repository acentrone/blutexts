import type { Metadata } from "next";
import SolutionPage from "../../_components/SolutionPage";

export const metadata: Metadata = {
  title: "iMessage for real estate agents — BluText",
  description:
    "Show photos, send voice notes, get replies on the listing your buyer actually saw. BluText puts real iMessage in your CRM — so your buyer knows the agent on the other end is a real person.",
  alternates: { canonical: "https://blutexts.com/solutions/real-estate" },
};

export default function RealEstateSolution() {
  return (
    <SolutionPage
      vertical="Real estate"
      eyebrow="Solutions · Real estate"
      headline={
        <>
          Listings land where your buyer <em>actually texts</em>.
        </>
      }
      lead="Buyers ignore listing-blast SMS. They open iMessages from their agent. BluText puts a real iMessage number in your CRM so showings, photos, and offers all land in the same blue thread."
      pillars={[
        {
          title: "Listing photos that load",
          body: "Send full-resolution photos and walkthroughs as native iMessage attachments. No 'click to view' — the buyer sees the kitchen instantly.",
          glyph: "1",
        },
        {
          title: "Voice notes from the showing",
          body: "30-second voice memo from the open house. Your buyer hears your tone — and decides to write the offer same day.",
          glyph: "2",
        },
        {
          title: "Two-way sync into your CRM",
          body: "Replies log straight to the contact in HighLevel, Follow Up Boss, kvCORE — wherever you already track the deal pipeline.",
          glyph: "3",
        },
      ]}
      preview={{
        out: [
          "Just left the walkthrough — pulling up around back now 📸",
          "Quick voice note coming with my read on it",
        ],
        in: ["love the kitchen", "want to write tonight"],
      }}
      quote={{
        body: (
          <>
            &ldquo;My buyers reply to BluText threads in seconds. The same
            messages over my old SMS tool sat unread for hours. Two extra
            offers a month, easy — <em>that paid for the year</em>.&rdquo;
          </>
        ),
        name: "Jenna Park",
        role: "Top-producing agent, Pacific NW",
      }}
    />
  );
}
