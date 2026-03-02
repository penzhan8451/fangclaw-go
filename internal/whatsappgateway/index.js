#!/usr/bin/env node
'use strict';

const http = require('node:http');
const { randomUUID } = require('node:crypto');

const PORT = parseInt(process.env.WHATSAPP_GATEWAY_PORT || '3009', 10);
const OPENFANG_URL = (process.env.OPENFANG_URL || 'http://127.0.0.1:4200').replace(/\/+$/, '');
const DEFAULT_AGENT = process.env.OPENFANG_DEFAULT_AGENT || 'assistant';

let sock = null;
let sessionId = '';
let qrDataUrl = '';
let connStatus = 'disconnected';
let qrExpired = false;
let statusMessage = 'Not started';

async function startConnection() {
  const { default: makeWASocket, useMultiFileAuthState, DisconnectReason, fetchLatestBaileysVersion } =
    await import('@whiskeysockets/baileys');
  const QRCode = (await import('qrcode')).default || await import('qrcode');
  const pino = (await import('pino')).default || await import('pino');

  const logger = pino({ level: 'warn' });
  const authDir = require('node:path').join(__dirname, 'auth_store');

  const { state, saveCreds } = await useMultiFileAuthState(
    require('node:path').join(__dirname, 'auth_store')
  );
  const { version } = await fetchLatestBaileysVersion();

  sessionId = randomUUID();
  qrDataUrl = '';
  qrExpired = false;
  connStatus = 'disconnected';
  statusMessage = 'Connecting...';

  sock = makeWASocket({
    version,
    auth: state,
    logger,
    printQRInTerminal: true,
    browser: ['FangClaw-go', 'Desktop', '1.0.0'],
  });

  sock.ev.on('creds.update', saveCreds);

  sock.ev.on('connection.update', async (update) => {
    const { connection, lastDisconnect, qr } = update;

    if (qr) {
      try {
        qrDataUrl = await QRCode.toDataURL(qr, { width: 256, margin: 2 });
        connStatus = 'qr_ready';
        qrExpired = false;
        statusMessage = 'Scan this QR code with WhatsApp → Linked Devices';
        console.log('[gateway] QR code ready — waiting for scan');
      } catch (err) {
        console.error('[gateway] QR generation failed:', err.message);
      }
    }

    if (connection === 'close') {
      const statusCode = lastDisconnect?.error?.output?.statusCode;
      const reason = lastDisconnect?.error?.output?.payload?.message || 'unknown';
      console.log('[gateway] Connection closed: ' + reason + ' (' + statusCode + ')');

      if (statusCode === DisconnectReason.loggedOut) {
        connStatus = 'disconnected';
        statusMessage = 'Logged out. Generate a new QR code to reconnect.';
        qrDataUrl = '';
        sock = null;
        const fs = require('node:fs');
        const path = require('node:path');
        const authPath = path.join(__dirname, 'auth_store');
        if (fs.existsSync(authPath)) {
          fs.rmSync(authPath, { recursive: true, force: true });
        }
      } else if (statusCode === DisconnectReason.restartRequired ||
                 statusCode === DisconnectReason.timedOut) {
        console.log('[gateway] Reconnecting...');
        statusMessage = 'Reconnecting...';
        setTimeout(() => startConnection(), 2000);
      } else {
        qrExpired = true;
        connStatus = 'disconnected';
        statusMessage = 'QR code expired. Click "Generate New QR" to retry.';
        qrDataUrl = '';
      }
    }

    if (connection === 'open') {
      connStatus = 'connected';
      qrExpired = false;
      qrDataUrl = '';
      statusMessage = 'Connected to WhatsApp';
      console.log('[gateway] Connected to WhatsApp!');
    }
  });

  sock.ev.on('messages.upsert', async ({ messages, type }) => {
    if (type !== 'notify') return;

    for (const msg of messages) {
      if (msg.key.fromMe) continue;
      if (msg.key.remoteJid === 'status@broadcast') continue;

      const sender = msg.key.remoteJid || '';
      const text = msg.message?.conversation
        || msg.message?.extendedTextMessage?.text
        || msg.message?.imageMessage?.caption
        || '';

      if (!text) continue;

      const phone = '+' + sender.replace(/@.*$/, '');
      const pushName = msg.pushName || phone;

      console.log('[gateway] Incoming from ' + pushName + ' (' + phone + '): ' + text.substring(0, 80));

      try {
        const response = await forwardToOpenFang(text, phone, pushName);
        if (response && sock) {
          await sock.sendMessage(sender, { text: response });
          console.log('[gateway] Replied to ' + pushName);
        }
      } catch (err) {
        console.error('[gateway] Forward/reply failed:', err.message);
      }
    }
  });
}

