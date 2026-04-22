"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

/**
 * Shared marketing nav. Sticky, glassy, with a hamburger drawer on mobile.
 *
 * The drawer mirrors desktop nav links + adds the secondary actions ("Sign in",
 * "Book a demo") so the entire menu is reachable on phones — no more
 * "where did the nav go" once we drop the inline links at <980px.
 *
 * Used by every marketing page (homepage, /pricing, /integrations, /solutions/*,
 * /demo, /privacy, /terms) so a nav change lands everywhere at once.
 */
export default function MarketingNav() {
  const [open, setOpen] = useState(false);

  // Close the drawer when the user navigates (anchor click, link to another
  // page, etc.) and lock body scroll while it's open.
  useEffect(() => {
    if (!open) return;
    const original = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = original;
    };
  }, [open]);

  // Esc to close — small accessibility nicety.
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open]);

  return (
    <nav className="nav">
      <div className="nav-inner">
        <Link href="/" className="logo-lockup" onClick={() => setOpen(false)}>
          <span className="logo-dot" />
          <span className="wordmark">
            <em>Blu</em>Text
          </span>
        </Link>
        <div className="nav-links">
          <Link href="/#features">Product</Link>
          <Link href="/integrations">Integrations</Link>
          <Link href="/pricing">Pricing</Link>
          <Link href="/#customers">Customers</Link>
        </div>
        <div className="nav-cta">
          <Link className="signin" href="/login">
            Sign in
          </Link>
          <a
            className="btn dark"
            href="/demo"
            data-iclosed-link="https://app.iclosed.io/e/blutext/imessage-demo"
            data-embed-type="popup"
          >
            Book a demo <span className="arrow">→</span>
          </a>
          <button
            type="button"
            className="nav-burger"
            aria-label={open ? "Close menu" : "Open menu"}
            aria-expanded={open}
            onClick={() => setOpen((v) => !v)}
          >
            <span />
            <span />
            <span />
          </button>
        </div>
      </div>

      {/* Mobile drawer. Hidden on >=980px via CSS. */}
      <div className={`nav-drawer${open ? " open" : ""}`} aria-hidden={!open}>
        <div className="nav-drawer-inner">
          <Link href="/#features" onClick={() => setOpen(false)}>
            Product
          </Link>
          <Link href="/integrations" onClick={() => setOpen(false)}>
            Integrations
          </Link>
          <Link href="/pricing" onClick={() => setOpen(false)}>
            Pricing
          </Link>
          <Link href="/#customers" onClick={() => setOpen(false)}>
            Customers
          </Link>
          <div className="nav-drawer-divider" />
          <Link href="/login" onClick={() => setOpen(false)}>
            Sign in
          </Link>
          <Link href="/signup" onClick={() => setOpen(false)}>
            Sign up
          </Link>
          <a
            className="btn dark"
            href="/demo"
            data-iclosed-link="https://app.iclosed.io/e/blutext/imessage-demo"
            data-embed-type="popup"
            onClick={() => setOpen(false)}
            style={{ marginTop: 8, justifyContent: "center" }}
          >
            Book a demo <span className="arrow">→</span>
          </a>
        </div>
      </div>
    </nav>
  );
}
