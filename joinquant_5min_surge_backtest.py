# -*- coding: utf-8 -*-
"""
聚宽回测脚本：5分钟涨速榜 + 隔夜超短买卖纪律

说明：
1. 这份脚本基于原始“5分钟涨速榜选股.py”的过滤骨架扩展，原始文件不做修改。
2. 买入逻辑保留原来的主板、涨停记忆、成交额、均线、5分钟涨速、KDJ 等过滤。
3. 新增用户交易体系里的“该跌不跌”“该涨不涨”“暴涨/暴跌反身性”、5/10日线风险。
4. 新增真实回测交易流程：09:30 先卖、再买，盘中检查止盈/止损/卖点。
"""

from jqdata import *

import datetime

import numpy as np
import pandas as pd

try:
    import talib
except Exception:
    talib = None


def initialize(context):
    set_benchmark('000300.XSHG')
    set_option('use_real_price', True)
    log.info('5分钟涨速榜 + 隔夜超短买卖纪律策略初始化完成')

    set_order_cost(
        OrderCost(
            close_tax=0.001,
            open_commission=0.0003,
            close_commission=0.0003,
            min_commission=5,
        ),
        type='stock',
    )

    # ===== 可调参数 =====
    g.max_hold_stock_num = 3
    g.single_stock_position_pct = 0.05
    g.buy_cash_ratio = 0.95  # 兼容旧版参数；当前买入按单票总资产占比执行。
    g.max_hold_days = 3

    # 原策略参数：保留并略按你的常用候选条件调整。
    g.speed_threshold = 3.0
    g.min_prev_turnover = 3e8
    g.limit_up_threshold = 9.8
    g.limit_up_days = 20
    g.opening_min_rise = 0.0001
    g.opening_max_rise = 0.03
    g.require_morning_trigger = False
    g.strict_individual_ma_risk = True

    # 买点确认参数。
    g.max_buy_current_rise_pct = 4.5
    g.open_support_tolerance = 0.995
    g.kdj_max_k = 85

    # 卖点纪律参数。
    g.weak_feedback_body_pct = -1.0
    g.sell_low_open_pct = -2.0
    g.deep_low_open_pct = -3.0
    g.take_profit_pct = 5.0
    g.after_limit_take_profit_pct = 3.0
    g.broken_limit_retreat_pct = 8.2
    g.hard_stop_loss_pct = -4.0

    g.today_buy_list = []
    g.today_signal_map = {}
    g.today_report = pd.DataFrame()
    g.hold_state = {}

    run_daily(market_open_trade, time='09:30', reference_security='000300.XSHG')
    run_daily(intraday_sell_check, time='10:00', reference_security='000300.XSHG')
    run_daily(take_profit_check, time='14:50', reference_security='000300.XSHG')
    run_daily(after_market_close, time='after_close', reference_security='000300.XSHG')


def before_trading_start(context):
    g.today_buy_list = []
    g.today_signal_map = {}
    g.today_report = pd.DataFrame()
    sync_hold_state(context)


def market_open_trade(context):
    """09:30 开盘先卖后买；卖出只处理 A 股 T+1 下可卖仓位。"""
    sync_hold_state(context)
    run_sell_rules(context, stage='open')
    market_open_buy(context)


def market_open_sell(context):
    """09:30 先按昨日反馈和今日开盘纪律处理持仓。"""
    sync_hold_state(context)
    run_sell_rules(context, stage='open')


def market_open_buy(context):
    """09:30 卖出后按原涨速榜过滤 + 改进买点逻辑买入。"""
    sync_hold_state(context)
    trade_date = context.current_dt.strftime('%Y-%m-%d')

    market_warning = get_market_ma_warning(context)
    if market_warning:
        log.warn('[大盘预警] %s；个股模式内买点不直接否决，但买后纪律要更紧。' % market_warning)

    df = find_surge_stocks_with_filter(
        trade_date=trade_date,
        speed_threshold=g.speed_threshold,
        min_prev_turnover=g.min_prev_turnover,
        limit_up_threshold=g.limit_up_threshold,
        limit_up_days=g.limit_up_days,
        opening_min_rise=g.opening_min_rise,
        opening_max_rise=g.opening_max_rise,
    )
    g.today_report = df

    if df is None or df.empty:
        log.info('[买入] 今日无候选')
        return

    candidates = [code for code in df['股票代码'].tolist() if code not in context.portfolio.positions]
    if not candidates:
        log.info('[买入] 候选均已持仓或无可买标的')
        return

    current_data = get_current_data()
    positions_count = len(context.portfolio.positions)
    available_slots = max(g.max_hold_stock_num - positions_count, 0)
    if available_slots <= 0:
        log.info('[买入] 持仓数量已达上限: %s' % g.max_hold_stock_num)
        return

    buy_list = []
    for code in candidates:
        if len(buy_list) >= available_slots:
            break
        if not can_buy_stock(code, current_data):
            continue
        buy_list.append(code)

    if not buy_list:
        log.info('[买入] 候选无法交易或触发涨停/停牌过滤')
        return

    remaining_cash = context.portfolio.available_cash
    for code in buy_list:
        cash_per_stock = get_single_stock_buy_value(context, available_cash=remaining_cash)
        if cash_per_stock <= 0:
            log.info('[买入停止] 可用现金不足，停止后续买入')
            break
        signal = g.today_signal_map.get(code, '买点')
        log.info('[买入下单] %s %s 单票总仓位 %.1f%% 目标金额 %.2f' % (
            code,
            signal,
            g.single_stock_position_pct * 100,
            cash_per_stock,
        ))
        order_value(code, cash_per_stock)
        remaining_cash -= cash_per_stock
        g.hold_state[code] = {
            'buy_date': context.current_dt.date(),
            'buy_price': get_current_price(code, current_data),
            'signal': signal,
            'had_limit_up': False,
            'wait_repair': False,
        }


