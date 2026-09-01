import fs from 'node:fs';
import http from 'node:http';
import path from 'node:path';
import { chromium } from 'playwright';

const SOCKET_PATH = process.env.BROWSER_SOCKET_PATH || '/run/gpt-mirror/browser.sock';
const NAVIGATION_TIMEOUT_MS = Number(process.env.BROWSER_NAVIGATION_TIMEOUT_MS || 90_000);
const SEND_TIMEOUT_MS = Number(process.env.BROWSER_SEND_TIMEOUT_MS || 1_200_000);
const MAX_BODY_BYTES = 2 * 1024 * 1024;
const POLL_MS = 500;
const STABLE_POLLS_REQUIRED = 10;

const COMPOSER = '#prompt-textarea';
const STOP_BUTTON = '[data-testid="stop-button"]';
const COPY_BUTTON = 'button[data-testid="copy-turn-action-button"]';
const ASSISTANT_TURN = [
  '[data-testid^="conversation-turn-"][data-turn="assistant"]',
  '[data-testid^="conversation-turn-"][data-message-author-role="assistant"]',
  '[data-testid^="conversation-turn-"]:has([data-message-author-role="assistant"])',
].join(', ');

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function sendLine(res, payload) {
  if (res.destroyed || res.writableEnded) return;
  res.write(`${JSON.stringify(payload)}\n`);
}

function sanitizeError(error) {
  const raw = error instanceof Error ? error.message : String(error || 'browser worker failed');
  return raw
    .replace(/(https?:\/\/)([^/@\s:]+):([^/@\s]+)@/gi, '$1***:***@')
    .replace(/[\r\n\t]+/g, ' ')
    .slice(0, 1000);
}

function classifyError(error, currentURL = '') {
  const message = sanitizeError(error).toLowerCase();
  if (/\/auth\b|\/login\b/.test(currentURL) || message.includes('not authenticated') || message.includes('sign in')) {
    return 'auth';
  }
  if (message.includes('timeout')) return 'unavailable';
  if (message.includes('proxy')) return 'transport';
  return 'unavailable';
}

function parseCookieHeader(header) {
  const cookies = [];
  for (const rawPart of String(header || '').split(';')) {
    const part = rawPart.trim();
    if (!part) continue;
    const separator = part.indexOf('=');
    if (separator <= 0) continue;
    const name = part.slice(0, separator).trim();
    const value = part.slice(separator + 1);
    if (!name) continue;
    cookies.push({ name, value, url: 'https://chatgpt.com' });
  }
  return cookies;
}

function parseProxy(proxyURL) {
  if (!proxyURL) return undefined;
  const parsed = new URL(proxyURL);
  let protocol = parsed.protocol.toLowerCase();
  if (protocol === 'socks5h:') protocol = 'socks5:';
  if (!['http:', 'https:', 'socks5:'].includes(protocol)) {
    throw new Error(`unsupported browser proxy scheme: ${parsed.protocol.replace(':', '')}`);
  }
  if (!parsed.hostname || !parsed.port) throw new Error('browser proxy requires host and port');

  const proxy = {
    server: `${protocol}//${parsed.hostname}:${parsed.port}`,
  };
  if (parsed.username) proxy.username = decodeURIComponent(parsed.username);
  if (parsed.password) proxy.password = decodeURIComponent(parsed.password);
  return proxy;
}

async function readJSON(req) {
  const chunks = [];
  let size = 0;
  for await (const chunk of req) {
    size += chunk.length;
    if (size > MAX_BODY_BYTES) throw new Error('request body exceeds 2 MiB');
    chunks.push(chunk);
  }
  const raw = Buffer.concat(chunks).toString('utf8');
  return JSON.parse(raw || '{}');
}

function conversationIDFromURL(rawURL) {
  try {
    const parsed = new URL(rawURL);
    const match = parsed.pathname.match(/^\/c\/([^/]+)$/);
    return match ? decodeURIComponent(match[1]) : '';
  } catch {
    return '';
  }
}

async function lastAssistantText(page, minimumTurnCount) {
  const turns = page.locator(ASSISTANT_TURN);
  const count = await turns.count();
  if (count <= minimumTurnCount) return { count, text: '' };

  const turn = turns.last();
  const markdown = turn.locator('.markdown').last();
  if ((await markdown.count()) > 0) {
    const text = await markdown.innerText().catch(() => '');
    if (text.trim()) return { count, text };
  }

  const authored = turn.locator('[data-message-author-role="assistant"]').last();
  if ((await authored.count()) > 0) {
    const text = await authored.innerText().catch(() => '');
    if (text.trim()) return { count, text };
  }

  return { count, text: await turn.innerText().catch(() => '') };
}

async function fillComposer(page, message) {
  const composer = page.locator(COMPOSER);
  await composer.waitFor({ state: 'visible', timeout: NAVIGATION_TIMEOUT_MS });
  await composer.click();
  await page.keyboard.insertText(message);

  const current = await composer.innerText().catch(() => '');
  if (!current.trim()) {
    await composer.click();
    await page.keyboard.type(message, { delay: 1 });
  }
}

