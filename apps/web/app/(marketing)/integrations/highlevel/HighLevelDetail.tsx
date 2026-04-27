"use client";

import Link from "next/link";
import "../../marketing.css";
import MarketingNav from "../../_components/MarketingNav";
import MarketingFooter from "../../_components/MarketingFooter";
import { useRiseOnScroll } from "../../_components/useRiseOnScroll";

/**
 * /integrations/highlevel — interactive client shell.
 *
 * Page anatomy (top to bottom):
 *   1. Hero with H1 "Send an iMessage from Go High Level" (the SEO target)
 *   2. Three "what it does" pillars
 *   3. Alternating feature rows showing the integration in context
 *   4. JSON-LD structured data (SoftwareApplication) — lives in the body
 *      because we need it on the rendered page, not just in <head>.
 *   5. Final CTA + pricing strip
 */
export default function HighLevelDetail() {
  useRiseOnScroll();

  return (
    <div className="marketing-page">
      <MarketingNav />

      {/* JSON-LD for the integration. Rendered in body — Google reads it
          either way and it lets us keep this in the client component. */}
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{
          __html: JSON.stringify({
            "@context": "https://schema.org",
            "@type": "SoftwareApplication",
            name: "BluText for Go High Level",
            applicationCategory: "BusinessApplication",
            operatingSystem: "Web",
            description:
              "Send real iMessages from Go High Level workflows. Real numbers, blue bubbles, two-way sync into the contact record.",
            url: "https://blutexts.com/integrations/highlevel",
            offers: {
              "@type": "Offer",
              price: "399",
              priceCurrency: "USD",
              priceSpecification: {
                "@type": "UnitPriceSpecification",
                price: "399",
                priceCurrency: "USD",
                billingDuration: "P1M",
              },
            },
          }),
        }}
      />

      {/* Hero */}
      <section className="integ-detail-hero">
        <div className="container">
          <div className="rise">
            <div className="crumb">
              <Link href="/integrations">Integrations</Link> · Go High Level
            </div>
            <h1>
              Send an iMessage from <em>Go High Level</em>.
            </h1>
            <p className="lead">
              Trigger real, blue-bubble iMessages from any HighLevel workflow.
              Real numbers. Real read receipts. Replies sync straight into the
              contact record — no Zapier glue, no fragile webhooks.
            </p>
            <div className="actions">
              <a
                className="btn primary"
                href="/demo"
                data-iclosed-link="https://app.iclosed.io/e/blutext/imessage-demo"
                data-embed-type="popup"
              >
                Book a demo <span className="arrow">→</span>
              </a>
              <Link className="btn ghost" href="/pricing">
                See pricing
              </Link>
            </div>
          </div>

          <div className="rise">
            <div className="glyph">
              <div className="pair">
                <div className="logo-tile">
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img src="/marketing/ghl.svg" alt="Go High Level" />
                </div>
                <span className="arrow-link">→</span>
                <div className="logo-tile brand" aria-label="BluText" />
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Pillars */}
      <section className="pillars">
        <div className="container">
          <div className="head rise">
            <span className="eyebrow">What it does</span>
            <h2 className="display" style={{ marginTop: 14, fontSize: 44 }}>
              Three things HighLevel users <em>actually need</em>.
            </h2>
          </div>
          <div className="pillar-grid rise-group">
            <div className="pillar-card">
              <div className="ic">a</div>
              <h3>Workflow trigger</h3>
              <p>
                Drop a &ldquo;Send iMessage&rdquo; action into any workflow.
                Same UX as the SMS step — the message ships through Apple
                instead of a carrier shortcode.
              </p>
            </div>
            <div className="pillar-card">
              <div className="ic">b</div>
              <h3>Two-way sync</h3>
              <p>
                Inbound replies log to the contact&apos;s conversation
                timeline. Tags, custom fields, pipelines — all updateable
                from a thread.
              </p>
            </div>
            <div className="pillar-card">
              <div className="ic">c</div>
              <h3>Replies sync into HighLevel</h3>
              <p>
                Inbound iMessages land on the contact&apos;s timeline in
                HighLevel automatically — same conversation surface your
                team already lives in.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* Alternating feature rows */}
      <section className="alt-row">
        <div className="container">
          <div className="copy rise">
            <span className="eyebrow">Real iMessage · not a lookalike</span>
            <h2>
              Same workflow builder. <em>Better channel.</em>
            </h2>
            <p>
              You already know how to build flows in HighLevel. Replace the
              SMS step with a BluText step and the next message your contact
              gets reads as a blue bubble from a real number — exactly like
              a friend texting them.
            </p>
            <p>
              Voice messages, tapbacks, effects, photos and video — all the
              things people actually do in iMessage, all from a HighLevel
              automation.
            </p>
          </div>
          <div className="visual rise">
            <div
              className="mini-imessage"
              style={{ maxWidth: 280, margin: "0 auto" }}
            >
              <div className="bubble in">are you a real person 😅</div>
              <div className="bubble out">Yep — Alex from Aesop 👋</div>
              <div className="bubble out" style={{ borderRadius: 19 }}>
                Sent from a HighLevel workflow, but it&apos;s me on the
                other end
              </div>
              <div className="im-status">Delivered · Read now</div>
            </div>
          </div>
        </div>
      </section>

      <section className="alt-row flip">
        <div className="container">
          <div className="visual rise">
            <div style={{ width: "100%" }}>
              {[
                { dot: "", name: "Workflow → Welcome series", time: "2:14 PM", body: "Step 3 · Send iMessage" },
                { dot: "read", name: "Workflow → Re-engagement", time: "11:02 AM", body: "Step 1 · Send iMessage" },
                { dot: "read", name: "Workflow → Cart abandon", time: "Yesterday", body: "Step 2 · Send iMessage" },
              ].map((row, i) => (
                <div
                  key={i}
                  className="inbox-row"
                  style={{
                    padding: "10px 12px",
                    borderBottom: i < 2 ? "1px solid var(--paper-2)" : "none",
                    display: "flex", alignItems: "center", gap: 10,
                  }}
                >
                  <span className={`dot ${row.dot}`} />
                  <div style={{ flex: 1 }}>
                    <div style={{ fontSize: 13, fontWeight: 600, color: "var(--ink)" }}>
                      {row.name}
                    </div>
                    <div style={{ fontSize: 11, color: "var(--muted)" }}>{row.body}</div>
                  </div>
                  <span style={{ fontSize: 11, color: "var(--muted-2)" }}>{row.time}</span>
                </div>
              ))}
            </div>
          </div>
          <div className="copy rise">
            <span className="eyebrow">Drop-in workflow step</span>
            <h2>
              Add it to a flow in <em>under a minute</em>.
            </h2>
            <p>
              Connect your HighLevel sub-account once. The BluText action
              shows up as a native step in the workflow builder. No code, no
              custom webhooks, no Zapier middleman.
            </p>
            <p>
              Use merge fields, conditional logic, A/B splits — same as you
              would for any other action.
            </p>
          </div>
        </div>
      </section>

      <section className="alt-row dark">
        <div className="container">
          <div className="copy rise">
            <span className="eyebrow">Reply rate</span>
            <h2>
              <em>3× more replies</em> than the SMS step.
            </h2>
            <p>
              Across HighLevel customers running the same flow on both
              channels, BluText averages 30%+ reply rate vs. ~10% on
              shortcode SMS. People reply when the message looks like a
              friend.
            </p>
            <p>
              And because replies sync back into the contact record, your
              reps can pick up the conversation in HighLevel — or in the
              BluText shared inbox, whichever fits the workflow.
            </p>
          </div>
          <div className="visual rise" style={{ background: "rgba(255,255,255,0.04)", border: "1px solid rgba(255,255,255,0.1)" }}>
            <div className="reply-compare" style={{ background: "transparent", border: 0, boxShadow: "none", padding: 0, color: "#fff", width: "100%" }}>
              <div className="rc-row sms">
                <div className="rc-label" style={{ color: "rgba(255,255,255,0.85)" }}>
                  SMS<small style={{ color: "rgba(255,255,255,0.55)" }}>shortcode</small>
                </div>
                <div className="rc-bar" style={{ background: "rgba(255,255,255,0.1)" }}>
                  <i />
                </div>
                <div className="rc-num" style={{ color: "#fff" }}>10%</div>
              </div>
              <div className="rc-row imsg">
                <div className="rc-label" style={{ color: "rgba(255,255,255,0.85)" }}>
                  iMessage<small style={{ color: "rgba(255,255,255,0.55)" }}>via BluText</small>
                </div>
                <div className="rc-bar" style={{ background: "rgba(255,255,255,0.1)" }}>
                  <i />
                </div>
                <div className="rc-num" style={{ color: "var(--blu-light)" }}>30%</div>
              </div>
              <div className="rc-caption" style={{ color: "rgba(255,255,255,0.55)", borderTopColor: "rgba(255,255,255,0.15)" }}>
                Aggregate BluText/HighLevel customer benchmark.
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Pricing strip */}
      <section className="pricing-strip">
        <div className="container">
          <div>
            <h2>
              HighLevel + BluText, <em>built in</em>.
            </h2>
            <p>
              The integration is included with every plan — no separate add-on
              fee. Connect your HighLevel sub-account in a couple of clicks
              and drop the BluText action into any workflow.
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

      <MarketingFooter />
    </div>
  );
}
