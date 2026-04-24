"use client";

import useSWR from "swr";
import Link from "next/link";
import { useState } from "react";

const API = process.env.NEXT_PUBLIC_API_URL;

type DateRange = "7d" | "30d" | "90d";

function fetcher(url: string) {
  const token = localStorage.getItem("access_token");
  return fetch(url, {
    headers: { Authorization: `Bearer ${token}` },
  }).then((r) => {
    if (r.status === 401) {
      localStorage.removeItem("access_token");
      localStorage.removeItem("refresh_token");
      window.location.href = "/login";
      throw new Error("unauthorized");
    }
    return r.json();
  });
}

interface ServiceStats {
  sent: number;
  delivered: number;
  contacts_messaged: number;
  contacts_replied: number;
  reply_rate: number;
}

interface DashboardStats {
  total_sent: number;
  total_delivered: number;
  total_replied: number;
  response_rate: number;
  active_conversations: number;
  today_new_contacts: number;
  daily_limit: number;
  breakdown: { imessage: ServiceStats; sms: ServiceStats };
  from: string;
  to: string;
}

interface AccountInfo {
  user: { email: string; first_name?: string };
  account: {
    id: string;
    name: string;
    status: string;
    plan: string;
    setup_complete: boolean;
  };
}

interface PhoneInfo {
  has_number: boolean;
  phone?: {
    number: string;
    imessage_address: string | null;
    display_name: string | null;
    status: string;
    device_name: string | null;
    device_status: string | null;
    device_last_seen: string | null;
  };
}

interface GHLStatus {
  connected: boolean;
  location_id?: string;
}

function formatPhone(s: string): string {
  const d = s.replace(/\D/g, "");
  if (d.length === 11 && d[0] === "1") {
    return `+1 (${d.slice(1, 4)}) ${d.slice(4, 7)}-${d.slice(7)}`;
  }
  if (d.length === 10) {
    return `+1 (${d.slice(0, 3)}) ${d.slice(3, 6)}-${d.slice(6)}`;
  }
  return s;
}