function forwardToOpenFang(text, phone, pushName) {
  return new Promise((resolve, reject) => {
    const payload = JSON.stringify({
      message: text,
      metadata: {
        channel: 'whatsapp',
        sender: phone,
        sender_name: pushName,
      },
    });

    const url = new URL(OPENFANG_URL + '/api/agents/' + encodeURIComponent(DEFAULT_AGENT) + '/message');

    const req = http.request(
      {
        hostname: url.hostname,
        port: url.port || 4200,
        path: url.pathname,
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Content-Length': Buffer.byteLength(payload),
        },
        timeout: 120000,
      },
      (res) => {
        let body = '';
        res.on('data', (chunk) => (body += chunk));
        res.on('end', () => {
          try {
            const data = JSON.parse(body);
            resolve(data.response || data.message || data.text || '');
          } catch {
            resolve(body.trim() || '');
          }
        });
      },
    );

    req.on('error', reject);
    req.on('timeout', () => {
      req.destroy();
      reject(new Error('OpenFang API timeout'));
    });
    req.write(payload);
    req.end();
  });
}

async function sendMessage(to, text) {
  if (!sock || connStatus !== 'connected') {
    throw new Error('WhatsApp not connected');
  }

  const jid = to.replace(/^\+/, '').replace(/@.*$/, '') + '@s.whatsapp.net';

  await sock.sendMessage(jid, { text });
}

function parseBody(req) {
  return new Promise((resolve, reject) => {
    let body = '';
    req.on('data', (chunk) => (body += chunk));
    req.on('end', () => {
      try {
        resolve(body ? JSON.parse(body) : {});
      } catch (e) {
        reject(new Error('Invalid JSON'));
      }
    });
    req.on('error', reject);
  });
}

function jsonResponse(res, status, data) {
  const body = JSON.stringify(data);
  res.writeHead(status, {
    'Content-Type': 'application/json',
    'Content-Length': Buffer.byteLength(body),
    'Access-Control-Allow-Origin': '*',
  });
  res.end(body);
}

const server = http.createServer(async (req, res) => {
  if (req.method === 'OPTIONS') {
    res.writeHead(204, {
      'Access-Control-Allow-Origin': '*',
      'Access-Control-Allow-Methods': 'GET, POST, OPTIONS',
      'Access-Control-Allow-Headers': 'Content-Type',
    });
    return res.end();
  }

  const url = new URL(req.url, 'http://localhost:' + PORT);
  const path = url.pathname;

  try {
    if (req.method === 'POST' && path === '/login/start') {
      if (connStatus === 'connected') {
        return jsonResponse(res, 200, {
          qr_data_url: '',
          session_id: sessionId,
          message: 'Already connected to WhatsApp',
          connected: true,
        });
      }

      await startConnection();

      let waited = 0;
      while (!qrDataUrl && connStatus !== 'connected' && waited < 15000) {
        await new Promise((r) => setTimeout(r, 300));
        waited += 300;
      }

      return jsonResponse(res, 200, {
        qr_data_url: qrDataUrl,
        session_id: sessionId,
        status: connStatus,
        message: statusMessage,
        connected: connStatus === 'connected',
        qr_expired: qrExpired,
      });
    }

    if (req.method === 'GET' && path === '/login/status') {
      return jsonResponse(res, 200, {
        status: connStatus,
        message: statusMessage,
        connected: connStatus === 'connected',
        qr_expired: qrExpired,
        qr_data_url: qrDataUrl,
        session_id: sessionId,
      });
    }

    if (req.method === 'POST' && path === '/logout') {
      if (sock) {
        const fs = require('node:fs');
        const path = require('node:path');
        const authPath = path.join(__dirname, 'auth_store');
        if (fs.existsSync(authPath)) {
          fs.rmSync(authPath, { recursive: true, force: true });
        }
        sock.end(undefined);
        sock = null;
      }
      connStatus = 'disconnected';
      qrDataUrl = '';
      statusMessage = 'Logged out';
      return jsonResponse(res, 200, { success: true, message: 'Logged out' });
    }

    if (req.method === 'POST' && path === '/send') {
      const body = await parseBody(req);
      if (!body.to || !body.text) {
        return jsonResponse(res, 400, { error: 'Missing "to" or "text"' });
      }
      await sendMessage(body.to, body.text);
      return jsonResponse(res, 200, { success: true });
    }

    if (req.method === 'GET' && path === '/health') {
      return jsonResponse(res, 200, {
        status: 'ok',
        connected: connStatus === 'connected',
        session_id: sessionId,
      });
    }

    jsonResponse(res, 404, { error: 'Not found' });
  } catch (err) {
    console.error('[gateway] Error handling request:', err);
    jsonResponse(res, 500, { error: err.message });
  }
});

server.listen(PORT, () => {
  console.log('[gateway] WhatsApp Web Gateway listening on http://127.0.0.1:' + PORT);
  console.log('[gateway] Forwarding to OpenFang at ' + OPENFANG_URL);
});