def intraday_sell_check(context):
    """10:00 检查深低开修复、5/10日线破位和盘中止盈。"""
    sync_hold_state(context)
    run_sell_rules(context, stage='mid')


def take_profit_check(context):
    """14:50 尾盘检查止盈、炸板回落、持仓周期。"""
    sync_hold_state(context)
    run_sell_rules(context, stage='late')


def after_market_close(context):
    trade_date = context.current_dt.strftime('%Y-%m-%d')
    log.info('========== %s 5分钟涨速榜选股 ==========' % trade_date)

    df = g.today_report
    if df is None or df.empty:
        df = find_surge_stocks_with_filter(
            trade_date=trade_date,
            speed_threshold=g.speed_threshold,
            min_prev_turnover=g.min_prev_turnover,
            limit_up_threshold=g.limit_up_threshold,
            limit_up_days=g.limit_up_days,
            opening_min_rise=g.opening_min_rise,
            opening_max_rise=g.opening_max_rise,
        )

    pd.set_option('display.max_rows', None)
    pd.set_option('display.max_columns', None)
    pd.set_option('display.width', None)
    pd.set_option('display.max_colwidth', 1000)
    pd.set_option('display.expand_frame_repr', False)

    print('\n【%s】满足条件的股票数: %s' % (trade_date, len(df)))
    if len(df) > 0:
        print('\n' + '=' * 160)
        print(df.to_string(index=False))
        print('=' * 160)


# =============================================================================
# 交易辅助
# =============================================================================


def sync_hold_state(context):
    """为已有持仓补齐状态，避免回测中途启动或状态缺失。"""
    for code, position in context.portfolio.positions.items():
        if position.total_amount <= 0:
            continue
        if code not in g.hold_state:
            g.hold_state[code] = {
                'buy_date': context.current_dt.date(),
                'buy_price': position.avg_cost,
                'signal': '已有持仓',
                'had_limit_up': False,
                'wait_repair': False,
            }

    for code in list(g.hold_state.keys()):
        if code not in context.portfolio.positions:
            g.hold_state.pop(code, None)


def can_buy_stock(code, current_data):
    data = get_current_data_for_code(current_data, code)
    if data is None:
        return False
    name = getattr(data, 'name', '')
    if data.paused or data.is_st or is_bad_stock_name(name):
        return False
    price = get_current_price(code, current_data)
    if price <= 0:
        return False
    if price >= data.high_limit * 0.995:
        log.info('[买入跳过] %s 已接近涨停，不追' % code)
        return False
    return True


def can_sell_stock(code, current_data):
    data = get_current_data_for_code(current_data, code)
    if data is None:
        return False
    if data.paused:
        return False
    price = get_current_price(code, current_data)
    if price <= 0:
        return False
    if price <= data.low_limit * 1.005:
        log.info('[卖出暂缓] %s 接近跌停，可能无法成交' % code)
        return False
    return True


def run_sell_rules(context, stage):
    current_data = get_current_data()
    for code in list(context.portfolio.positions.keys()):
        position = context.portfolio.positions[code]
        if position.total_amount <= 0:
            continue
        sellable_amount = get_sellable_amount(position)
        if sellable_amount <= 0:
            log.info('[卖出跳过-%s] %s 可卖数量为0，A股T+1不卖当天买入仓位' % (stage, code))
            continue
        if not can_sell_stock(code, current_data):
            continue

        reasons = collect_sell_reasons(context, code, stage, current_data)
        if not reasons:
            continue

        log.info('[卖出下单-%s] %s 可卖数量=%s 原因: %s' % (stage, code, sellable_amount, '；'.join(reasons)))
        order(code, -sellable_amount)
        if sellable_amount >= position.total_amount:
            g.hold_state.pop(code, None)


def get_sellable_amount(position):
    """A股 T+1：优先使用聚宽 position.closeable_amount，只卖可卖仓位。"""
    closeable_amount = getattr(position, 'closeable_amount', None)
    if closeable_amount is None:
        closeable_amount = getattr(position, 'total_amount', 0)
    try:
        return max(int(closeable_amount), 0)
    except Exception:
        return 0


def get_single_stock_buy_value(context, available_cash=None):
    """每只股票固定买入总资产的指定比例，默认 5%。"""
    if available_cash is None:
        available_cash = context.portfolio.available_cash
    total_value = getattr(context.portfolio, 'total_value', 0) or 0
    target_value = total_value * g.single_stock_position_pct
    return max(min(target_value, available_cash), 0)