export default function DashboardPage() {
  // Date range only affects the response-rate comparison panel — the
  // top-line stat cards still pull from the same endpoint, but the values
  // shift with the range so the dashboard tells one consistent story.
  const [range, setRange] = useState<DateRange>("30d");

  const { data: meData } = useSWR<AccountInfo>(`${API}/api/auth/me`, fetcher);
  const { data: stats } = useSWR<DashboardStats>(
    `${API}/api/dashboard/stats?range=${range}`,
    fetcher,
    { refreshInterval: 30000 }
  );
  const { data: phoneData } = useSWR<PhoneInfo>(
    `${API}/api/account/info`,
    fetcher,
    { refreshInterval: 15000 }
  );
  const { data: ghlStatus } = useSWR<GHLStatus>(
    meData?.account ? `${API}/api/integration/status` : null,
    fetcher,
    { refreshInterval: 15000 }
  );

  const account = meData?.account;
  const ghlConnected = ghlStatus?.connected === true;
  const hasNumber = phoneData?.has_number === true;
  const deviceOnline = phoneData?.phone?.device_status === "online";
  const setupComplete = hasNumber && deviceOnline && ghlConnected;
  const completedCount = [hasNumber, deviceOnline, ghlConnected].filter(Boolean).length;
  const progressClass =
    completedCount === 3 ? "progress-100" : completedCount === 2 ? "progress-66" : completedCount === 1 ? "progress-33" : "progress-0";

  const dailyPct =
    stats && stats.daily_limit > 0
      ? Math.min((stats.today_new_contacts / stats.daily_limit) * 100, 100)
      : 0;
  const remaining = stats ? stats.daily_limit - stats.today_new_contacts : 0;

  async function connectGHL() {
    if (!account) return;
    const token = localStorage.getItem("access_token");
    const res = await fetch(
      `${API}/api/oauth/connect?account_id=${account.id}`,
      { headers: { Authorization: `Bearer ${token}` } }
    );
    const data = await res.json();
    if (data.url) window.location.href = data.url;
  }

  return (
    <>
      <header className="page-header">
        <div className="titles">
          <div className="crumb">Home</div>
          <h1>
            Welcome back,{" "}
            <em>{meData?.user?.first_name || account?.name || "Admin"}</em>.
          </h1>
          <div className="sub">
            {setupComplete
              ? "Everything's running smoothly."
              : "Let's finish setting up your account."}
          </div>
        </div>
        <div style={{ display: "flex", gap: 10 }}>
          <Link href="/messages" className="btn secondary">
            <svg width="15" height="15" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.8">
              <path d="M10 3v14M3 10h14" strokeLinecap="round" />
            </svg>
            New message
          </Link>
        </div>
      </header>

      <div className="page-body">
        {!setupComplete && (
          <div className={`checklist-card ${progressClass}`}>
            <div className="title-row">
              <h3>Getting started</h3>
              <span className="progress-text">{completedCount} / 3 complete</span>
            </div>
            <div className="sub">Complete these steps to start sending iMessages.</div>

            <div className="checklist">
              <Step
                done={hasNumber}
                title="Phone number assigned"
                desc={
                  hasNumber && phoneData?.phone
                    ? <>Your dedicated number: <b>{formatPhone(phoneData.phone.number)}</b></>
                    : "Waiting for our team to assign your dedicated iMessage number."
                }
              />
              <Step
                done={deviceOnline}
                active={hasNumber && !deviceOnline}
                title="Sending device online"
                desc={
                  deviceOnline
                    ? <>Your dedicated iPhone is connected and ready to send.</>
                    : hasNumber
                    ? <>Your hosted iPhone is briefly offline — our team is bringing it back up. No action needed on your end. If this persists more than 30 minutes, ping <a href="mailto:hello@blutexts.com">support</a>.</>
                    : <>Activates automatically once your number is assigned. BluTexts hosts the iPhone — there&apos;s nothing to install.</>
                }
              />
              <Step
                done={ghlConnected}
                active={!ghlConnected && hasNumber}
                title="HighLevel connected"
                desc={
                  ghlConnected
                    ? "Messages will sync to your GHL location."
                    : "Connect your GHL account to enable message sync."
                }
                ctaLabel={!ghlConnected ? "Connect →" : undefined}
                onCtaClick={!ghlConnected ? connectGHL : undefined}
              />
            </div>
          </div>
        )}

        {hasNumber && phoneData?.phone && (
          <div className="number-card">
            <div>
              <div className="lbl">
                <svg viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5">
                  <path
                    d="M4.5 1h5l1 2.5-1.5 1a9 9 0 004 4l1-1.5L13 8v4.5a.5.5 0 01-.5.5C6 13 1 8 1 1.5A.5.5 0 011.5 1H4.5z"
                    strokeLinejoin="round"
                  />
                </svg>
                Your Blu number
              </div>
              <div className="num">{formatPhone(phoneData.phone.number)}</div>
              <div className="sub">
                Primary · dedicated to your workspace
                {phoneData.phone.display_name ? ` · ${phoneData.phone.display_name}` : ""}
              </div>
            </div>
            <div className="device">
              <span className={`pill${deviceOnline ? " online" : ""}`}>
                <span className="d" /> {deviceOnline ? "Online" : "Offline"}
              </span>
              {phoneData.phone.device_name && (
                <span className="did">{phoneData.phone.device_name}</span>
              )}
            </div>
          </div>
        )}

        <div className="stats">
          <div className="stat">
            <div className="ico">
              <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.8">
                <path d="M1.5 8L14.5 2 10 14.5l-2-6-6.5-.5z" strokeLinejoin="round" />
              </svg>
            </div>
            <div className="n">{(stats?.total_sent ?? 0).toLocaleString()}</div>
            <div className="label">Messages sent</div>
            <div className="sub">{rangeLabel(range)}</div>
          </div>

          <div className="stat success">
            <div className="ico">
              <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2">
                <circle cx="8" cy="8" r="6" />
                <path d="M5.5 8l2 2 3.5-3.5" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
            </div>
            <div className="n">{(stats?.total_delivered ?? 0).toLocaleString()}</div>
            <div className="label">Delivered</div>
            {stats && stats.total_sent > 0 && (
              <div className="sub">
                {Math.round((stats.total_delivered / stats.total_sent) * 100)}% delivery rate
              </div>
            )}
          </div>

          <div className="stat purple">
            <div className="ico">
              <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.8">
                <path d="M2.5 4a2 2 0 012-2h7a2 2 0 012 2v5a2 2 0 01-2 2h-5l-4 3V4z" strokeLinejoin="round" />
              </svg>
            </div>
            <div className="n">{(stats?.total_replied ?? 0).toLocaleString()}</div>
            <div className="label">Replies received</div>
            {stats?.response_rate != null && stats.total_sent > 0 && (
              <div className="sub">{stats.response_rate.toFixed(1)}% reply rate</div>
            )}
          </div>

          <div className="stat amber">
            <div className="ico">
              <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.8">
                <circle cx="6" cy="5.5" r="2.5" />
                <path
                  d="M1 13c.5-2.5 2.3-4 5-4s4.5 1.5 5 4M12 2.5a2.5 2.5 0 010 5M15 13c-.3-2-.8-3.5-2.5-4"
                  strokeLinecap="round"
                />
              </svg>
            </div>
            <div className="n">
              {stats?.today_new_contacts ?? 0}
              <span style={{ fontSize: 20, color: "var(--muted)", fontWeight: 500 }}>
                /{stats?.daily_limit ?? 50}
              </span>
            </div>
            <div className="label">New contacts today</div>
            <div className="sub">Daily limit</div>
            {stats && (
              <span className="trend flat">{remaining} remaining</span>
            )}
          </div>
        </div>

        {/* iMessage vs SMS reply-rate comparison.
            Hidden until there's at least some send activity so we don't
            display an empty bar chart on a brand-new account. */}
        {stats && (stats.breakdown.imessage.sent > 0 || stats.breakdown.sms.sent > 0) && (
          <ServiceComparison stats={stats} range={range} setRange={setRange} />
        )}

        {stats && stats.daily_limit > 0 && (
          <div className="limit-card">
            <div className="head">
              <div>
                <h4>Daily new contact limit</h4>
                <div className="sub">
                  {stats.today_new_contacts} of {stats.daily_limit} new contacts messaged today
                </div>
              </div>
              <div className="remaining">{remaining} remaining</div>
            </div>
            <div className="bar">
              <i style={{ width: `${dailyPct}%` }} />
            </div>
            <div className="foot">
              Resets at midnight · Existing conversation replies are unlimited.
            </div>
          </div>
        )}
      </div>
    </>
  );
}

