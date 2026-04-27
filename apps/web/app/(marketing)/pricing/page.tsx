"use client";

import "../marketing.css";
import MarketingNav from "../_components/MarketingNav";
import MarketingFooter from "../_components/MarketingFooter";
import { useRiseOnScroll } from "../_components/useRiseOnScroll";

/**
 * /pricing — public pricing page on the marketing apex.
 *
 * Two tiers only (no monthly/annual toggle):
 *   - Standard: $399/mo + $500 one-time startup fee.
 *     1 dedicated number, 5 seats, unlimited messages/day, 50 NEW
 *     conversations/day (Apple-compliance hard cap, NOT a marketing limit).
 *   - Enterprise: custom; starts at $199/line when committing to 5+ lines
 *     on an annual plan.
 *
 * Per founder: there are NO other plans, NO per-message billing, NO overages.
 * The 50-new-convos/day cap is a compliance ceiling we have to respect — we
 * call that out explicitly so prospects understand it's an Apple rule, not
 * a paywall move.
 */

const STANDARD_FEATURES = [
  "1 dedicated iMessage number",
  "5 user seats",
  "Unlimited messages per day",
  "50 new conversations / day (Apple compliance cap)",
  "HighLevel integration",
  "Tapbacks, effects, voice messages",
  "Reply-rate analytics",
  "Email + chat support",
];

const ENTERPRISE_FEATURES = [
  "Multiple dedicated numbers",
  "Volume discount: $199/line at 5+ lines (annual)",
  "Unlimited seats",
  "Unlimited messages per day",
  "50 new conversations / day per line (Apple compliance cap)",
  "Custom integrations (HubSpot, Close, Salesforce, etc.)",
  "SSO + SCIM",
  "Dedicated success manager",
  "White-glove onboarding",
];

const FAQ: { q: string; a: string }[] = [
  {
    q: "Why is there a 50 new-conversations-per-day limit?",
    a: "That's an Apple compliance ceiling — not a BluText marketing decision. Apple actively rate-limits accounts that open conversations with too many new contacts in a day, and exceeding the threshold gets the underlying number flagged. We hold every account to 50 new conversations per day per line so your number stays healthy. Replies and ongoing threads are unlimited.",
  },
  {
    q: "Are messages metered? What about overages?",
    a: "No. There's no per-message billing and no overage charges. Send as many messages per day as you want into existing threads. The only cap is the 50 new conversations per day per line that Apple requires.",
  },
  {
    q: "What's the $500 startup fee for?",
    a: "Provisioning your dedicated iMessage number, configuring your account, onboarding your team, and the manual identity steps Apple requires for business iMessage. It's a one-time fee at signup.",
  },
  {
    q: "How is BluText different from a regular SMS service?",
    a: "BluText sends through Apple's iMessage network on a real, dedicated number tied to your team — not a shortcode. Recipients see a blue bubble from a real phone number, exactly like a text from a friend.",
  },
  {
    q: "What if my recipient isn't on iMessage?",
    a: "BluText delivers iMessage only — no SMS fallback today. The vast majority of US consumers are on iPhone (~57% market share), and our typical customers see >90% iMessage deliverability on cleaned lists. If you need guaranteed-cross-platform delivery, pair BluText with your existing SMS provider for non-iPhone segments.",
  },
  {
    q: "Do I need an A2P 10DLC registration?",
    a: "No. Because we send via iMessage on a real number (not via carrier shortcodes), we sidestep the A2P registration process entirely.",
  },
  {
    q: "What does Enterprise unlock?",
    a: "Multiple dedicated lines, custom integrations, SSO, and white-glove onboarding. Pricing starts at $199 per line when you commit to 5 or more lines on an annual plan. Talk to us about your volume — we'll scope it on the call.",
  },
];