def collect_sell_reasons(context, code, stage, current_data):
    reasons = []
    daily = get_single_daily_df(code, count=20, end_date=get_prev_trade_date(context))
    if daily is None or len(daily) < 10:
        return reasons

    state = g.hold_state.get(code, {})
    price = get_current_price(code, current_data)
    day_open = get_today_open_price(code, current_data)
    high_limit = current_data[code].high_limit

    prev_open = float(daily['open'].iloc[-1])
    prev_close = float(daily['close'].iloc[-1])
    ma5 = float(daily['close'].iloc[-5:].mean())
    ma10 = float(daily['close'].iloc[-10:].mean())

    open_pct = pct_change(day_open, prev_close)
    current_pct = pct_change(price, prev_close)
    pnl_pct = pct_change(price, state.get('buy_price', price))
    hold_days = get_hold_days(context, code)

    prev_feedback_pct = pct_change(prev_close, prev_open)
    weak_yesterday_feedback = prev_feedback_pct <= g.weak_feedback_body_pct

    # 更新是否曾经涨停。涨停封住时不急着卖。
    high_so_far = get_intraday_high_so_far(code, current_data, context)
    if high_so_far >= high_limit * 0.995:
        state['had_limit_up'] = True
        g.hold_state[code] = state

    near_limit_up_now = price >= high_limit * 0.995

    # 卖点1：昨日买入后反馈弱，次日开盘不修复，优先处理。
    if hold_days <= 1 and weak_yesterday_feedback:
        if g.sell_low_open_pct <= open_pct <= 0:
            reasons.append('昨日收盘低于开盘超过1%，今日开在0到-2%，按纪律处理')
        elif g.deep_low_open_pct < open_pct < g.sell_low_open_pct:
            reasons.append('昨日反馈弱，今日低开接近-2%，开盘处理')
        elif open_pct <= g.deep_low_open_pct:
            state['wait_repair'] = True
            g.hold_state[code] = state
            if stage in ('mid', 'late') and current_pct >= g.sell_low_open_pct:
                reasons.append('低开超过-3%后修复到-2%附近，优先卖出')
            elif stage == 'late':
                reasons.append('低开超过-3%且全天修复不足，尾盘纪律处理')
        elif open_pct > 0 and price > prev_open:
            log.info('[持有观察] %s 高开并盖过昨日开盘，昨日阴线实体在修复' % code)

    # 卖点2：个股同时跌破5日线和10日线，触发风险闸门。
    if price < ma5 and price < ma10:
        reasons.append('个股同时跌破5日线和10日线')

    # 卖点3：一个涨停后次日再有约3%收益，除非秒板封住。
    if state.get('had_limit_up') and current_pct >= g.after_limit_take_profit_pct and not near_limit_up_now:
        reasons.append('一个涨停后再有约3%，按你的模式止盈')

    # 卖点4：涨停炸板后回落到约+8%附近。
    if high_so_far >= high_limit * 0.995 and not near_limit_up_now and current_pct <= g.broken_limit_retreat_pct:
        reasons.append('涨停炸板后回落到约+8%附近')

    # 卖点5：快速拉到+4/+5进入止盈观察区，自动回测按+5附近兑现。
    if stage in ('mid', 'late') and current_pct >= g.take_profit_pct and not near_limit_up_now:
        reasons.append('快速拉到+5%左右，进入止盈区')

    # 基础回测风控：防止隔夜超短变成长线扛单。
    if pnl_pct <= g.hard_stop_loss_pct:
        reasons.append('回测风控止损 %.2f%%' % pnl_pct)

    if stage == 'late' and hold_days >= g.max_hold_days and not near_limit_up_now:
        reasons.append('持仓超过%s个交易日，隔夜超短尾盘退出' % g.max_hold_days)

    return unique_texts(reasons)


def get_hold_days(context, code):
    state = g.hold_state.get(code, {})
    buy_date = state.get('buy_date')
    if not buy_date:
        return 0
    if isinstance(buy_date, datetime.datetime):
        buy_date = buy_date.date()
    trade_days = get_trade_days(start_date=buy_date, end_date=context.current_dt.date())
    return max(len(trade_days) - 1, 0)


# =============================================================================
# 原选股过滤：保留骨架，补强细节
# =============================================================================


def filter_main_board(stocks):
    """过滤1：仅保留主板股票（60/00开头），排除创业板、科创板、北交所。"""
    return [s for s in stocks if s.startswith('60') or s.startswith('00')]


def filter_tradable_and_not_st(stocks):
    """过滤：排除停牌、ST、退市风险名称。"""
    current_data = get_current_data()
    result = []
    for code in stocks:
        data = get_current_data_for_code(current_data, code)
        if data is None:
            continue
        name = getattr(data, 'name', '')
        if data.paused or data.is_st or is_bad_stock_name(name):
            continue
        result.append(code)
    return result


