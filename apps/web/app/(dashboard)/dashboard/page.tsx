"use client";

import { useEffect, useState } from "react";
import useSWR from "swr";
import {
  MessageCircleIcon,
  TrendingUpIcon,
  UsersIcon,
  SendIcon,
  AlertCircleIcon,
  CheckCircle2Icon,
  ClockIcon,
  ExternalLinkIcon,
} from "lucide-react";
import Link from "next/link";

const API = process.env.NEXT_PUBLIC_API_URL;

function fetcher(url: string) {
  const token = localStorage.getItem("access_token");
  return fetch(url, {
    headers: { Authorization: `Bearer ${token}` },
  }).then((r) => r.json());
}

interface DashboardStats {
  total_sent: number;
  total_delivered: number;
  total_replied: number;
  response_rate: number;
  active_conversations: number;
  today_new_contacts: number;
  daily_limit: number;
}

interface AccountInfo {
  account: {
    id: string;
    name: string;
    status: string;
    plan: string;
    setup_complete: boolean;
  };
}

function StatusBadge({ status }: { status: string }) {
  const configs: Record<string, { label: string; class: string; dot: string }> = {
    active: {
      label: "Active",
      class: "bg-green-50 text-green-700 border-green-100",
      dot: "bg-green-500",
    },
    setting_up: {
      label: "Setting Up",
      class: "bg-yellow-50 text-yellow-700 border-yellow-100",
      dot: "bg-yellow-400",
    },
    past_due: {
      label: "Payment Past Due",
      class: "bg-red-50 text-red-700 border-red-100",
      dot: "bg-red-500",
    },
    pending: {
      label: "Pending Setup",
      class: "bg-gray-50 text-gray-600 border-gray-100",
      dot: "bg-gray-400",
    },
  };

  const cfg = configs[status] || configs.pending;

  return (
    <div className={`inline-flex items-center gap-2 border rounded-full px-3 py-1.5 text-sm font-medium ${cfg.class}`}>
      <span className={`w-2 h-2 rounded-full animate-pulse ${cfg.dot}`} />
      {cfg.label}
    </div>
  );
}

function StatCard({
  icon: Icon,
  label,
  value,
  sub,
  color = "blue",
}: {
  icon: React.ElementType;
  label: string;
  value: string | number;
  sub?: string;
  color?: "blue" | "green" | "purple" | "orange";
}) {
  const colors = {
    blue: "bg-blue-50 text-[#007AFF]",
    green: "bg-green-50 text-green-600",
    purple: "bg-purple-50 text-purple-600",
    orange: "bg-orange-50 text-orange-600",
  };

  return (
    <div className="bg-white rounded-2xl border border-gray-100 p-6 hover:border-blue-100 transition-colors">
      <div className="flex items-start justify-between mb-4">
        <div className={`w-10 h-10 rounded-xl flex items-center justify-center ${colors[color]}`}>
          <Icon className="w-5 h-5" />
        </div>
      </div>
      <div className="text-3xl font-bold text-gray-900 mb-1">{value}</div>
      <div className="text-sm text-gray-500">{label}</div>
      {sub && <div className="text-xs text-gray-400 mt-1">{sub}</div>}
    </div>
  );
}