export default function PricingPage() {
  useRiseOnScroll();

  return (
    <div className="marketing-page">
      <MarketingNav />

      {/* Hero */}
      <section className="sub-hero">
        <div className="container">
          <span className="eyebrow">Pricing</span>
          <h1 className="display rise">
            Simple pricing. <em>One real number.</em>
          </h1>
          <p className="lead rise">
            One plan with everything you need to run real iMessage on a
            dedicated line. Scale up with Enterprise when you&apos;re ready
            for multiple lines.
          </p>
        </div>
      </section>

      {/* Tier cards — two columns, centered */}
      <section style={{ padding: "0 0 100px", background: "#fff" }}>
        <div className="container">
          <div
            className="price-grid rise-group"
            style={{
              gridTemplateColumns: "repeat(2, 1fr)",
              maxWidth: 880,
            }}
          >
            {/* Standard */}
            <div className="price-card featured">
              <span className="badge">Most popular</span>
              <span className="tier">Standard</span>
              <h3>One dedicated line, ready to go</h3>
              <p className="lede">
                Everything you need to run real iMessage from your team — on a
                real number Apple recognizes as yours.
              </p>
              <div className="price-row">
                <span className="num">$399</span>
                <span className="per">/mo</span>
              </div>
              <div className="price-sub">
                + $500 one-time startup fee
              </div>
              <div
                className="cta"
                style={{ display: "flex", flexDirection: "column", gap: 8 }}
              >
                <a className="btn primary" href="/signup">
                  Sign up <span className="arrow">→</span>
                </a>
                <a
                  className="btn ghost"
                  href="/demo"
                  data-iclosed-link="https://app.iclosed.io/e/blutext/imessage-demo"
                  data-embed-type="popup"
                >
                  Book a demo
                </a>
              </div>
              <ul className="features-list">
                {STANDARD_FEATURES.map((f) => (
                  <li key={f}>{f}</li>
                ))}
              </ul>
            </div>

            {/* Enterprise */}
            <div className="price-card enterprise">
              <span className="tier">Enterprise</span>
              <h3>Multiple lines, volume pricing</h3>
              <p className="lede">
                For teams running outbound at scale, agencies managing client
                accounts, or anyone needing more than one dedicated number.
              </p>
              <div className="price-row">
                <span className="num">From $199</span>
                <span className="per">/line/mo</span>
              </div>
              <div className="price-sub">
                5+ lines · annual commitment
              </div>
              <div className="cta">
                <a
                  className="btn dark"
                  href="/demo"
                  data-iclosed-link="https://app.iclosed.io/e/blutext/imessage-demo"
                  data-embed-type="popup"
                >
                  Talk to sales <span className="arrow">→</span>
                </a>
              </div>
              <ul className="features-list">
                {ENTERPRISE_FEATURES.map((f) => (
                  <li key={f}>{f}</li>
                ))}
              </ul>
            </div>
          </div>

          {/* Apple compliance note */}
          <div
            style={{
              maxWidth: 720,
              margin: "40px auto 0",
              padding: "20px 24px",
              background: "var(--paper)",
              border: "1px solid var(--rule)",
              borderRadius: 12,
              fontSize: 14,
              color: "var(--muted)",
              lineHeight: 1.5,
              textAlign: "center",
            }}
          >
            <strong style={{ color: "var(--ink)" }}>
              About the 50 new-conversations/day cap.
            </strong>{" "}
            That&apos;s an Apple compliance limit on every iMessage line — not
            a BluText paywall. Replies and ongoing threads are unlimited.
            Enterprise lines stack the cap (5 lines = 250 new convos/day).
          </div>
        </div>
      </section>

      {/* FAQ */}
      <section className="faq">
        <div className="container">
          <div className="head rise">
            <span className="eyebrow">Common questions</span>
            <h2 className="display" style={{ marginTop: 14, fontSize: 44 }}>
              The fine print, <em>up front</em>.
            </h2>
          </div>
          <div className="faq-list rise-group">
            {FAQ.map((item) => (
              <details key={item.q} className="faq-item">
                <summary>{item.q}</summary>
                <p>{item.a}</p>
              </details>
            ))}
          </div>
        </div>
      </section>

      {/* Final CTA */}
      <section className="final">
        <div className="container rise">
          <span className="eyebrow">Ready?</span>
          <h2 className="display" style={{ marginTop: 18 }}>
            See it on a real number, <em>live</em>.
          </h2>
          <p className="lead">
            We&apos;ll provision a number on the call and send you a real
            iMessage from it before we hang up. Founder to founder.
          </p>
          <div className="cta-row">
            <a
              className="btn primary"
              href="/demo"
              data-iclosed-link="https://app.iclosed.io/e/blutext/imessage-demo"
              data-embed-type="popup"
            >
              Book a demo <span className="arrow">→</span>
            </a>
          </div>
        </div>
      </section>

      <MarketingFooter />
    </div>
  );
}
