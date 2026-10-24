const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, '../../web/red_auction_notifications.js'), 'utf8');

function browser({ permission = 'granted', secure = true, storage = new Map(), blockedStorage = false } = {}) {
  const notifications = [];
  let focused = false;
  class Notification {
    static permission = permission;
    static requestPermission() { return Promise.resolve(this.permission); }
    constructor(title, options) {
      this.title = title;
      this.options = options;
      this.closed = false;
      notifications.push(this);
    }
    close() { this.closed = true; }
  }
  const window = {
    Notification,
    isSecureContext: secure,
    focus() { focused = true; },
    localStorage: {
      getItem(key) { if (blockedStorage) throw Error('blocked'); return storage.get(key) ?? null; },
      setItem(key, value) { if (blockedStorage) throw Error('blocked'); storage.set(key, value); },
    },
  };
  vm.runInNewContext(source, { window });
  return { window, api: window.redAuctionNotifications, notifications, focused: () => focused };
}

test('confirmed list notification contains count and click focuses and opens the red list', () => {
  const b = browser();
  let opened = 0;
  assert.equal(b.api.show('2026-10-23', 'user-a', 2, '浦发银行、招商银行', () => opened++), true);
  assert.equal(b.notifications.length, 1);
  const notification = b.notifications[0];
  assert.match(notification.title, /2.*只/);
  assert.match(notification.options.body, /浦发银行/);
  notification.onclick({ preventDefault() {} });
  assert.equal(opened, 1);
  assert.equal(b.focused(), true);
  assert.equal(notification.closed, true);
});

test('daily notification survives reload deduplication and is scoped to user and date', () => {
  const storage = new Map();
  const first = browser({ storage });
  assert.equal(first.api.show('2026-10-23', 'user-a', 0, '暂无符合条件的股票', () => {}), true);
  const reloaded = browser({ storage });
  assert.equal(reloaded.api.show('2026-10-23', 'user-a', 0, '', () => {}), false);
  assert.equal(reloaded.notifications.length, 0);
  assert.equal(reloaded.api.show('2026-10-23', 'user-b', 0, '', () => {}), true);
  assert.equal(reloaded.api.show('2026-10-26', 'user-a', 0, '', () => {}), true);
});

test('unsupported, insecure and denied browsers never create a notification', async () => {
  const insecure = browser({ secure: false });
  assert.equal(insecure.api.permission(), 'insecure');
  assert.equal(await insecure.api.requestPermission(), 'insecure');
  assert.equal(insecure.api.show('2026-10-23', 'a', 1, '', () => {}), false);
  const denied = browser({ permission: 'denied' });
  assert.equal(await denied.api.requestPermission(), 'denied');
  assert.equal(denied.api.show('2026-10-23', 'a', 1, '', () => {}), false);
  delete denied.window.Notification;
  assert.equal(denied.api.permission(), 'unsupported');
  assert.equal(await denied.api.requestPermission(), 'unsupported');
});

test('blocked storage still shows once without breaking the page', () => {
  const b = browser({ blockedStorage: true });
  assert.equal(b.api.show('2026-10-23', 'a', 1, '', () => {}), true);
  assert.equal(b.api.show('2026-10-23', 'a', 1, '', () => {}), false);
  assert.equal(b.notifications.length, 1);
});