def filter_float_market_cap(stocks, min_mv=60, max_mv=8000, date=None):
    """过滤：流通市值 60亿~8000亿。JoinQuant valuation 单位通常为亿元。"""
    if not stocks:
        return []
    try:
        q = query(valuation.code, valuation.circulating_market_cap).filter(valuation.code.in_(stocks))
        df = get_fundamentals(q, date=date)
    except Exception as err:
        log.warn('[流通市值过滤跳过] get_fundamentals失败: %s' % err)
        return stocks

    if df is None or df.empty:
        return stocks

    cap_map = dict(zip(df['code'], df['circulating_market_cap']))
    result = []
    for code in stocks:
        cap = cap_map.get(code)
        if cap is None or (min_mv <= cap <= max_mv):
            result.append(code)
    return result


def filter_limit_up_recent(stocks, daily_panel, days=20, threshold=9.8):
    """过滤2：近N个交易日内有过涨停/接近涨停。"""
    close_daily = daily_panel['close']
    result = []
    for code in stocks:
        if code not in close_daily.columns:
            continue
        daily_returns = close_daily[code].pct_change() * 100
        recent_returns = daily_returns.dropna().iloc[-days:]
        if (recent_returns >= threshold).any():
            result.append(code)
    return result


def filter_turnover(stocks, daily_panel, min_turnover=3e8):
    """过滤3：前一交易日成交额达标，确保流动性。"""
    money_daily = daily_panel['money']
    prev_turnover = money_daily.iloc[-1]
    result = []
    for code in stocks:
        if code not in prev_turnover.index:
            continue
        if prev_turnover[code] >= min_turnover:
            result.append(code)
    return result


def filter_ma20_above(stocks, daily_panel):
    """过滤4：前一交易日收盘价站在20日均线上方，处于相对趋势内。"""
    close_daily = daily_panel['close']
    result = []
    for code in stocks:
        if code not in close_daily.columns:
            continue
        ma20 = close_daily[code].mean()
        prev_close = close_daily[code].iloc[-1]
        if prev_close > ma20:
            result.append(code)
    return result


def filter_surge_speed(stocks, close_5m, threshold=3.0):
    """过滤5：前一交易日5分钟涨速≥阈值。"""
    result = []
    for code in stocks:
        if code not in close_5m.columns:
            continue
        returns_5m = close_5m[code].pct_change() * 100
        if (returns_5m.dropna() >= threshold).any():
            result.append(code)
    return result


def filter_time_before_10am(stocks, returns_5m):
    """过滤6：前一交易日5分钟涨速最大时刻在10:00前。"""
    result = []
    for code in stocks:
        if code not in returns_5m.columns:
            continue
        series_5m = returns_5m[code].dropna()
        if series_5m.empty:
            continue
        max_time = series_5m.idxmax()
        if max_time.hour < 10:
            result.append(code)
    return result


def filter_ma10_above_at_trigger(stocks, close_5m, daily_panel):
    """过滤7：前一交易日5分钟涨速最大时的价格站在10日均线上方。"""
    close_daily = daily_panel['close']
    result = []
    for code in stocks:
        if code not in close_5m.columns or code not in close_daily.columns:
            continue
        ma10 = close_daily[code].iloc[-10:].mean()
        series_5m = close_5m[code]
        max_time = series_5m.idxmax()
        trigger_price = close_5m[code][max_time]
        if trigger_price > ma10:
            result.append(code)
    return result


def filter_opening_range(stocks, today_panel, daily_panel, min_rise=0.0001, max_rise=0.03):
    """过滤8：今日开盘涨幅在0.01%~3%之间，高开但不过度高开。"""
    today_open = today_panel['open'].iloc[0]
    close_daily = daily_panel['close']
    result = []
    for code in stocks:
        if code not in today_open.index or code not in close_daily.columns:
            continue
        prev_close = close_daily[code].iloc[-1]
        if prev_close <= 0:
            continue
        opening_rise = (today_open[code] - prev_close) / prev_close
        if min_rise <= opening_rise <= max_rise:
            result.append(code)
    return result


def filter_kdj_k_below(stocks, daily_panel, max_k=85):
    """过滤9：前一交易日KDJ的K值≤85，避免超买状态。"""
    high_daily = daily_panel['high']
    low_daily = daily_panel['low']
    close_daily = daily_panel['close']
    result = []
    for code in stocks:
        if code not in high_daily.columns:
            continue
        k_value, d_value, _ = calc_kdj(
            high_daily[code].values,
            low_daily[code].values,
            close_daily[code].values,
        )
        if np.isnan(k_value):
            continue
        if k_value <= max_k:
            result.append(code)
    return result


