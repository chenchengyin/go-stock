"""
T0开盘日线选股 — 纯日线选股 + T0 09:25竞价确认
无5分钟涨速，仅使用日线动量+T0开盘确认

数据源：
  - 股票池、交易日历：Baostock（仅1次调用）
  - 日线K线：新浪API（money.finance.sina.com.cn）
  - T0实时行情：腾讯API（qt.gtimg.cn）

过滤链（按执行顺序）：
  过滤1: 主板（60/00开头）
  过滤2: 近7日有涨停（短期动量确认）
  过滤3: 前一交易日成交额 ≥ 5亿（流动性过滤）
  过滤4: 前一交易日收盘价 > MA20（中期趋势向上）
  过滤5: T0开盘涨幅 0.01% ~ 3%（竞价高开确认）

使用方式：
  每天09:25后运行，传入T0日期
  python3 T0开盘日线选股.py [trade_date]
  例: python3 T0开盘日线选股.py 2026-07-20

目的：
  日线强势股（7日涨停+MA20向上）+ T0竞价温和高开确认买入
"""

import os
import sys
import json
import time
import pickle
import subprocess
import datetime
import warnings
from concurrent.futures import ThreadPoolExecutor, as_completed

import numpy as np
import pandas as pd

# 清理代理环境变量
for key in ['http_proxy', 'https_proxy', 'HTTP_PROXY', 'HTTPS_PROXY', 'all_proxy', 'ALL_PROXY']:
    os.environ.pop(key, None)
os.environ['NO_PROXY'] = '*'

warnings.filterwarnings('ignore')

# ─────────────────────────────────────────────────────────────
# 数据获取层
# ─────────────────────────────────────────────────────────────


def _curl(url, timeout=15):
    """使用 curl --noproxy 获取HTTP内容，支持UTF-8/GBK自动解码。"""
    try:
        result = subprocess.run(
            ['curl', '--noproxy', '*', '-sL', url],
            capture_output=True, timeout=timeout
        )
        raw = result.stdout
        if not raw:
            return None
        try:
            return raw.decode('utf-8')
        except UnicodeDecodeError:
            return raw.decode('gbk', errors='replace')
    except Exception:
        return None


def _get_stock_pool_sina():
    """新浪分页获取全量A股 -> 过滤主板。较慢但可靠。"""
    base_url = ('http://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/'
                'Market_Center.getHQNodeData?page=%d&num=100&sort=code&asc=1'
                '&node=hs_a&symbol=&_s_r_a=page')
    all_stocks = []
    page = 1
    while page <= 100:
        text = _curl(base_url % page, timeout=10)
        if not text:
            break
        try:
            data = json.loads(text)
        except Exception:
            break
        if not data or len(data) == 0:
            break
        all_stocks.extend(data)
        if len(data) < 100:
            break
        page += 1

    stocks = []
    for s in all_stocks:
        code = s.get('code', '')
        name = s.get('name', '')
        if code.startswith('60'):
            stocks.append(('sh.' + code, code, name))
        elif code.startswith('00'):
            stocks.append(('sz.' + code, code, name))
    return stocks


def get_stock_pool(trade_date=None):
    """
    从新浪分页获取全量A股 -> 过滤主板(60/00开头)。
    返回: [(code, short_code, name), ...]
          code 格式为 'sh.600000' / 'sz.000001'（兼容下游代码）
    """
    stocks = _get_stock_pool_sina()
    if not stocks:
        print('[警告] 新浪股票池返回为空')
        return []
    return stocks


