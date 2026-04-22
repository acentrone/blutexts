import type { Metadata } from "next";
import SolutionPage from "../../_components/SolutionPage";

export const metadata: Metadata = {
  title: "iMessage for DTC brands — BluText",
  description:
    "Win-back, abandoned-cart, and post-purchase flows that read like a friend texting — not a brand blasting. Real iMessage on real numbers, with SMS fallback built in.",
  alternates: { canonical: "https://blutexts.com/solutions/dtc" },
};

export default function DTCSolution() {
  return (
    <SolutionPage
      vertical="DTC"
      eyebrow="Solutions · DTC brands"
      headline={
        <>
          Sound like a <em>friend</em>, not a shortcode.
        </>
      }
      lead="DTC SMS is dead on arrival — green bubbles get muted, swiped, and STOP'd. BluText puts your post-purchase, win-back, and product-drop flows on iMessage. Same automation. 3× the replies."
      pillars={[
        {
          title: "Post-purchase that gets opened",
          body: "Replace the 'Thanks for your order' SMS with a real iMessage from a real number. Higher open, higher review-request conversion, fewer 'who is this' replies.",
          glyph: "1",
        },
        {
          title: "Win-back that pulls real revenue",
          body: "Green-bubble win-back averages ~2% click-through. The same flow on BluText pulls 30%+ replies — and customers tell you which flavor to launch next, in the thread.",
          glyph: "2",
        },
        {
          title: "Drops that feel like an inside tip",
          body: "Voice messages from the founder. Photos of the new colorway. Tapbacks on customer reactions. The launch announcement reads like a friend, not a press release.",
          glyph: "3",
        },
      ]}
      preview={{
        out: [
          "Saved you the last vanilla — restock just landed",
          "Want me to pull one for your usual ship date?",
        ],
        in: ["yes please 🙌", "you're the man"],
      }}
      quote={{
        body: (
          <>
            &ldquo;Winback over SMS was dead on arrival. On BluText, the same
            flow pulled <em>30%+ reply rates</em> and customers started telling
            us which flavor to launch next — in the thread.&rdquo;
          </>
        ),
        name: "Scott Simmons",
        role: "Founder, protein cookie brand (Shopify DTC)",
      }}
    />
  );
}
