"use client";

import { useState } from "react";
import useSWR from "swr";
import {
  PlusIcon,
  WifiIcon,
  WifiOffIcon,
  AlertTriangleIcon,
  ClockIcon,
  CopyIcon,
  CheckIcon,
} from "lucide-react";

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

interface Device {
  id: string;
  name: string;
  type: string;
  status: string;
  last_seen_at: string | null;
  ip_address: string | null;
  agent_version: string | null;
  os_version: string | null;
  capacity: number;
  assigned_count: number;
  error_message: string | null;
  created_at: string;
}

interface RegisterResult {
  device_id: string;
  device_token: string;
  message: string;
}

function StatusPill({ status }: { status: string }) {
  const configs: Record<string, { label: string; class: string; icon: React.ElementType }> = {
    online: { label: "Online", class: "bg-green-50 text-green-700 border-green-100", icon: WifiIcon },
    offline: { label: "Offline", class: "bg-gray-50 text-gray-500 border-gray-100", icon: WifiOffIcon },
    error: { label: "Error", class: "bg-red-50 text-red-700 border-red-100", icon: AlertTriangleIcon },
    maintenance: { label: "Maintenance", class: "bg-yellow-50 text-yellow-700 border-yellow-100", icon: ClockIcon },
  };
  const cfg = configs[status] || configs.offline;
  return (
    <span className={`inline-flex items-center gap-1.5 border text-xs font-medium px-2.5 py-1 rounded-full ${cfg.class}`}>
      <cfg.icon className="w-3 h-3" />
      {cfg.label}
    </span>
  );
}

