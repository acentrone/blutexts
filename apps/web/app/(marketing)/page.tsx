"use client";

import Link from "next/link";
import { motion } from "framer-motion";
import { CheckIcon, ArrowRightIcon, MessageCircleIcon, ZapIcon, ShieldCheckIcon, BarChart3Icon } from "lucide-react";

const plans = [
  {
    name: "Monthly",
    setupFee: "$399",
    price: "$199",
    period: "/month",
    description: "One dedicated number. Unlimited existing conversations.",
    features: [
      "1 dedicated phone number",
      "50 new contacts/day",
      "Unlimited existing conversations",
      "Go High Level integration",
      "Real-time message sync",
      "Full message history",
      "CSV export",
      "Email support",
    ],
    cta: "Get Started",
    href: "/signup?plan=monthly",
    highlight: false,
  },
  {
    name: "Annual",
    setupFee: "$399",
    price: "$2,600",
    period: "/year",
    savings: "Save $788/yr",
    description: "Everything in Monthly, billed once a year.",
    features: [
      "1 dedicated phone number",
      "50 new contacts/day",
      "Unlimited existing conversations",
      "Go High Level integration",
      "Real-time message sync",
      "Full message history",
      "CSV export",
      "Priority support",
    ],
    cta: "Get Best Value",
    href: "/signup?plan=annual",
    highlight: true,
  },
];

const stats = [
  { value: "4–8×", label: "Higher open rates vs email" },
  { value: "45%", label: "Average reply rate" },
  { value: "<2min", label: "Typical response time" },
  { value: "100%", label: "Native iMessage delivery" },
];

const features = [
  {
    icon: MessageCircleIcon,
    title: "Real iMessages, not SMS",
    description:
      "Blue bubbles build trust. Recipients see your message in their native Messages app — indistinguishable from a personal text.",
  },
  {
    icon: ZapIcon,
    title: "Go High Level, fully automated",
    description:
      "Every message, contact, and conversation syncs bidirectionally with your GHL sub-account. No manual work after onboarding.",
  },
  {
    icon: ShieldCheckIcon,
    title: "Dedicated numbers, never shared",
    description:
      "Your number is yours. No shared infrastructure, no diluted deliverability, no cross-contamination with other senders.",
  },
  {
    icon: BarChart3Icon,
    title: "Built-in guardrails",
    description:
      "50 new contacts/day per number keeps your account in Apple's good graces. Existing conversations are unrestricted.",
  },
];

