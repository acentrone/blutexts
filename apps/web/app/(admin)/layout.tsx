"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import {
  MessageCircleIcon,
  LayoutDashboardIcon,
  UsersIcon,
  ServerIcon,
  CreditCardIcon,
  FileTextIcon,
  ShieldXIcon,
} from "lucide-react";

const API = process.env.NEXT_PUBLIC_API_URL;

const nav = [
  { href: "/admin", icon: LayoutDashboardIcon, label: "Overview" },
  { href: "/admin/customers", icon: UsersIcon, label: "Customers" },
  { href: "/admin/devices", icon: ServerIcon, label: "Devices" },
  { href: "/admin/billing", icon: CreditCardIcon, label: "Billing" },
  { href: "/admin/audit-log", icon: FileTextIcon, label: "Audit Log" },
];

export default function AdminLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const pathname = usePathname();
  const router = useRouter();
  const [status, setStatus] = useState<"checking" | "authorized" | "denied" | "no_token">("checking");

  useEffect(() => {
    const token = localStorage.getItem("access_token");
    if (!token) {
      setStatus("no_token");
      return;
    }
    fetch(`${API}/api/auth/me`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((r) => r.json())
      .then((data) => {
        if (data.user?.role === "admin") {
          setStatus("authorized");
        } else {
          setStatus("denied");
        }
      })
      .catch(() => setStatus("no_token"));
  }, []);

  if (status === "checking") {
    return (
      <div className="min-h-screen bg-gray-950 flex items-center justify-center">
        <div className="text-gray-500 text-sm">Verifying access...</div>
      </div>
    );
  }

  if (status === "no_token") {
    router.replace("/login");
    return null;
  }

  if (status === "denied") {
    return (
      <div className="min-h-screen bg-gray-950 flex items-center justify-center px-6">
        <div className="text-center">
          <ShieldXIcon className="w-12 h-12 text-red-500 mx-auto mb-4" />
          <h1 className="text-white text-xl font-bold mb-2">Access Denied</h1>
          <p className="text-gray-400 text-sm mb-6">Your account does not have admin privileges.</p>
          <Link href="/dashboard" className="text-[#007AFF] text-sm hover:underline">
            Back to dashboard
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-screen bg-gray-950 overflow-hidden">
      {/* Admin sidebar — dark theme */}
      <div className="w-60 flex-shrink-0 flex flex-col bg-gray-900 border-r border-gray-800">
        <div className="p-5 border-b border-gray-800">
          <div className="flex items-center gap-2">
            <div className="w-7 h-7 rounded-lg bg-[#007AFF] flex items-center justify-center">
              <MessageCircleIcon className="w-4 h-4 text-white" />
            </div>
            <div>
              <div className="text-white font-semibold text-sm">BlueSend</div>
              <div className="text-gray-500 text-xs">Admin</div>
            </div>
          </div>
        </div>
        <nav className="flex-1 p-3 space-y-0.5">
          {nav.map(({ href, icon: Icon, label }) => {
            const active =
              href === "/admin"
                ? pathname === "/admin"
                : pathname.startsWith(href);
            return (
              <Link
                key={href}
                href={href}
                className={`flex items-center gap-3 px-3 py-2.5 rounded-xl text-sm font-medium transition-colors ${
                  active
                    ? "bg-gray-800 text-white"
                    : "text-gray-400 hover:bg-gray-800/50 hover:text-gray-200"
                }`}
              >
                <Icon className="w-4 h-4 flex-shrink-0" />
                {label}
              </Link>
            );
          })}
        </nav>
        <div className="p-3 border-t border-gray-800">
          <Link
            href="/dashboard"
            className="flex items-center gap-2 px-3 py-2 text-xs text-gray-500 hover:text-gray-300 transition-colors"
          >
            ← Back to user dashboard
          </Link>
        </div>
      </div>

      {/* Main content */}
      <div className="flex-1 overflow-auto bg-gray-50">
        <main className="p-8 max-w-6xl mx-auto">{children}</main>
      </div>
    </div>
  );
}
