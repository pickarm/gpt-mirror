import apiClient from '../apiClient';
import { StorageEnum } from '#/enum';
import { UserToken } from '#/entity';
import { getItem } from '@/utils/storage';

export interface ChatGPTModel {
  id: string;
  slug: string;
  displayName: string;
  capabilities?: string[];
}

export interface ConversationSummary {
  id: string;
  title: string;
  archived: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface ConversationMessage {
  id: string;
  parentId?: string;
  role: string;
  content: string;
  createdAt: string;
}

export interface Conversation {
  id: string;
  title: string;
  messages: ConversationMessage[];
  archived: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface ConversationPage {
  items: ConversationSummary[];
  nextCursor?: string;
  hasMore: boolean;
}

export interface StreamEvent {
  type: 'conversation' | 'message_delta' | 'message_completed' | 'done' | string;
  conversationId?: string;
  messageId?: string;
  delta?: string;
  message?: ConversationMessage;
}

interface SendRequest {
  accountId: number;
  conversationId?: string;
  model?: string;
  parentMessageId?: string;
  message: string;
  temporary?: boolean;
}

const health = (accountId: number) =>
  apiClient.post<{ state: string; label: string; checkedAt: string }>({
    url: '/chatgpt/health',
    data: { accountId },
  });

const models = (accountId: number) =>
  apiClient.post<ChatGPTModel[]>({ url: '/chatgpt/models', data: { accountId } });

const listConversations = (accountId: number, cursor = '', limit = 50) =>
  apiClient.post<ConversationPage>({
    url: '/chatgpt/conversations/list',
    data: { accountId, cursor, limit },
  });

const getConversation = (accountId: number, conversationId: string) =>
  apiClient.post<Conversation>({
    url: '/chatgpt/conversations/get',
    data: { accountId, conversationId },
  });

const renameConversation = (accountId: number, conversationId: string, title: string) =>
  apiClient.post({
    url: '/chatgpt/conversations/rename',
    data: { accountId, conversationId, title },
  });

const archiveConversation = (accountId: number, conversationId: string, archived: boolean) =>
  apiClient.post({
    url: '/chatgpt/conversations/archive',
    data: { accountId, conversationId, archived },
  });

const deleteConversation = (accountId: number, conversationId: string) =>
  apiClient.post({
    url: '/chatgpt/conversations/delete',
    data: { accountId, conversationId },
  });

async function streamConversation(
  request: SendRequest,
  onEvent: (event: StreamEvent) => void,
  signal?: AbortSignal,
) {
  const token = getItem<UserToken>(StorageEnum.Token);
  const baseURL = (import.meta.env.VITE_APP_BASE_API as string) || '/api';
  const path = request.conversationId ? '/chatgpt/conversations/continue' : '/chatgpt/conversations/create';
  const response = await fetch(`${baseURL}${path}`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json;charset=utf-8',
      Authorization: `Bearer ${token?.accessToken || ''}`,
    },
    body: JSON.stringify(request),
    signal,
  });

  if (!response.ok) {
    let message = `ChatGPT request failed (${response.status})`;
    try {
      const payload = await response.json();
      if (payload?.message) message = payload.message;
    } catch {
      // Keep the status-based message when the response is not JSON.
    }
    throw new Error(message);
  }

  if (!response.body) throw new Error('Streaming response body is unavailable');

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';

  while (true) {
    const { value, done } = await reader.read();
    buffer += decoder.decode(value || new Uint8Array(), { stream: !done });

    let boundary = buffer.indexOf('\n\n');
    while (boundary >= 0) {
      const frame = buffer.slice(0, boundary);
      buffer = buffer.slice(boundary + 2);
      const lines = frame.split('\n');
      const eventName = lines.find((line) => line.startsWith('event:'))?.slice(6).trim();
      const data = lines
        .filter((line) => line.startsWith('data:'))
        .map((line) => line.slice(5).trimStart())
        .join('\n');

      if (eventName === 'error') {
        try {
          const payload = JSON.parse(data);
          throw new Error(payload?.message || 'ChatGPT stream failed');
        } catch (error) {
          if (error instanceof Error) throw error;
          throw new Error('ChatGPT stream failed');
        }
      }
      if (data) onEvent(JSON.parse(data) as StreamEvent);
      boundary = buffer.indexOf('\n\n');
    }

    if (done) break;
  }
}

export default {
  health,
  models,
  listConversations,
  getConversation,
  renameConversation,
  archiveConversation,
  deleteConversation,
  streamConversation,
};