export default function LandingPage() {
  return (
    <div className="flex flex-col min-h-screen bg-white">
      {/* Nav */}
      <header className="fixed top-0 w-full z-50 bg-white/80 backdrop-blur-xl border-b border-gray-100">
        <div className="max-w-6xl mx-auto px-6 h-16 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 rounded-xl bg-[#007AFF] flex items-center justify-center">
              <MessageCircleIcon className="w-5 h-5 text-white" />
            </div>
            <span className="font-semibold text-gray-900 text-lg tracking-tight">BlueSend</span>
          </div>
          <nav className="hidden md:flex items-center gap-8 text-sm text-gray-600">
            <a href="#features" className="hover:text-gray-900 transition-colors">Features</a>
            <a href="#pricing" className="hover:text-gray-900 transition-colors">Pricing</a>
            <Link href="/login" className="hover:text-gray-900 transition-colors">Sign in</Link>
          </nav>
          <Link
            href="/signup"
            className="bg-[#007AFF] text-white text-sm font-medium px-5 py-2.5 rounded-full hover:bg-blue-600 transition-colors"
          >
            Get Started
          </Link>
        </div>
      </header>

      <main className="flex-1">
        {/* Hero */}
        <section className="pt-32 pb-24 px-6">
          <div className="max-w-4xl mx-auto text-center">
            <motion.div
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.6, ease: [0.16, 1, 0.3, 1] }}
            >
              <div className="inline-flex items-center gap-2 bg-blue-50 text-[#007AFF] text-sm font-medium px-4 py-1.5 rounded-full mb-6">
                <span className="w-1.5 h-1.5 rounded-full bg-[#007AFF] animate-pulse" />
                iMessage-native CRM infrastructure
              </div>

              <h1 className="text-5xl md:text-7xl font-bold text-gray-900 tracking-tight leading-none mb-6">
                iMessage
                <br />
                <span className="text-[#007AFF]">for business.</span>
                <br />
                Finally.
              </h1>

              <p className="text-xl text-gray-500 max-w-2xl mx-auto mb-10 leading-relaxed">
                Send and receive iMessages through Go High Level. Blue bubbles that feel personal,
                backed by CRM infrastructure that scales. Dedicated numbers, real-time sync,
                zero bulk-messaging risk.
              </p>

              <div className="flex flex-col sm:flex-row items-center justify-center gap-4">
                <Link
                  href="/signup"
                  className="w-full sm:w-auto bg-[#007AFF] text-white font-semibold px-8 py-4 rounded-full text-lg hover:bg-blue-600 transition-all hover:shadow-lg hover:shadow-blue-500/25 flex items-center justify-center gap-2"
                >
                  Start sending today
                  <ArrowRightIcon className="w-5 h-5" />
                </Link>
                <a
                  href="#pricing"
                  className="w-full sm:w-auto border border-gray-200 text-gray-700 font-medium px-8 py-4 rounded-full text-lg hover:border-gray-300 hover:bg-gray-50 transition-colors text-center"
                >
                  See pricing
                </a>
              </div>
            </motion.div>

            {/* Message bubble mockup */}
            <motion.div
              initial={{ opacity: 0, y: 40 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.8, delay: 0.3, ease: [0.16, 1, 0.3, 1] }}
              className="mt-16 max-w-sm mx-auto"
            >
              <div className="bg-gray-100 rounded-3xl p-6 space-y-3 shadow-xl shadow-gray-200/50">
                <div className="flex justify-end">
                  <div className="bubble-outbound px-4 py-2.5 text-sm max-w-[75%]">
                    Hey Sarah! Quick follow-up on our proposal — any questions?
                  </div>
                </div>
                <div className="flex justify-start">
                  <div className="bubble-inbound px-4 py-2.5 text-sm max-w-[75%]">
                    Hey! Yes actually — can we schedule a 15-min call?
                  </div>
                </div>
                <div className="flex justify-end">
                  <div className="bubble-outbound px-4 py-2.5 text-sm max-w-[75%]">
                    Absolutely. How does Thursday at 2pm work?
                  </div>
                </div>
                <div className="text-center text-xs text-gray-400 pt-2">
                  Delivered • Read 2m ago
                </div>
              </div>
            </motion.div>
          </div>
        </section>

        {/* Stats */}
        <section className="py-16 px-6 bg-gray-50">
          <div className="max-w-6xl mx-auto grid grid-cols-2 md:grid-cols-4 gap-8">
            {stats.map((stat) => (
              <div key={stat.value} className="text-center">
                <div className="text-4xl font-bold text-[#007AFF] mb-1">{stat.value}</div>
                <div className="text-sm text-gray-500">{stat.label}</div>
              </div>
            ))}
          </div>
        </section>

        {/* Features */}
        <section id="features" className="py-24 px-6">
          <div className="max-w-6xl mx-auto">
            <div className="text-center mb-16">
              <h2 className="text-4xl font-bold text-gray-900 tracking-tight mb-4">
                Built for serious outreach
              </h2>
              <p className="text-lg text-gray-500 max-w-2xl mx-auto">
                Not a blast tool. A precision instrument for teams that care about reply rates.
              </p>
            </div>
            <div className="grid md:grid-cols-2 gap-8">
              {features.map((feature) => (
                <div
                  key={feature.title}
                  className="bg-white border border-gray-100 rounded-2xl p-8 hover:border-blue-100 hover:shadow-lg hover:shadow-blue-50 transition-all"
                >
                  <div className="w-12 h-12 bg-blue-50 rounded-2xl flex items-center justify-center mb-5">
                    <feature.icon className="w-6 h-6 text-[#007AFF]" />
                  </div>
                  <h3 className="text-xl font-semibold text-gray-900 mb-3">{feature.title}</h3>
                  <p className="text-gray-500 leading-relaxed">{feature.description}</p>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* Pricing */}
        <section id="pricing" className="py-24 px-6 bg-gray-50">
          <div className="max-w-5xl mx-auto">
            <div className="text-center mb-16">
              <h2 className="text-4xl font-bold text-gray-900 tracking-tight mb-4">
                Simple, transparent pricing
              </h2>
              <p className="text-lg text-gray-500">
                One-time $399 setup fee covers your dedicated number provisioning.
              </p>
            </div>
            <div className="grid md:grid-cols-2 gap-6 max-w-3xl mx-auto">
              {plans.map((plan) => (
                <div
                  key={plan.name}
                  className={`rounded-3xl p-8 ${
                    plan.highlight
                      ? "bg-[#007AFF] text-white shadow-2xl shadow-blue-500/30 ring-1 ring-blue-400"
                      : "bg-white border border-gray-200"
                  }`}
                >
                  {plan.savings && (
                    <div className="inline-block bg-white/20 text-white text-xs font-semibold px-3 py-1 rounded-full mb-4">
                      {plan.savings}
                    </div>
                  )}
                  <div className="mb-6">
                    <div className={`text-sm font-medium mb-1 ${plan.highlight ? "text-blue-100" : "text-gray-500"}`}>
                      {plan.name}
                    </div>
                    <div className="flex items-baseline gap-1">
                      <span className="text-5xl font-bold tracking-tight">{plan.price}</span>
                      <span className={`text-lg ${plan.highlight ? "text-blue-200" : "text-gray-400"}`}>
                        {plan.period}
                      </span>
                    </div>
                    <div className={`text-sm mt-1 ${plan.highlight ? "text-blue-200" : "text-gray-400"}`}>
                      + {plan.setupFee} one-time setup
                    </div>
                  </div>
                  <p className={`text-sm mb-6 ${plan.highlight ? "text-blue-100" : "text-gray-500"}`}>
                    {plan.description}
                  </p>
                  <ul className="space-y-3 mb-8">
                    {plan.features.map((f) => (
                      <li key={f} className="flex items-center gap-3 text-sm">
                        <CheckIcon className={`w-4 h-4 flex-shrink-0 ${plan.highlight ? "text-white" : "text-[#007AFF]"}`} />
                        <span className={plan.highlight ? "text-blue-50" : "text-gray-600"}>{f}</span>
                      </li>
                    ))}
                  </ul>
                  <Link
                    href={plan.href}
                    className={`block text-center font-semibold py-3.5 rounded-2xl transition-all ${
                      plan.highlight
                        ? "bg-white text-[#007AFF] hover:bg-blue-50"
                        : "bg-[#007AFF] text-white hover:bg-blue-600"
                    }`}
                  >
                    {plan.cta}
                  </Link>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* CTA */}
        <section className="py-24 px-6">
          <div className="max-w-3xl mx-auto text-center">
            <h2 className="text-4xl font-bold text-gray-900 tracking-tight mb-6">
              Ready to go blue?
            </h2>
            <p className="text-lg text-gray-500 mb-10">
              Setup takes minutes. You'll be sending iMessages through GHL the same day.
            </p>
            <Link
              href="/signup"
              className="inline-flex items-center gap-2 bg-[#007AFF] text-white font-semibold px-10 py-4 rounded-full text-lg hover:bg-blue-600 transition-all hover:shadow-xl hover:shadow-blue-500/30"
            >
              Get started — $399 setup
              <ArrowRightIcon className="w-5 h-5" />
            </Link>
          </div>
        </section>
      </main>

      <footer className="border-t border-gray-100 py-8 px-6">
        <div className="max-w-6xl mx-auto flex flex-col md:flex-row items-center justify-between gap-4 text-sm text-gray-400">
          <div className="flex items-center gap-2">
            <div className="w-6 h-6 rounded-lg bg-[#007AFF] flex items-center justify-center">
              <MessageCircleIcon className="w-4 h-4 text-white" />
            </div>
            <span className="font-medium text-gray-600">BlueSend</span>
          </div>
          <div className="flex items-center gap-6">
            <a href="/privacy" className="hover:text-gray-600 transition-colors">Privacy</a>
            <a href="/terms" className="hover:text-gray-600 transition-colors">Terms</a>
            <a href="mailto:support@blutexts.com" className="hover:text-gray-600 transition-colors">Support</a>
          </div>
          <span>© 2025 BlueSend. Not affiliated with Apple Inc.</span>
        </div>
      </footer>
    </div>
  );
}
