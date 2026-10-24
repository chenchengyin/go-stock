#!/usr/bin/env python3
"""Local runner for the GPT 09:25 task; credentials never appear in output."""
import argparse
import datetime as dt
import fcntl
import hashlib
import http.server
import json
import os
from pathlib import Path
import secrets
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from zoneinfo import ZoneInfo

ROOT = Path(__file__).resolve().parent.parent
ZONE = ZoneInfo("Asia/Shanghai")
BASE = "http://127.0.0.1:8080"
MODULE = "radar.red_shared_gob_strategy"
STATE = Path.home() / "Library/Application Support/go-stock/red-auction-reminder"
CONFIG = STATE / "connection.json"


class ReminderError(Exception):
    pass


def emit(status, **fields):
    print(json.dumps({"status": status, **fields}, ensure_ascii=False), flush=True)


def private_write(path, data):
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    temporary = path.with_name(path.name + "." + secrets.token_hex(6))
    fd = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    try:
        with os.fdopen(fd, "w") as f:
            json.dump(data, f, ensure_ascii=False)
        os.replace(temporary, path)
    finally:
        temporary.unlink(missing_ok=True)


def connection():
    config = {}
    if CONFIG.exists():
        if CONFIG.stat().st_mode & 0o077:
            raise ReminderError("本机连接文件权限必须为0600，请重新连接本机提醒")
        config = json.loads(CONFIG.read_text())
        if config.get("api_base_url") != BASE:
            raise ReminderError("连接地址不匹配，只允许已确认的本机8080服务")
    token = os.environ.get("GO_STOCK_ACCESS_TOKEN") or os.environ.get("GO_STOCK_API_TOKEN") or config.get("access_token")
    if not isinstance(token, str) or not token.strip():
        raise ReminderError("尚未连接网页登录，请执行 --pair 并点击网页的“连接本机提醒”")
    return token.strip()


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None  # Never forward the bearer token to another origin.


def request(path, token, timeout=15):
    req = urllib.request.Request(BASE + path, headers={"Authorization": "Bearer " + token, "Accept": "application/json"})
    try:
        with urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect).open(req, timeout=timeout) as response:
            body = response.read(5_000_001)
            if len(body) > 5_000_000:
                raise ReminderError("接口响应过大")
            value = json.loads(body)
            if not isinstance(value, dict):
                raise ValueError("Invalid JSON response")
            return value
    except urllib.error.HTTPError as error:
        if error.code in (401, 403):
            raise ReminderError("红策接口登录或模块授权已失效，请重新连接本机提醒") from None
        raise OSError("HTTP " + str(error.code)) from None


def trading_day(day):
    if day.weekday() >= 5:
        return False
    calendar_path = ROOT / "scripts/data" / ("ashare-calendar-" + str(day.year) + ".json")
    if not calendar_path.exists():
        raise ReminderError("缺少" + str(day.year) + "年交易所休市日历，不能按工作日猜测开市")
    calendar = json.loads(calendar_path.read_text())
    if calendar.get("year") != day.year or not calendar.get("source", "").startswith(("https://www.sse.com.cn/", "https://www.szse.cn/")):
        raise ReminderError("交易所日历来源或年度无效")
    ranges = calendar.get("closed_ranges")
    if not isinstance(ranges, list) or not ranges:
        raise ReminderError("交易所休市日历不完整")
    closed = False
    for start, end in ranges:
        first, last = dt.date.fromisoformat(start), dt.date.fromisoformat(end)
        if first.year != day.year or last.year != day.year or first > last:
            raise ReminderError("交易所休市日期范围无效")
        closed = closed or first <= day <= last
    return not closed


def native_binary():
    if sys.platform != "darwin":
        raise ReminderError("本机系统弹窗仅支持macOS")
    source = ROOT / "scripts/red-auction-alert.swift"
    digest = hashlib.sha256(source.read_bytes()).hexdigest()[:16]
    target = STATE / ("native-alert-" + digest)
    if not target.exists():
        result = subprocess.run(["/usr/bin/xcrun", "swiftc", "-O", "-framework", "AppKit", str(source), "-o", str(target)], capture_output=True)
        if result.returncode:
            raise ReminderError("原生提醒程序编译失败，请检查Xcode命令行工具")
        target.chmod(0o700)
    return target