def get_today_realtime_prices(stock_codes):
    """
    批量获取今日实时行情（腾讯API）。
    stock_codes: [(short_code, 'sh'/'sz'), ...]
    返回: {short_code: {'open': float, 'close': float, 'prev_close': float}}
    """
    if not stock_codes:
        return {}
    symbols = []
    code_map = {}
    for sc, prefix in stock_codes:
        sym = prefix + sc
        symbols.append(sym)
        code_map[sym] = sc

    url = 'http://qt.gtimg.cn/q=' + ','.join(symbols)
    text = _curl(url, timeout=30)
    if not text:
        return {}

    result = {}
    for line in text.strip().split('\n'):
        try:
            parts = line.split('~')
            if len(parts) < 6:
                continue
            eq_pos = parts[0].rfind('="')
            if eq_pos < 3:
                continue
            sym = parts[0][2:eq_pos]
            if sym not in code_map:
                continue
            sc = code_map[sym]
            current_p = float(parts[3]) if parts[3] else 0
            prev_close = float(parts[4]) if parts[4] else 0
            today_open = float(parts[5]) if parts[5] else 0
            result[sc] = {
                'open': today_open,
                'close': current_p,
                'prev_close': prev_close,
            }
        except (ValueError, IndexError):
            continue
    return result


def get_daily_data(code, end_date, days=25):
    """
    获取一只股票近N天的日线数据（新浪API）。
    code: Baostock 格式 'sz.002519'
    end_date: 截止日期 '2026-07-10'
    days: 取N天数据（需≥20用于MA20计算）
    返回: DataFrame (date, open, close, high, low, volume, amount_yi) 或 None
    """
    prefix = 'sh' if code.startswith('sh') else 'sz'
    sym = prefix + code.split('.')[1]
    url = ('http://money.finance.sina.com.cn/quotes_service/api/json_v2.php/'
           'CN_MarketData.getKLineData?symbol=%s&scale=240&ma=no&datalen=%d'
           % (sym, max(days + 5, 60)))
    text = _curl(url)
    if not text:
        return None
    try:
        raw = json.loads(text)
    except Exception:
        return None
    rows = []
    for item in raw:
        try:
            date_str = str(item['day'])
            open_p = float(item['open'])
            close_p = float(item['close'])
            high_p = float(item['high'])
            low_p = float(item['low'])
            volume = float(item['volume'])
            amount_yi = volume * close_p * 1e-8
            rows.append([date_str, open_p, close_p, high_p, low_p, volume, amount_yi])
        except (ValueError, KeyError):
            continue
    if not rows:
        return None
    df = pd.DataFrame(rows, columns=['date', 'open', 'close', 'high', 'low', 'volume', 'amount_yi'])
    df = df.sort_values('date').reset_index(drop=True)
    if end_date:
        df = df[df['date'] <= end_date].reset_index(drop=True)
    return df


# ─────────────────────────────────────────────────────────────
# 过滤层
# ─────────────────────────────────────────────────────────────


def filter_main_board(stocks):
    """过滤1：仅保留主板（60/00开头）。"""
    return stocks


def filter_limit_up_recent(stocks, data_cache, days=7, threshold=9.8):
    """过滤2：近N个交易日有涨停（日线涨幅 ≥ threshold%）。"""
    print('    ┌─ 过滤2(涨停记忆): %d只 → 检查近%d日涨幅≥%.1f%%...' % (len(stocks), days, threshold))
    stock_names = {sc: nm for _, sc, nm in stocks}
    result = []
    for code, short_code, name in stocks:
        df_daily = data_cache.get(short_code)
        if df_daily is None or len(df_daily) < days + 1:
            continue
        closes = df_daily['close'].values
        returns = (closes[1:] - closes[:-1]) / closes[:-1] * 100
        recent = returns[-days:]
        if (recent >= threshold).any():
            result.append((code, short_code, name))
    eliminated = set(sc for _, sc, _ in stocks) - set(sc for _, sc, _ in result)
    if eliminated and len(stocks) - len(result) <= 50:
        for sc in sorted(eliminated):
            max_ret = 0
            df_daily = data_cache.get(sc)
            if df_daily is not None and len(df_daily) >= days + 1:
                closes = df_daily['close'].values
                returns = (closes[1:] - closes[:-1]) / closes[:-1] * 100
                max_ret = round(float(returns[-days:].max()), 2)
            print('    └── ❌ %s %s (近%d日最大涨幅%.2f%% < %.1f%%)' % (sc, stock_names.get(sc, ''), days, max_ret, threshold))
    elif len(stocks) - len(result) > 50:
        print('    └── ❌ 淘汰%d只 (略)' % (len(stocks) - len(result)))
    print('    └─ ✅ 过滤2通过: %d只' % len(result))
    return result