def filter_buy_signal(stocks, daily_panel, today_panel, trade_date):
    """
    过滤10：买点判断。
    这里回归用户最初可选出股票的版本，仅基于 daily_panel + today_panel。
    - 该跌不跌(下跌中继): T-1大阴线(跌幅≥3%) + 今日高开。
    - 暴跌加暴跌: 连续两天大跌(各>2%,累计>5%) + 今日有反弹迹象(开盘>-1%)。
    """
    close_daily = daily_panel['close']
    today_open = today_panel['open'].iloc[0]
    result = []
    g.today_signal_map = {}

    for code in stocks:
        if code not in close_daily.columns or code not in today_open.index:
            continue

        closes = close_daily[code].tolist()
        if len(closes) < 3:
            continue

        prev_close = closes[-1]
        prev_prev_close = closes[-2]
        prev_prev_prev_close = closes[-3]
        if min(prev_close, prev_prev_close, prev_prev_prev_close) <= 0:
            continue

        prev_return = (prev_close - prev_prev_close) / prev_prev_close * 100
        prev_prev_return = (prev_prev_close - prev_prev_prev_close) / prev_prev_prev_close * 100

        today_open_price = today_open[code]
        opening_rise = (today_open_price - prev_close) / prev_close * 100

        is_buy_signal = False
        signal_type = ''

        if prev_return <= -3.0 and opening_rise > 0:
            is_buy_signal = True
            signal_type = '该跌不跌(中继)'
            log.info('[买点-%s] %s T-1涨幅=%.2f%% T-2涨幅=%.2f%% 开盘涨幅=%.2f%%' % (
                signal_type,
                code,
                prev_return,
                prev_prev_return,
                opening_rise,
            ))
        else:
            log.info('[过滤10-思路2不满足] %s: T-1涨幅=%.2f%% 开盘涨幅=%.2f%% (需T-1≤-3%%且开盘>0)' % (
                code,
                prev_return,
                opening_rise,
            ))

        if not is_buy_signal and prev_return < -2.0 and prev_prev_return < -2.0:
            total_drop = prev_return + prev_prev_return
            if total_drop < -5.0 and opening_rise > -1.0:
                is_buy_signal = True
                signal_type = '暴跌加暴跌'
                log.info('[买点-%s] %s T-1涨幅=%.2f%% T-2涨幅=%.2f%% 累计=%.2f%% 开盘涨幅=%.2f%%' % (
                    signal_type,
                    code,
                    prev_return,
                    prev_prev_return,
                    total_drop,
                    opening_rise,
                ))
            else:
                log.info('[过滤10-思路3不满足] %s: T-1=%.2f%% T-2=%.2f%% 累计=%.2f%% 开盘=%.2f%% (需累计<-5%%且开盘>-1%%)' % (
                    code,
                    prev_return,
                    prev_prev_return,
                    total_drop,
                    opening_rise,
                ))

        if is_buy_signal:
            result.append(code)
            g.today_signal_map[code] = signal_type
    return result


def analyze_buy_signal(code, daily_panel, today_open_map, today_last_map):
    open_daily = daily_panel['open']
    high_daily = daily_panel['high']
    low_daily = daily_panel['low']
    close_daily = daily_panel['close']

    if code not in close_daily.columns or code not in today_open_map:
        return False, '', '缺少日线或开盘数据'

    opens = open_daily[code].dropna()
    highs = high_daily[code].dropna()
    lows = low_daily[code].dropna()
    closes = close_daily[code].dropna()
    if len(closes) < 20 or len(opens) < 20:
        return False, '', '日线数量不足'

    prev_open = float(opens.iloc[-1])
    prev_close = float(closes.iloc[-1])
    prev_prev_close = float(closes.iloc[-2])
    prev_prev_prev_close = float(closes.iloc[-3])
    today_open = float(today_open_map[code])
    today_last = float(today_last_map.get(code, today_open))

    if min(prev_open, prev_close, prev_prev_close, today_open, today_last) <= 0:
        return False, '', '价格异常'

    prev_return = pct_change(prev_close, prev_prev_close)
    prev_prev_return = pct_change(prev_prev_close, prev_prev_prev_close)
    opening_rise = pct_change(today_open, prev_close)
    current_rise = pct_change(today_last, prev_close)

    prev_bull_body = bull_body_pct(prev_open, prev_close)
    prev_bear_body = bear_body_pct(prev_open, prev_close)
    prev_prev_bull_body = bull_body_pct(float(opens.iloc[-2]), float(closes.iloc[-2]))

    ma5 = float(closes.iloc[-5:].mean())
    ma10 = float(closes.iloc[-10:].mean())
    below_5_10 = today_open < ma5 and today_open < ma10
    if g.strict_individual_ma_risk and below_5_10:
        return False, '', '今日开盘同时低于5日线和10日线，个股风险闸门触发'

    if today_last < today_open * g.open_support_tolerance:
        return False, '', '开盘后承接走弱，低于开盘价过多'

    if current_rise > g.max_buy_current_rise_pct:
        return False, '', '当前涨幅 %.2f%% 已超过追买上限' % current_rise

    high_level_hold = is_high_level_should_fall_but_not(
        highs=highs,
        closes=closes,
        today_open=today_open,
        prev_close=prev_close,
    )
    weak_yesterday = prev_return <= -3.0 or prev_bear_body > 2.0 or is_late_selloff(prev_open, float(highs.iloc[-1]), prev_close)
    double_drop = prev_return < -2.0 and prev_prev_return < -2.0 and (prev_return + prev_prev_return) < -5.0
    strong_yesterday = prev_return >= 9.8 or prev_bull_body > 2.0
    just_first_big_up = (prev_return >= 9.8 or prev_bull_body > 4.0) and not high_level_hold
    double_big_up = prev_bull_body > 4.0 and prev_prev_bull_body > 4.0

    if double_big_up:
        return False, '', '暴涨加暴涨主要是卖点/防追高，不作为买入依据'

    if just_first_big_up:
        return False, '', '前一天刚暴涨或涨停，不能直接当作该跌不跌买点'

    if strong_yesterday and opening_rise < 0.5 and not high_level_hold:
        return False, '', '昨日强而今日溢价不足，触发该涨不涨风险'

    if high_level_hold and g.opening_min_rise * 100 <= opening_rise <= g.opening_max_rise * 100:
        reason = '前面涨过后浅回撤横住，今日开盘 %.2f%% 再次上冲，5/10日线未同时破位' % opening_rise
        return True, '该跌不跌(高位横住)', reason

    if weak_yesterday and g.opening_min_rise * 100 <= opening_rise <= g.opening_max_rise * 100:
        reason = '昨日弱势/中阴，正常应弱，今日开盘 %.2f%% 且承接未破' % opening_rise
        return True, '该跌不跌(弱转强)', reason

    if double_drop and opening_rise > -1.0 and current_rise > -0.5:
        reason = '连续两日下跌 %.2f%%，今日开盘 %.2f%% 有修复' % (prev_return + prev_prev_return, opening_rise)
        return True, '暴跌再暴跌修复', reason

    return False, '', '无该跌不跌/暴跌修复买点，T-1 %.2f%%，开盘 %.2f%%' % (prev_return, opening_rise)


