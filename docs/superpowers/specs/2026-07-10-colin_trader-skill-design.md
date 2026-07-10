# colin_trader Trae Skill Design

## Goal

Create a Trae-compatible skill for the user's personal overnight ultra-short trading system.

The skill helps with:

- Morning auction/opening buy analysis.
- Buy observation before 09:50 when the stock remains in the 0.01%-3% zone.
- Next-day sell decisions for positions bought yesterday.
- Post-trade review and rule learning from user-provided examples.

The skill must preserve the user's original trading language while translating it into executable reasoning rules. It is a decision-support and review assistant, not a deterministic prediction engine.

## Non-Goals

- Do not promise that a stock will rise or fall.
- Do not turn the user's common candidate filters into absolute buy rules.
- Do not build a full auto-trading system.
- Do not require the skill to run only inside the current go-stock project.
- Do not give position sizing advice by default. The user chose not to include sizing recommendations.

## Trading Profile

The user is an overnight ultra-short trader:

- Usually buys during 09:25 auction, near the 09:30 open, or before 09:50.
- Usually sells or decides whether to hold the next morning.
- Values discipline over hope when yesterday's buy feedback is wrong.
- Uses Granville's eight rules as a position framework, but combines them with market environment, expectation gaps, and personal sell rules.

## Skill Identity

Recommended skill metadata:

```markdown
---
name: colin_trader
description: 隔夜超短竞价交易助手。用于早盘竞价/开盘买入判断、9:50前观察买点、昨日买入个股次日卖出判断、盘后复盘学习。核心基于竞价超预期、该跌不跌、该涨不涨、格兰维尔八大法则、大盘过滤、5/10日线风险闸门、连续暴涨/暴跌反身性和用户个人卖出纪律。
---
```

## Output Contract

When the user asks "can I buy", "should I sell", "how does this stock look", or similar questions, the skill must answer in two stages:

1. Direct conclusion first.
   - Example: `结论：符合你的常用候选条件，但大盘触发 5/10 日线风险，不建议竞价直接买。`
   - Example: `结论：昨日买点反馈失败，若今日低开接近 -2%，优先按纪律处理。`
2. Rule breakdown second.
   - Common candidate conditions.
   - Core expectation-gap logic.
   - Market environment.
   - 5/10-day moving-average risk gate.
   - Granville position.
   - Buy or sell scenario script.
   - Missing data.

The answer must explain which rule was triggered. It must not only say "risk is high" or "observe first".

## Data Acquisition Design

The skill should not be tied to only one project. It should work in Trae or other environments by using whatever market data access is available.

Required data for buy analysis:

- Stock code or name.
- Current price, current percent change, open, previous close, high, low, amount, update time.
- 09:25 auction percent change when available.
- Recent 20-60 daily K lines with open, high, low, close, amount, and percent change.
- 5-day and 10-day moving averages.
- Whether it is main board.
- Whether it is ST or delisting-risk.
- Float market value.
- Previous trading day amount.
- Sector/theme and recent theme strength when available.

Required data for sell analysis:

- User's buy date.
- User's buy price or approximate buy percent change.
- Yesterday's open and close.
- Today's auction/open/current percent change.
- Today's high and whether it hit limit-up.
- Whether limit-up opened and how far it fell after opening.
- Current 5-day and 10-day moving average status.
- Market/index status.

Required data for market filter:

- Shanghai Composite, Shenzhen Component, ChiNext Index, or user-specified index.
- Real-time index position versus 5-day and 10-day moving averages.
- Recent index K lines.
- Market emotion indicators when available: up/down count, limit-up/down count, opened limit-up count, bearish/bullish unusual moves, and market amount.

Tool priority:

- If `QueryStockPriceInfo` is available, use it for real-time stock data.
- If `QueryStockKLine` is available, use it for recent K lines.
- If natural-language stock screening is available, use it for candidate discovery.
- If project tools are unavailable, search for or use other available data sources.
- If exact auction data is unavailable, use the open price versus previous close as an approximation and clearly label it as approximate.

