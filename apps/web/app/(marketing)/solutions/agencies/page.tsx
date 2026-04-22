import type { Metadata } from "next";
import SolutionPage from "../../_components/SolutionPage";

export const metadata: Metadata = {
  title: "iMessage for agencies — BluText",
  description:
    "Run BluText across every client account from one dashboard. Per-client numbers, per-client analytics, white-label-ready. Built for HighLevel agencies and growth shops.",
  alternates: { canonical: "https://blutexts.com/solutions/agencies" },
};

export default function AgenciesSolution() {
  return (
    <SolutionPage
      vertical="Agencies"
      eyebrow="Solutions · Agencies"
      headline={
        <>
          One dashboard. <em>Every client&apos;s</em> blue bubble.
        </>
      }
      lead="Agencies running outbound for 5, 10, 50 clients shouldn't juggle 50 logins. BluText gives you per-client iMessage numbers, per-client analytics, and a single agency console — built for HighLevel SaaS-mode operators."
      pillars={[
        {
          title: "Per-client numbers, one console",
          body: "Provision a dedicated iMessage number per client sub-account. Manage them all from one agency dashboard with per-client analytics rolled up.",
          glyph: "1",
        },
        {
          title: "Resells inside HighLevel SaaS-mode",
          body: "Bolt BluText onto your HighLevel SaaS plan. Your clients see it as a native iMessage workflow step — you bill the markup.",
          glyph: "2",
        },
        {
          title: "Reports your clients actually open",
          body: "Reply rate by campaign, by sender, by day. The kind of report that makes your monthly retainer review feel like a no-brainer renewal.",
          glyph: "3",
        },
      ]}
      preview={{
        out: [
          "Hey — your monthly recap is ready 👇",
          "Reply rate ↑ 34% this month. Want me to walk through it?",
        ],
        in: ["yes — got 5 min now?"],
      }}
      quote={{
        body: (
          <>
            &ldquo;We white-label BluText into our HighLevel SaaS plan and
            charge $300/mo per client on top. <em>40 clients in</em>, it&apos;s
            the highest-margin add-on we&apos;ve ever shipped.&rdquo;
          </>
        ),
        name: "Devon Asher",
        role: "Founder, HighLevel agency · 40-client roster",
      }}
    />
  );
}