export default function AdminDevicesPage() {
  const { data, mutate } = useSWR<{ devices: Device[] }>(
    `${API}/api/admin/devices`,
    adminFetcher,
    { refreshInterval: 5000 }
  );
  const [showRegister, setShowRegister] = useState(false);
  const [registerForm, setRegisterForm] = useState({ name: "", type: "mac_mini", serial: "" });
  const [registerResult, setRegisterResult] = useState<RegisterResult | null>(null);
  const [copied, setCopied] = useState(false);

  const devices = data?.devices ?? [];

  async function registerDevice() {
    const res = await fetch(`${API}/api/admin/devices/register`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...getAuthHeader(),
      },
      body: JSON.stringify({
        name: registerForm.name,
        type: registerForm.type,
        serial_number: registerForm.serial || undefined,
      }),
    });
    const data = await res.json();
    if (res.ok) {
      setRegisterResult(data);
      mutate();
    }
  }

  function copyToken() {
    if (registerResult) {
      navigator.clipboard.writeText(registerResult.device_token);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Physical Devices</h1>
          <p className="text-gray-500 text-sm mt-1">
            {devices.filter((d) => d.status === "online").length} online · {devices.length} total
          </p>
        </div>
        <button
          onClick={() => setShowRegister(true)}
          className="flex items-center gap-2 bg-[#007AFF] text-white text-sm font-medium px-4 py-2.5 rounded-xl hover:bg-blue-600 transition-colors"
        >
          <PlusIcon className="w-4 h-4" />
          Register device
        </button>
      </div>

      {/* Device grid */}
      <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-4">
        {devices.map((device) => (
          <div
            key={device.id}
            className={`bg-white rounded-2xl border p-5 ${
              device.status === "error"
                ? "border-red-100"
                : device.status === "online"
                ? "border-green-100"
                : "border-gray-100"
            }`}
          >
            <div className="flex items-start justify-between mb-3">
              <div>
                <h3 className="font-semibold text-gray-900">{device.name}</h3>
                <p className="text-xs text-gray-400 mt-0.5">
                  {device.type.replace("_", " ")}
                  {device.ip_address ? ` · ${device.ip_address}` : ""}
                </p>
              </div>
              <StatusPill status={device.status} />
            </div>

            {/* Numbers bar */}
            <div className="mb-3">
              <div className="flex justify-between text-xs text-gray-500 mb-1">
                <span>Numbers assigned</span>
                <span>
                  {device.assigned_count}/{device.capacity}
                </span>
              </div>
              <div className="w-full bg-gray-100 rounded-full h-1.5">
                <div
                  className="bg-[#007AFF] h-1.5 rounded-full transition-all"
                  style={{
                    width: `${Math.min((device.assigned_count / device.capacity) * 100, 100)}%`,
                  }}
                />
              </div>
            </div>

            <div className="text-xs text-gray-400 space-y-0.5">
              {device.agent_version && <p>Agent v{device.agent_version}</p>}
              {device.os_version && <p>macOS {device.os_version}</p>}
              {device.last_seen_at && (
                <p>
                  Last seen:{" "}
                  {new Date(device.last_seen_at).toLocaleString([], {
                    month: "short",
                    day: "numeric",
                    hour: "2-digit",
                    minute: "2-digit",
                  })}
                </p>
              )}
            </div>

            {device.error_message && (
              <div className="mt-3 bg-red-50 rounded-lg px-3 py-2 text-xs text-red-600">
                {device.error_message}
              </div>
            )}
          </div>
        ))}
      </div>

      {devices.length === 0 && (
        <div className="bg-white rounded-2xl border border-gray-100 p-12 text-center">
          <div className="w-12 h-12 bg-gray-100 rounded-full flex items-center justify-center mx-auto mb-4">
            <WifiOffIcon className="w-6 h-6 text-gray-400" />
          </div>
          <h3 className="font-medium text-gray-900 mb-2">No devices registered</h3>
          <p className="text-sm text-gray-500 mb-4">
            Register a Mac Mini or iPhone to start hosting phone numbers.
          </p>
          <button
            onClick={() => setShowRegister(true)}
            className="bg-[#007AFF] text-white text-sm font-medium px-5 py-2.5 rounded-xl hover:bg-blue-600 transition-colors"
          >
            Register first device
          </button>
        </div>
      )}

      {/* Register modal */}
      {showRegister && (
        <div className="fixed inset-0 bg-black/20 flex items-center justify-center z-50 p-4">
          <div className="bg-white rounded-2xl shadow-xl w-full max-w-md p-6">
            {!registerResult ? (
              <>
                <h3 className="font-semibold text-gray-900 mb-4">Register New Device</h3>
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1.5">
                      Device name
                    </label>
                    <input
                      type="text"
                      value={registerForm.name}
                      onChange={(e) =>
                        setRegisterForm({ ...registerForm, name: e.target.value })
                      }
                      placeholder="mac-mini-01"
                      className="w-full px-4 py-2.5 text-sm border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#007AFF]"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1.5">
                      Device type
                    </label>
                    <select
                      value={registerForm.type}
                      onChange={(e) =>
                        setRegisterForm({ ...registerForm, type: e.target.value })
                      }
                      className="w-full px-4 py-2.5 text-sm border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#007AFF] bg-white"
                    >
                      <option value="mac_mini">Mac Mini</option>
                      <option value="iphone">iPhone</option>
                    </select>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1.5">
                      Serial number (optional)
                    </label>
                    <input
                      type="text"
                      value={registerForm.serial}
                      onChange={(e) =>
                        setRegisterForm({ ...registerForm, serial: e.target.value })
                      }
                      placeholder="C02XXXXXX"
                      className="w-full px-4 py-2.5 text-sm border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#007AFF]"
                    />
                  </div>
                </div>
                <div className="flex gap-3 mt-6">
                  <button
                    onClick={() => setShowRegister(false)}
                    className="flex-1 py-2.5 rounded-xl border border-gray-200 text-sm text-gray-600"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={registerDevice}
                    disabled={!registerForm.name}
                    className="flex-1 py-2.5 rounded-xl bg-[#007AFF] text-white text-sm font-medium hover:bg-blue-600 transition-colors disabled:opacity-50"
                  >
                    Register
                  </button>
                </div>
              </>
            ) : (
              <>
                <div className="text-center mb-4">
                  <div className="w-12 h-12 bg-green-50 rounded-full flex items-center justify-center mx-auto mb-3">
                    <CheckIcon className="w-6 h-6 text-green-500" />
                  </div>
                  <h3 className="font-semibold text-gray-900">Device registered!</h3>
                  <p className="text-sm text-gray-500 mt-1">
                    Copy the token below and set it as DEVICE_TOKEN on the Mac Mini.
                  </p>
                </div>
                <div className="bg-gray-50 rounded-xl p-4 mb-4">
                  <div className="text-xs text-gray-500 mb-1">Device token (save this — shown once)</div>
                  <div className="font-mono text-sm text-gray-900 break-all">
                    {registerResult.device_token}
                  </div>
                </div>
                <div className="flex gap-3">
                  <button
                    onClick={copyToken}
                    className="flex-1 flex items-center justify-center gap-2 py-2.5 rounded-xl border border-gray-200 text-sm"
                  >
                    {copied ? <CheckIcon className="w-4 h-4 text-green-500" /> : <CopyIcon className="w-4 h-4" />}
                    {copied ? "Copied!" : "Copy token"}
                  </button>
                  <button
                    onClick={() => {
                      setShowRegister(false);
                      setRegisterResult(null);
                      setRegisterForm({ name: "", type: "mac_mini", serial: "" });
                    }}
                    className="flex-1 py-2.5 rounded-xl bg-[#007AFF] text-white text-sm font-medium"
                  >
                    Done
                  </button>
                </div>
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