If critical data is missing, the skill must say: `当前缺少 X，因此只能做条件判断。`

## Trading Time Windows

The skill must not treat 09:25 as the only possible buy time.

- 09:25 auction: highest-priority observation point.
- Around 09:30 open: main buy window, because many qualified stocks pull up immediately after the open.
- Before 09:50: still valid for observation if the stock remains around 0.01%-3%, has not lost support, and still shows buying intent.
- After 09:50: not a default first-buy window unless the user explicitly asks about an intraday opportunity.

## Common Candidate Conditions

These are the user's common screening conditions, not absolute buy rules:

1. At 09:25 or in the early observation window, percent change is between 0.01% and 3%.
2. During the previous 7 trading days, at least one day rose more than 9.8%.
3. Previous trading day amount is greater than 500 million RMB.
4. Float market value is between 6 billion and 800 billion RMB.
5. Main board only.
6. Exclude ST, delisting-risk, and abnormal liquidity.

If a stock does not meet these conditions, the skill must say it does not match the user's common candidate pool, but it must still analyze it with the core logic if the user asks.

## K-Line Strength Definitions

All middle/large candle definitions use real body first, not only full-day percent change.

Body formulas:

- Bullish body percent = `(close - open) / open * 100%`.
- Bearish body percent = `(open - close) / open * 100%`.

Definitions:

- 中阳线: bullish candle and body percent greater than 2%.
- 中阴线: bearish candle and body percent greater than 2%.
- 暴涨: large bullish candle and body percent greater than 4%.
- 暴跌: large bearish candle and body percent greater than 4%.

Notes:

- Long shadows do not count as body.
- A strong intraday spike that closes weakly does not count as a large bullish body.
- A low-open high-close day can count as 中阳线 or 暴涨 if the body threshold is met.

## Normal Expectation Inference

The skill must compare yesterday's state with today's auction/open. The key question is: `what should have happened today, and did the stock behave stronger or weaker than that?`

Weak yesterday normally implies weak next-day expectation:

- Yesterday limit-down, 中阴线, 暴跌, or late-day selloff normally implies today should open lower or continue weak.
- If today instead opens up 0.01%-3%, this can trigger "该跌不跌".

Strong yesterday normally implies strong next-day expectation:

- Yesterday limit-up, 中阳线, 暴涨, breakout, or sector climax normally implies today should have premium.
- If today is flat, low, or only weakly up, this can trigger "该涨不涨".

Special case for positions bought yesterday:

- If yesterday's close is more than 1% below yesterday's open, yesterday's intraday feedback is weak.
- The next day, if it opens between 0% and -2%, the user can directly handle or sell near the open.
- If the next day opens high and price moves above yesterday's open, the user can hold and observe because yesterday's bearish body is being repaired.

## Core Rule 1: 该跌不跌必有大涨

Preserve the user's original phrase:

> 该跌不跌必有大涨。

Executable meaning:

When yesterday's condition was weak and the normal expectation is that today's stock should open lower or continue falling, but today's auction/open is instead up 0.01%-3%, the stock is strongly above expectation.

Typical cases:

- Yesterday hit limit-down, but today opens slightly higher.
- Yesterday was a large bearish candle or 中阴线, but today opens higher.
- Yesterday sold off into the close, but today is not weak.
- The sector was weak yesterday, but the stock opens stronger than the sector.

Interpretation:

- Bearish expectation may be repaired.
- Selling pressure may have been absorbed.
- Fund intent may have shifted from selling to buying.

Required checks:

- Market environment.
- 5/10-day moving-average risk.
- Opening support after the first few minutes.
- Whether the move is only a trap-like high open followed by weakness.

## Core Rule 2: 该涨不涨必有暴跌

Preserve the user's original phrase:

> 该涨不涨必有暴跌。

Executable meaning:

When yesterday's condition was strong and the normal expectation is that today should open high, continue strong, or even quickly limit up, but today's auction/open is flat, down, or only weakly higher, the stock is below expectation.

Typical cases:

- Yesterday hit limit-up, but today has no meaningful premium.
- Yesterday's sector was euphoric, but the front-row stock is weak in auction.
- Yesterday broke out strongly, but today does not attract follow-through.

Interpretation:

- Follow-through capital may be insufficient.
- Profit-taking may dominate.
- It may easily pull up then fall back, or fall directly.

The skill must distinguish healthy disagreement from exhaustion, but default caution is higher for new buys and existing positions.

## Core Rule 3: 暴涨/暴跌反身性

Preserve the user's original phrase:

> 暴涨加暴涨就会调整，暴跌再暴跌就会暴涨。

Executable meaning:

暴涨加暴涨 includes:

- Two large bullish days followed by a likely third-day adjustment.
- A large bullish day followed by another strong morning, then a same-day afternoon pullback.
- Continuous acceleration that fails to seal limit-up, pulls back from highs, or weakens in the afternoon.

Use it as:

- A risk reminder.
- A take-profit reminder.
- A warning against chasing high after acceleration.

暴跌再暴跌 includes:

- Two 中阴线/large bearish/limit-down style days followed by possible repair.
- It is only an observation trigger, not a bottom-fishing instruction.

Required confirmation:

- Auction/open above expectation.
- Opening support.
- Market not continuing to break down.
- No simultaneous break below 5-day and 10-day moving averages.

## Granville Rule Usage

Granville's eight rules are used as a position framework, not as a standalone buy/sell button.

Bullish-leaning cases:

- Pullback to the 5-day or 10-day moving average without breaking down.
- Sharp decline far from moving averages followed by repair, confirmed by auction/open support.
- Weak expectation followed by "该跌不跌".

Bearish or avoid cases:

- Simultaneous break below 5-day and 10-day moving averages.
- Continuous strong rise far above moving averages with large short-term divergence.
- Strong expectation followed by "该涨不涨".
- High open or limit-up attempt that cannot hold, especially with later pullback.

## 5/10-Day Moving-Average Risk Gate

This rule applies to both market indexes and individual stocks.

If the real-time price simultaneously falls below the 5-day and 10-day moving averages:

- Treat it as a high-risk environment.
- Downgrade buy signals.
- Even if the stock matches the common candidate conditions, observe more and avoid impulsive buying.
- For existing positions, strengthen sell discipline.

Reverse is not true:

- Standing above 5-day and 10-day moving averages does not mean automatic bullishness.
- It only means this specific risk gate is not triggered.

## Market Filter

The market is an action gate, not background information.

Apply the same core logic to market indexes:

- Market simultaneously below 5-day and 10-day moving averages: downgrade individual-stock opportunities.
- Market after consecutive 中阳线/暴涨: watch for third-day disagreement or intraday pullback.
- Market after consecutive 中阴线/暴跌: watch for repair, but do not become blindly optimistic.
- Market "该跌不跌": environment may be repairing.
- Market "该涨不涨": risk increases.

If market emotion is available:

- Low emotion, many limit-downs, high opened-limit-up rate, or more bearish unusual moves should downgrade buy analysis.
- A single strong stock should not override a clearly hostile market.

## Buy Analysis Flow

When the user asks whether a stock can be bought:

1. Gather data or list missing data.
2. Check whether it matches the user's common candidate conditions.
3. Continue analysis even if it does not match those conditions.
4. Check market environment.
5. Check whether the stock or market triggered the 5/10-day risk gate.
6. Compare yesterday's state with today's auction/open to identify expectation gap.
7. Apply 暴涨/暴跌反身性.
8. Apply Granville position judgment.
9. Provide a conditional scenario:
   - Whether auction/open buy is reasonable.
   - Whether to wait until before 09:50.
   - What confirms the buy.
   - What cancels the trade.

