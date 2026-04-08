"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { MessageCircleIcon } from "lucide-react";

interface AccountData {
  email: string;
  password: string;
  firstName: string;
  lastName: string;
  company: string;
}

export default function SignupPage() {
  const router = useRouter();

  const [accountData, setAccountData] = useState<AccountData>({
    email: "",
    password: "",
    firstName: "",
    lastName: "",
    company: "",
  });
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleAccountSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");

    if (accountData.password.length < 8) {
      setError("Password must be at least 8 characters");
      return;
    }

    setLoading(true);
    try {
      const res = await fetch(
        `${process.env.NEXT_PUBLIC_API_URL}/api/auth/register`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            email: accountData.email,
            password: accountData.password,
            first_name: accountData.firstName,
            last_name: accountData.lastName,
            company: accountData.company,
          }),
        }
      );
      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || "Registration failed");
      }
      // Store tokens and go straight to dashboard
      localStorage.setItem("access_token", data.access_token);
      localStorage.setItem("refresh_token", data.refresh_token);
      router.push("/dashboard");
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="min-h-screen bg-gray-50 flex flex-col">
      {/* Header */}
      <header className="bg-white border-b border-gray-100 px-6 py-4">
        <div className="max-w-2xl mx-auto flex items-center justify-between">
          <Link href="/" className="flex items-center gap-2">
            <div className="w-8 h-8 rounded-xl bg-[#007AFF] flex items-center justify-center">
              <MessageCircleIcon className="w-5 h-5 text-white" />
            </div>
            <span className="font-semibold text-gray-900">BlueSend</span>
          </Link>
          <Link
            href="/login"
            className="text-sm text-gray-500 hover:text-gray-900 transition-colors"
          >
            Already have an account?
          </Link>
        </div>
      </header>

      <main className="flex-1 flex items-start justify-center px-6 py-12">
        <div className="w-full max-w-md">
          <div className="animate-slide-up">
            <h1 className="text-2xl font-bold text-gray-900 mb-2">Create your account</h1>
            <p className="text-gray-500 mb-8">You'll be sending iMessages in minutes.</p>

            <form onSubmit={handleAccountSubmit} className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1.5">
                    First name
                  </label>
                  <input
                    type="text"
                    required
                    value={accountData.firstName}
                    onChange={(e) =>
                      setAccountData({ ...accountData, firstName: e.target.value })
                    }
                    className="w-full px-4 py-3 rounded-xl border border-gray-200 focus:outline-none focus:ring-2 focus:ring-[#007AFF] focus:border-transparent text-sm"
                    placeholder="Jane"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1.5">
                    Last name
                  </label>
                  <input
                    type="text"
                    required
                    value={accountData.lastName}
                    onChange={(e) =>
                      setAccountData({ ...accountData, lastName: e.target.value })
                    }
                    className="w-full px-4 py-3 rounded-xl border border-gray-200 focus:outline-none focus:ring-2 focus:ring-[#007AFF] focus:border-transparent text-sm"
                    placeholder="Smith"
                  />
                </div>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1.5">
                  Company
                </label>
                <input
                  type="text"
                  required
                  value={accountData.company}
                  onChange={(e) =>
                    setAccountData({ ...accountData, company: e.target.value })
                  }
                  className="w-full px-4 py-3 rounded-xl border border-gray-200 focus:outline-none focus:ring-2 focus:ring-[#007AFF] focus:border-transparent text-sm"
                  placeholder="Acme Corp"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1.5">
                  Work email
                </label>
                <input
                  type="email"
                  required
                  value={accountData.email}
                  onChange={(e) =>
                    setAccountData({ ...accountData, email: e.target.value })
                  }
                  className="w-full px-4 py-3 rounded-xl border border-gray-200 focus:outline-none focus:ring-2 focus:ring-[#007AFF] focus:border-transparent text-sm"
                  placeholder="jane@acme.com"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1.5">
                  Password
                </label>
                <input
                  type="password"
                  required
                  minLength={8}
                  value={accountData.password}
                  onChange={(e) =>
                    setAccountData({ ...accountData, password: e.target.value })
                  }
                  className="w-full px-4 py-3 rounded-xl border border-gray-200 focus:outline-none focus:ring-2 focus:ring-[#007AFF] focus:border-transparent text-sm"
                  placeholder="Min 8 characters"
                />
              </div>

              {error && (
                <div className="text-sm text-red-600 bg-red-50 rounded-xl px-4 py-3">
                  {error}
                </div>
              )}

              <button
                type="submit"
                disabled={loading}
                className="w-full bg-[#007AFF] text-white font-semibold py-3.5 rounded-xl hover:bg-blue-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {loading ? "Creating account..." : "Get started — free"}
              </button>
            </form>
          </div>
        </div>
      </main>
    </div>
  );
}