def filter_turnover(stocks, data_cache, min_turnover_yi=5.0):
    """过滤3：前一交易日成交额 ≥ min_turnover_yi 亿。"""
    print('    ┌─ 过滤3(成交额≥%.1f亿): %d只...' % (min_turnover_yi, len(stocks)))
    result = []
    eliminated_detail = []
    for code, short_code, name in stocks:
        df_daily = data_cache.get(short_code)
        if df_daily is None or df_daily.empty:
            eliminated_detail.append((short_code, name, 0))
            continue
        prev_amount_yi = df_daily['amount_yi'].iloc[-1]
        if prev_amount_yi >= min_turnover_yi:
            result.append((code, short_code, name))
        elif len(stocks) - len(result) <= 50:
            eliminated_detail.append((short_code, name, round(prev_amount_yi, 2)))
    if eliminated_detail and len(stocks) - len(result) <= 50:
        for sc, nm, amt in sorted(eliminated_detail, key=lambda x: -x[2]):
            print('    └── ❌ %s %s (成交额%.2f亿 < %.1f亿)' % (sc, nm, amt, min_turnover_yi))
    elif len(stocks) - len(result) > 50:
        print('    └── ❌ 淘汰%d只 (略)' % (len(stocks) - len(result)))
    print('    └─ ✅ 过滤3通过: %d只' % len(result))
    return result


def filter_ma20_above(stocks, data_cache):
    """
    过滤4：前一交易日收盘价 > MA20（中期趋势向上）。
    涨停股回调不破20日线，说明中期趋势完好。
    """
    ma_days = 20
    print('    ┌─ 过滤4(收盘>%d日线): %d只...' % (ma_days, len(stocks)))
    result = []
    eliminated_detail = []
    for code, short_code, name in stocks:
        df_daily = data_cache.get(short_code)
        if df_daily is None or len(df_daily) < ma_days:
            eliminated_detail.append((short_code, name, '日线不足%d日' % ma_days))
            continue
        ma20 = df_daily['close'].iloc[-ma_days:].mean()
        prev_close = df_daily['close'].iloc[-1]
        if prev_close > ma20:
            data_cache['_ma20_' + short_code] = round(float(ma20), 2)
            result.append((code, short_code, name))
        else:
            eliminated_detail.append((short_code, name, '收盘%.2f≤MA20%.2f' % (prev_close, ma20)))
    if eliminated_detail:
        for sc, nm, reason in sorted(eliminated_detail, key=lambda x: x[2] if '收盘' in x[2] else ''):
            print('    └── ❌ %s %s (%s)' % (sc, nm, reason))
    print('    └─ ✅ 过滤4通过: %d只' % len(result))
    return result


def filter_open_gap(stocks, data_cache, min_gap=0.01, max_gap=3.0):
    """
    过滤5：T0开盘竞价涨幅 0.01% ~ 3%。
    T0开盘价存在 data_cache['_t0_open_'+code] 中。
    """
    print('    ┌─ 过滤5(T0开盘涨幅%.2f%%~%.1f%%): %d只...' % (min_gap, max_gap, len(stocks)))
    result = []
    eliminated_detail = []
    for code, short_code, name in stocks:
        df_daily = data_cache.get(short_code)
        t0_open = data_cache.get('_t0_open_' + short_code)
        if df_daily is None or t0_open is None or len(df_daily) < 1:
            eliminated_detail.append((short_code, name, '无T0开盘数据'))
            continue
        prev_close = df_daily['close'].iloc[-1]
        t0_prev_close = data_cache.get('_t0_prev_close_' + short_code)
        if t0_prev_close is not None and t0_prev_close > 0:
            prev_close = t0_prev_close
        if prev_close == 0:
            eliminated_detail.append((short_code, name, 'T-1收盘价为0'))
            continue
        open_gap = (t0_open - prev_close) / prev_close * 100
        if min_gap <= open_gap <= max_gap:
            data_cache['_open_gap_' + short_code] = round(open_gap, 2)
            result.append((code, short_code, name))
        else:
            eliminated_detail.append((short_code, name, 'T0开盘涨幅%.2f%%' % open_gap))
    if eliminated_detail:
        for sc, nm, reason in sorted(eliminated_detail, key=lambda x: abs(float(x[2].split('涨幅')[-1].replace('%', '').split(' ')[0])) if '涨幅' in x[2] else 999):
            print('    └── ❌ %s %s (%s)' % (sc, nm, reason))
    print('    └─ ✅ 过滤5通过: %d只' % len(result))
    return result


