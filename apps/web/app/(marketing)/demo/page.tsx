"use client";

import "../marketing.css";
import MarketingNav from "../_components/MarketingNav";
import MarketingFooter from "../_components/MarketingFooter";

/**
 * /demo — public booking page on the marketing apex.
 *
 * Renders the iClosed inline scheduler so customers can book a 30-minute
 * iMessage demo directly. The widget loader script is already loaded by
 * the root layout (see app/layout.tsx); this page just provides the
 * `<div class="iclosed-widget">` mount point that the loader scans for.
 *
 * The page uses the same nav + footer chrome as the marketing homepage
 * so the booking experience feels continuous, not like a third-party
 * tool dropped onto a blank page.
 */
export default function DemoPage() {
  return (
    <div className="marketing-page">
      <MarketingNav />

      {/* ═══ HERO ═══ */}
      <section
        style={{
          padding: "60px 0 24px",
          background:
            "radial-gradient(ellipse 80% 50% at 50% 20%, rgba(46,111,224,0.08), transparent 70%), linear-gradient(180deg, #FAFBFF 0%, #FFFFFF 70%)",
        }}
      >
        <div className="container" style={{ textAlign: "center", maxWidth: 720 }}>
          <span className="eyebrow">Book a demo</span>
          <h1
            className="display"
            style={{ marginTop: 18, fontSize: 56, lineHeight: 1.05 }}
          >
            Pick a time that works.
          </h1>
          <p className="lead" style={{ margin: "20px auto 0" }}>
            30 minutes, founder-to-founder. We&apos;ll provision a live number
            and send you a real iMessage from it before the call ends.
          </p>
        </div>
      </section>

      {/* ═══ ICLOSED INLINE EMBED ═══ */}
      <section style={{ padding: "0 0 80px", background: "#fff" }}>
        <div className="container" style={{ maxWidth: 1080 }}>
          <div
            className="iclosed-widget"
            data-url="https://app.iclosed.io/e/blutext/imessage-demo"
            title="BluText iMessage Demo"
            style={{ width: "100%", height: 720 }}
          />
        </div>
      </section>

      <MarketingFooter />
    </div>
  );
}
