"use client";

import { useEffect, useState } from "react";
import { useRouter, usePathname } from "next/navigation";
import Link from "next/link";
import {
  MessageCircleIcon,
  LayoutDashboardIcon,
  InboxIcon,
  CreditCardIcon,
  LogOutIcon,
  MenuIcon,
  XIcon,
} from "lucide-react";

const nav = [
  { href: "/dashboard", icon: LayoutDashboardIcon, label: "Dashboard" },
  { href: "/messages", icon: InboxIcon, label: "Messages" },
  { href: "/billing", icon: CreditCardIcon, label: "Billing" },
];

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const router = useRouter();
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);

  useEffect(() => {
    const token = localStorage.getItem("access_token");
    if (!token) {
      router.replace("/login");
    }
  }, [router]);

  function handleLogout() {
    const refresh = localStorage.getItem("refresh_token");
    fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/auth/logout`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refresh }),
    });
    localStorage.removeItem("access_token");
    localStorage.removeItem("refresh_token");
    router.replace("/login");
  }

  const Sidebar = () => (
    <div className="flex flex-col h-full bg-white border-r border-gray-100">
      {/* Logo */}
      <div className="p-6 border-b border-gray-50">
        <Link href="/dashboard" className="flex items-center gap-2">
          <div className="w-8 h-8 rounded-xl bg-[#007AFF] flex items-center justify-center">
            <MessageCircleIcon className="w-5 h-5 text-white" />
          </div>
          <span className="font-semibold text-gray-900 text-lg">BlueSend</span>
        </Link>
      </div>

      {/* Nav */}
      <nav className="flex-1 p-4 space-y-1">
        {nav.map(({ href, icon: Icon, label }) => {
          const active = pathname === href || pathname.startsWith(href + "/");
          return (
            <Link
              key={href}
              href={href}
              onClick={() => setMobileOpen(false)}
              className={`flex items-center gap-3 px-4 py-3 rounded-xl text-sm font-medium transition-colors ${
                active
                  ? "bg-blue-50 text-[#007AFF]"
                  : "text-gray-600 hover:bg-gray-50 hover:text-gray-900"
              }`}
            >
              <Icon className="w-5 h-5 flex-shrink-0" />
              {label}
            </Link>
          );
        })}
      </nav>

      {/* Footer */}
      <div className="p-4 border-t border-gray-50">
        <button
          onClick={handleLogout}
          className="flex items-center gap-3 px-4 py-3 rounded-xl text-sm font-medium text-gray-500 hover:bg-red-50 hover:text-red-600 transition-colors w-full"
        >
          <LogOutIcon className="w-5 h-5" />
          Sign out
        </button>
      </div>
    </div>
  );

  return (
    <div className="flex h-screen bg-gray-50 overflow-hidden">
      {/* Desktop sidebar */}
      <div className="hidden md:flex w-64 flex-shrink-0 flex-col">
        <Sidebar />
      </div>

      {/* Mobile sidebar overlay */}
      {mobileOpen && (
        <div className="md:hidden fixed inset-0 z-50 flex">
          <div
            className="fixed inset-0 bg-black/20"
            onClick={() => setMobileOpen(false)}
          />
          <div className="relative w-64 flex-shrink-0 flex flex-col shadow-xl">
            <Sidebar />
          </div>
        </div>
      )}

      {/* Main content */}
      <div className="flex-1 flex flex-col min-w-0 overflow-auto">
        {/* Mobile header */}
        <header className="md:hidden bg-white border-b border-gray-100 px-4 h-14 flex items-center justify-between">
          <button
            onClick={() => setMobileOpen(true)}
            className="p-2 text-gray-500 hover:text-gray-900"
          >
            <MenuIcon className="w-5 h-5" />
          </button>
          <div className="flex items-center gap-2">
            <div className="w-6 h-6 rounded-lg bg-[#007AFF] flex items-center justify-center">
              <MessageCircleIcon className="w-4 h-4 text-white" />
            </div>
            <span className="font-semibold text-gray-900">BlueSend</span>
          </div>
          <div className="w-9" />
        </header>

        {/* Page content */}
        <main className="flex-1 p-6 md:p-8 max-w-5xl mx-auto w-full">
          {children}
        </main>
      </div>
    </div>
  );
}
