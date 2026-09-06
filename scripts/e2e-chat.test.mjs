import assert from 'node:assert/strict';
import { chmod, mkdtemp, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';

import { parseEventLine, runE2E } from './e2e-chat.mjs';

test('parseEventLine rejects stdout contamination', () => {
  assert.throws(
    () => parseEventLine('[ERROR] SessionCipher old counter'),
    /sync stdout is not NDJSON/,
  );
});

test('runE2E drives text, selection, and reply through a follow daemon', async () => {
  const dir = await mkdtemp(join(tmpdir(), 'wacli-e2e-chat-'));
  const fake = join(dir, 'fake-wacli.mjs');
  const state = join(dir, 'state');
  await writeFile(fake, `#!/usr/bin/env node
import { readFileSync, writeFileSync } from 'node:fs';
const args = process.argv.slice(2);
const state = process.env.FAKE_WACLI_STATE;
if (args.includes('sync')) {
  console.log(JSON.stringify({event:'ready',data:{jid:'me',socket:'fake'},ts:Date.now()}));
  let seen = '';
  setInterval(() => {
    let next = '';
    try { next = readFileSync(state, 'utf8'); } catch {}
    if (next === seen) return;
    seen = next;
    if (next === 'text') console.log(JSON.stringify({event:'message',data:{id:'PROMPT',chat_jid:'bot@lid',from_me:false,text:'Choose',buttons:[{type:'quick_reply',display_text:'More',id:'more'}]},ts:Date.now()}));
    if (next === 'select') console.log(JSON.stringify({event:'message',data:{id:'REPLY',chat_jid:'bot@lid',from_me:false,text:'Details'},ts:Date.now()}));
  }, 10);
  process.on('SIGINT', () => process.exit(0));
} else if (args.includes('text')) {
  writeFileSync(state, 'text');
  console.log(JSON.stringify({success:true,data:{sent:true,id:'SENT',to:'bot@lid'}}));
} else if (args.includes('select')) {
  writeFileSync(state, 'select');
  console.log(JSON.stringify({success:true,data:{sent:true,id:'SELECTED',target:'PROMPT',to:'bot@lid',selected:{id:'more',display_text:'More',type:'quick_reply'}}}));
}
`);
  await chmod(fake, 0o755);

  const previous = process.env.FAKE_WACLI_STATE;
  process.env.FAKE_WACLI_STATE = state;
  try {
    const result = await runE2E({
      bin: fake,
      store: dir,
      to: 'bot@s.whatsapp.net',
      message: 'help',
      button_id: 'more',
      expect: 'Details',
      timeoutMs: 2_000,
      maxMessages: 100,
    });
    assert.equal(result.success, true);
    assert.equal(result.transport, 'quoted_text');
    assert.equal(result.prompt.button.id, 'more');
    assert.equal(result.reply.text, 'Details');
  } finally {
    if (previous === undefined) delete process.env.FAKE_WACLI_STATE;
    else process.env.FAKE_WACLI_STATE = previous;
  }
});
