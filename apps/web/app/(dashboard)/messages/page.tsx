"use client";

import React, { useState, useEffect, useRef, useCallback } from "react";
import useSWR from "swr";
import Link from "next/link";
import {
  SendIcon,
  SearchIcon,
  PlayIcon,
  PauseIcon,
  MicIcon,
  XIcon,
  ImageIcon,
  UserIcon,
  CopyIcon,
  CheckIcon,
  ChevronLeftIcon,
  SparklesIcon,
  ClockIcon,
  PenSquareIcon,
  LoaderIcon,
} from "lucide-react";

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
  contact_id: string;
  contact_address: string;
  contact_name: string | null;
  phone_number: string;
  phone_number_id: string;
  last_message_preview: string | null;
  last_message_at: string | null;
  unread_count: number;
  message_count: number;
}

interface Attachment {
  url: string;
  type: string;
  filename: string;
  size: number;
  web_url?: string;
}

interface Message {
  id: string;
  conversation_id?: string;
  direction: "inbound" | "outbound";
  content: string;
  attachments?: Attachment[];
  status: string;
  error_message?: string | null;
  created_at: string;
}

// ─── Helpers ──────────────────────────────────────────────────

function formatTime(ts: string | null) {
  if (!ts) return "";
  const d = new Date(ts);
  const now = new Date();
  const diff = now.getTime() - d.getTime();
  if (diff < 60000) return "now";
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m`;
  if (diff < 86400000)
    return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  if (diff < 604800000) return `${Math.floor(diff / 86400000)}d`;
  return d.toLocaleDateString([], { month: "short", day: "numeric" });
}

function getInitials(name: string | null, address: string): string {
  if (name) {
    const parts = name.trim().split(/\s+/);
    if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase();
    return name.slice(0, 2).toUpperCase();
  }
  // Phone number — last 2 digits
  const digits = address.replace(/\D/g, "");
  return digits.slice(-2);
}

function formatPhoneDisplay(phone: string): string {
  const digits = phone.replace(/\D/g, "");
  if (digits.length === 11 && digits[0] === "1") {
    return `+1 ${digits.slice(1, 4)} ${digits.slice(4, 7)} ${digits.slice(7)}`;
  }
  if (digits.length === 10) {
    return `+1 ${digits.slice(0, 3)} ${digits.slice(3, 6)} ${digits.slice(6)}`;
  }
  return phone;
}

// ─── Audio Bubble ─────────────────────────────────────────────

function AudioBubble({ url, direction }: { url: string; direction: string }) {
  const [playing, setPlaying] = useState(false);
  const [progress, setProgress] = useState(0);
  const [duration, setDuration] = useState(0);
  const audioRef = useRef<HTMLAudioElement>(null);
  const animRef = useRef<number>(0);

  function togglePlay() {
    const audio = audioRef.current;
    if (!audio) return;
    if (playing) {
      audio.pause();
      cancelAnimationFrame(animRef.current);
    } else {
      audio.play();
      const tick = () => {
        if (audio && !audio.paused) {
          setProgress(audio.currentTime / (audio.duration || 1));
          animRef.current = requestAnimationFrame(tick);
        }
      };
      tick();
    }
    setPlaying(!playing);
  }

  function formatDur(s: number) {
    if (!s || !isFinite(s)) return "0:00";
    return `${Math.floor(s / 60)}:${Math.floor(s % 60)
      .toString()
      .padStart(2, "0")}`;
  }

  const isOutbound = direction === "outbound";
  const bars = Array.from({ length: 28 }, (_, i) => {
    const h = ((i * 7 + url.charCodeAt(i % url.length)) % 20) + 4;
    return h;
  });

  return (
    <div className="flex items-center gap-2.5 min-w-[200px]">
      <audio
        ref={audioRef}
        src={url}
        preload="metadata"
        onLoadedMetadata={() => setDuration(audioRef.current?.duration ?? 0)}
        onEnded={() => {
          setPlaying(false);
          setProgress(0);
          cancelAnimationFrame(animRef.current);
        }}
      />
      <button
        onClick={togglePlay}
        className={`w-8 h-8 rounded-full flex items-center justify-center flex-shrink-0 ${
          isOutbound
            ? "bg-white/20 hover:bg-white/30"
            : "bg-black/10 hover:bg-black/15"
        }`}
      >
        {playing ? (
          <PauseIcon
            className={`w-3.5 h-3.5 ${
              isOutbound ? "text-white" : "text-gray-700"
            }`}
          />
        ) : (
          <PlayIcon
            className={`w-3.5 h-3.5 ${
              isOutbound ? "text-white" : "text-gray-700"
            }`}
          />
        )}
      </button>
      <div className="flex-1 flex flex-col gap-1">
        <div className="flex items-end gap-[2px] h-6">
          {bars.map((h, i) => {
            const filled = progress > i / bars.length;
            return (
              <div
                key={i}
                className="rounded-full transition-colors duration-150"
                style={{
                  width: 2.5,
                  height: h,
                  backgroundColor: filled
                    ? isOutbound
                      ? "rgba(255,255,255,0.9)"
                      : "#007AFF"
                    : isOutbound
                    ? "rgba(255,255,255,0.35)"
                    : "rgba(0,0,0,0.15)",
                }}
              />
            );
          })}
        </div>
        <span
          className={`text-[10px] ${
            isOutbound ? "text-white/70" : "text-gray-500"
          }`}
        >
          {playing
            ? formatDur(audioRef.current?.currentTime ?? 0)
            : formatDur(duration)}
        </span>
      </div>
    </div>
  );
}

// ─── Attachment Renderer ──────────────────────────────────────

function AttachmentRenderer({
  attachments,
  direction,
}: {
  attachments: Attachment[];
  direction: string;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      {attachments.map((att, i) => {
        if (att.type.startsWith("audio/")) {
          return (
            <AudioBubble
              key={i}
              url={att.web_url || att.url}
              direction={direction}
            />
          );
        }
        if (att.type.startsWith("image/")) {
          return (
            <a key={i} href={att.url} target="_blank" rel="noopener noreferrer">
              <img
                src={att.url}
                alt={att.filename}
                className="max-w-[240px] rounded-xl"
                loading="lazy"
              />
            </a>
          );
        }
        if (att.type.startsWith("video/")) {
          return (
            <video
              key={i}
              src={att.url}
              controls
              className="max-w-[240px] rounded-xl"
              preload="metadata"
            />
          );
        }
        return (
          <a
            key={i}
            href={att.url}
            target="_blank"
            rel="noopener noreferrer"
            className={`text-xs underline ${
              direction === "outbound" ? "text-white/80" : "text-blue-500"
            }`}
          >
            {att.filename} ({Math.round(att.size / 1024)}KB)
          </a>
        );
      })}
    </div>
  );
}

// ─── Status Dot ───────────────────────────────────────────────

function StatusDot({ status, errorMessage }: { status: string; errorMessage?: string | null }) {
  if (status === "delivered" || status === "read")
    return <span className="text-xs text-blue-400" title="Delivered">✓✓</span>;
  if (status === "sent")
    return <span className="text-xs text-gray-400" title="Sent">✓</span>;
  if (status === "failed")
    return (
      <span
        className="text-xs text-red-500 font-bold"
        title={errorMessage ? `Failed: ${errorMessage}` : "Failed to send"}
      >
        ⚠
      </span>
    );
  return <span className="text-xs text-gray-300" title="Pending">○</span>;
}

// ─── Pending File Type ────────────────────────────────────────

interface PendingFile {
  file: File;
  previewUrl: string;
  uploaded?: Attachment;
}

// ─── iMessage Effects ────────────────────────────────────────

const IMESSAGE_EFFECTS = [
  { id: "slam", label: "Slam", emoji: "💥" },
  { id: "loud", label: "Loud", emoji: "📢" },
  { id: "gentle", label: "Gentle", emoji: "🤫" },
  { id: "invisible_ink", label: "Invisible Ink", emoji: "👀" },
  { id: "echo", label: "Echo", emoji: "🔊" },
  { id: "spotlight", label: "Spotlight", emoji: "💡" },
  { id: "balloons", label: "Balloons", emoji: "🎈" },
  { id: "confetti", label: "Confetti", emoji: "🎊" },
  { id: "love", label: "Love", emoji: "❤️" },
  { id: "lasers", label: "Lasers", emoji: "🔦" },
  { id: "fireworks", label: "Fireworks", emoji: "🎆" },
  { id: "celebration", label: "Celebration", emoji: "🎉" },
];

// ═══════════════════════════════════════════════════════════════
// Main Component
// ═══════════════════════════════════════════════════════════════

export default function MessagesPage() {
  const [selectedConv, setSelectedConv] = useState<Conversation | null>(null);
  const [messageInput, setMessageInput] = useState("");
  const [sending, setSending] = useState(false);
  const [sendStatus, setSendStatus] = useState("");
  const [messages, setMessages] = useState<Message[]>([]);
  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState<"all" | "unread">("all");
  const [copied, setCopied] = useState(false);
  const [mobileShowThread, setMobileShowThread] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [pendingFiles, setPendingFiles] = useState<PendingFile[]>([]);
  const [recording, setRecording] = useState(false);
  const [recordingTime, setRecordingTime] = useState(0);
  const [selectedEffect, setSelectedEffect] = useState("");
  const [showEffects, setShowEffects] = useState(false);
  const [showSchedule, setShowSchedule] = useState(false);
  const [scheduleDate, setScheduleDate] = useState("");
  const [scheduleTime, setScheduleTime] = useState("");
  const [showCompose, setShowCompose] = useState(false);
  const mediaRecorderRef = useRef<MediaRecorder | null>(null);
  const recordedChunksRef = useRef<Blob[]>([]);
  const recordingTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const { data: convsData, mutate: mutateConvs } = useSWR<{
    conversations: Conversation[];
  }>(`${API}/api/conversations`, fetcher, { refreshInterval: 30000 });

  const conversations = convsData?.conversations ?? [];

  const filtered = conversations.filter((c) => {
    if (filter === "unread" && c.unread_count === 0) return false;
    if (!search) return true;
    return (
      c.contact_address.includes(search) ||
      (c.contact_name &&
        c.contact_name.toLowerCase().includes(search.toLowerCase()))
    );
  });

  const unreadCount = conversations.filter((c) => c.unread_count > 0).length;

  const selectedConvRef = useRef<Conversation | null>(null);
  useEffect(() => {
    selectedConvRef.current = selectedConv;
  }, [selectedConv]);

  // Load messages
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

  // WebSocket
  useEffect(() => {
    const token = localStorage.getItem("access_token");
    let ws: WebSocket | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let closed = false;

    const connect = () => {
      ws = new WebSocket(`${WS_URL}/api/ws?token=${token}`);
      wsRef.current = ws;

      ws.onmessage = (e) => {
        const event = JSON.parse(e.data);
        if (event.type === "new_message") {
          const msg = event.payload as Message;
          const active = selectedConvRef.current;
          if (active && msg.conversation_id === active.id) {
            setMessages((prev) => {
              if (msg.id && prev.some((m) => m.id === msg.id)) return prev;
              return [...prev, msg];
            });
          }
          mutateConvs();
        }
        if (
          event.type === "message_delivered" ||
          event.type === "message_read" ||
          event.type === "message_failed"
        ) {
          setMessages((prev) =>
            prev.map((m) =>
              m.id === event.payload.message_id
                ? {
                    ...m,
                    status: event.payload.status,
                    error_message:
                      event.payload.error_message ?? m.error_message,
                  }
                : m
            )
          );
        }
      };

      ws.onclose = () => {
        if (closed) return;
        reconnectTimer = setTimeout(connect, 2000);
      };
      ws.onerror = () => {
        try {
          ws?.close();
        } catch {}
      };
    };

    connect();
    return () => {
      closed = true;
      if (reconnectTimer) clearTimeout(reconnectTimer);
      try {
        ws?.close();
      } catch {}
    };
  }, [mutateConvs]);

  function authHeaders() {
    return {
      Authorization: `Bearer ${localStorage.getItem("access_token")}`,
    };
  }

  async function uploadFile(file: File): Promise<Attachment> {
    const fd = new FormData();
    fd.append("file", file, file.name);
    const res = await fetch(`${API}/api/messages/upload`, {
      method: "POST",
      headers: authHeaders(),
      body: fd,
    });
    if (!res.ok) {
      const err = await res.json();
      throw new Error(err.error || "Upload failed");
    }
    return res.json();
  }

  const sendMessage = useCallback(async () => {
    if (!selectedConv) return;
    const hasText = messageInput.trim().length > 0;
    const hasFiles = pendingFiles.length > 0;
    if (!hasText && !hasFiles) return;

    setSending(true);
    setSendStatus("");

    try {
      const uploadedAttachments: Attachment[] = [];
      for (let i = 0; i < pendingFiles.length; i++) {
        const pf = pendingFiles[i];
        if (pf.uploaded) {
          uploadedAttachments.push(pf.uploaded);
          continue;
        }
        setSendStatus(`Uploading ${i + 1}/${pendingFiles.length}...`);
        const att = await uploadFile(pf.file);
        uploadedAttachments.push(att);
      }

      setSendStatus("Sending...");
      const res = await fetch(`${API}/api/messages/send`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({
          phone_number_id: selectedConv.phone_number_id,
          to_address: selectedConv.contact_address,
          content: messageInput.trim(),
          attachments:
            uploadedAttachments.length > 0 ? uploadedAttachments : undefined,
          effect: selectedEffect || undefined,
        }),
      });
      const data = await res.json();

      if (data.rate_limited) {
        setSendStatus(
          `Daily limit reached (${data.rate_limit.daily_new_contacts_used} new contacts).`
        );
        return;
      }
      if (!res.ok) {
        setSendStatus(data.error || "Send failed");
        return;
      }
      if (data.message) {
        setMessages((prev) => [...prev, data.message]);
        setMessageInput("");
        setSelectedEffect("");
        pendingFiles.forEach((pf) => URL.revokeObjectURL(pf.previewUrl));
        setPendingFiles([]);
        mutateConvs();
        setSendStatus("");
      }
    } catch (err: unknown) {
      setSendStatus(err instanceof Error ? err.message : "Send failed");
    } finally {
      setSending(false);
    }
  }, [selectedConv, messageInput, pendingFiles, selectedEffect, mutateConvs]);

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  }

  function handleFileSelect(e: React.ChangeEvent<HTMLInputElement>) {
    const files = Array.from(e.target.files || []);
    for (const file of files) {
      if (file.type.startsWith("image/") && file.size > 25 * 1024 * 1024) {
        setSendStatus(`${file.name} exceeds 25MB image limit`);
        continue;
      }
      if (file.type.startsWith("video/") && file.size > 100 * 1024 * 1024) {
        setSendStatus(`${file.name} exceeds 100MB video limit`);
        continue;
      }
      setPendingFiles((prev) => [
        ...prev,
        { file, previewUrl: URL.createObjectURL(file) },
      ]);
    }
    e.target.value = "";
  }

  function removePendingFile(idx: number) {
    setPendingFiles((prev) => {
      URL.revokeObjectURL(prev[idx].previewUrl);
      return prev.filter((_, i) => i !== idx);
    });
  }

  async function toggleRecording() {
    if (recording) {
      mediaRecorderRef.current?.stop();
      setRecording(false);
      if (recordingTimerRef.current) clearInterval(recordingTimerRef.current);
      return;
    }
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      recordedChunksRef.current = [];
      const mr = new MediaRecorder(stream, { mimeType: "audio/webm" });
      mr.ondataavailable = (e) => {
        if (e.data.size > 0) recordedChunksRef.current.push(e.data);
      };
      mr.onstop = () => {
        stream.getTracks().forEach((t) => t.stop());
        const blob = new Blob(recordedChunksRef.current, {
          type: "audio/webm",
        });
        const file = new File([blob], `voice-memo-${Date.now()}.webm`, {
          type: "audio/webm",
        });
        setPendingFiles((prev) => [
          ...prev,
          { file, previewUrl: URL.createObjectURL(file) },
        ]);
        setRecordingTime(0);
      };
      mr.start();
      mediaRecorderRef.current = mr;
      setRecording(true);
      setRecordingTime(0);
      recordingTimerRef.current = setInterval(() => {
        setRecordingTime((t) => t + 1);
      }, 1000);
    } catch {
      setSendStatus("Microphone access denied");
    }
  }

  async function scheduleMessage() {
    if (!selectedConv || (!messageInput.trim() && pendingFiles.length === 0)) return;
    if (!scheduleDate || !scheduleTime) {
      setSendStatus("Pick a date and time to schedule");
      return;
    }
    setSending(true);
    setSendStatus("Scheduling...");
    try {
      const scheduledAt = new Date(`${scheduleDate}T${scheduleTime}`).toISOString();
      const res = await fetch(`${API}/api/messages/schedule`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({
          phone_number_id: selectedConv.phone_number_id,
          to_address: selectedConv.contact_address,
          content: messageInput.trim(),
          effect: selectedEffect || undefined,
          scheduled_at: scheduledAt,
        }),
      });
      const data = await res.json();
      if (!res.ok) {
        setSendStatus(data.error || "Schedule failed");
        return;
      }
      setMessageInput("");
      setSelectedEffect("");
      setShowSchedule(false);
      setScheduleDate("");
      setScheduleTime("");
      setSendStatus("Message scheduled!");
      setTimeout(() => setSendStatus(""), 3000);
    } catch {
      setSendStatus("Schedule failed");
    } finally {
      setSending(false);
    }
  }

  function formatRecTime(s: number) {
    return `${Math.floor(s / 60)}:${(s % 60).toString().padStart(2, "0")}`;
  }

  function copyPhone(phone: string) {
    navigator.clipboard.writeText(phone);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }

  function selectConversation(conv: Conversation) {
    setSelectedConv(conv);
    setMobileShowThread(true);
  }

  // ─── Date separator logic ────────────────────────────────────
  function getDateLabel(ts: string): string {
    const d = new Date(ts);
    const today = new Date();
    const yesterday = new Date(today);
    yesterday.setDate(yesterday.getDate() - 1);

    if (d.toDateString() === today.toDateString()) return "Today";
    if (d.toDateString() === yesterday.toDateString()) return "Yesterday";
    return d.toLocaleDateString([], {
      weekday: "long",
      month: "short",
      day: "numeric",
    });
  }

  return (
    <div className="messages-grid">
      {/* ── Threads pane ── */}
      <section
        className="threads-pane"
        style={mobileShowThread ? { display: "none" } : undefined}
      >
        <div className="head">
          <h2>Conversations</h2>
          <button
            type="button"
            className="compose-btn"
            onClick={() => setShowCompose(true)}
            title="New message"
          >
            <PenSquareIcon style={{ width: 16, height: 16 }} />
          </button>
        </div>
        <div className="search">
          <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.8">
            <circle cx="7" cy="7" r="5" />
            <path d="M14 14l-3-3" strokeLinecap="round" />
          </svg>
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search conversations…"
          />
        </div>
        <div className="filters">
          <button
            type="button"
            className={filter === "all" ? "active" : ""}
            onClick={() => setFilter("all")}
          >
            All
          </button>
          <button
            type="button"
            className={filter === "unread" ? "active" : ""}
            onClick={() => setFilter("unread")}
          >
            Unread {unreadCount > 0 && <span style={{ opacity: 0.7 }}>{unreadCount}</span>}
          </button>
        </div>

        <div className="list">
          {filtered.length === 0 && (
            <div className="empty">{search ? "No results" : "No conversations yet"}</div>
          )}
          {filtered.map((conv) => {
            const active = selectedConv?.id === conv.id;
            const initials = getInitials(conv.contact_name, conv.contact_address);
            const unread = conv.unread_count > 0;
            const avatarCls = active ? "avatar solid" : "avatar";
            return (
              <button
                key={conv.id}
                onClick={() => selectConversation(conv)}
                className={`thread${active ? " selected" : ""}${unread ? " unread" : ""}`}
                type="button"
              >
                <div className={avatarCls}>{initials}</div>
                <div className="body">
                  <div className="name-row">
                    <span className="nm">
                      {conv.contact_name || formatPhoneDisplay(conv.contact_address)}
                    </span>
                    <span className="time">{formatTime(conv.last_message_at)}</span>
                  </div>
                  <div className="preview">
                    {conv.last_message_preview || "No messages yet"}
                  </div>
                </div>
                {unread && <span className="unread-dot" />}
              </button>
            );
          })}
        </div>
      </section>

      {/* ── Conversation pane ── */}
      <section
        className="convo-pane"
        style={!mobileShowThread && !selectedConv ? undefined : !mobileShowThread ? undefined : undefined}
      >
        {!selectedConv ? (
          <div className="empty">
            <div className="big">No conversation selected.</div>
            <div>Pick a thread on the left to start messaging.</div>
          </div>
        ) : (
          <>
            {/* Conversation header */}
            <div className="c-head">
              <button
                onClick={() => setMobileShowThread(false)}
                className="action"
                style={{ display: "none" }}
                title="Back"
              >
                <ChevronLeftIcon style={{ width: 18, height: 18 }} />
              </button>
              <div className="avatar solid" style={{ width: 40, height: 40, fontSize: 13 }}>
                {getInitials(selectedConv.contact_name, selectedConv.contact_address)}
              </div>
              <div className="name-block" style={{ flex: 1, minWidth: 0 }}>
                <div className="nm">
                  {selectedConv.contact_name ||
                    formatPhoneDisplay(selectedConv.contact_address)}
                </div>
                <div className="sub">
                  <span>{formatPhoneDisplay(selectedConv.contact_address)}</span>
                  <span className="dot" />
                  <span className="badge blue">iMESSAGE</span>
                  <span className="dot" />
                  <button
                    onClick={() => copyPhone(selectedConv.contact_address)}
                    title="Copy number"
                    style={{
                      background: "none",
                      border: 0,
                      cursor: "pointer",
                      padding: 0,
                      color: "var(--muted)",
                      display: "inline-flex",
                      alignItems: "center",
                      gap: 4,
                    }}
                  >
                    {copied ? (
                      <CheckIcon style={{ width: 12, height: 12, color: "var(--success)" }} />
                    ) : (
                      <CopyIcon style={{ width: 12, height: 12 }} />
                    )}
                  </button>
                </div>
              </div>
              <span className="spacer" />
              <Link
                href={`/contacts/${selectedConv.contact_id}`}
                className="action"
                title="View contact"
              >
                <UserIcon style={{ width: 18, height: 18 }} />
              </Link>
            </div>

            {/* Body */}
            <div className="c-body">
              {messages.map((msg, idx) => {
                const showDate =
                  idx === 0 ||
                  getDateLabel(msg.created_at) !==
                    getDateLabel(messages[idx - 1].created_at);
                const isOut = msg.direction === "outbound";
                const isFailed = isOut && msg.status === "failed";

                let bubCls = isOut ? "bub out" : "bub in";
                if (isFailed) bubCls = "bub out failed";

                const time = new Date(msg.created_at).toLocaleTimeString([], {
                  hour: "2-digit",
                  minute: "2-digit",
                });
                let metaText = time;
                if (isOut) {
                  if (msg.status === "read") metaText = `${time} · Read`;
                  else if (msg.status === "delivered") metaText = `${time} · Delivered`;
                  else if (msg.status === "sent") metaText = `${time} · Sent`;
                  else if (isFailed) metaText = msg.error_message ? `${time} · ${msg.error_message}` : `${time} · Not delivered`;
                  else metaText = `${time} · Sending…`;
                }

                return (
                  <React.Fragment key={msg.id}>
                    {showDate && <div className="day">{getDateLabel(msg.created_at)}</div>}
                    <div
                      className={bubCls}
                      title={
                        isFailed && msg.error_message
                          ? `Failed: ${msg.error_message}`
                          : undefined
                      }
                    >
                      {msg.attachments && msg.attachments.length > 0 && (
                        <AttachmentRenderer
                          attachments={msg.attachments}
                          direction={msg.direction}
                        />
                      )}
                      {msg.content && (
                        <div
                          style={
                            msg.attachments && msg.attachments.length > 0
                              ? { marginTop: 6 }
                              : undefined
                          }
                        >
                          {msg.content}
                        </div>
                      )}
                      {!msg.content &&
                        (!msg.attachments || msg.attachments.length === 0) && (
                          <span style={{ fontStyle: "italic", opacity: 0.5 }}>empty</span>
                        )}
                    </div>
                    <div className={`bub-meta ${isOut ? "out" : "in"}${isFailed ? " fail" : ""}`}>
                      {metaText}
                    </div>
                  </React.Fragment>
                );
              })}
              <div ref={messagesEndRef} />
            </div>

            {/* Composer */}
            <div className="composer">
              {pendingFiles.length > 0 && (
                <div style={{ marginBottom: 10, display: "flex", gap: 8, flexWrap: "wrap" }}>
                  {pendingFiles.map((pf, i) => (
                    <div key={i} style={{ position: "relative" }}>
                      {pf.file.type.startsWith("image/") ? (
                        <img
                          src={pf.previewUrl}
                          style={{
                            width: 60, height: 60, objectFit: "cover",
                            borderRadius: 8, border: "1px solid var(--rule)",
                          }}
                          alt=""
                        />
                      ) : pf.file.type.startsWith("video/") ? (
                        <div
                          style={{
                            width: 60, height: 60, borderRadius: 8,
                            border: "1px solid var(--rule)", background: "var(--paper-2)",
                            display: "flex", alignItems: "center", justifyContent: "center",
                          }}
                        >
                          <PlayIcon style={{ width: 18, height: 18, color: "var(--muted)" }} />
                        </div>
                      ) : (
                        <div
                          style={{
                            height: 60, padding: "0 12px", borderRadius: 8,
                            border: "1px solid var(--rule)", background: "var(--paper)",
                            display: "flex", alignItems: "center", gap: 6,
                          }}
                        >
                          <MicIcon style={{ width: 14, height: 14, color: "var(--blu)" }} />
                          <span style={{ fontSize: 11, color: "var(--ink-2)", maxWidth: 80, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                            {pf.file.name}
                          </span>
                        </div>
                      )}
                      <button
                        onClick={() => removePendingFile(i)}
                        style={{
                          position: "absolute", top: -6, right: -6,
                          width: 18, height: 18, background: "var(--ink)",
                          color: "#fff", borderRadius: "50%", border: 0,
                          display: "flex", alignItems: "center", justifyContent: "center",
                          cursor: "pointer",
                        }}
                      >
                        <XIcon style={{ width: 10, height: 10 }} />
                      </button>
                    </div>
                  ))}
                </div>
              )}

              {recording && (
                <div style={{ marginBottom: 8, display: "flex", alignItems: "center", gap: 8, fontSize: 13, color: "var(--danger)" }}>
                  <span style={{ width: 10, height: 10, background: "var(--danger)", borderRadius: "50%" }} />
                  Recording {formatRecTime(recordingTime)} · tap mic to stop
                </div>
              )}

              {selectedEffect && (
                <div style={{ marginBottom: 8 }}>
                  <span
                    style={{
                      display: "inline-flex", alignItems: "center", gap: 6,
                      fontSize: 12, fontWeight: 500,
                      background: "#f3e8ff", color: "#7c3aed",
                      padding: "4px 10px", borderRadius: 999,
                    }}
                  >
                    <SparklesIcon style={{ width: 12, height: 12 }} />
                    {IMESSAGE_EFFECTS.find((e) => e.id === selectedEffect)?.emoji}{" "}
                    {IMESSAGE_EFFECTS.find((e) => e.id === selectedEffect)?.label}
                    <button
                      onClick={() => setSelectedEffect("")}
                      style={{ background: "none", border: 0, cursor: "pointer", color: "inherit", padding: 0, display: "inline-flex" }}
                    >
                      <XIcon style={{ width: 12, height: 12 }} />
                    </button>
                  </span>
                </div>
              )}

              {showEffects && (
                <div
                  style={{
                    marginBottom: 8,
                    background: "#fff",
                    border: "1px solid var(--rule)",
                    borderRadius: 12,
                    boxShadow: "var(--shadow-md)",
                    padding: 12,
                  }}
                >
                  <div className="kicker" style={{ marginBottom: 8 }}>iMessage effect</div>
                  <div style={{ display: "grid", gridTemplateColumns: "repeat(6, 1fr)", gap: 6 }}>
                    {IMESSAGE_EFFECTS.map((effect) => (
                      <button
                        key={effect.id}
                        onClick={() => {
                          setSelectedEffect(selectedEffect === effect.id ? "" : effect.id);
                          setShowEffects(false);
                        }}
                        style={{
                          display: "flex", flexDirection: "column", alignItems: "center", gap: 2,
                          padding: "8px 4px",
                          borderRadius: 8, fontSize: 11,
                          background: selectedEffect === effect.id ? "#f3e8ff" : "transparent",
                          color: selectedEffect === effect.id ? "#7c3aed" : "var(--ink-2)",
                          border: 0, cursor: "pointer",
                        }}
                      >
                        <span style={{ fontSize: 18 }}>{effect.emoji}</span>
                        <span style={{ fontSize: 10 }}>{effect.label}</span>
                      </button>
                    ))}
                  </div>
                </div>
              )}

              {showSchedule && (
                <div
                  style={{
                    marginBottom: 8,
                    background: "#fff",
                    border: "1px solid var(--rule)",
                    borderRadius: 12,
                    boxShadow: "var(--shadow-md)",
                    padding: 14,
                  }}
                >
                  <div className="kicker" style={{ marginBottom: 10 }}>Schedule message</div>
                  <div style={{ display: "flex", gap: 10, alignItems: "flex-end" }}>
                    <div style={{ flex: 1 }}>
                      <label style={{ display: "block", fontSize: 11, color: "var(--muted)", marginBottom: 4 }}>Date</label>
                      <input
                        className="input"
                        type="date"
                        value={scheduleDate}
                        onChange={(e) => setScheduleDate(e.target.value)}
                        min={new Date().toISOString().split("T")[0]}
                      />
                    </div>
                    <div style={{ flex: 1 }}>
                      <label style={{ display: "block", fontSize: 11, color: "var(--muted)", marginBottom: 4 }}>Time</label>
                      <input
                        className="input"
                        type="time"
                        value={scheduleTime}
                        onChange={(e) => setScheduleTime(e.target.value)}
                      />
                    </div>
                    <button
                      className="btn primary"
                      onClick={scheduleMessage}
                      disabled={sending || !scheduleDate || !scheduleTime || (!messageInput.trim() && pendingFiles.length === 0)}
                    >
                      Schedule
                    </button>
                  </div>
                </div>
              )}

              {sendStatus && (
                <div style={{ marginBottom: 8, fontSize: 12, color: "var(--warn)" }}>{sendStatus}</div>
              )}

              <div className="composer-row">
                <input
                  ref={fileInputRef}
                  type="file"
                  multiple
                  accept="image/*,video/*"
                  style={{ display: "none" }}
                  onChange={handleFileSelect}
                />
                <button
                  type="button"
                  className="tool"
                  onClick={() => fileInputRef.current?.click()}
                  title="Attach image or video"
                >
                  <ImageIcon style={{ width: 17, height: 17 }} />
                </button>
                <button
                  type="button"
                  className="tool"
                  onClick={toggleRecording}
                  title={recording ? "Stop recording" : "Record voice memo"}
                  style={recording ? { color: "var(--danger)" } : undefined}
                >
                  <MicIcon style={{ width: 17, height: 17 }} />
                </button>
                <button
                  type="button"
                  className="tool"
                  onClick={() => {
                    setShowEffects(!showEffects);
                    setShowSchedule(false);
                  }}
                  title="iMessage effects"
                  style={
                    showEffects || selectedEffect
                      ? { color: "#7c3aed" }
                      : undefined
                  }
                >
                  <SparklesIcon style={{ width: 17, height: 17 }} />
                </button>
                <button
                  type="button"
                  className="tool"
                  onClick={() => {
                    setShowSchedule(!showSchedule);
                    setShowEffects(false);
                  }}
                  title="Schedule"
                  style={showSchedule ? { color: "var(--blu)" } : undefined}
                >
                  <ClockIcon style={{ width: 17, height: 17 }} />
                </button>
                <textarea
                  value={messageInput}
                  onChange={(e) => setMessageInput(e.target.value)}
                  onKeyDown={handleKeyDown}
                  placeholder="Type a message…"
                  rows={1}
                  disabled={sending}
                />
                <button
                  type="button"
                  className="send"
                  onClick={sendMessage}
                  disabled={
                    sending || (!messageInput.trim() && pendingFiles.length === 0)
                  }
                  title="Send"
                >
                  <SendIcon style={{ width: 14, height: 14 }} />
                </button>
              </div>

              <div className="composer-foot">
                <div className="left">
                  {selectedConv && (
                    <span className="imk">
                      <span className="blu-dot" />
                      Sending as iMessage
                    </span>
                  )}
                </div>
                <span>↵ to send · ⇧↵ for new line</span>
              </div>
            </div>
          </>
        )}
      </section>

      {showCompose && (
        <NewMessageDialog
          onClose={() => setShowCompose(false)}
          onSent={async (toAddress, phoneNumberID) => {
            // Refresh conversations and select the newly created one once it appears.
            const res = await mutateConvs();
            const target = res?.conversations.find(
              (c) =>
                c.contact_address === toAddress &&
                c.phone_number_id === phoneNumberID
            );
            if (target) {
              selectConversation(target);
            }
            setShowCompose(false);
          }}
        />
      )}
    </div>
  );
}

// ─── New Message Dialog (Compose) ───
// Sends the first message to a new (or existing) recipient. The send hits
// /api/messages/send which transparently upserts the contact and creates the
// conversation, so we don't need a separate "create conversation" call.

interface PhoneNumber {
  id: string;
  number: string;
  imessage_address: string;
  status: string;
}

interface ContactSearchHit {
  id: string;
  imessage_address: string;
  name: string | null;
}

function NewMessageDialog({
  onClose,
  onSent,
}: {
  onClose: () => void;
  onSent: (toAddress: string, phoneNumberID: string) => void;
}) {
  const [recipient, setRecipient] = useState("");
  const [showResults, setShowResults] = useState(false);
  const [phoneNumberID, setPhoneNumberID] = useState("");
  const [content, setContent] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  const { data: pnData } = useSWR<{ phone_numbers: PhoneNumber[] }>(
    `${API}/api/phone-numbers`,
    fetcher
  );
  const phoneNumbers = (pnData?.phone_numbers ?? []).filter(
    (p) => p.status === "active"
  );

  // Auto-select the first active phone number.
  useEffect(() => {
    if (!phoneNumberID && phoneNumbers.length > 0) {
      setPhoneNumberID(phoneNumbers[0].id);
    }
  }, [phoneNumbers, phoneNumberID]);

  // Live contact search as the user types in the recipient field.
  const debouncedQ = recipient.trim();
  const { data: searchData } = useSWR<{ contacts: ContactSearchHit[] }>(
    debouncedQ.length >= 2
      ? `${API}/api/contacts?search=${encodeURIComponent(debouncedQ)}&limit=5`
      : null,
    fetcher
  );
  const matches = searchData?.contacts ?? [];

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    if (!recipient.trim()) {
      setError("Please enter a phone number, email, or pick a contact");
      return;
    }
    if (!content.trim()) {
      setError("Message can't be empty");
      return;
    }
    if (!phoneNumberID) {
      setError("Pick a phone number to send from");
      return;
    }

    setSubmitting(true);
    try {
      const res = await fetch(`${API}/api/messages/send`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${localStorage.getItem("access_token")}`,
        },
        body: JSON.stringify({
          phone_number_id: phoneNumberID,
          to_address: recipient.trim(),
          content: content.trim(),
        }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error || "Failed to send message");
        setSubmitting(false);
        return;
      }
      if (data.rate_limited) {
        setError(
          "Daily new-contact limit reached. Try again tomorrow or message an existing contact."
        );
        setSubmitting(false);
        return;
      }
      onSent(recipient.trim(), phoneNumberID);
    } catch {
      setError("Network error. Please try again.");
      setSubmitting(false);
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 bg-black/30 flex items-center justify-center p-4"
      onClick={onClose}
    >
      <div
        className="bg-white rounded-2xl shadow-xl w-full max-w-md max-h-[90vh] overflow-auto"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-100">
          <h2 className="text-lg font-semibold text-gray-900">New message</h2>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-600"
          >
            <XIcon className="w-5 h-5" />
          </button>
        </div>
        <form onSubmit={submit} className="p-6 space-y-4">
          {phoneNumbers.length === 0 && (
            <div className="text-sm text-amber-700 bg-amber-50 border border-amber-100 rounded-xl px-4 py-2.5">
              No active phone number on your account. Contact support to get set up.
            </div>
          )}
          {phoneNumbers.length > 1 && (
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1.5">
                From
              </label>
              <select
                value={phoneNumberID}
                onChange={(e) => setPhoneNumberID(e.target.value)}
                className="w-full px-3 py-2.5 text-sm border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#007AFF] bg-white"
              >
                {phoneNumbers.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.number}
                  </option>
                ))}
              </select>
            </div>
          )}
          <div className="relative">
            <label className="block text-xs font-medium text-gray-500 mb-1.5">
              To
            </label>
            <input
              type="text"
              value={recipient}
              onChange={(e) => {
                setRecipient(e.target.value);
                setShowResults(true);
              }}
              onFocus={() => setShowResults(true)}
              placeholder="Phone, email, or contact name"
              required
              autoFocus
              className="w-full px-3 py-2.5 text-sm border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#007AFF]"
            />
            {showResults && matches.length > 0 && (
              <div className="absolute left-0 right-0 mt-1 bg-white border border-gray-200 rounded-xl shadow-lg max-h-48 overflow-auto z-10">
                {matches.map((c) => (
                  <button
                    key={c.id}
                    type="button"
                    onClick={() => {
                      setRecipient(c.imessage_address);
                      setShowResults(false);
                    }}
                    className="w-full text-left px-3 py-2 text-sm hover:bg-gray-50 flex items-center justify-between gap-2"
                  >
                    <span className="font-medium text-gray-900 truncate">
                      {c.name || c.imessage_address}
                    </span>
                    {c.name && (
                      <span className="text-xs text-gray-400 truncate">
                        {c.imessage_address}
                      </span>
                    )}
                  </button>
                ))}
              </div>
            )}
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-500 mb-1.5">
              Message
            </label>
            <textarea
              value={content}
              onChange={(e) => setContent(e.target.value)}
              placeholder="Type your message..."
              rows={4}
              required
              className="w-full px-3 py-2.5 text-sm border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#007AFF] resize-none"
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
              disabled={submitting || phoneNumbers.length === 0}
              className="inline-flex items-center gap-1.5 bg-[#007AFF] text-white text-sm font-semibold px-4 py-2 rounded-xl hover:bg-blue-600 disabled:opacity-50"
            >
              {submitting && <LoaderIcon className="w-3.5 h-3.5 animate-spin" />}
              {submitting ? "Sending..." : "Send"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
