"use client";

import { useEffect } from "react";
import "./marketing.css";
import MarketingNav from "./_components/MarketingNav";
import MarketingFooter from "./_components/MarketingFooter";

/**
 * BluTexts marketing homepage.
 *
 * 1:1 implementation of homepage/direction-a.html from the Claude Design
 * handoff. The visual structure, copy, and animations are kept identical
 * to the design — see apps/web/app/(marketing)/marketing.css for the full
 * stylesheet. Only the routing/hrefs are wired into the real product
 * (login → /login, signup CTAs → /signup).
 *
 * Reveal-on-scroll uses an IntersectionObserver mirroring the design's
 * inline script. The "tweaks panel" from the design source (an editor-
 * preview tool) is intentionally omitted — not production behavior.
 */
export default function MarketingHomePage() {
  useEffect(() => {
    const io = new IntersectionObserver(
      (entries) => {
        entries.forEach((e) => {
          if (e.isIntersecting) {
            e.target.classList.add("in");
            io.unobserve(e.target);
          }
        });
      },
      { threshold: 0.1, rootMargin: "0px 0px -60px 0px" }
    );
    document
      .querySelectorAll(".marketing-page .rise, .marketing-page .rise-group")
      .forEach((el) => io.observe(el));
    return () => io.disconnect();
  }, []);

  return (
    <div className="marketing-page">
      <MarketingNav />

      {/* ═══ HERO ═══ */}
      <section className="hero">
        <div className="container">
          <div className="rise">
            <span className="eyebrow">Human conversation · not another blast</span>
            <h1 className="display">
              Sell where your customers <em>actually text</em>.
            </h1>
            <p className="lead">
              Sales stopped feeling like a conversation. Blu brings it back —
              real iMessage, real people, real replies. There&apos;s a person
              on the other end of the phone, and it finally looks like it.
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
              <a className="btn ghost" href="#how-it-works">
                See how it works
              </a>
            </div>
            <div className="meta-row">
              <span className="check">Dedicated numbers</span>
              <span className="check">No A2P roadblocks</span>
              <span className="check">Replies route to your CRM</span>
            </div>
          </div>

          <div className="device-wrap rise">
            <div className="device-glow" />
            <div className="iphone">
              <div className="iphone-screen">
                <div className="iphone-notch" />
                <div className="iphone-statusbar">
                  <span>9:41</span>
                  <span className="icons">
                    <span className="signal">
                      <i /><i /><i /><i />
                    </span>
                    <span style={{ fontWeight: 500 }}>5G</span>
                    <span className="battery">
                      <i />
                    </span>
                  </span>
                </div>
                <div className="imessage">
                  <div className="im-header">
                    <div className="avatar">A</div>
                    <div className="meta">
                      <div className="name">Scott · Truly Foods</div>
                      <div className="sub">iMessage · Active now</div>
                    </div>
                    <div className="icons">
                      <svg viewBox="0 0 20 20" fill="currentColor">
                        <path d="M10 2a8 8 0 100 16 8 8 0 000-16zm-.5 4a.5.5 0 011 0v4.3l3 2a.5.5 0 01-.6.8l-3.4-2.4V6z" />
                      </svg>
                    </div>
                  </div>
                  <div className="im-body">
                    <div className="im-daystamp">
                      <b>Today</b> 2:14 PM
                    </div>
                    <div className="bubble out" style={{ animationDelay: "0.3s" }}>
                      We saved you the last vanilla protein powder 👀
                    </div>
                    <div
                      className="bubble out"
                      style={{ animationDelay: "0.7s", borderRadius: "19px" }}
                    >
                      Want me to lock it in for you?
                    </div>
                    <div className="im-status" style={{ animationDelay: "0.9s" }}>
                      Delivered · Read 2:14 PM
                    </div>
                    <div className="bubble in" style={{ animationDelay: "1.5s" }}>
                      Yes, absolutely 🙌
                    </div>
                    <div
                      className="bubble in"
                      style={{ animationDelay: "1.8s", borderRadius: "19px" }}
                    >
                      you&apos;re the man
                    </div>
                    <div className="voice-msg" style={{ animationDelay: "2.5s" }}>
                      <div className="play">
                        <svg viewBox="0 0 10 10">
                          <path d="M2 1l7 4-7 4z" />
                        </svg>
                      </div>
                      <div className="wave">
                        {[30, 60, 90, 70, 40, 80, 50, 90, 30, 60, 80, 40, 50, 70, 20, 40].map(
                          (h, i) => (
                            <i key={i} style={{ height: `${h}%` }} />
                          )
                        )}
                      </div>
                      <span className="dur">0:12</span>
                    </div>
                    <div className="typing" style={{ animationDelay: "3.2s" }}>
                      <span /><span /><span />
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ═══ TRUST ═══ */}
      <section className="trust rise">
        <div className="trust-label">Trusted by conversation-driven brands</div>
        <div className="trust-logos">
          <span className="brand">Maison Noir</span>
          <span className="brand sans">Arcwell</span>
          <span className="brand">Ferment &amp; Co.</span>
          <span className="brand mono">meridian/</span>
          <span className="brand sans">HELIX</span>
          <span className="brand">Pale Harbor</span>
        </div>
      </section>

      {/* ═══ PROBLEM ═══ */}
      <section className="problem">
        <div className="container">
          <div className="head rise">
            <span className="eyebrow">The channel gap</span>
            <h2 className="display" style={{ marginTop: 18 }}>
              Sales stopped feeling human. <em>Let&apos;s bring it back.</em>
            </h2>
            <p className="lead" style={{ marginTop: 20 }}>
              Shortcode blasts get filtered, ignored, unsubscribed. iMessage is
              where your customers actually talk — to friends, to family, to the
              people they trust. There are real people on both sides of the
              conversation. It should look like it.
            </p>
          </div>

          <div className="compare rise-group">
            <div className="card bad">
              <span className="tag">Before · SMS blast</span>
              <h3>Green bubbles people swipe away.</h3>
              <p>
                Shortcodes, unsubscribe links, no reply expected. Reads as an ad
                — because it is one.
              </p>
              <div className="mini-chat">
                <div className="mini-bubble green">
                  VANILLA PROTEIN BACK IN STOCK — 20% off today only with code
                  VAN20. Reply STOP to unsubscribe.{" "}
                  <a
                    href="#"
                    style={{ color: "#fff", textDecoration: "underline" }}
                  >
                    bluelink.co/xyz
                  </a>
                </div>
              </div>
              <div className="stat-row">
                <div>
                  <span className="n">2.3%</span>Click-through
                </div>
                <div>
                  <span className="n">8 min</span>Avg. response
                </div>
                <div>
                  <span className="n">18%</span>Unsub rate
                </div>
              </div>
            </div>

            <div className="card good">
              <span className="tag">With Blu · iMessage</span>
              <h3>Blue bubbles that read like a friend.</h3>
              <p>
                Real numbers, real read receipts, real replies. The message
                reads human because it is.
              </p>
              <div className="mini-chat">
                <div className="mini-bubble blue">
                  We saved you the last vanilla protein powder. Did you want to
                  get that order in?
                </div>
                <div className="mini-bubble gray">
                  Yes, absolutely — you&apos;re the man 🙌
                </div>
              </div>
              <div className="stat-row">
                <div>
                  <span className="n">38%</span>Reply rate
                </div>
                <div>
                  <span className="n">48 sec</span>Avg. response
                </div>
                <div>
                  <span className="n">9×</span>Revenue per send
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ═══ HOW IT WORKS ═══ */}
      <section className="how" id="how-it-works">
        <div className="container">
          <div className="head rise">
            <span className="eyebrow">How it works</span>
            <h2 className="display" style={{ marginTop: 18 }}>
              A real number. Real people. <em>Real conversation.</em>
            </h2>
            <p className="lead">
              Skip the A2P paperwork and the shortcode theater. Blu drops your
              team into a web inbox — human on both ends.
            </p>
          </div>
          <div className="flow rise-group">
            <div className="step step1">
              <div className="vis">
                <div className="shield">
                  <svg viewBox="0 0 74 84" fill="none">
                    <defs>
                      <linearGradient id="shieldgrad" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="0" stopColor="#6FA7FF" />
                        <stop offset="1" stopColor="#2E6FE0" />
                      </linearGradient>
                    </defs>
                    <path
                      d="M37 2 L69 12 V40 C69 60 55 75 37 82 C19 75 5 60 5 40 V12 Z"
                      fill="url(#shieldgrad)"
                      stroke="#1D4ED8"
                      strokeWidth="1.2"
                    />
                    <path
                      d="M22 42 L33 53 L53 31"
                      fill="none"
                      stroke="#fff"
                      strokeWidth="5"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                    />
                  </svg>
                  <span className="stamp">Approved</span>
                </div>
              </div>
              <div className="n">01</div>
              <h4>A dedicated number, ready in minutes.</h4>
              <p>
                No A2P applications, no shortcode registration, no carrier
                gatekeeping. Your team gets a real number and a real inbox.
              </p>
            </div>
            <div className="arrow-sep">
              <svg
                width="24"
                height="16"
                viewBox="0 0 24 16"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.5"
              >
                <path d="M0 8h22M16 2l6 6-6 6" />
              </svg>
            </div>
            <div className="step step2">
              <div className="vis">
                <div className="cloud-box">B</div>
              </div>
              <div className="n">02</div>
              <h4>Messages flow through Blu.</h4>
              <p>
                Send from the web. Automate from your CRM. Every message
                lands as a native iMessage from your dedicated number — and
                replies route straight back into your inbox.
              </p>
            </div>
            <div className="arrow-sep">
              <svg
                width="24"
                height="16"
                viewBox="0 0 24 16"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.5"
              >
                <path d="M0 8h22M16 2l6 6-6 6" />
              </svg>
            </div>
            <div className="step step3">
              <div className="vis">
                <div style={{ width: "100%" }}>
                  {[
                    { dot: "", name: "Alex M.", preview: "saved for you", time: "2:14" },
                    { dot: "", name: "Priya S.", preview: "on my way", time: "2:09" },
                    { dot: "read", name: "Jordan L.", preview: "thanks!", time: "1:58" },
                    { dot: "read", name: "Marco V.", preview: "ill try it", time: "1:42" },
                    { dot: "read", name: "Casey N.", preview: "perfect", time: "1:31", last: true },
                  ].map((row, i) => (
                    <div
                      key={i}
                      className="inbox-row"
                      style={row.last ? { border: "none" } : undefined}
                    >
                      <span className={row.dot ? `dot ${row.dot}` : "dot"} />
                      <span className="name">{row.name}</span>
                      <span className="preview">{row.preview}</span>
                      <span className="time">{row.time}</span>
                    </div>
                  ))}
                </div>
              </div>
              <div className="n">03</div>
              <h4>Your team, one thread.</h4>
              <p>
                Shared inbox, routing, notes, assignments. Reps work where your
                customers already are — and finally sound like humans there.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* ═══ FEATURES ═══ */}
      <section className="features" id="features">
        <div className="container">
          <div className="head rise">
            <span className="eyebrow">Built for real conversation</span>
            <h2 className="display" style={{ marginTop: 18 }}>
              Everything iMessage does. <em>Natively.</em>
            </h2>
          </div>

          <div className="feat-grid rise-group">
            {/* Wide: Real iMessage */}
            <div className="feat wide">
              <div className="copy">
                <span className="kicker">Real iMessage · not a lookalike</span>
                <h3>Blue bubbles. Real numbers. Real read receipts.</h3>
                <p>
                  Every send lands as a native iMessage — dedicated number,
                  native read receipts, native typing indicators. Your customer
                  sees what they&apos;d see from a friend, because on their
                  phone, that&apos;s exactly how it looks.
                </p>
              </div>
              <div className="viz">
                <div className="mini-imessage">
                  <div className="bubble in" style={{ animationDelay: "0.2s" }}>
                    are you texting from your personal cell?
                  </div>
                  <div className="bubble out" style={{ animationDelay: "0.6s" }}>
                    Yep — it&apos;s Alex from Aesop 👋
                  </div>
                  <div className="bubble out" style={{ animationDelay: "0.8s" }}>
                    Same number going forward
                  </div>
                  <div className="im-status" style={{ animationDelay: "1s" }}>
                    Delivered · Read now
                  </div>
                </div>
              </div>
            </div>

            {/* Media */}
            <div className="feat feat-media">
              <span className="kicker">Images &amp; video</span>
              <h3>Send the look, not a link.</h3>
              <p>
                Share high-quality photos and video clips right in the thread —
                full resolution, no &ldquo;click to view.&rdquo; The product
                lands next to the pitch.
              </p>
              <div className="viz">
                <div className="media-viz">
                  <div className="media-card video">
                    <span className="tag-pill">Outgoing · Video</span>
                    <div className="play-badge">
                      <svg viewBox="0 0 10 10" fill="currentColor">
                        <path d="M2 1l7 4-7 4z" />
                      </svg>
                    </div>
                    <span className="dur">0:24</span>
                  </div>
                  <div className="reply-bubbles">
                    <div className="bb blue">Your car has arrived 🚗 Check it out</div>
                    <div className="bb blue">
                      Keys ready at the front desk whenever you are
                    </div>
                    <div className="bb gray">On my way down now — thank you!</div>
                  </div>
                </div>
              </div>
            </div>

            {/* Effects */}
            <div className="feat feat-effects">
              <span className="kicker">Effects &amp; reactions</span>
              <h3>Slam, confetti, tapbacks — all native.</h3>
              <p>
                Every iMessage effect Apple ships. Tapbacks that render
                in-thread on both sides. The playful stuff still feels playful —
                because it&apos;s the same stuff people send to friends.
              </p>
              <div className="viz">
                <div className="effects">
                  {[
                    { emo: "💥", label: "Slam", blue: true },
                    { emo: "🔊", label: "Loud" },
                    { emo: "🫧", label: "Gentle" },
                    { emo: "🫥", label: "Invisible Ink" },
                    { emo: "🎉", label: "Confetti", blue: true },
                    { emo: "🎈", label: "Balloons" },
                    { emo: "❤️", label: "Heart" },
                    { emo: "👍", label: "Thumbs up" },
                    { emo: "😂", label: "Haha" },
                    { emo: "‼️", label: "Emphasis" },
                  ].map((c) => (
                    <span key={c.label} className={c.blue ? "chip blue" : "chip"}>
                      <span className="emo">{c.emo}</span>
                      {c.label}
                    </span>
                  ))}
                </div>
              </div>
            </div>

            {/* Voice */}
            <div className="feat feat-voice">
              <span className="kicker">Voice messages</span>
              <h3>Founders on the phone, not on email.</h3>
              <p>
                Send real iMessage voice notes with waveform playback. Receive
                inbound the same way. This is the feature that makes
                founder-led and concierge outreach feel unmistakably human.
              </p>
              <div className="viz">
                <div className="voice-stack">
                  <div className="voice-msg out">
                    <div className="play">
                      <svg viewBox="0 0 10 10">
                        <path d="M2 1l7 4-7 4z" />
                      </svg>
                    </div>
                    <div className="wave">
                      {[30, 60, 90, 70, 40, 80, 50, 90, 30, 60, 80, 40, 50, 70, 20, 40, 75, 55].map(
                        (h, i) => (
                          <i key={i} style={{ height: `${h}%` }} />
                        )
                      )}
                    </div>
                    <span className="dur">0:18</span>
                  </div>
                  <div className="voice-msg in">
                    <div className="play">
                      <svg viewBox="0 0 10 10">
                        <path d="M2 1l7 4-7 4z" />
                      </svg>
                    </div>
                    <div className="wave">
                      {[40, 70, 30, 85, 55, 45, 75, 35, 90, 50, 65, 25, 80, 40, 60].map(
                        (h, i) => (
                          <i key={i} style={{ height: `${h}%` }} />
                        )
                      )}
                    </div>
                    <span className="dur">0:09</span>
                  </div>
                </div>
              </div>
            </div>

            {/* Integrations */}
            <div className="feat feat-ghl">
              <span className="kicker">Integrations</span>
              <h3>Plugs into the stack you already run.</h3>
              <p>
                Trigger Blu sends from HighLevel workflows. Push conversation
                data back into contact records. HubSpot, Close, and Salesforce
                next.
              </p>
              <div className="viz">
                <div className="integration">
                  <div className="logo-pill active">
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img src="/marketing/ghl.svg" alt="HighLevel" />
                    <span>HighLevel</span>
                    <span className="status">Live</span>
                  </div>
                  <div className="logo-pill soon">
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img src="/marketing/hubspot.png" alt="HubSpot" />
                    <span>HubSpot</span>
                    <span className="status">Soon</span>
                  </div>
                  <div className="logo-pill soon">
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img src="/marketing/close.png" alt="Close" />
                    <span>Close</span>
                    <span className="status">Soon</span>
                  </div>
                  <div className="logo-pill soon">
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img src="/marketing/salesforce.png" alt="Salesforce" />
                    <span>Salesforce</span>
                    <span className="status">Soon</span>
                  </div>
                </div>
              </div>
            </div>

            {/* Reply-rate analytics */}
            <div className="feat feat-analytics wide">
              <div className="copy">
                <span className="kicker">Reply rate · the metric that matters</span>
                <span className="hero-lift">3×</span>
                <div className="hero-lift-sub">Average lift in reply rate</div>
                <h3>
                  Blue bubbles get{" "}
                  <em
                    style={{
                      fontFamily: "'Instrument Serif',serif",
                      color: "var(--blu)",
                      fontStyle: "italic",
                    }}
                  >
                    answered
                  </em>
                  .
                </h3>
                <p>
                  When messages read as a conversation, customers treat them
                  like one. Across aggregate Blu customer data, brands see a{" "}
                  <b style={{ color: "var(--blu)", fontWeight: 600 }}>
                    3× lift
                  </b>{" "}
                  in replies compared to the same list over SMS.
                </p>
              </div>
              <div className="viz">
                <div className="reply-compare">
                  <div className="rc-row sms">
                    <div className="rc-label">
                      SMS<small>shortcode blast</small>
                    </div>
                    <div className="rc-bar">
                      <i />
                    </div>
                    <div className="rc-num">10%</div>
                  </div>
                  <div className="rc-row imsg">
                    <div className="rc-label">
                      iMessage<small>via Blu</small>
                    </div>
                    <div className="rc-bar">
                      <i />
                    </div>
                    <div className="rc-num">30%</div>
                  </div>
                  <div className="rc-caption">
                    Aggregate Blu customer data · reply rate benchmark.
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ═══ TESTIMONIAL ═══ */}
      <section className="testimonial" id="customers">
        <div className="container rise-group">
          <div className="story rise">
            <div className="label">Customer story · Concierge medicine</div>
            <div>
              <div className="brand-name">Nicholas Venazio</div>
              <div className="brand-meta">
                Owner · concierge medicine chain · $4M/yr
              </div>
            </div>
            <blockquote>
              &ldquo;Our members expect a text back from a real person. Blu made
              that possible at scale — <em>reply rates tripled</em> and our
              front desk finally stopped drowning in voicemails.&rdquo;
            </blockquote>
            <cite>
              <b>Nicholas Venazio</b>
              Owner, concierge medicine chain
            </cite>
          </div>
          <div className="story rise">
            <div className="label">Customer story · DTC · Shopify</div>
            <div>
              <div className="brand-name">Scott Simmons</div>
              <div className="brand-meta">
                Founder · protein cookie brand · 8-figure DTC
              </div>
            </div>
            <blockquote>
              &ldquo;Winback over SMS was dead on arrival. On Blu, the same
              flow pulled <em>30%+ reply rates</em> and customers started
              telling us which flavor to launch next — in the thread.&rdquo;
            </blockquote>
            <cite>
              <b>Scott Simmons</b>
              Founder, protein cookie brand (Shopify DTC)
            </cite>
          </div>
        </div>
      </section>

      {/* ═══ FINAL CTA ═══ */}
      <section className="final" id="pricing">
        <div className="container rise">
          <span className="eyebrow">Ready?</span>
          <h2 className="display" style={{ marginTop: 18 }}>
            Put a <em>person</em> back in the conversation.
          </h2>
          <p className="lead">
            Book a 30-minute demo. We&apos;ll provision a live number and send
            you a real iMessage from it before the call ends — founder to
            founder.
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
            <a className="btn ghost" href="/signup">
              Sign up
            </a>
          </div>
          <div className="note">No apps. No shortcodes. Just real iMessage.</div>
        </div>
      </section>

      <MarketingFooter />
    </div>
  );
}