# ─────────────────────────────────────────────────────────────
# 主函数
# ─────────────────────────────────────────────────────────────


def find_surge_stocks_with_filter(
    trade_date=None,
    min_prev_turnover_yi=5.0,
    limit_up_threshold=9.8,
    limit_up_days=7,       # 7日涨停记忆
):
    """
    执行完整选股链（纯日线版，无5分钟涨速）。
    trade_date: 交易日字符串 '2026-07-20'，不传默认为今天
    返回: DataFrame
    """
    import baostock as bs

    if trade_date is None:
        trade_date = datetime.date.today().strftime('%Y-%m-%d')

    t_start_total = time.time()
    _step_times = []  # [(name, t_start, t_end, in_cnt, out_cnt), ...]
    def _record_step(name, t_start, t_end, in_cnt, out_cnt):
        _step_times.append((name, t_start, t_end, in_cnt, out_cnt))

    t_bs = time.time()
    lg = bs.login()
    if lg.error_code != '0':
        print('[错误] Baostock 登录失败: %s' % lg.error_msg, file=sys.stderr)
        return pd.DataFrame()

    try:
        # 检查交易日
        rs = bs.query_trade_dates(start_date=trade_date, end_date=trade_date)
        is_trade_day = False
        while rs.next():
            row = rs.get_row_data()
            if len(row) >= 2 and row[0] == trade_date and row[1] == '1':
                is_trade_day = True
                break

        if not is_trade_day:
            t_bs_end = time.time()
            _record_step('Baostock登录&校验', t_bs, t_bs_end, 0, 0)
            print('\n[提示] %s 不是交易日' % trade_date)
            return pd.DataFrame()

        t_bs_end = time.time()
        _record_step('Baostock登录&校验', t_bs, t_bs_end, 0, 0)

        # ── 1. 获取股票池 ──
        print('=' * 80)
        print('T0开盘日线选股 | 基准日: %s' % trade_date)
        print('过滤链: 主板 → 涨停记忆(%d天≥%.1f%%) → 成交额≥%.1f亿 → 站上20日线 → T0开盘涨幅0.01%%~3.0%%'
              % (limit_up_days, limit_up_threshold, min_prev_turnover_yi))
        print('=' * 80)

        print('\n[1/6] 获取股票池...', end=' ')
        t1 = time.time()
        all_stocks = get_stock_pool(trade_date)
        main_board = filter_main_board(all_stocks)
        t1e = time.time()
        print('主板: %d 只' % len(main_board))
        print('  └─ 耗时: %.1fs' % (t1e - t1))
        _record_step('获取股票池', t1, t1e, len(all_stocks), len(main_board))

        if not main_board:
            print('\n总耗时: %.1fs' % (time.time() - t_start_total))
            print('[结果] 无主板股票数据')
            return pd.DataFrame()

        # ── 2. 批量获取日线（新浪API，并行20线程 + 文件缓存）──
        t2_start = time.time()
        _CACHE_FILE = '/tmp/t0_daily_cache.pkl'

        def _load_daily_cache():
            try:
                with open(_CACHE_FILE, 'rb') as f:
                    return pickle.load(f)
            except Exception:
                return {}

        def _save_daily_cache(cache):
            tmp = _CACHE_FILE + '.tmp'
            try:
                with open(tmp, 'wb') as f:
                    pickle.dump(cache, f)
                os.replace(tmp, _CACHE_FILE)
            except Exception:
                pass

        cached = _load_daily_cache()
        cache_key = 'daily_' + trade_date
        if cache_key in cached:
            print('\n[2/6] 读取日线缓存...', end=' ', flush=True)
            t2_read = time.time()
            data_cache = {}
            data_cache_lock = __import__('threading').Lock()
            for sc, df_dict in cached[cache_key].items():
                data_cache['_daily_full_' + sc] = pd.DataFrame(df_dict['full'])
                data_cache[sc] = pd.DataFrame(df_dict['short'])
            success = len(cached[cache_key])
            t2_read_end = time.time()
            print('命中缓存: %d只 (读取耗时 %.2fs)' % (success, t2_read_end - t2_read))
        else:
            print('\n[2/6] 并行获取日线(%d只, 20线程, 新浪API)...' % len(main_board), end=' ', flush=True)
            data_cache = {}
            data_cache_lock = __import__('threading').Lock()
            success = 0

            def _fetch_daily(code, short_code, name):
                df = get_daily_data(code, trade_date, days=25)
                if df is not None and len(df) >= 2:
                    with data_cache_lock:
                        data_cache['_daily_full_' + short_code] = df.copy()
                        data_cache[short_code] = df.iloc[:-1].reset_index(drop=True)
                    return 1
                return 0

            with ThreadPoolExecutor(max_workers=20) as executor:
                futures = [executor.submit(_fetch_daily, code, sc, name)
                           for code, sc, name in main_board]
                done_count = 0
                t_last_report = time.time()
                for f in as_completed(futures):
                    success += f.result()
                    done_count += 1
                    if done_count % 400 == 0:
                        now = time.time()
                        speed = done_count / (now - t_last_report + 0.001)
                        print('\n    └─ 进度: %d/%d (%.0f只/s, 已用%.0fs)' %
                              (done_count, len(main_board), speed, now - t2_start), end='', flush=True)
                        t_last_report = now

            # 写入缓存
            t_save = time.time()
            cache_entry = {}
            for sc in [item[1] for item in main_board]:
                full_key = '_daily_full_' + sc
                short_key = sc
                if full_key in data_cache and short_key in data_cache:
                    cache_entry[sc] = {
                        'full': data_cache[full_key].to_dict('list'),
                        'short': data_cache[short_key].to_dict('list'),
                    }
            if cache_entry:
                cached[cache_key] = cache_entry
                _save_daily_cache(cached)
                print('\n    └─ 缓存保存: %d只 (%.1fs)' % (len(cache_entry), time.time() - t_save))

            print('')
            print('    └─ 成功: %d / %d' % (success, len(main_board)))

        t2_end = time.time()
        print('    └─ 耗时: %.1fs' % (t2_end - t2_start))
        _record_step('获取日线数据', t2_start, t2_end, len(main_board), success)

        # ── 3. 日线过滤（涨停记忆 + 成交额 + MA20）──
        t3_start = time.time()
        print('\n[3/6] 日线过滤...')

        t_f2 = time.time()
        step1 = filter_limit_up_recent(main_board, data_cache, days=limit_up_days, threshold=limit_up_threshold)
        print('    └─ 耗时: %.1fs (输入%d -> 输出%d)' % (time.time() - t_f2, len(main_board), len(step1)))

        t_f3 = time.time()
        step2 = filter_turnover(step1, data_cache, min_turnover_yi=min_prev_turnover_yi)
        print('    └─ 耗时: %.1fs (输入%d -> 输出%d)' % (time.time() - t_f3, len(step1), len(step2)))

        if not step2:
            t3_end = time.time()
            print('    └─ 耗时合计: %.1fs' % (t3_end - t3_start))
            _record_step('日线过滤', t3_start, t3_end, len(main_board), 0)
            print('\n总耗时: %.1fs' % (time.time() - t_start_total))
            print('[结果] 日线过滤后无股票')
            return pd.DataFrame()

        t_f4 = time.time()
        step3 = filter_ma20_above(step2, data_cache)
        print('    └─ 耗时: %.1fs (输入%d -> 输出%d)' % (time.time() - t_f4, len(step2), len(step3)))

        if not step3:
            t3_end = time.time()
            print('    └─ 耗时合计: %.1fs' % (t3_end - t3_start))
            _record_step('日线过滤', t3_start, t3_end, len(main_board), 0)
            print('\n总耗时: %.1fs' % (time.time() - t_start_total))
            print('[结果] MA20过滤后无股票')
            return pd.DataFrame()

        t3_end = time.time()
        print('  └─ 日线过滤合计: %.1fs' % (t3_end - t3_start))
        _record_step('日线过滤', t3_start, t3_end, len(main_board), len(step3))

        # ── 4. 获取T0开盘/收盘数据 ──
        t4_start = time.time()
        print('\n[4/6] 获取T0数据...')
        is_today = trade_date == datetime.date.today().strftime('%Y-%m-%d')
        if is_today:
            print('  使用腾讯实时API(%d只)...' % len(step3), end=' ', flush=True)
            t0_stock_codes = [(sc, 'sh' if code.startswith('sh') else 'sz')
                              for code, sc, name in step3]
            t0_realtime = get_today_realtime_prices(t0_stock_codes)
            print('成功: %d / %d' % (len(t0_realtime), len(step3)))
            for sc, rt in t0_realtime.items():
                data_cache['_t0_open_' + sc] = rt['open']
                data_cache['_t0_close_' + sc] = rt['close']
                data_cache['_t0_prev_close_' + sc] = rt['prev_close']
        else:
            print('  使用日线数据(%d只)...' % len(step3), end=' ', flush=True)
            count = 0
            for code, sc, name in step3:
                full_df = data_cache.get('_daily_full_' + sc)
                if full_df is not None and len(full_df) >= 1:
                    data_cache['_t0_open_' + sc] = full_df['open'].iloc[-1]
                    data_cache['_t0_close_' + sc] = full_df['close'].iloc[-1]
                    data_cache['_t0_prev_close_' + sc] = full_df['close'].iloc[-2] if len(full_df) >= 2 else full_df['close'].iloc[-1]
                    count += 1
            print('成功: %d / %d' % (count, len(step3)))
        t4_end = time.time()
        print('  └─ 耗时: %.1fs' % (t4_end - t4_start))
        _record_step('获取T0数据', t4_start, t4_end, len(step3), len(step3))

        # ── 5. T0开盘高开过滤 ──
        t5_start = time.time()
        print('\n[5/6] T0开盘高开过滤...')

        step4 = filter_open_gap(step3, data_cache, min_gap=0.01, max_gap=3.0)

        if not step4:
            t5_end = time.time()
            print('  └─ 耗时: %.1fs' % (t5_end - t5_start))
            _record_step('T0开盘过滤', t5_start, t5_end, len(step3), 0)
            total_elapsed = time.time() - t_start_total
            print('\n总耗时: %.1fs' % total_elapsed)
            print('[结果] T0高开过滤后无股票')
            return pd.DataFrame()

        t5_end = time.time()
        print('  └─ 耗时: %.1fs (输入%d -> 输出%d)' % (t5_end - t5_start, len(step3), len(step4)))
        _record_step('T0开盘过滤', t5_start, t5_end, len(step3), len(step4))

        final_stocks = step4

        # ── 6. 组装最终结果 ──
        t6_start = time.time()
        print('\n[6/6] 组装结果...')
        rows = []
        for code, short_code, name in final_stocks:
            df_daily = data_cache.get(short_code)
            open_gap = data_cache.get('_open_gap_' + short_code)
            ma20 = data_cache.get('_ma20_' + short_code, 0)
            if df_daily is None or open_gap is None:
                continue

            prev_amount_yi = df_daily['amount_yi'].iloc[-1] if len(df_daily) >= 1 else 0
            prev_close = df_daily['close'].iloc[-1] if len(df_daily) >= 1 else 0
            prev_ret = (df_daily['close'].iloc[-1] - df_daily['close'].iloc[-2]) / df_daily['close'].iloc[-2] * 100 if len(df_daily) >= 2 else 0

            t0_close = data_cache.get('_t0_close_' + short_code)
            t0_close_ret = round((t0_close - prev_close) / prev_close * 100, 2) if t0_close and prev_close != 0 else 0

            # 检查近7日内是否有涨停（用于显示）
            closes = df_daily['close'].values
            returns = (closes[1:] - closes[:-1]) / closes[:-1] * 100
            recent_returns = returns[-limit_up_days:]
            limit_up_dates = []
            for i, r in enumerate(recent_returns):
                if r >= limit_up_threshold:
                    date_idx = len(df_daily) - limit_up_days + i
                    if date_idx < len(df_daily):
                        limit_up_dates.append(df_daily['date'].iloc[date_idx] if 'date' in df_daily.columns else '')
            limit_up_info = ', '.join(limit_up_dates[-3:]) if limit_up_dates else '-'

            market_suffix = '.XSHG' if code.startswith('sh') else '.XSHE'
            user_code = short_code + market_suffix

            rows.append({
                '时间': trade_date,
                'T0开盘涨幅(%)': open_gap,
                'T0收盘涨幅(%)': t0_close_ret,
                '涨停日期': limit_up_info,
                'MA20': round(ma20, 2) if ma20 else 0,
                '成交额(亿)': round(prev_amount_yi, 2),
                '股票代码': user_code,
                '股票名称': name,
                '前一交易日收盘': round(prev_close, 2),
                '前一交易日收盘涨幅(%)': round(prev_ret, 2),
            })

        if not rows:
            t6_end = time.time()
            _record_step('组装结果', t6_start, t6_end, len(final_stocks), 0)
            return pd.DataFrame()

        df_result = pd.DataFrame(rows)
        df_result = df_result.sort_values('成交额(亿)', ascending=False).reset_index(drop=True)

        t6_end = time.time()
        _record_step('组装结果', t6_start, t6_end, len(final_stocks), len(df_result))

        total_elapsed = time.time() - t_start_total
        print('\n' + '-' * 80)
        print('执行耗时明细')
        print('-' * 80)
        print('  %-20s %10s %10s %10s' % ('步骤', '耗时(s)', '输入', '输出'))
        print('  ' + '-' * 54)
        for name, ts, te, incnt, outcnt in _step_times:
            print('  %-20s %10.1f %10d %10d' % (name, te - ts, incnt, outcnt))
        print('  ' + '-' * 54)
        print('  %-20s %10.1f' % ('总计', total_elapsed))
        print('-' * 80)

        total_elapsed = time.time() - t_start_total
        print('\n总耗时: %.1fs' % total_elapsed)
        return df_result
    finally:
        bs.logout()


