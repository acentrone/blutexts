"use client";

import { useState } from "react";
import useSWR from "swr";
import { SearchIcon, CheckCircle2Icon, AlertTriangleIcon, ClockIcon, XCircleIcon } from "lucide-react";

const API = process.env.NEXT_PUBLIC_API_URL;
const ADMIN_KEY = process.env.NEXT_PUBLIC_ADMIN_KEY;

function adminFetcher(url: string) {
  return fetch(url, { headers: { "X-Admin-Key": ADMIN_KEY || "" } }).then((r) => r.json());
}

interface Account {
  id: string;
  name: string;
  email: string;
  status: string;
  plan: string;
  setup_complete: boolean;
  messages_30d: number;
  active_numbers: number;
  created_at: string;
}

const statusConfig: Record<string, { label: string; icon: React.ElementType; class: string }> = {
  active: { label: "Active", icon: CheckCircle2Icon, class: "text-green-600 bg-green-50" },
  setting_up: { label: "Setting Up", icon: ClockIcon, class: "text-yellow-600 bg-yellow-50" },
  past_due: { label: "Past Due", icon: AlertTriangleIcon, class: "text-red-600 bg-red-50" },
  cancelled: { label: "Cancelled", icon: XCircleIcon, class: "text-gray-500 bg-gray-50" },
  suspended: { label: "Suspended", icon: XCircleIcon, class: "text-orange-600 bg-orange-50" },
  pending: { label: "Pending", icon: ClockIcon, class: "text-gray-500 bg-gray-50" },
};

export default function AdminCustomersPage() {
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [selectedAccount, setSelectedAccount] = useState<Account | null>(null);
  const [actionStatus, setActionStatus] = useState("");

  const params = new URLSearchParams();
  if (statusFilter) params.set("status", statusFilter);
  const { data, mutate } = useSWR<{ accounts: Account[] }>(
    `${API}/api/admin/accounts?${params}`,
    adminFetcher,
    { refreshInterval: 30000 }
  );

  const accounts = data?.accounts ?? [];
  const filtered = accounts.filter(
    (a) =>
      !search ||
      a.name.toLowerCase().includes(search.toLowerCase()) ||
      a.email.toLowerCase().includes(search.toLowerCase())
  );

  async function updateStatus(accountID: string, status: string) {
    await fetch(`${API}/api/admin/accounts/${accountID}/status`, {
      method: "PATCH",
      headers: {
        "Content-Type": "application/json",
        "X-Admin-Key": ADMIN_KEY || "",
      },
      body: JSON.stringify({ status, reason: `Admin action: set to ${status}` }),
    });
    mutate();
    setSelectedAccount(null);
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Customer Accounts</h1>
        <p className="text-gray-500 text-sm mt-1">{filtered.length} accounts</p>
      </div>

      {/* Filters */}
      <div className="flex gap-3">
        <div className="relative flex-1 max-w-xs">
          <SearchIcon className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search name or email..."
            className="w-full pl-9 pr-4 py-2.5 text-sm border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#007AFF]"
          />
        </div>
        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          className="px-4 py-2.5 text-sm border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#007AFF] bg-white"
        >
          <option value="">All statuses</option>
          <option value="active">Active</option>
          <option value="setting_up">Setting Up</option>
          <option value="past_due">Past Due</option>
          <option value="cancelled">Cancelled</option>
        </select>
      </div>

      {/* Table */}
      <div className="bg-white rounded-2xl border border-gray-100 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-50 bg-gray-50/50">
                <th className="text-left px-6 py-3 font-medium text-gray-500">Account</th>
                <th className="text-left px-6 py-3 font-medium text-gray-500">Status</th>
                <th className="text-left px-6 py-3 font-medium text-gray-500">Plan</th>
                <th className="text-right px-6 py-3 font-medium text-gray-500">Msgs (30d)</th>
                <th className="text-right px-6 py-3 font-medium text-gray-500">Numbers</th>
                <th className="text-left px-6 py-3 font-medium text-gray-500">Joined</th>
                <th className="px-6 py-3" />
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-50">
              {filtered.map((account) => {
                const sc = statusConfig[account.status] || statusConfig.pending;
                return (
                  <tr key={account.id} className="hover:bg-gray-50/50 transition-colors">
                    <td className="px-6 py-4">
                      <div className="font-medium text-gray-900">{account.name}</div>
                      <div className="text-xs text-gray-400 mt-0.5">{account.email}</div>
                    </td>
                    <td className="px-6 py-4">
                      <span className={`inline-flex items-center gap-1.5 text-xs font-medium px-2.5 py-1 rounded-full ${sc.class}`}>
                        <sc.icon className="w-3.5 h-3.5" />
                        {sc.label}
                      </span>
                    </td>
                    <td className="px-6 py-4 capitalize text-gray-600">{account.plan}</td>
                    <td className="px-6 py-4 text-right text-gray-600">
                      {account.messages_30d.toLocaleString()}
                    </td>
                    <td className="px-6 py-4 text-right text-gray-600">
                      {account.active_numbers}
                    </td>
                    <td className="px-6 py-4 text-gray-400 text-xs">
                      {new Date(account.created_at).toLocaleDateString()}
                    </td>
                    <td className="px-6 py-4">
                      <button
                        onClick={() => setSelectedAccount(account)}
                        className="text-xs text-[#007AFF] hover:underline font-medium"
                      >
                        Manage
                      </button>
                    </td>
                  </tr>
                );
              })}
              {filtered.length === 0 && (
                <tr>
                  <td colSpan={7} className="px-6 py-12 text-center text-sm text-gray-400">
                    No accounts found
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Account action modal */}
      {selectedAccount && (
        <div className="fixed inset-0 bg-black/20 flex items-center justify-center z-50 p-4">
          <div className="bg-white rounded-2xl shadow-xl w-full max-w-md p-6">
            <h3 className="font-semibold text-gray-900 mb-1">{selectedAccount.name}</h3>
            <p className="text-sm text-gray-500 mb-6">{selectedAccount.email}</p>

            <div className="space-y-2 mb-6">
              <p className="text-sm font-medium text-gray-700">Change status:</p>
              {["active", "suspended", "cancelled"].map((s) => (
                <button
                  key={s}
                  onClick={() => updateStatus(selectedAccount.id, s)}
                  disabled={s === selectedAccount.status}
                  className="w-full text-left px-4 py-2.5 rounded-xl border border-gray-200 text-sm hover:border-blue-200 hover:bg-blue-50 transition-colors disabled:opacity-40 disabled:cursor-not-allowed capitalize"
                >
                  {s === selectedAccount.status ? `✓ ${s} (current)` : `Set to ${s}`}
                </button>
              ))}
            </div>

            <button
              onClick={() => setSelectedAccount(null)}
              className="w-full py-2.5 rounded-xl border border-gray-200 text-sm text-gray-600 hover:bg-gray-50 transition-colors"
            >
              Close
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
