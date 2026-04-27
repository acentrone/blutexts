"use client";

import { useState } from "react";
import useSWR from "swr";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  XIcon,
  LoaderIcon,
} from "lucide-react";

const API = process.env.NEXT_PUBLIC_API_URL;

function fetcher(url: string) {
  const token = localStorage.getItem("access_token");
  return fetch(url, {
    headers: { Authorization: `Bearer ${token}` },
  }).then((r) => r.json());
}

interface Contact {
  id: string;
  imessage_address: string;
  name: string | null;
  email: string | null;
  company: string | null;
  notes: string;
  tags: string[];
  message_count: number;
  conversation_count: number;
  first_message_at: string | null;
  last_message_at: string | null;
  created_at: string;
}

function getInitials(name: string | null, address: string): string {
  if (name) {
    const parts = name.trim().split(/\s+/);
    if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase();
    return name.slice(0, 2).toUpperCase();
  }
  const digits = address.replace(/\D/g, "");
  return digits.slice(-2);
}

function formatPhone(phone: string): string {
  const digits = phone.replace(/\D/g, "");
  if (digits.length === 11 && digits[0] === "1") {
    return `(${digits.slice(1, 4)}) ${digits.slice(4, 7)}-${digits.slice(7)}`;
  }
  if (digits.length === 10) {
    return `(${digits.slice(0, 3)}) ${digits.slice(3, 6)}-${digits.slice(6)}`;
  }
  return phone;
}

// All contacts are iMessage — we only ship to iMessage. Single dot.
function ServiceDot() {
  return <span className="bb" title="iMessage" />;
}

function tagClass(tag: string): string {
  const t = tag.toLowerCase();
  if (t === "vip") return "mini-tag vip";
  if (t === "lead" || t === "prospect") return "mini-tag lead";
  if (t === "active" || t === "customer") return "mini-tag active";
  if (t === "follow" || t === "follow-up" || t === "followup") return "mini-tag follow";
  return "mini-tag neutral";
}

function formatRelative(ts: string | null): string {
  if (!ts) return "Never";
  const d = new Date(ts);
  const diff = Date.now() - d.getTime();
  if (diff < 60000) return "Just now";
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`;
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`;
  if (diff < 2592000000) return `${Math.floor(diff / 86400000)}d ago`;
  return d.toLocaleDateString([], { month: "short", day: "numeric" });
}

export default function ContactsPage() {
  const router = useRouter();
  const [search, setSearch] = useState("");
  const [selectedTag, setSelectedTag] = useState("");
  const [showNewContact, setShowNewContact] = useState(false);

  const params = new URLSearchParams();
  if (search) params.set("search", search);
  if (selectedTag) params.set("tag", selectedTag);
  params.set("limit", "200");

  const { data, mutate } = useSWR<{ contacts: Contact[]; total: number }>(
    `${API}/api/contacts?${params}`,
    fetcher,
    { refreshInterval: 30000 }
  );

  const { data: tagsData } = useSWR<{ tags: string[] }>(
    `${API}/api/contacts/tags`,
    fetcher
  );

  const contacts = data?.contacts ?? [];
  const total = data?.total ?? 0;
  const tags = tagsData?.tags ?? [];


  return (
    <>
      <header className="page-header">
        <div className="titles">
          <div className="crumb">Contacts</div>
          <h1>
            Your <em>people</em>.
          </h1>
          <div className="sub">Every person you&apos;ve messaged from Blu, in one place.</div>
        </div>
        <div style={{ display: "flex", gap: 10 }}>
          <button
            type="button"
            className="btn primary"
            onClick={() => setShowNewContact(true)}
          >
            <svg width="15" height="15" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.8">
              <path d="M10 3v14M3 10h14" strokeLinecap="round" />
            </svg>
            New contact
          </button>
        </div>
      </header>

      {showNewContact && (
        <NewContactDialog
          onClose={() => setShowNewContact(false)}
          onCreated={(id) => {
            setShowNewContact(false);
            mutate();
            router.push(`/contacts/${id}`);
          }}
        />
      )}

      <div className="page-body">
        <div className="toolbar">
          <div className="search-wrap">
            <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.8">
              <circle cx="7" cy="7" r="5" />
              <path d="M14 14l-3-3" strokeLinecap="round" />
            </svg>
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search by name, phone, email, or company…"
            />
          </div>
          {tags.slice(0, 6).map((tag) => (
            <button
              key={tag}
              type="button"
              className={`filter-chip${selectedTag === tag ? " active" : ""}`}
              onClick={() => setSelectedTag(selectedTag === tag ? "" : tag)}
            >
              <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.8">
                <path d="M2 3h12M4 8h8M6 13h4" strokeLinecap="round" />
              </svg>
              {tag}
              {selectedTag === tag && (
                <XIcon style={{ width: 12, height: 12, marginLeft: 2 }} />
              )}
            </button>
          ))}
        </div>

        <div className="count-row">
          <div className="count">
            <b>
              {total} {total === 1 ? "contact" : "contacts"}
            </b>
          </div>
          <div className="sort-by">SORTED BY LAST ACTIVE</div>
        </div>

        {contacts.length === 0 ? (
          <div
            style={{
              background: "#fff",
              border: "1px solid var(--rule)",
              borderRadius: "var(--r-lg)",
              padding: 60,
              textAlign: "center",
              color: "var(--muted)",
            }}
          >
            <div className="serif" style={{ fontSize: 28, color: "var(--ink)", marginBottom: 8 }}>
              No contacts yet.
            </div>
            <div style={{ fontSize: 14 }}>
              Contacts appear here when you send or receive messages.
            </div>
          </div>
        ) : (
          <div className="contacts-table">
            <div className="ct-head">
              <span>Contact</span>
              <span>Company · Email</span>
              <span>Last active</span>
              <span>Messages</span>
              <span>Tags</span>
              <span />
            </div>

            {contacts.map((contact) => {
              const initials = getInitials(contact.name, contact.imessage_address);
              const display = contact.name || formatPhone(contact.imessage_address);
              return (
                <Link
                  key={contact.id}
                  href={`/contacts/${contact.id}`}
                  className="ct-row"
                >
                  <div className="contact">
                    <div className="avatar blu">
                      {initials}
                    </div>
                    <div className="meta">
                      <div className="name">
                        {display}
                        <ServiceDot />
                      </div>
                      <div className="phone">
                        {formatPhone(contact.imessage_address)}
                      </div>
                    </div>
                  </div>
                  {contact.company || contact.email ? (
                    <span className="cell">
                      {contact.company}
                      {contact.company && contact.email ? " · " : ""}
                      {contact.email}
                    </span>
                  ) : (
                    <span className="cell muted">—</span>
                  )}
                  <span className="cell muted">
                    {formatRelative(contact.last_message_at)}
                  </span>
                  <span className="msgs">
                    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.8">
                      <path d="M2 4a2 2 0 012-2h8a2 2 0 012 2v6a2 2 0 01-2 2H6l-4 3V4z" strokeLinejoin="round" />
                    </svg>
                    {contact.message_count}
                  </span>
                  <div className="tags-cell">
                    {contact.tags.slice(0, 2).map((tag) => (
                      <span key={tag} className={tagClass(tag)}>
                        {tag}
                      </span>
                    ))}
                  </div>
                  <span className="chevron">›</span>
                </Link>
              );
            })}
          </div>
        )}
      </div>
    </>
  );
}