def show(rows, day, marker):
    names = [str(r.get("股票名称", "")) + " " + str(r.get("股票代码", "")) for r in rows[:12]]
    body = day + "，共" + str(len(rows)) + "只\n" + ("\n".join(names) if names else "今日暂无符合条件的股票")
    if len(rows) > 12:
        body += "\n其余股票请打开红策查看"
    body += "\n\n点击确定打开今日红策。"
    url = BASE + "/?" + urllib.parse.urlencode({"radar_module": MODULE, "radar_date": day})
    payload = {"title": "今日红策已更新", "body": body[:2400], "url": url, "receipt": str(marker), "date": day, "count": len(rows)}
    chrome_state = Path.home() / "Library/Application Support/Google/Chrome/Local State"
    try:
        profiles = json.loads(chrome_state.read_text()).get("profile", {}).get("last_active_profiles", [])
        if len(profiles) == 1 and isinstance(profiles[0], str):
            payload["browserProfile"] = profiles[0]
    except (OSError, ValueError, TypeError):
        pass
    with (STATE / "native-alert.log").open("ab") as log:
        process = subprocess.Popen([str(native_binary())], stdin=subprocess.PIPE, stdout=log, stderr=log, start_new_session=True)
        process.stdin.write(json.dumps(payload, ensure_ascii=False).encode())
        process.stdin.close()
    deadline = time.monotonic() + 8
    while time.monotonic() < deadline:
        if marker.exists():
            emit("shown", date=day, count=len(rows), url=url)
            return
        if process.poll() is not None:
            raise ReminderError("原生弹窗在显示前退出，未记录已提醒")
        time.sleep(0.1)
    process.terminate()
    raise ReminderError("未收到原生窗口显示回执，未记录已提醒；请检查是否锁屏或缺少桌面会话")


def run():
    now = dt.datetime.now(ZONE)
    day = now.date()
    if not trading_day(day):
        emit("market_closed", date=day.isoformat())
        return
    begin = now.replace(hour=9, minute=25, second=10, microsecond=0)
    cutoff = now.replace(hour=9, minute=26, second=0, microsecond=0)
    if now < begin.replace(second=0) or now >= cutoff:
        emit("outside_window", date=day.isoformat(), message="仅在09:25执行；未补发旧结果")
        return
    token = connection()
    marker = STATE / (day.isoformat() + ".json")
    if marker.exists():
        emit("already_shown", date=day.isoformat())
        return
    native_binary()  # Built during setup, before the first trading-day run.
    if now < begin:
        time.sleep(max(0, (begin - dt.datetime.now(ZONE)).total_seconds()))
    reason = "红策尚未确认"
    while dt.datetime.now(ZONE) < cutoff:
        try:
            data = request("/api/t0-selection?" + urllib.parse.urlencode({"module_code": MODULE, "date": day.isoformat()}), token)
            rows = data.get("results")
            if data.get("date") == day.isoformat() and data.get("no_data") is True:
                raise ReminderError("交易日接口返回no_data，未取得当天红策；不能据此当成休市")
            if (data.get("date") == day.isoformat() and data.get("confirmed") is True
                    and not data.get("data_incomplete") and not data.get("error")
                    and isinstance(rows, list) and all(isinstance(row, dict) for row in rows)
                    and type(data.get("count")) is int and data["count"] >= 0 and data["count"] == len(rows)):
                if dt.datetime.now(ZONE).date() != day:
                    raise ReminderError("请求返回时已跨日，不显示过期提醒")
                show(rows, day.isoformat(), marker)
                return
            reason = "当天红策尚未完整确认"
        except (OSError, ValueError, TimeoutError):
            reason = "红策请求暂时失败"
        remaining = (cutoff - dt.datetime.now(ZONE)).total_seconds()
        if remaining < 5:
            break
        time.sleep(5)
    raise ReminderError(reason + "，已到09:26停止新请求")


