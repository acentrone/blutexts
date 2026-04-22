import Link from "next/link";

/**
 * Shared marketing footer.
 *
 * Footer schema (deliberate, post-launch cleanup):
 *   - Product:   Features, Integrations, Pricing
 *   - Solutions: DTC brands, High-ticket, Real estate, Agencies
 *   - Company:   About, Customers, Careers, Contact
 *   - Legal:     Privacy, Terms
 *
 * Removed (per founder direction): TCPA, Compliance, Security — those were
 * placeholder links with no destination. They'll come back when there's a
 * real page for each (likely as part of an /trust hub later).
 */
export default function MarketingFooter() {
  return (
    <footer className="footer" id="company">
      <div className="container">
        <div>
          <Link href="/" className="logo-lockup">
            <span className="logo-dot" />
            <span className="wordmark">
              <em>Blu</em>Text
            </span>
          </Link>
          <p className="tag">
            Real iMessage for business. Bringing human connection back to the
            sales experience — one blue bubble at a time.
          </p>
        </div>
        <div>
          <h5>Product</h5>
          <ul>
            <li><Link href="/#features">Features</Link></li>
            <li><Link href="/integrations">Integrations</Link></li>
            <li><Link href="/pricing">Pricing</Link></li>
          </ul>
        </div>
        <div>
          <h5>Solutions</h5>
          <ul>
            <li><Link href="/solutions/dtc">DTC brands</Link></li>
            <li><Link href="/solutions/high-ticket">High-ticket</Link></li>
            <li><Link href="/solutions/real-estate">Real estate</Link></li>
            <li><Link href="/solutions/agencies">Agencies</Link></li>
          </ul>
        </div>
        <div>
          <h5>Company</h5>
          <ul>
            <li><Link href="/#company">About</Link></li>
            <li><Link href="/#customers">Customers</Link></li>
            <li><a href="mailto:hello@blutexts.com">Contact</a></li>
          </ul>
        </div>
        <div>
          <h5>Legal</h5>
          <ul>
            <li><Link href="/privacy">Privacy</Link></li>
            <li><Link href="/terms">Terms</Link></li>
          </ul>
        </div>
      </div>
      <div className="container footer-bottom">
        <div>© {new Date().getFullYear()} BluText</div>
        <div>
          iMessage is a trademark of Apple Inc. BluText is not affiliated
          with Apple.
        </div>
      </div>
    </footer>
  );
}