async function runSend(payload, res) {
  const cookieHeader = String(payload.cookie || '').trim();
  const message = String(payload.message || '').trim();
  const conversationID = String(payload.conversationId || '').trim();
  const model = String(payload.model || '').trim();

  if (!cookieHeader) throw new Error('browser-backed writes require a browser session cookie');
  if (!message) throw new Error('message is required');
  if (payload.temporary) throw new Error('temporary chat is not supported by the browser fallback yet');

  const cookies = parseCookieHeader(cookieHeader);
  if (cookies.length === 0) throw new Error('browser session cookie could not be parsed');

  const launchOptions = {
    headless: false,
    args: ['--no-first-run', '--no-default-browser-check'],
  };
  const proxy = parseProxy(String(payload.proxyUrl || '').trim());
  if (proxy) launchOptions.proxy = proxy;

  let browser;
  let page;
  try {
    browser = await chromium.launch(launchOptions);
    const context = await browser.newContext({
      viewport: { width: 1440, height: 1000 },
      locale: 'en-US',
    });
    await context.addCookies(cookies);
    page = await context.newPage();

    let targetURL = 'https://chatgpt.com/';
    if (conversationID) {
      targetURL = `https://chatgpt.com/c/${encodeURIComponent(conversationID)}`;
    } else if (model && model !== 'auto') {
      targetURL = `https://chatgpt.com/?model=${encodeURIComponent(model)}`;
    }

    await page.goto(targetURL, { waitUntil: 'domcontentloaded', timeout: NAVIGATION_TIMEOUT_MS });

    const currentURL = page.url();
    if (/\/auth\b|\/login\b/.test(new URL(currentURL).pathname)) {
      throw new Error('ChatGPT browser session is not authenticated');
    }

    const assistantTurnsBefore = await page.locator(ASSISTANT_TURN).count();
    await fillComposer(page, message);
    await page.keyboard.press('Enter');

    const deadline = Date.now() + SEND_TIMEOUT_MS;
    let emittedConversationID = conversationID;
    let previousText = '';
    let stablePolls = 0;
    let sawAssistantTurn = false;

    if (emittedConversationID) {
      sendLine(res, { type: 'conversation', conversationId: emittedConversationID });
    }

    while (Date.now() < deadline) {
      if (res.destroyed || res.writableEnded) return;

      const discoveredID = conversationIDFromURL(page.url());
      if (discoveredID && discoveredID !== emittedConversationID) {
        emittedConversationID = discoveredID;
        sendLine(res, { type: 'conversation', conversationId: emittedConversationID });
      }

      const { text } = await lastAssistantText(page, assistantTurnsBefore);
      const normalized = text.trimEnd();
      if (normalized) {
        sawAssistantTurn = true;
        if (normalized === previousText) {
          stablePolls += 1;
        } else {
          stablePolls = 0;
          let delta = normalized;
          if (previousText && normalized.startsWith(previousText)) {
            delta = normalized.slice(previousText.length);
          }
          previousText = normalized;
          if (delta) {
            sendLine(res, {
              type: 'message_delta',
              conversationId: emittedConversationID || undefined,
              delta,
            });
          }
        }
      }

      const stopVisible = await page.locator(STOP_BUTTON).isVisible().catch(() => false);
      const copyVisible = await page.locator(COPY_BUTTON).last().isVisible().catch(() => false);
      if (sawAssistantTurn && previousText && !stopVisible && (copyVisible || stablePolls >= STABLE_POLLS_REQUIRED)) {
        sendLine(res, {
          type: 'message_completed',
          conversationId: emittedConversationID || undefined,
          message: {
            role: 'assistant',
            content: previousText,
          },
        });
        sendLine(res, { type: 'done', conversationId: emittedConversationID || undefined });
        return;
      }

      await sleep(POLL_MS);
    }

    throw new Error('ChatGPT browser send timed out before completion');
  } catch (error) {
    const currentURL = page?.url?.() || '';
    sendLine(res, {
      type: 'error',
      kind: classifyError(error, currentURL),
      message: sanitizeError(error),
    });
  } finally {
    await browser?.close().catch(() => {});
  }
}

const server = http.createServer(async (req, res) => {
  if (req.method === 'GET' && req.url === '/health') {
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end('{"status":"ok"}');
    return;
  }

  if (req.method !== 'POST' || req.url !== '/v1/send') {
    res.writeHead(404, { 'content-type': 'application/json' });
    res.end('{"error":"not found"}');
    return;
  }

  let payload;
  try {
    payload = await readJSON(req);
  } catch (error) {
    res.writeHead(400, { 'content-type': 'application/json' });
    res.end(JSON.stringify({ error: sanitizeError(error) }));
    return;
  }

  res.writeHead(200, {
    'content-type': 'application/x-ndjson; charset=utf-8',
    'cache-control': 'no-cache, no-transform',
    connection: 'keep-alive',
  });
  await runSend(payload, res);
  if (!res.writableEnded) res.end();
});

fs.mkdirSync(path.dirname(SOCKET_PATH), { recursive: true });
try {
  fs.unlinkSync(SOCKET_PATH);
} catch (error) {
  if (error?.code !== 'ENOENT') throw error;
}

server.listen(SOCKET_PATH, () => {
  fs.chmodSync(SOCKET_PATH, 0o666);
  console.log(`browser worker listening on unix://${SOCKET_PATH}`);
});

function shutdown() {
  server.close(() => process.exit(0));
  setTimeout(() => process.exit(1), 5000).unref();
}

process.on('SIGTERM', shutdown);
process.on('SIGINT', shutdown);
