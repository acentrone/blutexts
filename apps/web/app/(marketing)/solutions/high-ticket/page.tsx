import type { Metadata } from "next";
import SolutionPage from "../../_components/SolutionPage";

export const metadata: Metadata = {
  title: "iMessage for high-ticket sales — BluText",
  description:
    "$5K+ offers don't close in a green bubble. BluText gives high-ticket closers a real iMessage channel — voice notes, founder energy, real read receipts.",
  alternates: { canonical: "https://blutexts.com/solutions/high-ticket" },
};

export default function HighTicketSolution() {
  return (
    <SolutionPage
      vertical="High-ticket"
      eyebrow="Solutions · High-ticket sales"
      headline={
        <>
          $5K offers close in <em>conversations</em>, not blasts.
        </>
      }
      lead="When the average deal is $5K+, the lead deserves a human. BluText is the iMessage layer for closers — voice notes, screen-shared calendar links, real read receipts on the proposal you just sent."
      pillars={[
        {
          title: "Founder-led outreach, at scale",
          body: "Send real iMessage voice notes with waveform playback. The 30-second voice memo is the difference between a $5K close and a ghost.",
          glyph: "1",
        },
        {
          title: "Read receipts that mean something",
          body: "Know exactly when the proposal was opened. Follow up the moment they saw it — the way you would if it were a friend on the other end.",
          glyph: "2",
        },
        {
          title: "Show-rate that doesn't crater",
          body: "iMessage confirmations get acknowledged. The day-of reminder lands as a blue bubble from the human they just spoke to — not a robotext.",
          glyph: "3",
        },
      ]}
      preview={{
        out: [
          "Just sent the proposal over — quick voice note in the next msg",
          "(0:42 voice message)",
        ],
        in: ["just opened it", "this is exactly what we talked about. let's go"],
      }}
      quote={{
        body: (
          <>
            &ldquo;Our show rate jumped from 41% to 68% the week we moved
            confirmations to BluText. The voice notes alone <em>broke our
            close rate</em> on $8K offers.&rdquo;
          </>
        ),
        name: "Marcus Reed",
        role: "Sales lead, $2M/yr coaching practice",
      }}
    />
  );
}