function Step({
  done,
  active,
  title,
  desc,
  ctaLabel,
  ctaHref,
  onCtaClick,
}: {
  done: boolean;
  active?: boolean;
  title: string;
  desc: React.ReactNode;
  ctaLabel?: string;
  ctaHref?: string;
  onCtaClick?: () => void;
}) {
  const cls = done ? "step done" : active ? "step active" : "step pending";
  return (
    <div className={cls}>
      <div className="marker">
        {done && (
          <svg viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M2.5 6.5l2.5 2.5L9.5 3.5" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        )}
      </div>
      <div className="body">
        <div className="name">{title}</div>
        <div className="desc">{desc}</div>
      </div>
      {ctaLabel && ctaHref && (
        <Link className="cta" href={ctaHref}>
          {ctaLabel}
        </Link>
      )}
      {ctaLabel && onCtaClick && (
        <button
          className="cta"
          onClick={onCtaClick}
          type="button"
          style={{ background: "none", border: 0, cursor: "pointer" }}
        >
          {ctaLabel}
        </button>
      )}
    </div>
  );
}

function rangeLabel(r: DateRange): string {
  return r === "7d" ? "Last 7 days" : r === "90d" ? "Last 90 days" : "Last 30 days";
}

/**
 * iMessage vs SMS reply-rate comparison panel.
 *
 * Anatomy:
 *   - Header with title + range tabs (7d / 30d / 90d) — clicking a tab
 *     re-fetches the parent's stats query with the new ?range param.
 *   - Two side-by-side rows, one per service. Each shows:
 *       reply rate (the headline metric)
 *       contacts replied / contacts messaged (the underlying numbers)
 *       a horizontal bar visualizing the two rates against the same scale
 *       sent / delivered counts as secondary stats
 *
 * The bar scale is normalized to whichever rate is higher, capped at 100%,
 * so the visual difference between e.g. 32% iMessage and 8% SMS reads at
 * a glance — the iMessage bar is fully extended, the SMS bar shows ~25%
 * of that. Same scale = direct comparison.
 */
function ServiceComparison({
  stats,
  range,
  setRange,
}: {
  stats: DashboardStats;
  range: DateRange;
  setRange: (r: DateRange) => void;
}) {
  const im = stats.breakdown.imessage;
  const sms = stats.breakdown.sms;
  const maxRate = Math.max(im.reply_rate, sms.reply_rate, 1); // avoid /0

  // Pick the winning channel. Reads as a one-glance verdict at the top
  // of the panel, even before the customer parses the bars below.
  let verdict: React.ReactNode = null;
  if (im.contacts_messaged > 0 && sms.contacts_messaged > 0) {
    const ratio = sms.reply_rate > 0 ? im.reply_rate / sms.reply_rate : Infinity;
    if (Number.isFinite(ratio) && ratio >= 1.1) {
      verdict = (
        <>iMessage is pulling <b>{ratio.toFixed(1)}× more replies</b> than SMS this period.</>
      );
    } else if (ratio < 0.9) {
      verdict = (
        <>SMS is outperforming iMessage on this list — worth a look.</>
      );
    } else {
      verdict = <>Both channels performing similarly this period.</>;
    }
  } else if (im.contacts_messaged > 0) {
    verdict = <>All sends went via iMessage this period.</>;
  } else if (sms.contacts_messaged > 0) {
    verdict = <>All sends went via SMS this period.</>;
  }

  return (
    <div className="limit-card" style={{ marginTop: 0 }}>
      <div
        className="head"
        style={{ alignItems: "flex-start", flexWrap: "wrap", gap: 16 }}
      >
        <div>
          <h4>iMessage vs SMS reply rate</h4>
          <div className="sub">{verdict ?? rangeLabel(range)}</div>
        </div>
        <div
          role="tablist"
          aria-label="Date range"
          style={{
            display: "inline-flex",
            background: "var(--paper, #f4f5f7)",
            borderRadius: 8,
            padding: 3,
            fontSize: 12,
            fontWeight: 600,
          }}
        >
          {(["7d", "30d", "90d"] as DateRange[]).map((r) => {
            const on = r === range;
            return (
              <button
                key={r}
                type="button"
                role="tab"
                aria-selected={on}
                onClick={() => setRange(r)}
                style={{
                  padding: "5px 12px",
                  borderRadius: 6,
                  border: 0,
                  cursor: "pointer",
                  background: on ? "#fff" : "transparent",
                  color: on ? "var(--ink, #0b1220)" : "var(--muted, #6b7280)",
                  boxShadow: on ? "0 1px 3px rgba(11,18,32,0.08)" : "none",
                }}
              >
                {r === "7d" ? "7 days" : r === "30d" ? "30 days" : "90 days"}
              </button>
            );
          })}
        </div>
      </div>

      <div style={{ marginTop: 18, display: "grid", gap: 14 }}>
        <ServiceRow
          label="iMessage"
          accent="var(--blu, #2e6fe0)"
          stats={im}
          maxRate={maxRate}
        />
        <ServiceRow
          label="SMS"
          accent="#3fc34c"
          stats={sms}
          maxRate={maxRate}
        />
      </div>

      <div className="foot" style={{ marginTop: 14 }}>
        Reply rate = unique contacts who replied ÷ unique contacts messaged in the window.
        A contact who got both channels and replied counts for both.
      </div>
    </div>
  );
}

function ServiceRow({
  label,
  accent,
  stats,
  maxRate,
}: {
  label: string;
  accent: string;
  stats: ServiceStats;
  maxRate: number;
}) {
  // Bar width relative to the higher of the two rates. Floor at 2% so a
  // tiny but non-zero rate still shows a sliver instead of disappearing.
  const barPct = stats.reply_rate > 0 ? Math.max((stats.reply_rate / maxRate) * 100, 2) : 0;

  return (
    <div
      style={{
        display: "grid",
        gridTemplateColumns: "100px 1fr auto",
        gap: 16,
        alignItems: "center",
      }}
    >
      <div>
        <div style={{ fontSize: 13, fontWeight: 600, color: "var(--ink, #0b1220)" }}>
          {label}
        </div>
        <div style={{ fontSize: 11, color: "var(--muted, #6b7280)", marginTop: 2 }}>
          {stats.sent.toLocaleString()} sent · {stats.delivered.toLocaleString()} delivered
        </div>
      </div>

      <div
        style={{
          height: 14,
          background: "var(--paper-2, #f4f5f7)",
          borderRadius: 7,
          overflow: "hidden",
          position: "relative",
        }}
      >
        <div
          style={{
            width: `${barPct}%`,
            height: "100%",
            background: accent,
            borderRadius: 7,
            transition: "width 0.4s cubic-bezier(0.22, 1, 0.36, 1)",
          }}
        />
      </div>

      <div style={{ minWidth: 110, textAlign: "right" }}>
        <div
          style={{
            fontFamily: "'Instrument Serif', serif",
            fontStyle: "italic",
            fontSize: 28,
            lineHeight: 1,
            letterSpacing: "-0.4px",
            color: stats.reply_rate > 0 ? accent : "var(--muted, #6b7280)",
          }}
        >
          {stats.reply_rate.toFixed(1)}%
        </div>
        <div style={{ fontSize: 11, color: "var(--muted, #6b7280)", marginTop: 2 }}>
          {stats.contacts_replied.toLocaleString()} / {stats.contacts_messaged.toLocaleString()} contacts
        </div>
      </div>
    </div>
  );
}