export default function DashboardPage() {
  const { data: meData } = useSWR<AccountInfo>(`${API}/api/auth/me`, fetcher);
  const { data: stats, isLoading } = useSWR<DashboardStats>(
    `${API}/api/dashboard/stats`,
    fetcher,
    { refreshInterval: 30000 }
  );

  const account = meData?.account;
  const isSettingUp = account?.status === "setting_up" || account?.status === "pending";

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">
            {account ? `Welcome back, ${account.name}` : "Dashboard"}
          </h1>
          <p className="text-gray-500 text-sm mt-1">Last 30 days overview</p>
        </div>
        {account && <StatusBadge status={account.status} />}
      </div>

      {/* Setup banner */}
      {isSettingUp && (
        <div className="bg-blue-50 border border-blue-100 rounded-2xl p-6 flex items-start gap-4">
          <div className="w-10 h-10 bg-[#007AFF] rounded-xl flex items-center justify-center flex-shrink-0">
            <ClockIcon className="w-5 h-5 text-white" />
          </div>
          <div>
            <h3 className="font-semibold text-gray-900 mb-1">
              We're setting up your account
            </h3>
            <p className="text-sm text-gray-600 mb-3">
              Your dedicated phone number is being provisioned and your Go High Level
              integration is being configured. This typically takes 15–30 minutes.
              We'll email you as soon as you're live.
            </p>
            <div className="flex items-center gap-4 text-sm">
              <div className="flex items-center gap-1.5 text-[#007AFF]">
                <CheckCircle2Icon className="w-4 h-4" />
                Payment confirmed
              </div>
              <div className="flex items-center gap-1.5 text-yellow-600">
                <ClockIcon className="w-4 h-4" />
                Number provisioning
              </div>
              <div className="flex items-center gap-1.5 text-gray-400">
                <ClockIcon className="w-4 h-4" />
                GHL integration
              </div>
            </div>
          </div>
        </div>
      )}

      {/* GHL connection banner */}
      {account?.status === "active" && !account?.setup_complete && (
        <div className="bg-amber-50 border border-amber-100 rounded-2xl p-6 flex items-start gap-4">
          <AlertCircleIcon className="w-6 h-6 text-amber-500 flex-shrink-0 mt-0.5" />
          <div className="flex-1">
            <h3 className="font-semibold text-gray-900 mb-1">
              Connect Go High Level
            </h3>
            <p className="text-sm text-gray-600 mb-3">
              Connect your GHL account to enable bidirectional message sync.
            </p>
            <Link
              href={`${API}/api/oauth/connect?account_id=${account.id}`}
              className="inline-flex items-center gap-1.5 bg-amber-500 text-white text-sm font-medium px-4 py-2 rounded-lg hover:bg-amber-600 transition-colors"
            >
              Connect GHL
              <ExternalLinkIcon className="w-3.5 h-3.5" />
            </Link>
          </div>
        </div>
      )}

      {/* Stats grid */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          icon={SendIcon}
          label="Messages sent"
          value={isLoading ? "—" : (stats?.total_sent ?? 0).toLocaleString()}
          sub="Last 30 days"
          color="blue"
        />
        <StatCard
          icon={CheckCircle2Icon}
          label="Delivered"
          value={isLoading ? "—" : (stats?.total_delivered ?? 0).toLocaleString()}
          sub={
            stats && stats.total_sent > 0
              ? `${Math.round((stats.total_delivered / stats.total_sent) * 100)}% rate`
              : undefined
          }
          color="green"
        />
        <StatCard
          icon={MessageCircleIcon}
          label="Replies received"
          value={isLoading ? "—" : (stats?.total_replied ?? 0).toLocaleString()}
          sub={
            stats
              ? `${stats.response_rate.toFixed(1)}% response rate`
              : undefined
          }
          color="purple"
        />
        <StatCard
          icon={UsersIcon}
          label="New contacts today"
          value={
            isLoading
              ? "—"
              : `${stats?.today_new_contacts ?? 0}/${stats?.daily_limit ?? 50}`
          }
          sub="Daily limit"
          color="orange"
        />
      </div>

      {/* Daily limit progress */}
      {stats && (
        <div className="bg-white rounded-2xl border border-gray-100 p-6">
          <div className="flex items-center justify-between mb-3">
            <div>
              <h3 className="font-semibold text-gray-900">Daily new contact limit</h3>
              <p className="text-sm text-gray-500 mt-0.5">
                {stats.today_new_contacts} of {stats.daily_limit} new contacts messaged today
              </p>
            </div>
            <span className="text-sm font-medium text-gray-500">
              {stats.daily_limit - stats.today_new_contacts} remaining
            </span>
          </div>
          <div className="w-full bg-gray-100 rounded-full h-2.5">
            <div
              className={`h-2.5 rounded-full transition-all ${
                stats.today_new_contacts / stats.daily_limit > 0.9
                  ? "bg-red-500"
                  : stats.today_new_contacts / stats.daily_limit > 0.7
                  ? "bg-yellow-400"
                  : "bg-[#007AFF]"
              }`}
              style={{
                width: `${Math.min(
                  (stats.today_new_contacts / stats.daily_limit) * 100,
                  100
                )}%`,
              }}
            />
          </div>
          <p className="text-xs text-gray-400 mt-2">
            Resets at midnight. Existing conversation replies are unlimited.
          </p>
        </div>
      )}

      {/* Quick links */}
      <div className="grid md:grid-cols-3 gap-4">
        <Link
          href="/messages"
          className="bg-white border border-gray-100 rounded-2xl p-6 hover:border-blue-100 hover:shadow-md hover:shadow-blue-50 transition-all group"
        >
          <div className="w-10 h-10 bg-blue-50 rounded-xl flex items-center justify-center mb-4 group-hover:bg-[#007AFF] transition-colors">
            <MessageCircleIcon className="w-5 h-5 text-[#007AFF] group-hover:text-white transition-colors" />
          </div>
          <h3 className="font-semibold text-gray-900 mb-1">Messages</h3>
          <p className="text-sm text-gray-500">
            {stats?.active_conversations ?? 0} active conversation
            {stats?.active_conversations !== 1 ? "s" : ""}
          </p>
        </Link>

        <Link
          href="/billing"
          className="bg-white border border-gray-100 rounded-2xl p-6 hover:border-blue-100 hover:shadow-md hover:shadow-blue-50 transition-all group"
        >
          <div className="w-10 h-10 bg-blue-50 rounded-xl flex items-center justify-center mb-4 group-hover:bg-[#007AFF] transition-colors">
            <TrendingUpIcon className="w-5 h-5 text-[#007AFF] group-hover:text-white transition-colors" />
          </div>
          <h3 className="font-semibold text-gray-900 mb-1">Billing</h3>
          <p className="text-sm text-gray-500 capitalize">
            {account?.plan ?? "—"} plan
          </p>
        </Link>

        <a
          href="mailto:support@blutexts.com"
          className="bg-white border border-gray-100 rounded-2xl p-6 hover:border-blue-100 hover:shadow-md hover:shadow-blue-50 transition-all group"
        >
          <div className="w-10 h-10 bg-blue-50 rounded-xl flex items-center justify-center mb-4 group-hover:bg-[#007AFF] transition-colors">
            <AlertCircleIcon className="w-5 h-5 text-[#007AFF] group-hover:text-white transition-colors" />
          </div>
          <h3 className="font-semibold text-gray-900 mb-1">Support</h3>
          <p className="text-sm text-gray-500">support@blutexts.com</p>
        </a>
      </div>
    </div>
  );
}
