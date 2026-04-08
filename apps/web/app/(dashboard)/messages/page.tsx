"use client";

import { useState, useEffect, useRef } from "react";
import useSWR from "swr";
import { SendIcon, SearchIcon } from "lucide-react";

const API = process.env.NEXT_PUBLIC_API_URL;
const WS_URL = process.env.NEXT_PUBLIC_WS_URL;

function fetcher(url: string) {
  const token = localStorage.getItem("access_token");
  return fetch(url, {
    headers: { Authorization: `Bearer ${token}` },
  }).then((r) => r.json());
}

interface Conversation {
  id: string;
  contact_address: string;
  contact_name: string | null;
  phone_number: string;
  last_message_preview: string | null;
  last_message_at: string | null;
  unread_count: number;
}

interface Message {
  id: string;
  direction: "inbound" | "outbound";
  content: string;
  status: string;
  created_at: string;
}

function formatTime(ts: string | null) {
  if (!ts) return "";
  const d = new Date(ts);
  const now = new Date();
  const diff = now.getTime() - d.getTime();
  if (diff < 60000) return "now";
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m`;
  if (diff < 86400000) return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  return d.toLocaleDateString([], { month: "short", day: "numeric" });
}

export default function MessagesPage() {
  const [selectedConv, setSelectedConv] = useState<Conversation | null>(null);
  const [messageInput, setMessageInput] = useState("");
  const [sending, setSending] = useState(false);
  const [messages, setMessages] = useState<Message[]>([]);
  const [search, setSearch] = useState("");
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);

  const { data: convsData, mutate: mutateConvs } = useSWR<{ conversations: Conversation[] }>(
    `${API}/api/conversations`,
    fetcher,
    { refreshInterval: 10000 }
  );

  const conversations = convsData?.conversations ?? [];
  const filtered = conversations.filter(
    (c) =>
      !search ||
      c.contact_address.includes(search) ||
      (c.contact_name && c.contact_name.toLowerCase().includes(search.toLowerCase()))
  );

  // Load messages for selected conversation
  useEffect(() => {
    if (!selectedConv) return;
    const token = localStorage.getItem("access_token");
    fetch(`${API}/api/conversations/${selectedConv.id}/messages`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((r) => r.json())
      .then((d) => setMessages((d.messages ?? []).reverse()));
  }, [selectedConv]);

  // Scroll to bottom
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  // WebSocket for real-time updates
  useEffect(() => {
    const token = localStorage.getItem("access_token");
    const ws = new WebSocket(`${WS_URL}/api/ws?token=${token}`);
    wsRef.current = ws;

    ws.onmessage = (e) => {
      const event = JSON.parse(e.data);
      if (event.type === "new_message") {
        const payload = event.payload;
        // If this conversation is selected, append to messages
        if (selectedConv && payload.conversation_id === selectedConv.id) {
          setMessages((prev) => [
            ...prev,
            {
              id: crypto.randomUUID(),
              direction: "inbound",
              content: payload.content,
              status: "delivered",
              created_at: payload.received_at,
            },
          ]);
        }
        mutateConvs();
      }
      if (event.type === "message_delivered" || event.type === "message_read") {
        setMessages((prev) =>
          prev.map((m) =>
            m.id === event.payload.message_id
              ? { ...m, status: event.payload.status }
              : m
          )
        );
      }
    };

    return () => ws.close();
  }, [selectedConv]);

  async function sendMessage() {
    if (!messageInput.trim() || !selectedConv) return;
    setSending(true);

    const token = localStorage.getItem("access_token");
    try {
      const res = await fetch(`${API}/api/messages/send`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          phone_number_id: selectedConv.phone_number,
          to_address: selectedConv.contact_address,
          content: messageInput.trim(),
        }),
      });
      const data = await res.json();

      if (data.rate_limited) {
        alert(
          `Daily limit reached. You've messaged ${data.rate_limit.daily_new_contacts_used} new contacts today. Resets at midnight.`
        );
        return;
      }

      if (data.message) {
        setMessages((prev) => [...prev, data.message]);
        setMessageInput("");
        mutateConvs();
      }
    } finally {
      setSending(false);
    }
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  }

  const StatusDot = ({ status }: { status: string }) => {
    if (status === "delivered" || status === "read") {
      return <span className="text-xs text-blue-400">✓✓</span>;
    }
    if (status === "sent") return <span className="text-xs text-gray-400">✓</span>;
    if (status === "failed") return <span className="text-xs text-red-400">!</span>;
    return <span className="text-xs text-gray-300">○</span>;
  };

  return (
    <div className="h-[calc(100vh-8rem)] -m-6 md:-m-8 flex border border-gray-100 rounded-2xl overflow-hidden bg-white shadow-sm">
      {/* Conversation list */}
      <div className="w-80 flex-shrink-0 border-r border-gray-100 flex flex-col">
        <div className="p-4 border-b border-gray-50">
          <h2 className="font-semibold text-gray-900 mb-3">Messages</h2>
          <div className="relative">
            <SearchIcon className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search conversations..."
              className="w-full pl-9 pr-4 py-2 text-sm bg-gray-50 rounded-xl border-0 focus:outline-none focus:ring-2 focus:ring-[#007AFF]"
            />
          </div>
        </div>

        <div className="flex-1 overflow-y-auto">
          {filtered.length === 0 && (
            <div className="p-8 text-center text-sm text-gray-400">
              No conversations yet
            </div>
          )}
          {filtered.map((conv) => (
            <button
              key={conv.id}
              onClick={() => setSelectedConv(conv)}
              className={`w-full text-left px-4 py-4 border-b border-gray-50 hover:bg-gray-50 transition-colors ${
                selectedConv?.id === conv.id ? "bg-blue-50" : ""
              }`}
            >
              <div className="flex items-center justify-between mb-1">
                <span className="font-medium text-sm text-gray-900 truncate">
                  {conv.contact_name || conv.contact_address}
                </span>
                <span className="text-xs text-gray-400 flex-shrink-0 ml-2">
                  {formatTime(conv.last_message_at)}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-xs text-gray-500 truncate">
                  {conv.last_message_preview || "No messages yet"}
                </span>
                {conv.unread_count > 0 && (
                  <span className="ml-2 bg-[#007AFF] text-white text-xs rounded-full w-5 h-5 flex items-center justify-center flex-shrink-0">
                    {conv.unread_count}
                  </span>
                )}
              </div>
            </button>
          ))}
        </div>
      </div>

      {/* Message thread */}
      <div className="flex-1 flex flex-col min-w-0">
        {!selectedConv ? (
          <div className="flex-1 flex items-center justify-center">
            <div className="text-center text-gray-400">
              <div className="w-16 h-16 bg-gray-100 rounded-full flex items-center justify-center mx-auto mb-4">
                <SendIcon className="w-8 h-8 text-gray-300" />
              </div>
              <p className="font-medium text-gray-500">Select a conversation</p>
              <p className="text-sm mt-1">or wait for an incoming message</p>
            </div>
          </div>
        ) : (
          <>
            {/* Thread header */}
            <div className="px-6 py-4 border-b border-gray-100 bg-white">
              <div className="font-semibold text-gray-900">
                {selectedConv.contact_name || selectedConv.contact_address}
              </div>
              <div className="text-xs text-gray-400 mt-0.5">
                via {selectedConv.phone_number}
              </div>
            </div>

            {/* Messages */}
            <div className="flex-1 overflow-y-auto p-6 space-y-2 bg-gray-50">
              {messages.map((msg) => (
                <div
                  key={msg.id}
                  className={`flex ${
                    msg.direction === "outbound" ? "justify-end" : "justify-start"
                  }`}
                >
                  <div className="max-w-[70%]">
                    <div
                      className={`px-4 py-2.5 text-sm leading-relaxed ${
                        msg.direction === "outbound"
                          ? "bubble-outbound"
                          : "bubble-inbound"
                      }`}
                    >
                      {msg.content}
                    </div>
                    {msg.direction === "outbound" && (
                      <div className="flex justify-end mt-1 gap-1 items-center">
                        <span className="text-xs text-gray-400">
                          {new Date(msg.created_at).toLocaleTimeString([], {
                            hour: "2-digit",
                            minute: "2-digit",
                          })}
                        </span>
                        <StatusDot status={msg.status} />
                      </div>
                    )}
                  </div>
                </div>
              ))}
              <div ref={messagesEndRef} />
            </div>

            {/* Input */}
            <div className="p-4 bg-white border-t border-gray-100">
              <div className="flex items-end gap-3">
                <textarea
                  value={messageInput}
                  onChange={(e) => setMessageInput(e.target.value)}
                  onKeyDown={handleKeyDown}
                  placeholder="iMessage..."
                  rows={1}
                  className="flex-1 resize-none px-4 py-3 bg-gray-100 rounded-2xl text-sm focus:outline-none focus:ring-2 focus:ring-[#007AFF] max-h-32"
                  style={{ height: "auto" }}
                />
                <button
                  onClick={sendMessage}
                  disabled={sending || !messageInput.trim()}
                  className="w-10 h-10 bg-[#007AFF] rounded-full flex items-center justify-center hover:bg-blue-600 transition-colors disabled:opacity-40 flex-shrink-0"
                >
                  <SendIcon className="w-4 h-4 text-white" />
                </button>
              </div>
              <p className="text-xs text-gray-400 mt-2 text-center">
                Press Enter to send · Shift+Enter for new line
              </p>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
