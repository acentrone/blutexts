"use client";

import { useState } from "react";
import Link from "next/link";

interface AccountData {
  email: string;
  password: string;
  firstName: string;
  lastName: string;
  company: string;
  preferredAreaCode: string;
}

// We only ship one self-serve plan: $399/mo + $500 one-time startup.
// Enterprise (5+ lines, $199/line, annual) is sales-led — those customers
// hit /demo, not /signup. So this form has no plan picker.
export default function SignupPage() {
  const [accountData, setAccountData] = useState<AccountData>({
    email: "",
    password: "",
    firstName: "",
    lastName: "",
    company: "",
    preferredAreaCode: "",
  });
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleAccountSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");

    if (accountData.password.length < 8) {
      setError("Password must be at least 8 characters");
      return;
    }

    setLoading(true);
    try {
      const registerRes = await fetch(
        `${process.env.NEXT_PUBLIC_API_URL}/api/auth/register`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            email: accountData.email,
            password: accountData.password,
            first_name: accountData.firstName,
            last_name: accountData.lastName,
            company: accountData.company,
            preferred_area_code: accountData.preferredAreaCode,
          }),
        }
      );
      const registerData = await registerRes.json();
      if (!registerRes.ok) {
        throw new Error(registerData.error || "Registration failed");
      }
      localStorage.setItem("access_token", registerData.access_token);
      localStorage.setItem("refresh_token", registerData.refresh_token);

      const checkoutRes = await fetch(
        `${process.env.NEXT_PUBLIC_API_URL}/api/billing/checkout`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${registerData.access_token}`,
          },
          body: JSON.stringify({
            // Single plan — server uses STRIPE_PRICE_MONTHLY for the
            // recurring line item + STRIPE_PRICE_SETUP for the one-time fee.
            plan: "monthly",
            email: accountData.email,
            first_name: accountData.firstName,
            last_name: accountData.lastName,
            company: accountData.company,
          }),
        }
      );
      const checkoutData = await checkoutRes.json();
      if (!checkoutRes.ok || !checkoutData.url) {
        throw new Error(
          checkoutData.error ||
            "Could not start checkout. You can complete payment from the billing page."
        );
      }
      window.location.href = checkoutData.url;
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Registration failed");
      setLoading(false);
    }
  }

  function update<K extends keyof AccountData>(key: K, value: AccountData[K]) {
    setAccountData((prev) => ({ ...prev, [key]: value }));
  }

  return (
    <div className="app-root signup-page">
      <div className="signup-top">
        <Link href="/" className="brand">
          <span className="dot" />
          <span className="wordmark">Blu</span>
        </Link>
        <div className="right">
          Already have an account?
          <Link href="/login">Sign in</Link>
        </div>
      </div>

      <div className="signup-wrap">
        <aside className="signup-side">
          <div className="kicker-line">Sending iMessage at scale, since 2024</div>
          <h2>
            Blue bubbles, <em>answered</em>.
          </h2>
          <p className="lede">
            A dedicated iMessage number for your team, wired directly into your
            CRM. No A2P paperwork, no shortcode theater. Just a real thread with
            a real person on the other side.
          </p>

          <div className="mock-thread">
            <div className="bub out">We saved you the last vanilla protein 👀</div>
            <div className="bub out">Want me to lock it in?</div>
            <div className="bub in">Yes, absolutely 🙌</div>
            <div className="bub in">you&apos;re the man</div>
            <div className="caption">Real iMessage · via Blu · 30%+ reply rate</div>
          </div>
        </aside>

        <div className="signup-form">
          <form className="form-inner" onSubmit={handleAccountSubmit}>
            <h1>Create your account</h1>
            <div className="sub-h">You&apos;ll be sending iMessages in minutes.</div>

            <div className="field-grid">
              <div className="field">
                <label>First name</label>
                <input
                  className="input"
                  required
                  value={accountData.firstName}
                  onChange={(e) => update("firstName", e.target.value)}
                  placeholder="Jane"
                  autoComplete="given-name"
                />
              </div>
              <div className="field">
                <label>Last name</label>
                <input
                  className="input"
                  required
                  value={accountData.lastName}
                  onChange={(e) => update("lastName", e.target.value)}
                  placeholder="Smith"
                  autoComplete="family-name"
                />
              </div>
            </div>

            <div className="field">
              <label>Company</label>
              <input
                className="input"
                required
                value={accountData.company}
                onChange={(e) => update("company", e.target.value)}
                placeholder="Acme Corp"
                autoComplete="organization"
              />
            </div>

            <div className="field">
              <label>Work email</label>
              <input
                className="input"
                type="email"
                required
                value={accountData.email}
                onChange={(e) => update("email", e.target.value)}
                placeholder="you@company.com"
                autoComplete="email"
              />
            </div>

            <div className="field">
              <label>Password</label>
              <input
                className="input"
                type="password"
                required
                minLength={8}
                value={accountData.password}
                onChange={(e) => update("password", e.target.value)}
                placeholder="At least 8 characters"
                autoComplete="new-password"
              />
            </div>

            <div className="field">
              <label>Preferred area code</label>
              <input
                className="input"
                value={accountData.preferredAreaCode}
                onChange={(e) => update("preferredAreaCode", e.target.value)}
                placeholder="e.g. 305"
                inputMode="numeric"
                maxLength={3}
              />
              <div
                style={{
                  background: "var(--blu-tint)",
                  border: "1px solid var(--rule-2)",
                  borderRadius: 10,
                  padding: "10px 12px",
                  fontSize: 12.5,
                  color: "var(--ink-2)",
                  marginTop: 6,
                  display: "flex",
                  gap: 8,
                  alignItems: "flex-start",
                }}
              >
                <svg
                  style={{ color: "var(--blu)", flexShrink: 0, marginTop: 1 }}
                  width={14}
                  height={14}
                  viewBox="0 0 16 16"
                  fill="currentColor"
                >
                  <path d="M8 1a7 7 0 100 14A7 7 0 008 1zm0 4a1 1 0 110 2 1 1 0 010-2zm1 8H7v-5h2v5z" />
                </svg>
                <span>
                  We&apos;ll provision a dedicated number with this area code.
                  Usually takes less than 10 minutes.
                </span>
              </div>
            </div>

            <div className="field" style={{ marginTop: 8 }}>
              <label>Plan</label>
              <div
                className="plan selected"
                style={{
                  display: "flex",
                  flexDirection: "column",
                  gap: 4,
                  padding: "16px 18px",
                  cursor: "default",
                }}
              >
                <div className="name">BluTexts</div>
                <div className="price">$399/mo</div>
                <div style={{ fontSize: 12.5, color: "var(--muted)", marginTop: 4 }}>
                  1 dedicated number · 5 seats · unlimited messages ·
                  50 new conversations / day (Apple compliance cap)
                </div>
              </div>
              <div className="setup-fee-hint">
                Plus a one-time $500 startup fee · payment details next step.
                Need 5+ lines? <a href="/demo">Talk to sales</a> instead.
              </div>
            </div>

            {error && <div className="err" style={{ marginTop: 14 }}>{error}</div>}

            <button
              type="submit"
              disabled={loading}
              className="btn primary lg full submit"
            >
              {loading ? "Creating account…" : "Continue to payment →"}
            </button>
            <div className="tos">
              By continuing you agree to our{" "}
              <a href="/terms" style={{ color: "var(--muted)", textDecoration: "underline" }}>
                Terms
              </a>{" "}
              &amp;{" "}
              <a href="/privacy" style={{ color: "var(--muted)", textDecoration: "underline" }}>
                Privacy Policy
              </a>
              .
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}