# ─────────────────────────────────────────────────────────────
# 入口
# ─────────────────────────────────────────────────────────────

if __name__ == '__main__':
    date_arg = sys.argv[1] if len(sys.argv) > 1 else None
    if date_arg:
        parts = date_arg.split('-')
        if len(parts) == 3:
            date_arg = '%s-%02d-%02d' % (parts[0], int(parts[1]), int(parts[2]))
    df = find_surge_stocks_with_filter(trade_date=date_arg)

    if df.empty:
        print('\n' + '=' * 80)
        print('无符合条件的股票')
        print('=' * 80)
    else:
        print('\n' + '=' * 80)
        print('选股结果: %d 只' % len(df))
        print('=' * 80)
        cols = ['时间', 'T0开盘涨幅(%)', 'T0收盘涨幅(%)', '涨停日期', 'MA20', '成交额(亿)',
                '股票代码', '股票名称', '前一交易日收盘', '前一交易日收盘涨幅(%)']
        pd.set_option('display.max_columns', 20)
        pd.set_option('display.width', 140)
        pd.set_option('display.max_rows', 200)
        print(df[cols].to_string(index=False))

        print('\n' + '=' * 80)
        print('预选股票列表（用于次日观察）：')
        print('=' * 80)
        for _, row in df.iterrows():
            print('%s  %s' % (row['股票代码'], row['股票名称']))