def pair():
    nonce = secrets.token_hex(32)
    expected_path = "/connect/" + nonce

    class Handler(http.server.BaseHTTPRequestHandler):
        success = False

        def log_message(self, *_):
            pass

        def valid(self):
            return (self.headers.get("Origin") == BASE
                    and self.headers.get("Host") == "127.0.0.1:" + str(self.server.server_port)
                    and secrets.compare_digest(self.path, expected_path))

        def reply(self, code, data):
            self.send_response(code)
            if self.valid():
                self.send_header("Access-Control-Allow-Origin", BASE)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(json.dumps(data).encode())

        def do_OPTIONS(self):
            if not self.valid():
                self.reply(403, {})
                return
            self.send_response(204)
            self.send_header("Access-Control-Allow-Origin", BASE)
            self.send_header("Access-Control-Allow-Methods", "POST")
            self.send_header("Access-Control-Allow-Headers", "Content-Type")
            self.end_headers()

        def do_POST(self):
            if not self.valid() or Handler.success:
                self.reply(403, {})
                return
            try:
                size = int(self.headers.get("Content-Length", "0"))
                if not 0 < size <= 8192:
                    raise ValueError("Invalid size")
                token = json.loads(self.rfile.read(size)).get("accessToken")
                if not isinstance(token, str) or not token or len(token) > 4096:
                    raise ValueError("Invalid token")
                # Verify both session validity and red-module access, without a new login.
                request("/api/auth/me", token, timeout=5)
                request("/api/t0-selection?module_code=" + MODULE + "&list_dates=1", token, timeout=10)
                private_write(CONFIG, {"api_base_url": BASE, "access_token": token})
                Handler.success = True
                self.reply(200, {"connected": True})
            except (ValueError, OSError, ReminderError):
                self.reply(400, {"connected": False, "message": "现有登录状态或红策权限不可用，请刷新网页后重试"})

    server = http.server.HTTPServer(("127.0.0.1", 0), Handler)
    server.timeout = 0.5
    url = BASE + "/?" + urllib.parse.urlencode({"reminder_pair_port": server.server_port, "reminder_pair_nonce": nonce})
    pending = STATE / "pending-connection.json"
    private_write(pending, {"url": url})
    subprocess.run(["/usr/bin/osascript", "-e", "on run argv\nopen location (item 1 of argv)\nend run", url], check=True, capture_output=True)
    emit("waiting_for_connection", message="请在新打开的网页点击“连接本机提醒”，无需提供密码")
    deadline = time.monotonic() + 300
    while time.monotonic() < deadline and not Handler.success:
        server.handle_request()
    server.server_close()
    pending.unlink(missing_ok=True)
    if not Handler.success:
        raise ReminderError("本机配对已超时；请重新执行 --pair")
    emit("connected", api_base_url=BASE)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    group = parser.add_mutually_exclusive_group()
    group.add_argument("--pair", action="store_true")
    group.add_argument("--check", action="store_true")
    group.add_argument("--build-alert", action="store_true")
    args = parser.parse_args()
    STATE.mkdir(parents=True, mode=0o700, exist_ok=True)
    STATE.chmod(0o700)
    if args.pair:
        pair()
    elif args.check:
        token = connection()
        request("/api/auth/me", token, timeout=5)
        request("/api/t0-selection?module_code=" + MODULE + "&list_dates=1", token, timeout=10)
        emit("ready", api_base_url=BASE, calendar_year=dt.datetime.now(ZONE).year)
    elif args.build_alert:
        native_binary()
        emit("alert_built")
    else:
        with (STATE / "run.lock").open("a") as lock:
            try:
                fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            except BlockingIOError:
                emit("already_running")
                return
            run()


if __name__ == "__main__":
    try:
        main()
    except (ReminderError, OSError, ValueError, KeyError, TypeError) as error:
        # Never print raw request/response/config values or tokens.
        message = str(error) if isinstance(error, ReminderError) else "本机提醒执行失败（配置、网络或文件不可用）"
        emit("error", message=message)
        sys.exit(1)