def find_surge_stocks_with_filter(
    trade_date,
    speed_threshold=3.0,
    min_prev_turnover=3e8,
    limit_up_threshold=9.8,
    limit_up_days=20,
    opening_min_rise=0.0001,
    opening_max_rise=0.03,
):
    """
    5分钟涨速榜选股逻辑。

    基准日：
    - 选股链路回归用户最初可选出股票的版本。
    - 过滤1-6和8基于前一交易日数据计算。
    - 过滤7基于今日1日线开盘数据。
    """
    stocks = get_all_securities(types=['stock'], date=trade_date).index.tolist()
    print('[初始] 全市场股票数: %s' % len(stocks))

    trade_dt = datetime.datetime.strptime(trade_date, '%Y-%m-%d').date()
    trade_days = get_trade_days(end_date=trade_dt, count=5)
    if len(trade_days) < 2:
        return pd.DataFrame()
    prev_date = trade_days[-2]
    print('[基准] 前一交易日: %s' % prev_date)

    # --- 过滤条件1: 仅10%涨跌幅限制的主板股票（60/00开头） ---
    stocks = filter_main_board(stocks)
    print('[过滤1] 主板股票数（60/00开头）: %s' % len(stocks))

    daily_panel = get_price(
        stocks,
        count=21,
        end_date=str(prev_date),
        frequency='1d',
        fields=['close', 'high', 'low', 'money'],
        panel=True,
    )

    # --- 过滤条件2: 近N个交易日有过涨停（涨幅≥9.8%） ---
    stocks = filter_limit_up_recent(stocks, daily_panel, days=limit_up_days, threshold=limit_up_threshold)
    print('[过滤2] 近%s日有涨停（≥%.1f%%）: %s' % (limit_up_days, limit_up_threshold, len(stocks)))

    # --- 过滤条件3: 前一日成交额 ---
    stocks = filter_turnover(stocks, daily_panel, min_turnover=min_prev_turnover)
    print('[过滤3] 前一日成交额≥%.0f亿: %s' % (min_prev_turnover / 1e8, len(stocks)))

    # --- 过滤条件4: 前一日收盘价 > 20日均线 ---
    stocks = filter_ma20_above(stocks, daily_panel)
    print('[过滤4] 收盘价站在20日线上: %s' % len(stocks))

    if not stocks:
        return pd.DataFrame()

    prev_start_time = '%s 09:30:00' % prev_date
    prev_end_time = '%s 15:00:00' % prev_date

    prev_min5_panel = get_price(
        stocks,
        start_date=prev_start_time,
        end_date=prev_end_time,
        frequency='5m',
        fields=['close'],
        panel=True,
        skip_paused=False,
    )
    prev_close_5m = prev_min5_panel['close']
    prev_returns_5m = prev_close_5m.pct_change() * 100

    # --- 过滤条件5: 前一交易日5分钟涨速 ≥ 阈值 ---
    stocks = filter_surge_speed(stocks, prev_close_5m, threshold=speed_threshold)
    print('[过滤5] 前一交易日5分钟涨速≥%.1f%%: %s' % (speed_threshold, len(stocks)))

    # --- 过滤条件6: 前一交易日触发时间在10:00前（默认保留为可选） ---
    if g.require_morning_trigger:
        stocks = filter_time_before_10am(stocks, prev_returns_5m)
        print('[过滤6] 前一交易日触发时间在10:00前: %s' % len(stocks))

    if not stocks:
        return pd.DataFrame()

    today_panel = get_price(
        stocks,
        start_date=trade_date,
        end_date=trade_date,
        frequency='1d',
        fields=['open', 'close'],
        panel=True,
    )

    # --- 过滤条件7: 今日开盘涨幅 0.01%~3% ---
    stocks = filter_opening_range(
        stocks,
        today_panel,
        daily_panel,
        min_rise=opening_min_rise,
        max_rise=opening_max_rise,
    )
    print('[过滤7] 今日开盘涨幅%.2f%%~%.2f%%: %s' % (opening_min_rise * 100, opening_max_rise * 100, len(stocks)))

    # --- 过滤条件8: 前一交易日触发时价格 > 10日均线 ---
    stocks = filter_ma10_above_at_trigger(stocks, prev_close_5m, daily_panel)
    print('[过滤8] 前一交易日触发时价格站在10日线上: %s' % len(stocks))

    # --- 过滤条件9: 前一交易日KDJ的K值 ≤ 85（避免超买） ---
    stocks = filter_kdj_k_below(stocks, daily_panel, max_k=g.kdj_max_k)
    print('[过滤9] 前一交易日KDJ的K值≤%s: %s' % (g.kdj_max_k, len(stocks)))

    # --- 过滤条件10: 买点判断 ---
    stocks = filter_buy_signal(stocks, daily_panel, today_panel, trade_date)
    print('[过滤10] 买点信号: %s' % len(stocks))

    if not stocks:
        return pd.DataFrame()

    close_daily = daily_panel['close']
    high_daily = daily_panel['high']
    low_daily = daily_panel['low']
    money_daily = daily_panel['money']
    today_open = today_panel['open'].iloc[0]
    today_close = today_panel['close'].iloc[0]

    records = []
    for code in stocks:
        prev_close = close_daily[code].iloc[-1]
        prev_turnover = money_daily.iloc[-1][code]

        series_5m = prev_returns_5m[code].dropna()
        max_time = series_5m.idxmax()
        max_return = series_5m.max()
        opening_rise = (today_open[code] - prev_close) / prev_close
        close_rise = (today_close[code] - prev_close) / prev_close

        k_value, d_value, j_value = calc_kdj(
            high_daily[code].values,
            low_daily[code].values,
            close_daily[code].values,
        )

        records.append({
            '前一交易日5分钟涨幅(%)': round(max_return, 2),
            '前一交易日触发时间': max_time.strftime('%H:%M'),
            '成交额(亿)': round(prev_turnover / 1e8, 2),
            '股票代码': code,
            '股票名称': get_security_info(code).display_name,
            '今日开盘': round(today_open[code], 2),
            '开盘涨幅(%)': round(opening_rise * 100, 2),
            '今日收盘涨幅(%)': round(close_rise * 100, 2),
            '买点信号': g.today_signal_map.get(code, ''),
            'KDJ(K,D,J)': '(%s,%s,%s)' % (round(k_value, 1), round(d_value, 1), round(j_value, 1)),
        })

    result = pd.DataFrame(records)
    result = result[[
        '前一交易日5分钟涨幅(%)',
        '前一交易日触发时间',
        '成交额(亿)',
        '股票代码',
        '股票名称',
        '今日开盘',
        '开盘涨幅(%)',
        '今日收盘涨幅(%)',
        '买点信号',
        'KDJ(K,D,J)',
    ]]
    result = result.sort_values('前一交易日5分钟涨幅(%)', ascending=False)
    print('[最终] 满足所有条件的股票数: %s' % len(result))
    return result


