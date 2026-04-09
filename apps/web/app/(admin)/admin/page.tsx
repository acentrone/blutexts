"use client";

import useSWR from "swr";
import {
  UsersIcon,
  ServerIcon,
  TrendingUpIcon,
  AlertTriangleIcon,
  CheckCircle2Icon,
  ClockIcon,
  WifiOffIcon,
} from "lucide-react";
import Link from "next/link";

const API = process.env.NEXT_PUBLIC_API_URL;

function adminFetcher(url: string) {
  const token = localStorage.getItem("access_token");
  return fetch(url, {
    headers: { Authorization: `Bearer ${token}` },
  }).then((r) => {
    if (r.status === 401 || r.status === 403) {
      window.location.href = "/login";
      throw new Error("unauthorized");
    }
    return r.json();
  });
}

interface SystemStats {
  active_accounts: number;
  setting_up: number;
  past_due: number;
  total_messages_30d: number;
  online_devices: number;
  estimated_mrr: number;
  as_of: string;
}

interface Device {
  id: string;
  name: string;
  type: string;
  status: string;
  last_seen_at: string | null;
  assigned_count: number;
  capacity: number;
  agent_version: string | null;
  error_message: string | null;
}

function DeviceStatusIcon({ status }: { status: string }) {
  if (status === "online") return <CheckCircle2Icon className="w-4 h-4 text-green-500" />;
  if (status === "error") return <AlertTriangleIcon className="w-4 h-4 text-red-500" />;
  if (status === "maintenance") return <ClockIcon className="w-4 h-4 text-yellow-500" />;
  return <WifiOffIcon className="w-4 h-4 text-gray-400" />;
}

export default function AdminOverviewPage() {
  const { data: stats } = useSWR<SystemStats>(
    `${API}/api/admin/stats`,
    adminFetcher,
    { refreshInterval: 15000 }
  );
  const { data: devicesData } = useSWR<{ devices: Device[] }>(
    `${API}/api/admin/devices`,
    adminFetcher,
    { refreshInterval: 10000 }
  );

  const devices = devicesData?.devices ?? [];
  const onlineDevices = devices.filter((d) => d.status === "online");
  const errorDevices = devices.filter((d) => d.status === "error");

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Operator Dashboard</h1>
        <p className="text-gray-500 text-sm mt-1">
          Last updated: {stats ? new Date(stats.as_of).toLocaleTimeString() : "—"}
        </p>
      </div>

      {/* Alerts */}
      {errorDevices.length > 0 && (
        <div className="bg-red-50 border border-red-100 rounded-2xl p-4 flex items-center gap-3">
          <AlertTriangleIcon className="w-5 h-5 text-red-500 flex-shrink-0" />
          <div className="text-sm text-red-700">
            <strong>{errorDevices.length} device(s) in error state:</strong>{" "}
            {errorDevices.map((d) => d.name).join(", ")}
            <Link href="/admin/devices" className="underline ml-2">
              View devices →
            </Link>
          </div>
        </div>
      )}

      {/* Stats */}
      <div className="grid grid-cols-2 lg:grid-cols-3 gap-4">
        {[
          {
            icon: UsersIcon,
            label: "Active accounts",
            value: stats?.active_accounts ?? "—",
            color: "text-green-600 bg-green-50",
          },
          {
            icon: ClockIcon,
            label: "Setting up",
            value: stats?.setting_up ?? "—",
            color: "text-yellow-600 bg-yellow-50",
          },
          {
            icon: AlertTriangleIcon,
            label: "Past due",
            value: stats?.past_due ?? "—",
            color: "text-red-600 bg-red-50",
          },
          {
            icon: TrendingUpIcon,
            label: "Messages (30d)",
            value: stats ? stats.total_messages_30d.toLocaleString() : "—",
            color: "text-blue-600 bg-blue-50",
          },
          {
            icon: ServerIcon,
            label: "Online devices",
            value: stats ? `${stats.online_devices}/${devices.length}` : "—",
            color: "text-purple-600 bg-purple-50",
          },
          {
            icon: TrendingUpIcon,
            label: "Est. MRR",
            value: stats ? `$${Math.round(stats.estimated_mrr).toLocaleString()}` : "—",
            color: "text-emerald-600 bg-emerald-50",
          },
        ].map((item) => (
          <div key={item.label} className="bg-white rounded-2xl border border-gray-100 p-5">
            <div className={`w-9 h-9 rounded-xl flex items-center justify-center mb-3 ${item.color}`}>
              <item.icon className="w-4 h-4" />
            </div>
            <div className="text-2xl font-bold text-gray-900">{item.value}</div>
            <div className="text-sm text-gray-500 mt-0.5">{item.label}</div>
          </div>
        ))}
      </div>

      {/* Device status */}
      <div className="bg-white rounded-2xl border border-gray-100 overflow-hidden">
        <div className="px-6 py-4 border-b border-gray-50 flex items-center justify-between">
          <h2 className="font-semibold text-gray-900">Physical devices</h2>
          <Link
            href="/admin/devices"
            className="text-sm text-[#007AFF] hover:underline"
          >
            View all
          </Link>
        </div>
        {devices.length === 0 ? (
          <div className="px-6 py-10 text-center text-sm text-gray-400">
            No devices registered.{" "}
            <span className="text-gray-600">Run the device installer on your Mac Mini.</span>
          </div>
        ) : (
          <div className="divide-y divide-gray-50">
            {devices.slice(0, 5).map((device) => (
              <div key={device.id} className="px-6 py-4 flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <DeviceStatusIcon status={device.status} />
                  <div>
                    <div className="text-sm font-medium text-gray-900">{device.name}</div>
                    <div className="text-xs text-gray-400">
                      {device.type.replace("_", " ")} · v{device.agent_version ?? "unknown"}
                      {device.last_seen_at &&
                        ` · Last seen ${new Date(device.last_seen_at).toLocaleTimeString()}`}
                    </div>
                  </div>
                </div>
                <div className="text-right">
                  <div className="text-sm font-medium text-gray-900">
                    {device.assigned_count}/{device.capacity}
                  </div>
                  <div className="text-xs text-gray-400">numbers</div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Quick actions */}
      <div className="grid md:grid-cols-3 gap-4">
        <Link
          href="/admin/customers"
          className="bg-white border border-gray-100 rounded-2xl p-5 hover:border-blue-100 transition-colors"
        >
          <UsersIcon className="w-6 h-6 text-[#007AFF] mb-3" />
          <h3 className="font-semibold text-gray-900 mb-1">Customer accounts</h3>
          <p className="text-sm text-gray-500">View, manage, and support all accounts</p>
        </Link>
        <Link
          href="/admin/devices"
          className="bg-white border border-gray-100 rounded-2xl p-5 hover:border-blue-100 transition-colors"
        >
          <ServerIcon className="w-6 h-6 text-[#007AFF] mb-3" />
          <h3 className="font-semibold text-gray-900 mb-1">Device management</h3>
          <p className="text-sm text-gray-500">Register, monitor, and configure devices</p>
        </Link>
        <Link
          href="/admin/billing"
          className="bg-white border border-gray-100 rounded-2xl p-5 hover:border-blue-100 transition-colors"
        >
          <TrendingUpIcon className="w-6 h-6 text-[#007AFF] mb-3" />
          <h3 className="font-semibold text-gray-900 mb-1">Revenue overview</h3>
          <p className="text-sm text-gray-500">Subscriptions, MRR, and billing events</p>
        </Link>
      </div>
    </div>
  );
}
