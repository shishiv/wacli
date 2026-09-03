#!/usr/bin/env node

import { spawn } from 'node:child_process';
import { mkdir, writeFile } from 'node:fs/promises';
import { dirname } from 'node:path';
import { setTimeout as sleep } from 'node:timers/promises';
import { fileURLToPath } from 'node:url';

function parseArgs(argv) {
  const options = {
    bin: './dist/wacli',
    timeoutMs: 30_000,
    maxMessages: 20_000,
  };
  for (let i = 0; i < argv.length; i++) {
    const key = argv[i];
    if (key === '--') continue;
    if (!key.startsWith('--')) throw new Error(`unexpected argument: ${key}`);
    const name = key.slice(2).replaceAll('-', '_');
    const value = argv[++i];
    if (value === undefined) throw new Error(`${key} requires a value`);
    options[name] = value;
  }
  options.timeoutMs = Number(options.timeout_ms ?? options.timeoutMs);
  options.maxMessages = Number(options.max_messages ?? options.maxMessages);
  options.index = options.index === undefined ? undefined : Number(options.index);
  for (const required of ['to', 'message']) {
    if (!options[required]) throw new Error(`--${required} is required`);
  }
  const selectors = ['button_id', 'label', 'index'].filter((name) => options[name] !== undefined);
  if (selectors.length !== 1) throw new Error('exactly one of --button-id, --label, or --index is required');
  if (!Number.isFinite(options.timeoutMs) || options.timeoutMs < 1) throw new Error('--timeout-ms must be positive');
  if (!Number.isInteger(options.maxMessages) || options.maxMessages < 1) throw new Error('--max-messages must be positive');
  if (options.index !== undefined && (!Number.isInteger(options.index) || options.index < 1)) {
    throw new Error('--index must be 1 or greater');
  }
  return options;
}

export function parseEventLine(line) {
  let event;
  try {
    event = JSON.parse(line);
  } catch {
    throw new Error(`sync stdout is not NDJSON: ${line}`);
  }
  if (!event || typeof event.event !== 'string') throw new Error(`invalid sync event: ${line}`);
  return event;
}

function selectableButtons(buttons) {
  return buttons.filter((button) => button.type === 'quick_reply' || button.type === 'list_row');
}

export function matchingButton(buttons, options) {
  if (!Array.isArray(buttons)) return undefined;
  if (options.button_id !== undefined) return buttons.find((button) => button.id?.trim() === options.button_id.trim());
  if (options.label !== undefined) return buttons.find((button) => button.display_text?.trim() === options.label.trim());
  return selectableButtons(buttons)[options.index - 1];
}

async function waitForEvent(state, from, predicate, timeoutMs, description) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (state.error) throw state.error;
    if (state.exited) throw new Error(`sync exited before ${description}`);
    const event = state.events.slice(from).find(predicate);
    if (event) return event;
    await sleep(20);
  }
  throw new Error(`timed out waiting for ${description}`);
}

async function runCommand(bin, args, timeoutMs) {
  const child = spawn(bin, args, { stdio: ['ignore', 'pipe', 'pipe'] });
  let stdout = '';
  let stderr = '';
  child.stdout.setEncoding('utf8').on('data', (chunk) => (stdout += chunk));
  child.stderr.setEncoding('utf8').on('data', (chunk) => (stderr += chunk));
  const timer = setTimeout(() => child.kill('SIGKILL'), timeoutMs);
  const code = await new Promise((resolve, reject) => {
    child.once('error', reject);
    child.once('exit', resolve);
  });
  clearTimeout(timer);
  if (code !== 0) throw new Error(`${args.join(' ')} failed (${code}): ${stderr.trim() || stdout.trim()}`);
  const lines = stdout.trim().split('\n').filter(Boolean);
  return JSON.parse(lines.at(-1));
}

async function stopSync(child) {
  if (child.exitCode !== null) return;
  await new Promise((resolve) => {
    const timer = setTimeout(() => child.kill('SIGKILL'), 3_000);
    child.once('exit', () => {
      clearTimeout(timer);
      resolve();
    });
    child.kill('SIGINT');
  });
}

