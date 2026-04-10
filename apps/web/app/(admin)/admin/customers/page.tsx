"use client";

import { useState } from "react";
import useSWR from "swr";
import { SearchIcon, CheckCircle2Icon, AlertTriangleIcon, ClockIcon, XCircleIcon, PhoneIcon, LinkIcon } from "lucide-react";

const API = process.env.NEXT_PUBLIC_API_URL;

function getAuthHeader(): Record<string, string> {
  const token = localStorage.getItem("access_token");
  return { Authorization: `Bearer ${token}` };
}

function adminFetcher(url: string) {
  return fetch(url, { headers: getAuthHeader() }).then((r) => {
    if (r.status === 401 || r.status === 403) {
      window.location.href = "/login";
      throw new Error("unauthorized");
    }
    return r.json();
  });
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

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Customer Accounts</h1>
        <p className="text-gray-500 text-sm mt-1">{filtered.length} accounts</p>
      </div>

      <div className="flex gap-3">
        <div className="relative flex-1 max-w-xs">
          <SearchIcon className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
          <input type="text" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search name or email..." className="w-full pl-9 pr-4 py-2.5 text-sm border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#007AFF]" />
        </div>
        <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} className="px-4 py-2.5 text-sm border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#007AFF] bg-white">
          <option value="">All statuses</option>
          <option value="active">Active</option>
          <option value="setting_up">Setting Up</option>
          <option value="past_due">Past Due</option>
          <option value="cancelled">Cancelled</option>
        </select>
      </div>

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
                    <td className="px-6 py-4 text-right text-gray-600">{account.messages_30d?.toLocaleString() ?? 0}</td>
                    <td className="px-6 py-4 text-right text-gray-600">{account.active_numbers}</td>
                    <td className="px-6 py-4 text-gray-400 text-xs">{new Date(account.created_at).toLocaleDateString()}</td>
                    <td className="px-6 py-4">
                      <button onClick={() => setSelectedAccount(account)} className="text-xs text-[#007AFF] hover:underline font-medium">Manage</button>
                    </td>
                  </tr>
                );
              })}
              {filtered.length === 0 && (
                <tr><td colSpan={7} className="px-6 py-12 text-center text-sm text-gray-400">No accounts found</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      {selectedAccount && (
        <AccountModal
          account={selectedAccount}
          onClose={() => setSelectedAccount(null)}
          onUpdateStatus={(id: string, s: string) => { mutate(); setSelectedAccount(null); }}
          devices={[]}
        />
      )}
    </div>
  );
}