# =============================================================================
# 指标与规则函数
# =============================================================================


def is_high_level_should_fall_but_not(highs, closes, today_open, prev_close):
    """
    前面已经涨过，正常应回落，但后面浅回撤横住，今天再次小幅高开。
    """
    if len(closes) < 8:
        return False

    close_returns = closes.pct_change() * 100
    recent_returns = close_returns.dropna().iloc[-7:]
    had_limit_or_big_memory = (recent_returns.iloc[:-2] >= 9.8).any()

    base = closes.iloc[-7]
    recent_gain = pct_change(closes.iloc[-3], base) if base > 0 else 0
    had_range_gain = recent_gain >= 10
    if not (had_limit_or_big_memory or had_range_gain):
        return False

    recent_high = float(highs.iloc[-7:].max())
    last_two_min_close = float(closes.iloc[-2:].min())
    pullback_pct = (recent_high - last_two_min_close) / recent_high * 100 if recent_high > 0 else 0
    hold_range_pct = abs(pct_change(float(closes.iloc[-1]), float(closes.iloc[-2])))
    opening_rise = pct_change(today_open, prev_close)

    shallow_pullback = pullback_pct <= 8.0
    sideways_hold = hold_range_pct <= 4.5
    opening_confirm = g.opening_min_rise * 100 <= opening_rise <= g.opening_max_rise * 100

    return shallow_pullback and sideways_hold and opening_confirm


def is_late_selloff(day_open, day_high, day_close):
    if min(day_open, day_high, day_close) <= 0:
        return False
    high_to_close = pct_change(day_high, day_close)
    close_vs_open = pct_change(day_close, day_open)
    return high_to_close >= 4.0 and close_vs_open < 1.0


