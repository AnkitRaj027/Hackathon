// AIAssistantDrawer Component
// Minimalist, infrastructure-grade AI Copilot for Vault distributed storage operations.
// Supports natural language diagnosis, status queries, and human-in-the-loop approval workflows.

import React, { useState, useEffect, useRef } from 'react';
import { BaseColors, StateColors } from '../design/tokens';
import { FontFamily, FontSize } from '../design/typography';
import { Bot, Send, X, ShieldAlert, Check, RefreshCw } from 'lucide-react';

interface AIAssistantDrawerProps {
  isOpen: boolean;
  onClose: () => void;
  onActionExecuted?: () => void;
}

interface Message {
  id: string;
  sender: 'user' | 'agent' | 'system';
  text: string;
  pendingAction?: {
    action_id: string;
    tool: string;
    arguments: Record<string, any>;
    risk_level: string;
    reason: string;
  };
  time: string;
}

export const AIAssistantDrawer: React.FC<AIAssistantDrawerProps> = ({
  isOpen,
  onClose,
  onActionExecuted,
}) => {
  const [messages, setMessages] = useState<Message[]>([
    {
      id: 'welcome',
      sender: 'agent',
      text: 'Vault AI Control Plane ready. Query cluster state, audit integrity, or initiate supervised operations.',
      time: new Date().toLocaleTimeString(),
    },
  ]);
  const [input, setInput] = useState('');
  const [loading, setLoading] = useState(false);
  const [sessionId] = useState(() => 'vault-session-' + Math.random().toString(36).substring(2, 9));
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  if (!isOpen) return null;

  const sendMessage = async (textToSend?: string) => {
    const query = (textToSend || input).trim();
    if (!query || loading) return;

    const userMsg: Message = {
      id: Math.random().toString(36).substring(2, 9),
      sender: 'user',
      text: query,
      time: new Date().toLocaleTimeString(),
    };
    setMessages((prev) => [...prev, userMsg]);
    if (!textToSend) setInput('');
    setLoading(true);

    try {
      // Try coordinator proxy first, then fallback to direct port 8000
      let res: Response | null = null;
      try {
        const r = await fetch('/api/ai/chat', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ message: query, session_id: sessionId }),
        });
        if (r.ok) {
          res = r;
        }
      } catch {
        // network error on proxy
      }

      if (!res) {
        res = await fetch('http://localhost:8000/api/ai/chat', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ message: query, session_id: sessionId }),
        });
      }

      if (!res.ok) {
        const errJson = await res.json().catch(() => ({}));
        throw new Error(errJson.error || errJson.detail || `Server error ${res.status}`);
      }

      const data = await res.json();

      if (data.type === 'answer') {
        setMessages((prev) => [
          ...prev,
          {
            id: Math.random().toString(36).substring(2, 9),
            sender: 'agent',
            text: data.message,
            time: new Date().toLocaleTimeString(),
          },
        ]);
      } else if (data.type === 'approval_required') {
        setMessages((prev) => [
          ...prev,
          {
            id: Math.random().toString(36).substring(2, 9),
            sender: 'agent',
            text: `Action requires operator authorization before execution.`,
            pendingAction: {
              action_id: data.action_id,
              tool: data.tool,
              arguments: data.arguments,
              risk_level: data.risk_level,
              reason: data.reason,
            },
            time: new Date().toLocaleTimeString(),
          },
        ]);
      } else if (data.type === 'error') {
        setMessages((prev) => [
          ...prev,
          {
            id: Math.random().toString(36).substring(2, 9),
            sender: 'system',
            text: data.message || 'Operation error.',
            time: new Date().toLocaleTimeString(),
          },
        ]);
      }
    } catch (err: any) {
      setMessages((prev) => [
        ...prev,
        {
          id: Math.random().toString(36).substring(2, 9),
          sender: 'system',
          text: `AI API Error: ${err.message}. Ensure backend is running on :8000.`,
          time: new Date().toLocaleTimeString(),
        },
      ]);
    } finally {
      setLoading(false);
    }
  };

  const handleApprove = async (actionId: string) => {
    setLoading(true);
    try {
      let res: Response | null = null;
      try {
        const r = await fetch(`/api/ai/approve/${actionId}`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ approved: true }),
        });
        if (r.ok) res = r;
      } catch {
        // network error on proxy
      }

      if (!res) {
        res = await fetch(`http://localhost:8000/api/ai/approve/${actionId}`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ approved: true }),
        });
      }

      const data = await res.json();
      setMessages((prev) => [
        ...prev,
        {
          id: Math.random().toString(36).substring(2, 9),
          sender: 'agent',
          text: `Action [${data.tool}] approved and executed successfully.\nResult: ${JSON.stringify(data.result, null, 2)}`,
          time: new Date().toLocaleTimeString(),
        },
      ]);
      onActionExecuted?.();
    } catch (err: any) {
      setMessages((prev) => [
        ...prev,
        {
          id: Math.random().toString(36).substring(2, 9),
          sender: 'system',
          text: `Approval failed: ${err.message}`,
          time: new Date().toLocaleTimeString(),
        },
      ]);
    } finally {
      setLoading(false);
    }
  };

  const handleReject = async (actionId: string) => {
    setLoading(true);
    try {
      let res: Response | null = null;
      try {
        const r = await fetch(`/api/ai/reject/${actionId}`, { method: 'POST' });
        if (r.ok) res = r;
      } catch {
        // network error on proxy
      }

      if (!res) {
        res = await fetch(`http://localhost:8000/api/ai/reject/${actionId}`, { method: 'POST' });
      }

      setMessages((prev) => [
        ...prev,
        {
          id: Math.random().toString(36).substring(2, 9),
          sender: 'system',
          text: `Action rejected by operator. State remains unchanged.`,
          time: new Date().toLocaleTimeString(),
        },
      ]);
    } catch (err: any) {
      setMessages((prev) => [
        ...prev,
        {
          id: Math.random().toString(36).substring(2, 9),
          sender: 'system',
          text: `Rejection failed: ${err.message}`,
          time: new Date().toLocaleTimeString(),
        },
      ]);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      style={{
        position: 'absolute',
        top: '54px',
        right: '12px',
        bottom: '44px',
        width: '420px',
        background: BaseColors.surface,
        border: `1px solid ${BaseColors.border}`,
        borderRadius: '2px',
        display: 'flex',
        flexDirection: 'column',
        zIndex: 55,
        fontFamily: FontFamily.sans,
        color: BaseColors.textPrimary,
        overflow: 'hidden',
      }}
    >
      {/* Header */}
      <div
        style={{
          height: '42px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '0 12px',
          borderBottom: `1px solid ${BaseColors.border}`,
          background: BaseColors.surfaceElevated,
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <Bot size={16} color={BaseColors.accent} />
          <span style={{ fontSize: FontSize.md, fontWeight: 600, letterSpacing: '0.03em' }}>
            VAULT AI COPILOT
          </span>
          <span
            style={{
              fontSize: FontSize.xxs,
              fontFamily: FontFamily.mono,
              background: 'rgba(56, 189, 248, 0.1)',
              color: BaseColors.accent,
              padding: '2px 6px',
              borderRadius: '2px',
            }}
          >
            ACTIVE
          </span>
        </div>
        <button
          onClick={onClose}
          style={{ background: 'transparent', border: 'none', color: BaseColors.textMuted, cursor: 'pointer', padding: '2px' }}
        >
          <X size={16} />
        </button>
      </div>

      {/* Quick Prompts */}
      <div
        style={{
          display: 'flex',
          gap: '6px',
          padding: '8px 12px',
          borderBottom: `1px solid ${BaseColors.border}`,
          background: BaseColors.bg,
          overflowX: 'auto',
        }}
      >
        {['Show node health', 'Audit object integrity', 'List objects'].map((prompt) => (
          <button
            key={prompt}
            onClick={() => sendMessage(prompt)}
            disabled={loading}
            style={{
              padding: '4px 10px',
              background: BaseColors.surface,
              border: `1px solid ${BaseColors.border}`,
              borderRadius: '2px',
              color: BaseColors.textSecondary,
              fontSize: FontSize.xs,
              whiteSpace: 'nowrap',
              cursor: 'pointer',
              fontFamily: FontFamily.mono,
            }}
          >
            {prompt}
          </button>
        ))}
      </div>

      {/* Message Stream */}
      <div style={{ flex: 1, overflowY: 'auto', padding: '12px', display: 'flex', flexDirection: 'column', gap: '10px' }}>
        {messages.map((m) => (
          <div
            key={m.id}
            style={{
              alignSelf: m.sender === 'user' ? 'flex-end' : 'flex-start',
              maxWidth: '92%',
              background: m.sender === 'user' ? BaseColors.surfaceElevated : BaseColors.bg,
              border: `1px solid ${BaseColors.border}`,
              borderRadius: '2px',
              padding: '10px 12px',
              fontSize: FontSize.md,
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', gap: '8px', marginBottom: '5px', fontSize: FontSize.xxs, color: BaseColors.textMuted }}>
              <span style={{ fontWeight: 600, color: m.sender === 'user' ? BaseColors.accent : BaseColors.textSecondary }}>
                {m.sender.toUpperCase()}
              </span>
              <span>{m.time}</span>
            </div>

            <div style={{ whiteSpace: 'pre-wrap', lineHeight: 1.55, fontFamily: m.sender === 'agent' ? FontFamily.sans : FontFamily.mono }}>
              {m.text}
            </div>

            {/* Pending Action Approval Card */}
            {m.pendingAction && (
              <div
                style={{
                  marginTop: '10px',
                  padding: '10px',
                  background: BaseColors.surface,
                  border: `1px solid ${StateColors.SUSPECT}`,
                  borderRadius: '2px',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: '6px', color: StateColors.SUSPECT, fontWeight: 600, fontSize: FontSize.sm, marginBottom: '4px' }}>
                  <ShieldAlert size={14} />
                  <span>OPERATOR APPROVAL REQUIRED</span>
                </div>
                <div style={{ fontSize: FontSize.xs, color: BaseColors.textMuted, marginBottom: '8px' }}>
                  {m.pendingAction.reason}
                </div>
                <div
                  style={{
                    background: BaseColors.bg,
                    padding: '6px 8px',
                    borderRadius: '2px',
                    fontFamily: FontFamily.mono,
                    fontSize: FontSize.xs,
                    marginBottom: '10px',
                    color: BaseColors.textSecondary,
                  }}
                >
                  {m.pendingAction.tool}({JSON.stringify(m.pendingAction.arguments)})
                </div>
                <div style={{ display: 'flex', gap: '8px' }}>
                  <button
                    onClick={() => handleApprove(m.pendingAction!.action_id)}
                    disabled={loading}
                    style={{
                      flex: 1,
                      padding: '6px 12px',
                      background: StateColors.HEALTHY,
                      border: 'none',
                      color: '#000',
                      borderRadius: '2px',
                      fontWeight: 600,
                      cursor: 'pointer',
                      fontSize: FontSize.xs,
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      gap: '4px',
                    }}
                  >
                    <Check size={12} /> Approve
                  </button>
                  <button
                    onClick={() => handleReject(m.pendingAction!.action_id)}
                    disabled={loading}
                    style={{
                      flex: 1,
                      padding: '6px 12px',
                      background: 'transparent',
                      border: `1px solid ${BaseColors.border}`,
                      color: BaseColors.textSecondary,
                      borderRadius: '2px',
                      cursor: 'pointer',
                      fontSize: FontSize.xs,
                    }}
                  >
                    Reject
                  </button>
                </div>
              </div>
            )}
          </div>
        ))}
        {loading && (
          <div style={{ alignSelf: 'flex-start', color: BaseColors.textMuted, fontSize: FontSize.xs, fontFamily: FontFamily.mono, display: 'flex', alignItems: 'center', gap: '6px' }}>
            <RefreshCw size={12} className="spin" /> Consulting Vault AI model...
          </div>
        )}
        <div ref={messagesEndRef} />
      </div>

      {/* Input Box */}
      <form
        onSubmit={(e) => {
          e.preventDefault();
          sendMessage();
        }}
        style={{
          display: 'flex',
          borderTop: `1px solid ${BaseColors.border}`,
          padding: '8px 10px',
          background: BaseColors.surfaceElevated,
          gap: '8px',
        }}
      >
        <input
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="Ask Vault AI or give operational commands..."
          disabled={loading}
          style={{
            flex: 1,
            padding: '8px 12px',
            background: BaseColors.bg,
            border: `1px solid ${BaseColors.border}`,
            borderRadius: '2px',
            color: BaseColors.textPrimary,
            fontSize: FontSize.md,
            fontFamily: FontFamily.sans,
            outline: 'none',
          }}
        />
        <button
          type="submit"
          disabled={loading || !input.trim()}
          style={{
            padding: '8px 14px',
            background: BaseColors.accent,
            border: 'none',
            color: '#000000',
            borderRadius: '2px',
            cursor: 'pointer',
          }}
        >
          <Send size={14} />
        </button>
      </form>
    </div>
  );
};