function AccountModal({ account, onClose, onUpdateStatus, devices: _devices }: any) {
  const [showAssign, setShowAssign] = useState(false);
  const [form, setForm] = useState({ number: "", imessage_address: "", device_id: "", display_name: "" });
  const [result, setResult] = useState("");

  const { data: numbersData, mutate: mutateNumbers } = useSWR(`${API}/api/admin/accounts/${account.id}/numbers`, adminFetcher);
  const { data: ghlData, mutate: mutateGhl } = useSWR(`${API}/api/admin/accounts/${account.id}/ghl`, adminFetcher);
  const { data: devicesData } = useSWR(`${API}/api/admin/devices`, adminFetcher);

  const numbers = numbersData?.numbers ?? [];
  const ghl = ghlData || { connected: false };
  const devices = devicesData?.devices ?? [];

  async function disconnectGhl() {
    await fetch(`${API}/api/admin/accounts/${account.id}/ghl`, {
      method: "DELETE",
      headers: getAuthHeader(),
    });
    mutateGhl();
  }

  async function assignNumber() {
    setResult("");
    const res = await fetch(`${API}/api/admin/accounts/${account.id}/assign-number`, {
      method: "POST",
      headers: { "Content-Type": "application/json", ...getAuthHeader() },
      body: JSON.stringify(form),
    });
    if (res.ok) {
      setResult("Number assigned!");
      setForm({ number: "", imessage_address: "", device_id: "", display_name: "" });
      setShowAssign(false);
      mutateNumbers();
    } else {
      const data = await res.json();
      setResult(data.error || "Failed");
    }
  }

  async function updateStatus(status: string) {
    await fetch(`${API}/api/admin/accounts/${account.id}/status`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json", ...getAuthHeader() },
      body: JSON.stringify({ status, reason: `Admin: set to ${status}` }),
    });
    onUpdateStatus(account.id, status);
  }

  return (
    <div className="fixed inset-0 bg-black/20 flex items-center justify-center z-50 p-4">
      <div className="bg-white rounded-2xl shadow-xl w-full max-w-lg p-6 max-h-[90vh] overflow-y-auto">
        <h3 className="font-semibold text-gray-900 mb-1">{account.name}</h3>
        <p className="text-sm text-gray-500 mb-6">{account.email}</p>

        {/* GHL Status */}
        <div className="mb-6">
          <div className="flex items-center gap-2 mb-2">
            <LinkIcon className="w-4 h-4 text-gray-500" />
            <p className="text-sm font-medium text-gray-700">Go High Level</p>
          </div>
          {ghl.connected ? (
            <div className="bg-green-50 border border-green-100 rounded-xl px-4 py-3 text-sm">
              <div className="flex items-center justify-between">
                <div>
                  <div className="flex items-center gap-2 text-green-700 font-medium"><CheckCircle2Icon className="w-4 h-4" /> Connected</div>
                  <div className="text-xs text-green-600 mt-1">Location: {ghl.location_id}</div>
                </div>
                <button onClick={disconnectGhl} className="text-xs text-red-500 hover:underline">Disconnect</button>
              </div>
            </div>
          ) : (
            <div className="bg-gray-50 border border-gray-200 rounded-xl px-4 py-3 text-sm text-gray-500">Not connected</div>
          )}
        </div>

        {/* Phone Numbers */}
        <div className="mb-6">
          <div className="flex items-center justify-between mb-2">
            <div className="flex items-center gap-2">
              <PhoneIcon className="w-4 h-4 text-gray-500" />
              <p className="text-sm font-medium text-gray-700">Phone Numbers ({numbers.length})</p>
            </div>
            <button onClick={() => setShowAssign(!showAssign)} className="text-xs text-[#007AFF] font-medium hover:underline">
              {showAssign ? "Cancel" : "+ Assign number"}
            </button>
          </div>

          {numbers.map((n: any) => (
            <div key={n.id} className="bg-gray-50 border border-gray-200 rounded-xl px-4 py-3 mb-2">
              <div className="flex items-center justify-between">
                <div>
                  <div className="text-sm font-medium text-gray-900">{n.number}</div>
                  {n.imessage_address && <div className="text-xs text-gray-400 mt-0.5">iMessage: {n.imessage_address}</div>}
                </div>
                <div className="text-right">
                  <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${n.status === "active" ? "bg-green-50 text-green-700" : "bg-yellow-50 text-yellow-700"}`}>{n.status}</span>
                  {n.device_name && <div className="text-xs text-gray-400 mt-1">{n.device_name}</div>}
                </div>
              </div>
              {n.display_name && <div className="text-xs text-gray-400 mt-1">{n.display_name}</div>}
            </div>
          ))}
          {numbers.length === 0 && !showAssign && (
            <div className="text-sm text-gray-400 bg-gray-50 rounded-xl px-4 py-3 text-center">No numbers assigned</div>
          )}

          {showAssign && (
            <div className="space-y-3 bg-blue-50 border border-blue-100 rounded-xl p-4 mt-2">
              <input type="text" value={form.number} onChange={(e) => setForm({ ...form, number: e.target.value })} placeholder="Phone number (+1...)" className="w-full px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-[#007AFF]" />
              <input type="text" value={form.imessage_address} onChange={(e) => setForm({ ...form, imessage_address: e.target.value })} placeholder="iMessage email (optional)" className="w-full px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-[#007AFF]" />
              <select value={form.device_id} onChange={(e) => setForm({ ...form, device_id: e.target.value })} className="w-full px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-[#007AFF] bg-white">
                <option value="">Select device...</option>
                {devices.map((d: any) => (<option key={d.id} value={d.id}>{d.name} ({d.status})</option>))}
              </select>
              <input type="text" value={form.display_name} onChange={(e) => setForm({ ...form, display_name: e.target.value })} placeholder="Display name (optional)" className="w-full px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-[#007AFF]" />
              {result && <div className={`text-xs px-3 py-2 rounded-lg ${result.includes("!") ? "bg-green-50 text-green-700" : "bg-red-50 text-red-700"}`}>{result}</div>}
              <button onClick={assignNumber} disabled={!form.number || !form.device_id} className="w-full py-2 rounded-lg bg-[#007AFF] text-white text-sm font-medium hover:bg-blue-600 disabled:opacity-40">Assign Number</button>
            </div>
          )}
        </div>

        {/* Status */}
        <div className="space-y-2 mb-6">
          <p className="text-sm font-medium text-gray-700">Change status:</p>
          {["active", "suspended", "cancelled"].map((s) => (
            <button key={s} onClick={() => updateStatus(s)} disabled={s === account.status} className="w-full text-left px-4 py-2.5 rounded-xl border border-gray-200 text-sm hover:border-blue-200 hover:bg-blue-50 transition-colors disabled:opacity-40 capitalize">
              {s === account.status ? `Current: ${s}` : `Set to ${s}`}
            </button>
          ))}
        </div>

        <button onClick={onClose} className="w-full py-2.5 rounded-xl border border-gray-200 text-sm text-gray-600 hover:bg-gray-50">Close</button>
      </div>
    </div>
  );
}