def bull_body_pct(day_open, day_close):
    if day_open <= 0 or day_close <= day_open:
        return 0.0
    return (day_close - day_open) / day_open * 100


def bear_body_pct(day_open, day_close):
    if day_open <= 0 or day_close >= day_open:
        return 0.0
    return (day_open - day_close) / day_open * 100


def calc_kdj(high_arr, low_arr, close_arr):
    high_arr = np.array(high_arr, dtype=float)
    low_arr = np.array(low_arr, dtype=float)
    close_arr = np.array(close_arr, dtype=float)

    if len(close_arr) < 9:
        return np.nan, np.nan, np.nan

    if talib is not None:
        slowk, slowd = talib.STOCH(
            high_arr,
            low_arr,
            close_arr,
            fastk_period=9,
            slowk_period=3,
            slowk_matype=0,
            slowd_period=3,
            slowd_matype=0,
        )
        k_value = slowk[-1]
        d_value = slowd[-1]
        j_value = 3 * k_value - 2 * d_value
        return k_value, d_value, j_value

    high_s = pd.Series(high_arr)
    low_s = pd.Series(low_arr)
    close_s = pd.Series(close_arr)
    low_min = low_s.rolling(9).min()
    high_max = high_s.rolling(9).max()
    rsv = (close_s - low_min) / (high_max - low_min) * 100
    k = rsv.ewm(com=2, adjust=False).mean()
    d = k.ewm(com=2, adjust=False).mean()
    j = 3 * k - 2 * d
    return float(k.iloc[-1]), float(d.iloc[-1]), float(j.iloc[-1])


def get_market_ma_warning(context):
    try:
        prev_date = get_prev_trade_date(context)
        df = get_single_daily_df('000001.XSHG', count=11, end_date=prev_date)
        if df is None or len(df) < 10:
            return ''
        current_data = get_current_data()
        price = get_current_price('000001.XSHG', current_data)
        if price <= 0:
            price = float(df['close'].iloc[-1])
        ma5 = float(df['close'].iloc[-5:].mean())
        ma10 = float(df['close'].iloc[-10:].mean())
        if price < ma5 and price < ma10:
            return '上证指数同时跌破5日线和10日线'
    except Exception as err:
        log.warn('[大盘预警计算失败] %s' % err)
    return ''


# =============================================================================
# 数据和兼容函数
# =============================================================================


def get_today_open_map(stocks):
    current_data = get_current_data()
    result = {}
    for code in stocks:
        result[code] = get_today_open_price(code, current_data)
    return result


def get_today_last_map(stocks):
    current_data = get_current_data()
    result = {}
    for code in stocks:
        result[code] = get_current_price(code, current_data)
    return result


def get_today_open_price(code, current_data):
    data = get_current_data_for_code(current_data, code)
    if data is None:
        return 0.0
    day_open = getattr(data, 'day_open', 0) or 0
    if day_open and day_open > 0:
        return float(day_open)
    return get_current_price(code, current_data)


def get_current_price(code, current_data):
    data = get_current_data_for_code(current_data, code)
    if data is None:
        return 0.0
    price = getattr(data, 'last_price', 0) or 0
    if price and price > 0:
        return float(price)
    day_open = getattr(data, 'day_open', 0) or 0
    return float(day_open or 0)


def get_current_data_for_code(current_data, code):
    """聚宽 current_data 支持按代码取值，但 membership 判断不稳定。"""
    try:
        return current_data[code]
    except Exception:
        return None


def get_intraday_high_so_far(code, current_data, context=None):
    try:
        if context is not None:
            start = '%s 09:30:00' % context.current_dt.strftime('%Y-%m-%d')
            end = context.current_dt.strftime('%Y-%m-%d %H:%M:%S')
            hist = get_price(
                code,
                start_date=start,
                end_date=end,
                frequency='1m',
                fields=['high'],
                panel=False,
                skip_paused=True,
            )
        else:
            hist = attribute_history(code, count=60, unit='1m', fields=['high'], skip_paused=True)
        if hist is not None and len(hist) > 0:
            return float(hist['high'].max())
    except Exception:
        pass
    return get_current_price(code, current_data)


def get_prev_trade_date(context):
    trade_days = get_trade_days(end_date=context.current_dt.date(), count=2)
    if len(trade_days) < 2:
        return context.current_dt.date()
    return trade_days[-2]


def get_single_daily_df(code, count, end_date):
    try:
        df = get_price(
            code,
            count=count,
            end_date=str(end_date),
            frequency='1d',
            fields=['open', 'close', 'high', 'low', 'money'],
            panel=False,
            skip_paused=False,
        )
        return df
    except Exception as err:
        log.warn('[日线获取失败] %s %s' % (code, err))
        return None


def pct_change(new_value, old_value):
    if old_value is None or old_value == 0:
        return 0.0
    return (float(new_value) - float(old_value)) / float(old_value) * 100


def is_bad_stock_name(name):
    if name is None:
        return False
    bad_words = ['ST', '*ST', '退']
    return any(word in str(name) for word in bad_words)


def unique_texts(items):
    result = []
    for item in items:
        if item not in result:
            result.append(item)
    return result
