"use client";

import Link from "next/link";
import "../marketing.css";
import MarketingNav from "./MarketingNav";
import MarketingFooter from "./MarketingFooter";
import { useRiseOnScroll } from "./useRiseOnScroll";

/**
 * Shared layout for /solutions/* sub-niche pages.
 *
 * Each vertical (DTC, high-ticket, real estate, agencies) has the same shape:
 *   - Eyebrow + headline + sub-copy in the hero
 *   - Three "use case" pillars
 *   - One quote (testimonial)
 *   - Pricing strip + final CTA
 *
 * Only the copy + chat-preview content differs between verticals — kept here
 * so all four pages stay visually + structurally consistent and so a tweak
 * to the layout lands everywhere at once.
 */

export type Pillar = { title: string; body: string; glyph: string };

export type SolutionPageProps = {
  vertical: string; // "DTC", "High-ticket", etc.
  eyebrow: string;
  headline: React.ReactNode; // includes the <em> highlight
  lead: string;
  pillars: Pillar[];
  preview: { in: string[]; out: string[] }; // sample chat
  quote: { body: React.ReactNode; name: string; role: string };
};

export default function SolutionPage(props: SolutionPageProps) {
  useRiseOnScroll();

  // Interleave bubbles: out, in, out, in… for a natural-feeling preview.
  const bubbles: { side: "in" | "out"; text: string }[] = [];
  const max = Math.max(props.preview.in.length, props.preview.out.length);
  for (let i = 0; i < max; i++) {
    if (props.preview.out[i]) bubbles.push({ side: "out", text: props.preview.out[i] });
    if (props.preview.in[i]) bubbles.push({ side: "in", text: props.preview.in[i] });
  }

  return (
    <div className="marketing-page">
      <MarketingNav />

      <section className="sub-hero">
        <div className="container">
          <span className="eyebrow">{props.eyebrow}</span>
          <h1 className="display rise">{props.headline}</h1>
          <p className="lead rise">{props.lead}</p>
          <div className="cta-row" style={{ marginTop: 28, justifyContent: "center", display: "flex", gap: 12 }}>
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
      </section>

      {/* Pillars */}
      <section className="pillars">
        <div className="container">
          <div className="head rise">
            <span className="eyebrow">Built for {props.vertical}</span>
            <h2 className="display" style={{ marginTop: 14, fontSize: 44 }}>
              The plays <em>that actually convert</em>.
            </h2>
          </div>
          <div className="pillar-grid rise-group">
            {props.pillars.map((p) => (
              <div key={p.title} className="pillar-card">
                <div className="ic">{p.glyph}</div>
                <h3>{p.title}</h3>
                <p>{p.body}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Chat preview row */}
      <section className="alt-row">
        <div className="container">
          <div className="copy rise">
            <span className="eyebrow">Real conversation</span>
            <h2>
              What it actually <em>looks like</em>.
            </h2>
            <p>
              Your customer sees a blue bubble from a real number. They reply
              the way they reply to a friend — because, on their phone,
              that&apos;s exactly what it looks like.
            </p>
            <p>
              Your team picks it up in the BluText shared inbox or wherever
              your CRM already routes the contact.
            </p>
          </div>
          <div className="visual rise">
            <div className="mini-imessage" style={{ maxWidth: 320, margin: "0 auto" }}>
              {bubbles.map((b, i) => (
                <div
                  key={i}
                  className={`bubble ${b.side}`}
                  style={i === bubbles.length - 1 ? { borderRadius: 19 } : undefined}
                >
                  {b.text}
                </div>
              ))}
              <div className="im-status">Delivered · Read now</div>
            </div>
          </div>
        </div>
      </section>

      {/* Quote */}
      <section className="testimonial">
        <div
          className="container"
          style={{ gridTemplateColumns: "1fr", maxWidth: 820 }}
        >
          <div className="story rise">
            <div className="label">Customer story · {props.vertical}</div>
            <blockquote>{props.quote.body}</blockquote>
            <cite>
              <b>{props.quote.name}</b>
              {props.quote.role}
            </cite>
          </div>
        </div>
      </section>

      {/* Pricing strip */}
      <section className="pricing-strip">
        <div className="container">
          <div>
            <h2>
              One line. <em>Everything included.</em>
            </h2>
            <p>
              The Standard plan covers everything most{" "}
              {props.vertical.toLowerCase()} teams need. Enterprise unlocks
              multiple lines + volume pricing.
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