// ─── New Contact Dialog ───
// Creates a contact ahead of any messaging. Uses POST /api/contacts which
// upserts on (account_id, imessage_address) so re-submitting an existing
// number just updates fields rather than failing.

function NewContactDialog({
  onClose,
  onCreated,
}: {
  onClose: () => void;
  onCreated: (id: string) => void;
}) {
  const [address, setAddress] = useState("");
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [company, setCompany] = useState("");
  const [tagsInput, setTagsInput] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    if (!address.trim()) {
      setError("Phone number or email is required");
      return;
    }
    setSubmitting(true);
    try {
      const tags = tagsInput
        .split(",")
        .map((t) => t.trim())
        .filter(Boolean);
      const res = await fetch(`${API}/api/contacts`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${localStorage.getItem("access_token")}`,
        },
        body: JSON.stringify({
          imessage_address: address.trim(),
          name: name.trim(),
          email: email.trim(),
          company: company.trim(),
          tags,
        }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error || "Failed to create contact");
        setSubmitting(false);
        return;
      }
      onCreated(data.id);
    } catch {
      setError("Network error. Please try again.");
      setSubmitting(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 bg-black/30 flex items-center justify-center p-4" onClick={onClose}>
      <div
        className="bg-white rounded-2xl shadow-xl w-full max-w-md max-h-[90vh] overflow-auto"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-100">
          <h2 className="text-lg font-semibold text-gray-900">New contact</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600">
            <XIcon className="w-5 h-5" />
          </button>
        </div>
        <form onSubmit={submit} className="p-6 space-y-4">
          <div>
            <label className="block text-xs font-medium text-gray-500 mb-1.5">
              Phone or email <span className="text-red-500">*</span>
            </label>
            <input
              type="text"
              value={address}
              onChange={(e) => setAddress(e.target.value)}
              placeholder="+15551234567 or name@example.com"
              required
              autoFocus
              className="w-full px-3 py-2.5 text-sm border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#007AFF]"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-500 mb-1.5">
              Name
            </label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="w-full px-3 py-2.5 text-sm border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#007AFF]"
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1.5">
                Email
              </label>
              <input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="w-full px-3 py-2.5 text-sm border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#007AFF]"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1.5">
                Company
              </label>
              <input
                type="text"
                value={company}
                onChange={(e) => setCompany(e.target.value)}
                className="w-full px-3 py-2.5 text-sm border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#007AFF]"
              />
            </div>
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-500 mb-1.5">
              Tags <span className="text-gray-400 font-normal">(comma separated)</span>
            </label>
            <input
              type="text"
              value={tagsInput}
              onChange={(e) => setTagsInput(e.target.value)}
              placeholder="lead, vip, follow-up"
              className="w-full px-3 py-2.5 text-sm border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#007AFF]"
            />
          </div>

          {error && (
            <div className="text-sm text-red-600 bg-red-50 border border-red-100 rounded-xl px-4 py-2.5">
              {error}
            </div>
          )}

          <div className="flex items-center justify-end gap-2 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="text-sm font-medium text-gray-600 hover:text-gray-900 px-4 py-2"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={submitting}
              className="inline-flex items-center gap-1.5 bg-[#007AFF] text-white text-sm font-semibold px-4 py-2 rounded-xl hover:bg-blue-600 disabled:opacity-50"
            >
              {submitting && <LoaderIcon className="w-3.5 h-3.5 animate-spin" />}
              {submitting ? "Creating..." : "Create contact"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