export async function runE2E(options) {
  const globalArgs = options.store ? ['--store', options.store] : [];
  const sync = spawn(options.bin, [
    ...globalArgs,
    'sync',
    '--follow',
    '--json',
    '--presence-mode',
    'quiet',
    '--max-messages',
    String(options.maxMessages),
  ], { stdio: ['ignore', 'pipe', 'pipe'] });
  const state = { events: [], error: undefined, exited: false };
  let buffered = '';
  sync.stdout.setEncoding('utf8').on('data', (chunk) => {
    buffered += chunk;
    const lines = buffered.split('\n');
    buffered = lines.pop();
    for (const line of lines.filter(Boolean)) {
      try {
        state.events.push(parseEventLine(line));
      } catch (error) {
        state.error ??= error;
      }
    }
  });
  sync.stderr.setEncoding('utf8').on('data', (chunk) => process.stderr.write(chunk));
  sync.once('error', (error) => (state.error ??= error));
  sync.once('exit', () => (state.exited = true));

  try {
    const ready = await waitForEvent(state, 0, (event) => event.event === 'ready', options.timeoutMs, 'sync ready');

    const textStart = Date.now();
    const promptFrom = state.events.length;
    const sent = await runCommand(options.bin, [
      ...globalArgs,
      'send',
      'text',
      '--to',
      options.to,
      '--message',
      options.message,
      '--post-send-wait',
      '0',
      '--json',
    ], options.timeoutMs);
    const textFinished = Date.now();
    const prompt = await waitForEvent(
      state,
      promptFrom,
      (event) => event.event === 'message' && event.data?.from_me === false && matchingButton(event.data?.buttons, options),
      options.timeoutMs,
      'interactive prompt',
    );

    const button = matchingButton(prompt.data.buttons, options);
    const selector = options.button_id !== undefined
      ? ['--button-id', options.button_id]
      : options.label !== undefined
        ? ['--label', options.label]
        : ['--index', String(options.index)];
    const selectStart = Date.now();
    const replyFrom = state.events.length;
    const selected = await runCommand(options.bin, [
      ...globalArgs,
      'send',
      'select',
      '--to',
      prompt.data.chat_jid,
      '--id',
      prompt.data.id,
      ...selector,
      '--post-send-wait',
      '0',
      '--json',
    ], options.timeoutMs);
    const selectFinished = Date.now();
    const reply = await waitForEvent(
      state,
      replyFrom,
      (event) => event.event === 'message' && event.data?.from_me === false && event.data?.chat_jid === prompt.data.chat_jid,
      options.timeoutMs,
      'post-selection reply',
    );
    if (options.expect && !reply.data?.text?.includes(options.expect)) {
      throw new Error(`reply did not contain ${JSON.stringify(options.expect)}: ${JSON.stringify(reply.data?.text ?? '')}`);
    }

    return {
      success: true,
      transport: button.response_type === 'template_button_reply' ? 'template_button_reply' : 'quoted_text',
      ready: ready.data,
      sent: { ...sent.data, command_ms: textFinished - textStart },
      prompt: {
        id: prompt.data.id,
        chat_jid: prompt.data.chat_jid,
        text: prompt.data.text,
        button,
        latency_ms: prompt.ts - textStart,
        after_command_ms: prompt.ts - textFinished,
      },
      selected: { ...selected.data, command_ms: selectFinished - selectStart },
      reply: {
        id: reply.data.id,
        text: reply.data.text,
        latency_ms: reply.ts - selectStart,
        after_command_ms: reply.ts - selectFinished,
      },
    };
  } finally {
    await stopSync(sync);
  }
}

async function main() {
  const options = parseArgs(process.argv.slice(2));
  const result = await runE2E(options);
  const output = `${JSON.stringify(result, null, 2)}\n`;
  if (options.output) {
    await mkdir(dirname(options.output), { recursive: true });
    await writeFile(options.output, output);
  }
  process.stdout.write(output);
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  main().catch((error) => {
    console.error(error.message);
    process.exitCode = 1;
  });
}
