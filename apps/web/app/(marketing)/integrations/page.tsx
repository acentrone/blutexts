"use client";

import Link from "next/link";
import "../marketing.css";
import MarketingNav from "../_components/MarketingNav";
import MarketingFooter from "../_components/MarketingFooter";
import { useRiseOnScroll } from "../_components/useRiseOnScroll";

/**
 * /integrations — index of CRM + automation tools BluText connects to.
 *
 * Live integrations link to a per-integration detail page (e.g. /integrations/highlevel)
 * so SEO can target the long-tail "[CRM] iMessage integration" queries.
 *
 * "Soon" cards are intentionally listed (HubSpot, Close, Salesforce, etc.)
 * — they signal roadmap commitment to evaluators comparing vendors and they
 * give us a way to capture interest while we build them.
 */

type Integ = {
  slug: string | null; // null = no detail page yet (Soon)
  name: string;
  blurb: string;
  logo: string;
  status: "live" | "soon";
};

const INTEGRATIONS: Integ[] = [
  {
    slug: "highlevel",
    name: "Go High Level",
    blurb:
      "Trigger blue-bubble sends from any HighLevel workflow. Two-way sync into the contact record.",
    logo: "/marketing/ghl.svg",
    status: "live",
  },
  {
    slug: null,
    name: "HubSpot",
    blurb:
      "Send iMessages from HubSpot sequences and log every reply on the contact timeline.",
    logo: "/marketing/hubspot.png",
    status: "soon",
  },
  {
    slug: null,
    name: "Close",
    blurb:
      "Power Close.com call + email cadences with a real iMessage step in between.",
    logo: "/marketing/close.png",
    status: "soon",
  },
  {
    slug: null,
    name: "Salesforce",
    blurb:
      "Native Salesforce object for iMessage threads. Workflow- and Flow-triggerable.",
    logo: "/marketing/salesforce.png",
    status: "soon",
  },
  {
    slug: null,
    name: "Zapier",
    blurb:
      "Connect any of 6,000+ apps. Trigger sends from a row in Sheets or a row in Airtable.",
    logo: "/marketing/ghl.svg",
    status: "soon",
  },
  {
    slug: null,
    name: "Make",
    blurb:
      "Visual scenario builder support — fork on reply, branch on tapback, route to humans.",
    logo: "/marketing/ghl.svg",
    status: "soon",
  },
];

export default function IntegrationsPage() {
  useRiseOnScroll();

  return (
    <div className="marketing-page">
      <MarketingNav />

      <section className="sub-hero">
        <div className="container">
          <span className="eyebrow">Integrations</span>
          <h1 className="display rise">
            Plugs into the stack <em>you already run</em>.
          </h1>
          <p className="lead rise">
            BluText is the iMessage layer for your existing CRM. Trigger sends
            from workflows you already trust. Two-way sync, real read receipts,
            real replies — all in the contact record.
          </p>
        </div>
      </section>

      <section style={{ padding: "0 0 100px", background: "#fff" }}>
        <div className="container">
          <div className="integ-grid rise-group">
            {INTEGRATIONS.map((i) => {
              const inner = (
                <>
                  <div className="ic-head">
                    <div className="ic-logo">
                      {/* eslint-disable-next-line @next/next/no-img-element */}
                      <img src={i.logo} alt={i.name} />
                    </div>
                    <span className="ic-status">
                      {i.status === "live" ? "Live" : "Soon"}
                    </span>
                  </div>
                  <h3>{i.name}</h3>
                  <p>{i.blurb}</p>
                  <span className="ic-cta">
                    {i.status === "live" ? (
                      <>
                        Learn more <span className="arrow">→</span>
                      </>
                    ) : (
                      <>Notify me</>
                    )}
                  </span>
                </>
              );

              if (i.slug && i.status === "live") {
                return (
                  <Link
                    key={i.name}
                    href={`/integrations/${i.slug}`}
                    className="integ-card live"
                  >
                    {inner}
                  </Link>
                );
              }
              return (
                <a
                  key={i.name}
                  href="mailto:hello@blutexts.com?subject=Integration%20interest"
                  className={`integ-card ${i.status}`}
                >
                  {inner}
                </a>
              );
            })}
          </div>
        </div>
      </section>

      {/* Pricing strip */}
      <section className="pricing-strip">
        <div className="container">
          <div>
            <h2>
              Same number. Same inbox. <em>Every tool you use.</em>
            </h2>
            <p>
              Every integration is included on the Standard plan — no
              per-connector fees, no add-ons. Enterprise gets custom
              integrations scoped into the contract.
            </p>
          </div>
          <div className="actions">
            <Link className="btn primary" href="/pricing">
              See pricing <span className="arrow">→</span>
            </Link>
            <a
              className="btn ghost"
              href="/demo"
              data-iclosed-link="https://app.iclosed.io/e/blutext/imessage-demo"
              data-embed-type="popup"
            >
              Book a demo
            </a>
          </div>
        </div>
      </section>

      {/* Final CTA */}
      <section className="final">
        <div className="container rise">
          <span className="eyebrow">Don&apos;t see your tool?</span>
          <h2 className="display" style={{ marginTop: 18 }}>
            We&apos;ll build it. <em>Tell us which one.</em>
          </h2>
          <p className="lead">
            Email integrations roadmap requests to the founders. Most ship
            within a quarter.
          </p>
          <div className="cta-row">
            <a
              className="btn primary"
              href="mailto:hello@blutexts.com?subject=Integration%20request"
            >
              Email the founders <span className="arrow">→</span>
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
        </div>
      </section>

      <MarketingFooter />
    </div>
  );
}
