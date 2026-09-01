import { useEffect, useMemo, useRef, useState } from 'react';
import {
  Button,
  Card,
  Empty,
  Input,
  List,
  Select,
  Space,
  Spin,
  Tag,
  Typography,
  message as antdMessage,
} from 'antd';

import accountService from '@/api/services/accountService';
import chatgptService, {
  ChatGPTModel,
  Conversation,
  ConversationMessage,
  ConversationSummary,
  StreamEvent,
} from '@/api/services/chatgptService';
import { Account } from '#/entity';

const { Text, Title, Paragraph } = Typography;
const { TextArea } = Input;

function ChatPage() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [accountId, setAccountId] = useState<number>();
  const [models, setModels] = useState<ChatGPTModel[]>([]);
  const [model, setModel] = useState('auto');
  const [conversations, setConversations] = useState<ConversationSummary[]>([]);
  const [conversation, setConversation] = useState<Conversation>();
  const [draft, setDraft] = useState('');
  const [loading, setLoading] = useState(false);
  const [historyLoading, setHistoryLoading] = useState(false);
  const abortRef = useRef<AbortController>();

  const selectedAccount = useMemo(
    () => accounts.find((item) => item.id === accountId),
    [accounts, accountId],
  );

  useEffect(() => {
    accountService
      .searchAccountList('', 'chatgpt')
      .then((items) => {
        setAccounts(items);
        if (items.length > 0) setAccountId(items[0].id);
      })
      .catch(() => setAccounts([]));
  }, []);

  useEffect(() => {
    if (!accountId) return;
    setConversation(undefined);
    setModels([]);
    loadAccount(accountId);
  }, [accountId]);

  const loadAccount = async (id: number) => {
    setHistoryLoading(true);
    try {
      const [availableModels, page] = await Promise.all([
        chatgptService.models(id),
        chatgptService.listConversations(id),
      ]);
      setModels(availableModels);
      setConversations(page.items);
      const preferred = availableModels.find((item) => item.slug === 'auto' || item.id === 'auto');
      if (preferred) setModel(preferred.slug || preferred.id);
      else if (availableModels.length > 0) setModel(availableModels[0].slug || availableModels[0].id);
    } finally {
      setHistoryLoading(false);
    }
  };

  const openConversation = async (item: ConversationSummary) => {
    if (!accountId) return;
    setLoading(true);
    try {
      setConversation(await chatgptService.getConversation(accountId, item.id));
    } finally {
      setLoading(false);
    }
  };

  const newConversation = () => {
    abortRef.current?.abort();
    setConversation(undefined);
    setDraft('');
  };

  const refreshHistory = async () => {
    if (!accountId) return;
    const page = await chatgptService.listConversations(accountId);
    setConversations(page.items);
  };

  const send = async () => {
    if (!accountId || !draft.trim() || loading) return;

    const content = draft.trim();
    setDraft('');
    setLoading(true);

    const existing = conversation;
    const userMessage: ConversationMessage = {
      id: `local-user-${Date.now()}`,
      role: 'user',
      content,
      createdAt: new Date().toISOString(),
    };
    const assistantMessage: ConversationMessage = {
      id: `local-assistant-${Date.now()}`,
      role: 'assistant',
      content: '',
      createdAt: new Date().toISOString(),
    };

    const working: Conversation = existing
      ? { ...existing, messages: [...existing.messages, userMessage, assistantMessage] }
      : {
          id: '',
          title: 'New chat',
          messages: [userMessage, assistantMessage],
          archived: false,
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        };
    setConversation(working);

    const controller = new AbortController();
    abortRef.current = controller;
    let upstreamConversationId = existing?.id || '';
    let assistantContent = '';

    const onEvent = (event: StreamEvent) => {
      if (event.conversationId) upstreamConversationId = event.conversationId;
      if (event.type === 'message_delta' && event.delta) assistantContent += event.delta;
      if (event.type === 'message_completed' && event.message?.content) assistantContent = event.message.content;

      setConversation((current) => {
        if (!current) return current;
        const messages = [...current.messages];
        const last = messages[messages.length - 1];
        if (last?.role === 'assistant') {
          messages[messages.length - 1] = {
            ...last,
            id: event.messageId || event.message?.id || last.id,
            content: assistantContent,
          };
        }
        return { ...current, id: upstreamConversationId || current.id, messages };
      });
    };

    try {
      const parentMessageId = existing?.messages?.[existing.messages.length - 1]?.id;
      await chatgptService.streamConversation(
        {
          accountId,
          conversationId: existing?.id || undefined,
          model,
          parentMessageId,
          message: content,
        },
        onEvent,
        controller.signal,
      );
      await refreshHistory();
      if (upstreamConversationId) {
        setConversation(await chatgptService.getConversation(accountId, upstreamConversationId));
      }
    } catch (error) {
      if (!controller.signal.aborted) {
        antdMessage.error(error instanceof Error ? error.message : 'ChatGPT stream failed');
      }
    } finally {
      setLoading(false);
      if (abortRef.current === controller) abortRef.current = undefined;
    }
  };

  const deleteCurrent = async () => {
    if (!accountId || !conversation?.id) return;
    await chatgptService.deleteConversation(accountId, conversation.id);
    setConversation(undefined);
    await refreshHistory();
  };

  return (
    <div style={{ height: 'calc(100vh - 112px)', minHeight: 620 }}>
      <Card
        styles={{ body: { padding: 0, height: '100%' } }}
        style={{ height: '100%', overflow: 'hidden' }}
      >
        <div style={{ display: 'grid', gridTemplateColumns: '300px minmax(0, 1fr)', height: '100%' }}>
          <aside style={{ borderRight: '1px solid rgba(128,128,128,.2)', padding: 16, overflow: 'auto' }}>
            <Title level={4} style={{ marginTop: 0 }}>GPT Mirror</Title>
            <Text type="secondary">Official cloud conversation IDs and history</Text>

            <div style={{ marginTop: 16 }}>
              <Text strong>Account</Text>
              <Select
                style={{ width: '100%', marginTop: 6 }}
                value={accountId}
                placeholder="Add a ChatGPT account first"
                onChange={setAccountId}
                options={accounts.map((item) => ({
                  value: item.id,
                  label: item.email || `Account #${item.id}`,
                }))}
              />
              {selectedAccount && (
                <Space size={6} style={{ marginTop: 8 }}>
                  <Tag>{selectedAccount.credentialState || 'unknown'}</Tag>
                  {selectedAccount.proxyConfigured && <Tag>proxy</Tag>}
                </Space>
              )}
            </div>

            <div style={{ marginTop: 14 }}>
              <Text strong>Model</Text>
              <Select
                showSearch
                style={{ width: '100%', marginTop: 6 }}
                value={model}
                onChange={setModel}
                options={models.map((item) => ({
                  value: item.slug || item.id,
                  label: item.displayName || item.slug || item.id,
                }))}
              />
            </div>

            <Button type="primary" block style={{ marginTop: 16 }} onClick={newConversation}>
              New chat
            </Button>

            <div style={{ marginTop: 18, marginBottom: 8, display: 'flex', justifyContent: 'space-between' }}>
              <Text strong>Cloud history</Text>
              <Button size="small" type="text" onClick={refreshHistory}>Refresh</Button>
            </div>

            <Spin spinning={historyLoading}>
              <List
                size="small"
                dataSource={conversations}
                locale={{ emptyText: 'No conversations' }}
                renderItem={(item) => (
                  <List.Item
                    onClick={() => openConversation(item)}
                    style={{ cursor: 'pointer', paddingInline: 8, borderRadius: 6 }}
                  >
                    <div style={{ minWidth: 0, width: '100%' }}>
                      <Text ellipsis style={{ display: 'block' }}>{item.title || 'Untitled'}</Text>
                      <Text type="secondary" style={{ fontSize: 11 }}>{item.id.slice(0, 12)}</Text>
                    </div>
                  </List.Item>
                )}
              />
            </Spin>
          </aside>

          <main style={{ display: 'grid', gridTemplateRows: 'auto minmax(0, 1fr) auto', minWidth: 0 }}>
            <div style={{ padding: '12px 18px', borderBottom: '1px solid rgba(128,128,128,.2)', display: 'flex', justifyContent: 'space-between' }}>
              <div style={{ minWidth: 0 }}>
                <Text strong>{conversation?.title || 'New chat'}</Text>
                {conversation?.id && <Text type="secondary" style={{ marginLeft: 10, fontSize: 11 }}>{conversation.id}</Text>}
              </div>
              {conversation?.id && <Button danger type="text" onClick={deleteCurrent}>Delete</Button>}
            </div>

            <div style={{ overflow: 'auto', padding: '24px clamp(18px, 8vw, 120px)' }}>
              {!conversation?.messages?.length ? (
                <Empty description="Select a cloud conversation or start a new chat" />
              ) : (
                <Space direction="vertical" size={18} style={{ width: '100%' }}>
                  {conversation.messages.map((item, index) => (
                    <div key={`${item.id}-${index}`} style={{ maxWidth: 900 }}>
                      <Text strong>{item.role === 'assistant' ? 'ChatGPT' : item.role}</Text>
                      <Paragraph style={{ whiteSpace: 'pre-wrap', marginTop: 6, marginBottom: 0 }}>
                        {item.content || (item.role === 'assistant' && loading ? '…' : '')}
                      </Paragraph>
                    </div>
                  ))}
                </Space>
              )}
            </div>

            <div style={{ padding: '14px clamp(18px, 8vw, 120px) 18px', borderTop: '1px solid rgba(128,128,128,.2)' }}>
              <Space.Compact style={{ width: '100%', alignItems: 'flex-end' }}>
                <TextArea
                  autoSize={{ minRows: 2, maxRows: 8 }}
                  value={draft}
                  disabled={!accountId}
                  placeholder={accountId ? 'Message ChatGPT…' : 'Add/select a ChatGPT account first'}
                  onChange={(event) => setDraft(event.target.value)}
                  onPressEnter={(event) => {
                    if (!event.shiftKey) {
                      event.preventDefault();
                      send();
                    }
                  }}
                />
                <Button type="primary" loading={loading} disabled={!accountId || !draft.trim()} onClick={send}>
                  Send
                </Button>
              </Space.Compact>
              <Text type="secondary" style={{ display: 'block', marginTop: 8, fontSize: 11 }}>
                Messages are written to the selected ChatGPT account. Browser-challenged sessions may require a future browser-backed adapter.
              </Text>
            </div>
          </main>
        </div>
      </Card>
    </div>
  );
}

export default ChatPage;