The skill must not reject analysis only because the stock fails the common candidate filter.

## Sell Analysis Flow

When the user asks whether to sell a stock bought yesterday, first judge yesterday's feedback.

### Yesterday Intraday Loss Feedback

Definition:

- Yesterday's close is more than 1% below yesterday's open.
- This means the user bought into a day where intraday feedback was weak.

Next-day handling:

- If the next day does not open much lower, sell directly or handle near the open.
- "Does not open much lower" is defined as opening between 0% and -2%.
- If the next day opens high and price moves above yesterday's open, the user can hold and observe.
- Moving above yesterday's open means yesterday's bearish body is being repaired.

### Low Open Around -2%

If the stock bought yesterday opens near or below -2%:

- Default interpretation: yesterday's buy was likely wrong.
- Prefer auction/open quick handling.
- Do not let a small mistake become a deep-water loss.

### Large Low Open Below -3%

If today's open is below -3%:

- Do not necessarily sell in auction immediately.
- Observe until before 10:00 for repair.
- If it rebounds to around -2% or -1%, sell first.
- If no repair appears and it weakens further, do not fantasize.

### Flat or High Open

If today opens flat or high:

- Do not rush to sell.
- Watch opening support.
- If it quickly reaches +4% or +5%, enter take-profit observation.

### Limit-Up and Opened Limit-Up

- If it hits and holds limit-up, it can continue to be held.
- If limit-up opens and falls back to around +8%, consider selling.

### One Limit-Up Plus About 3%

In the user's model:

- One limit-up followed by about +3% is often enough to take profit.
- The user does not aim to eat the whole move.
- Exception: if the day after a limit-up instantly limit-ups again, strength is above expectation and the user can consider holding one more day.
- If that second strong move opens and falls back, sell discipline returns.

## Review Learning Mode

When the user provides buy/sell points, the skill should review the trade as a sample:

- Was it inside the user's model?
- Did it match common candidate conditions?
- Did it have "该跌不跌" or "该涨不涨"?
- Did the market support it?
- Was the 5/10-day risk gate triggered?
- Was the buy correct?
- Did the sell follow discipline?
- What should be done next time in a similar situation?

The skill must separate process quality from outcome:

- Correct process, random loss.
- Wrong process, lucky profit.
- Good buy, poor sell.
- Poor buy, disciplined sell.

## Error Handling

- If data is stale, say the data time and avoid strong conclusions.
- If auction data is approximated by the open price, say so.
- If the stock code is ambiguous, ask for the code.
- If market data cannot be fetched, provide conditional analysis and mark the market filter as unknown.
- If the user asks for a definite prediction, reframe into scenarios and invalidation conditions.

## Testing and Review Plan

Manual test prompts:

1. `这只股今天能不能买？昨天跌停，今天高开 1.5%。`
   - Expected: identify "该跌不跌", still check market and 5/10 risk.
2. `昨天涨停，今天平开，能买吗？`
   - Expected: identify "该涨不涨", caution for new buy.
3. `昨天竞价买了，今天低开 -2.1%，怎么办？`
   - Expected: treat yesterday's buy feedback as failed and prefer discipline.
4. `昨天买了，今天低开 -3.8%，怎么办？`
   - Expected: wait until before 10:00 for repair toward -2%/-1%, then sell.
5. `不符合7日涨停条件，但今天该跌不跌，怎么看？`
   - Expected: say it does not match common candidate pool, but still analyze core logic.
6. `大盘跌破5日和10日线，这个股符合竞价条件能买吗？`
   - Expected: downgrade opportunity because market risk gate is triggered.

Spec self-check:

- No placeholder sections remain.
- Candidate conditions are explicitly not absolute buy rules.
- 5/10-day moving-average reverse condition is explicitly not bullish.
- Sell rules include yesterday intraday loss feedback and the 0% to -2% definition.
- Data design supports both go-stock project tools and non-project Trae environments.
