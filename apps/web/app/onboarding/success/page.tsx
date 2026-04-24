"use client";

import { Suspense, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { CheckCircle2Icon, LoaderIcon, AlertCircleIcon, MessageCircleIcon } from "lucide-react";
import Link from "next/link";

const API = process.env.NEXT_PUBLIC_API_URL;

// Next 14 requires `useSearchParams()` to live under a Suspense boundary or
// the page can't be statically prerendered. Wrap the real content in a
// Suspense fallback so the build doesn't bail out.
export default function OnboardingSuccessPage() {
  return (
    <Suspense fallback={<OnboardingSuccessFallback />}>
      <OnboardingSuccessContent />
    </Suspense>
  );
}

function OnboardingSuccessFallback() {
  return (
    <div className="min-h-screen bg-gray-50 flex items-center justify-center">
      <LoaderIcon className="w-8 h-8 text-[#007AFF] animate-spin" />
    </div>
  );
}

// After Stripe checkout succeeds, Stripe redirects here. We poll the
// subscription endpoint until the webhook flips the account to active,
// then hand the user off to the dashboard. 60s hard timeout — webhooks
// usually land in <5s but we don't want to hang forever.
function OnboardingSuccessContent() {
  const router = useRouter();
  const params = useSearchParams();
  const sessionID = params.get("session_id");

  const [state, setState] = useState<"polling" | "ready" | "timeout" | "unauthenticated">("polling");
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    const token = typeof window !== "undefined" ? localStorage.getItem("access_token") : null;
    if (!token) {
      setState("unauthenticated");
      return;
    }

    let cancelled = false;
    const deadline = Date.now() + 60_000;

    async function poll() {
      if (cancelled) return;
      try {
        const res = await fetch(`${API}/api/billing/subscription`, {
          headers: { Authorization: `Bearer ${token}` },
        });
        if (res.ok) {
          const data = await res.json();
          // Active or setting_up both mean the checkout webhook landed and
          // the customer has a subscription on file — safe to proceed.
          if (data.has_subscription && (data.status === "active" || data.status === "setting_up")) {
            if (!cancelled) setState("ready");
            // No auto-redirect — we want the customer to download the Mac
            // app FROM this page. Otherwise they land on the dashboard
            // and have to hunt for the install link.
            return;
          }
        }
      } catch {
        // Transient — fall through to retry
      }

      if (Date.now() > deadline) {
        if (!cancelled) setState("timeout");
        return;
      }
      setAttempt((n) => n + 1);
      setTimeout(poll, 2000);
    }

    poll();
    return () => {
      cancelled = true;
    };
  }, [router]);

  return (
    <div className="min-h-screen bg-gray-50 flex flex-col">
      <header className="bg-white border-b border-gray-100 px-6 py-4">
        <div className="max-w-2xl mx-auto">
          <Link href="/" className="flex items-center gap-2">
            <div className="w-8 h-8 rounded-xl bg-[#007AFF] flex items-center justify-center">
              <MessageCircleIcon className="w-5 h-5 text-white" />
            </div>
            <span className="font-semibold text-gray-900">BluTexts</span>
          </Link>
        </div>
      </header>

      <main className="flex-1 flex items-center justify-center px-6">
        <div className="w-full max-w-md text-center">
          {state === "polling" && (
            <>
              <div className="w-16 h-16 mx-auto rounded-full bg-blue-50 flex items-center justify-center mb-6">
                <LoaderIcon className="w-8 h-8 text-[#007AFF] animate-spin" />
              </div>
              <h1 className="text-2xl font-bold text-gray-900 mb-2">Finalizing your subscription…</h1>
              <p className="text-gray-500">
                Payment received. We're activating your account now — this usually takes a few seconds.
              </p>
              {attempt > 5 && (
                <p className="text-xs text-gray-400 mt-6">
                  Still working… Stripe occasionally takes up to a minute.
                </p>
              )}
            </>
          )}

          {state === "ready" && (
            <>
              <div className="w-16 h-16 mx-auto rounded-full bg-green-50 flex items-center justify-center mb-6">
                <CheckCircle2Icon className="w-8 h-8 text-green-600" />
              </div>
              <h1 className="text-2xl font-bold text-gray-900 mb-2">
                You&rsquo;re in. We&rsquo;re provisioning your number.
              </h1>
              <p className="text-gray-500 mb-6">
                Apple requires a manual identity step on our end before a
                new iMessage line goes live — typically a few business hours.
                You&rsquo;ll get an email the moment your number is ready
                to send from. In the meantime, you can connect Go High Level
                and invite your team from the dashboard.
              </p>

              <button
                onClick={() => router.replace("/dashboard")}
                className="inline-block bg-[#2E6FE0] text-white font-semibold px-6 py-3 rounded-xl hover:bg-blue-700 transition-colors"
              >
                Open dashboard
              </button>
            </>
          )}

          {state === "timeout" && (
            <>
              <div className="w-16 h-16 mx-auto rounded-full bg-yellow-50 flex items-center justify-center mb-6">
                <AlertCircleIcon className="w-8 h-8 text-yellow-600" />
              </div>
              <h1 className="text-2xl font-bold text-gray-900 mb-2">Payment is processing</h1>
              <p className="text-gray-500 mb-6">
                Your payment went through, but activation is taking longer than usual. You can safely
                continue to the dashboard — we'll finish setup in the background. If you don't see
                your account active within a few minutes, email support.
              </p>
              <button
                onClick={() => router.replace("/dashboard")}
                className="bg-[#007AFF] text-white font-semibold px-6 py-3 rounded-xl hover:bg-blue-600 transition-colors"
              >
                Continue to dashboard
              </button>
              {sessionID && (
                <p className="text-xs text-gray-400 mt-6 font-mono truncate">Session: {sessionID}</p>
              )}
            </>
          )}

          {state === "unauthenticated" && (
            <>
              <div className="w-16 h-16 mx-auto rounded-full bg-yellow-50 flex items-center justify-center mb-6">
                <AlertCircleIcon className="w-8 h-8 text-yellow-600" />
              </div>
              <h1 className="text-2xl font-bold text-gray-900 mb-2">Please sign in</h1>
              <p className="text-gray-500 mb-6">
                Your payment succeeded. Sign in with the email you used at checkout to access your
                new account.
              </p>
              <Link
                href="/login"
                className="inline-block bg-[#007AFF] text-white font-semibold px-6 py-3 rounded-xl hover:bg-blue-600 transition-colors"
              >
                Sign in
              </Link>
            </>
          )}
        </div>
      </main>
    </div>
  );
}
