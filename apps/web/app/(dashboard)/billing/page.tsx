"use client";

import { useState } from "react";
import useSWR from "swr";
import { CreditCardIcon, DownloadIcon, ExternalLinkIcon, CheckCircle2Icon } from "lucide-react";

const API = process.env.NEXT_PUBLIC_API_URL;

function fetcher(url: string) {
  const token = localStorage.getItem("access_token");
  return fetch(url, { headers: { Authorization: `Bearer ${token}` } }).then((r) => r.json());
}

interface Invoice {
  id: string;
  stripe_invoice_id: string;
  amount_paid: number;
  currency: string;
  status: string;
  invoice_pdf_url: string | null;
  period_start: string | null;
  period_end: string | null;
  paid_at: string | null;
  created_at: string;
}

interface SubscriptionInfo {
  status: string;
  plan: string;
  setup_fee_paid: boolean;
  has_subscription: boolean;
}

export default function BillingPage() {
  const { data: subData } = useSWR<SubscriptionInfo>(`${API}/api/billing/subscription`, fetcher);
  const { data: invoiceData } = useSWR<{ invoices: Invoice[] }>(`${API}/api/billing/invoices`, fetcher);
  const [loadingPortal, setLoadingPortal] = useState(false);

  async function openPortal() {
    setLoadingPortal(true);
    const token = localStorage.getItem("access_token");
    const res = await fetch(`${API}/api/billing/portal`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
    });
    const data = await res.json();
    if (data.url) {
      window.open(data.url, "_blank");
    }
    setLoadingPortal(false);
  }

  const planLabel = subData?.plan === "annual" ? "Annual" : "Monthly";
  const planPrice = subData?.plan === "annual" ? "$2,600/year" : "$199/month";

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Billing</h1>
        <p className="text-gray-500 text-sm mt-1">Manage your subscription and invoices</p>
      </div>

      {/* Subscription status */}
      <div className="bg-white rounded-2xl border border-gray-100 p-6">
        <div className="flex items-start justify-between">
          <div>
            <h2 className="font-semibold text-gray-900 mb-1">Current plan</h2>
            <div className="flex items-center gap-3 mt-2">
              <div className="text-2xl font-bold text-gray-900">{planLabel}</div>
              <span className="text-gray-500">{planPrice}</span>
              {subData?.status === "active" && (
                <span className="inline-flex items-center gap-1.5 text-xs font-medium bg-green-50 text-green-700 px-2.5 py-1 rounded-full border border-green-100">
                  <CheckCircle2Icon className="w-3 h-3" />
                  Active
                </span>
              )}
            </div>
            {subData?.setup_fee_paid && (
              <p className="text-sm text-gray-400 mt-2">
                <CheckCircle2Icon className="inline w-3.5 h-3.5 text-green-500 mr-1" />
                $399 setup fee paid
              </p>
            )}
          </div>
          <button
            onClick={openPortal}
            disabled={loadingPortal}
            className="flex items-center gap-2 bg-gray-900 text-white text-sm font-medium px-4 py-2.5 rounded-xl hover:bg-gray-800 transition-colors disabled:opacity-50"
          >
            <CreditCardIcon className="w-4 h-4" />
            {loadingPortal ? "Loading..." : "Manage billing"}
            <ExternalLinkIcon className="w-3.5 h-3.5" />
          </button>
        </div>
        <p className="text-xs text-gray-400 mt-4">
          Managed via Stripe. Update payment method, download invoices, or cancel from the portal.
        </p>
      </div>

      {/* Invoice history */}
      <div className="bg-white rounded-2xl border border-gray-100 overflow-hidden">
        <div className="px-6 py-4 border-b border-gray-50">
          <h2 className="font-semibold text-gray-900">Invoice history</h2>
        </div>

        {!invoiceData?.invoices || invoiceData.invoices.length === 0 ? (
          <div className="px-6 py-12 text-center text-gray-400 text-sm">
            No invoices yet
          </div>
        ) : (
          <div className="divide-y divide-gray-50">
            {invoiceData.invoices.map((inv) => (
              <div key={inv.id} className="px-6 py-4 flex items-center justify-between">
                <div>
                  <div className="text-sm font-medium text-gray-900">
                    ${(inv.amount_paid / 100).toFixed(2)}{" "}
                    <span className="uppercase text-gray-400 font-normal">{inv.currency}</span>
                  </div>
                  <div className="text-xs text-gray-400 mt-0.5">
                    {inv.paid_at
                      ? new Date(inv.paid_at).toLocaleDateString([], {
                          year: "numeric",
                          month: "long",
                          day: "numeric",
                        })
                      : new Date(inv.created_at).toLocaleDateString()}
                  </div>
                </div>
                <div className="flex items-center gap-3">
                  <span
                    className={`text-xs font-medium px-2.5 py-1 rounded-full ${
                      inv.status === "paid"
                        ? "bg-green-50 text-green-700"
                        : "bg-gray-50 text-gray-500"
                    }`}
                  >
                    {inv.status.charAt(0).toUpperCase() + inv.status.slice(1)}
                  </span>
                  {inv.invoice_pdf_url && (
                    <a
                      href={inv.invoice_pdf_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-gray-400 hover:text-gray-600 transition-colors"
                      title="Download PDF"
                    >
                      <DownloadIcon className="w-4 h-4" />
                    </a>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
