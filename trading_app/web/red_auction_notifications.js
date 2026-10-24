(function (window) {
  'use strict';

  const notified = new Map();
  let localReminder = false;

  function localReminderEnabled() {
    try {
      if (window.location && window.location.origin === 'http://127.0.0.1:8080' &&
          new URLSearchParams(window.location.search).get('reminder_mode') === 'local') {
        localReminder = true;
        window.localStorage.setItem('red-auction:local-reminder', '1');
      }
      return localReminder || window.localStorage.getItem('red-auction:local-reminder') === '1';
    } catch (_) { return localReminder; }
  }

  function permission() {
    if (!('Notification' in window)) return 'unsupported';
    if (!window.isSecureContext) return 'insecure';
    return window.Notification.permission;
  }

  function requestPermission() {
    const current = permission();
    if (current !== 'default') return Promise.resolve(current);
    try {
      // Called directly from the user's permission-button click.
      return window.Notification.requestPermission();
    } catch (_) {
      return Promise.resolve('unsupported');
    }
  }

  function display(title, body, tag, onTap) {
    try {
      const notification = new window.Notification(title, {
        body: body,
        icon: 'favicon.png',
        tag: tag,
        renotify: false
      });
      notification.onclick = function (event) {
        if (event && event.preventDefault) event.preventDefault();
        window.focus();
        try {
          onTap();
        } finally {
          notification.close();
        }
      };
      return true;
    } catch (_) {
      return false;
    }
  }

  function show(date, userId, count, body, onTap) {
    if (localReminderEnabled()) return false;
    if (permission() !== 'granted' || !userId ||
        !/^\d{4}-\d{2}-\d{2}$/.test(date) ||
        !Number.isInteger(count) || count < 0) return false;
    const key = 'red-auction:last-notified:' + encodeURIComponent(userId);
    if (notified.get(userId) === date) return false;
    try {
      if (window.localStorage.getItem(key) === date) return false;
    } catch (_) {
      // Private browsing or storage policy can block localStorage.
    }
    if (!display('红策已更新，共 ' + count + ' 只', body, key + ':' + date, onTap)) return false;
    notified.set(userId, date);
    try { window.localStorage.setItem(key, date); } catch (_) {}
    return true;
  }

  function pairingTarget() {
    if (!window.location || window.location.origin !== 'http://127.0.0.1:8080') return null;
    const parameters = new URLSearchParams(window.location.search);
    const port = parameters.get('reminder_pair_port') || '';
    const nonce = parameters.get('reminder_pair_nonce') || '';
    if (!/^[0-9]{1,5}$/.test(port) || Number(port) < 1024 || Number(port) > 65535 ||
        !/^[a-f0-9]{64}$/.test(nonce)) return null;
    return 'http://127.0.0.1:' + port + '/connect/' + nonce;
  }

  function pairingAvailable() {
    return pairingTarget() !== null;
  }

  async function pairLocalReminder(accessToken) {
    const target = pairingTarget();
    if (!target || !accessToken) return 'error:连接地址或登录状态无效';
    try {
      const response = await window.fetch(target, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ accessToken }),
        redirect: 'error',
        credentials: 'omit',
        signal: AbortSignal.timeout(20000)
      });
      const data = await response.json();
      if (!response.ok || data.connected !== true) return 'error:现有登录状态或红策权限不可用';
      localReminder = true;
      try { window.localStorage.setItem('red-auction:local-reminder', '1'); } catch (_) {}
      const url = new URL(window.location.href);
      url.searchParams.delete('reminder_pair_port');
      url.searchParams.delete('reminder_pair_nonce');
      window.history.replaceState(null, '', url.toString());
      return 'connected';
    } catch (_) {
      return 'error:本机连接已超时，请重新发起连接';
    }
  }

  window.redAuctionNotifications = {
    permission, requestPermission, show, pairingAvailable, pairLocalReminder,
    localReminderEnabled
  };
})(window);
