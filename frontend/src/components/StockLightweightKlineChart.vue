<script setup>
import { GetStockEastMoneyKLine, GetStockEastMoneyKLinePage, GetStockKLineWithFallback, GetStockKLinePageWithFallback } from '../../wailsjs/go/main/App'
import {
  CandlestickSeries,
  createChart,
  HistogramSeries,
  LineSeries,
  LineStyle,
  TickMarkType,
} from 'lightweight-charts'
import { NButton, NFlex, NInput, NSpin, NText, NTooltip } from 'naive-ui'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'

/** A 股配色：涨红跌绿 */
const CLR_RISE = '#ef5350'
const CLR_FALL = '#26a69a'

/** 东方财富 klt；日/周/月/季/年：横轴只显示日期 */
const DAILY_LIKE_KLT = new Set(['101', '102', '103', '104', '106'])
const CN_TZ = 'Asia/Shanghai'

/** 向左拖动接近左侧边缘时，请求更早 K 线 */
const HISTORY_PAGE_SIZE = 400
const BARS_BEFORE_LOAD_MORE = 45
/** 首次加载 / 切换周期后默认可见 K 线根数（过密时看不清） */
const DEFAULT_VISIBLE_BARS = 180
/** 逻辑坐标上右侧多留的「空档」，最新 K 不靠最右边（与左拖分页后的 range 平移兼容） */
const DEFAULT_RIGHT_LOGICAL_GAP = 18
/** 为 false 时不在 K 线工具栏显示「筹码分布」按钮（计算与面板逻辑仍保留） */
const SHOW_CHIP_TOOLBAR_BUTTON = false

const INTERVALS = [
  { klt: '1', label: '1分', limit: 1000 },
  { klt: '5', label: '5分', limit: 600 },
  { klt: '15', label: '15分', limit: 500 },
  { klt: '30', label: '30分', limit: 500 },
  { klt: '60', label: '60分', limit: 500 },
  { klt: '101', label: '日K', limit: 800 },
  { klt: '102', label: '周K', limit: 520 },
  { klt: '103', label: '月K', limit: 240 },
  { klt: '104', label: '季K', limit: 120 },
  { klt: '106', label: '年K', limit: 40 },
]

const props = defineProps({
  code: { type: String, default: '' },
  stockName: { type: String, default: '' },
  darkTheme: { type: Boolean, default: false },
  chartHeight: { type: Number, default: 400 },
  /** 定时拉取当前周期最新 K 线，毫秒；0 关闭；默认 60 秒 */
  realtimeIntervalMs: { type: Number, default: 1000*60 },
  /** 多单开仓价；传入则与内部输入同步，未传入（undefined）时不向父组件 emit */
  longEntryPrice: { type: [String, Number], default: undefined },
  /** 多单止损价 */
  longStopLossPrice: { type: [String, Number], default: undefined },
  /** 多单止盈价 */
  longTakeProfitPrice: { type: [String, Number], default: undefined },
  /** 成本价 */
  costPrice: { type: [String, Number], default: undefined },
})

const emit = defineEmits([
  'update:longEntryPrice',
  'update:longStopLossPrice',
  'update:longTakeProfitPrice',
  'update:costPrice',
])
const chartContainerRef = ref(null)
/** 十字线当前 K 对应的原始行（东财字段） */
const hoverRawRow = ref(null)
/** 无十字线时展示：当前数据中时间最新的一根 K 线 */
const defaultLatestRawRow = ref(null)
const activeKlt = ref('101')
const showMA = ref(false)
const showBOLL = ref(false)
const showOBV = ref(false)
const showMACD = ref(false)
const showKDJ = ref(false)
const showRSI = ref(false)
const showATR = ref(false)
const showVWAP = ref(false)
const showMFI = ref(false)
const showKAMA = ref(false)
const showKeltner = ref(false)
const showSupertrend = ref(false)
const showEMA = ref(false)
const showIchimoku = ref(false)
const showCCI = ref(false)
const showTTMSqueeze = ref(false)
const showSAR = ref(false)
const showDonchian = ref(false)
const showADX = ref(false)
const showWilliamsR = ref(false)
const showStochRSI = ref(false)
const showCMF = ref(false)
const showAroon = ref(false)
const showCMO = ref(false)
const showForceIndex = ref(false)
const showPivot = ref(false)
const showDEMA = ref(false)
const showZigZag = ref(false)
const showSATS = ref(false)
const showChip = ref(false)
const chipBins = ref(80)
const chipCanvasRef = ref(null)
const chipItems = ref([])
const chipMeta = ref({ avgCost: 0, profitRatio: 0, current: 0, hoverDate: '', minPrice: 0, maxPrice: 0 })
/** TradingView 风格「多单」：开仓 / 止损 / 止盈 价位线 */
const showLongPosition = ref(false)
const longEntryStr = ref('')
const longStopStr = ref('')
const longTakeProfitStr = ref('')
const longCostStr = ref('')
/** 在 K 线主区点击，按顺序写入开仓 → 止损 → 止盈（再点回到开仓） */
const longClickPickEnabled = ref(false)
const longClickNextField = ref('entry')
/** 点过开仓/止损/止盈输入框后，下一次主图点击写入对应价位（blur 延迟清除以兼容「先失焦后 click」） */
const longFocusedPriceField = ref(null)
/** 由 props 写入价位时抑制 emit，避免与 v-model 循环 */
const suppressLongPriceEmit = ref(false)
const loading = ref(false)
const loadingHistory = ref(false)
const errorText = ref('')
const activeDataSource = ref('')

let chart = null
let candleSeries = null
let volSeries = null
let pollTimer = null
/** 已合并的后端原始 K 线（按时间升序） */
let mergedRawRows = []
const hasMoreOlder = ref(true)
let loadOlderDebounceTimer = null
/** Wails/WebView 下可见区回调偶发不触发，用轻量轮询兜底 */
let historyVisiblePollTimer = null
let logicalRangeHandler = null
let visibleTimeRangeHandler = null
let crosshairMoveHandler = null
let chartClickHandler = null
/** 上一次请求更早 K 线使用的 end，用于识别「重叠返回」避免误判无更多数据 */
let lastOlderHistoryEndTried = ''
/** >0 时表示由代码在改时间轴（fitContent / setData / setVisibleLogicalRange），不触发分页加载 */
let programmaticRangeDepth = 0
/** 多单标注：createPriceLine 返回的句柄，需在 dispose / 重绘前 remove */
let longPositionPriceLines = []
/** 各价位线句柄，用于命中与拖动时 applyOptions */
let longLineByKind = { entry: null, stop: null, takeProfit: null }
/** 正在拖动某条多单价位线时禁止 watch 里整表重建线条 */
let longPositionDragActive = false
let longDragKind = null
/** 命中线后抑制一次「图上点击设价」，避免拖/点冲突 */
let longSuppressChartClick = false
let longPaneDragEl = null
/** window 上拖动用监听器是否已挂（避免重复解绑 / 重复 up） */
let longDragWindowListenersOn = false
/** 拖动过程中最近一次指针 Y，用于松手后恢复「可拖」光标 */
let longLastPointerClientY = null
/** 垂直方向命中容差（px） */
const LONG_PRICE_LINE_HIT_PX = 12
/** 输入框 blur 后延迟清除「待图表取价」状态（ms），需大于 click 相对 blur 的间隔 */
const LONG_FOCUS_BLUR_CLEAR_MS = 600
let longFocusBlurTimer = null

/** 指标线系列（与主图生命周期同步） */
const ind = {
  ma5: null,
  ma10: null,
  ma20: null,
  ma60: null,
  bollU: null,
  bollM: null,
  bollL: null,
  obv: null,
  macdHist: null,
  macdDif: null,
  macdDea: null,
  kdjK: null,
  kdjD: null,
  kdjJ: null,
  rsi: null,
  atr: null,
  vwap: null,
  mfi: null,
  kama: null,
  keltnerU: null,
  keltnerM: null,
  keltnerL: null,
  supertrend: null,
  ema12: null,
  ema21: null,
  ichTenkan: null,
  ichKijun: null,
  ichSpanA: null,
  ichSpanB: null,
  ichChikou: null,
  cci: null,
  ttmHist: null,
  ttmDots: null,
  sar: null,
  donchianU: null,
  donchianM: null,
  donchianL: null,
  adx: null,
  adxDiP: null,
  adxDiM: null,
  williamsR: null,
  stochRsi: null,
  stochRsiD: null,
  cmf: null,
  aroonUp: null,
  aroonDown: null,
  cmo: null,
  forceIndex: null,
  pivotPP: null,
  pivotS1: null,
  pivotS2: null,
  pivotR1: null,
  pivotR2: null,
  dema: null,
  zigzag: null,
  satsLine: null,
  satsUpper: null,
  satsLower: null,
}

const indicatorTips = {
  ma: '趋势指标 | 用法：MA5/10看短期，MA20/60看中长线\n✅ 多头排列(5>10>20>60)持股，空头排列观望\n✅ 金叉买入，死叉卖出\n🔄 组合：MA+BOLL看支撑压力，MA+MACD确认趋势',
  boll: '波动率指标 | BOLL(20,2)\n✅ 收口=变盘前兆，开口=趋势启动\n✅ 触上轨超买，触下轨超卖(震荡市)\n✅ 中轨(20日均线)是多空分水岭\n🔄 组合：BOLL+Keltner=TTM Squeeze挤压策略',
  obv: '量价指标 | 能量潮\n✅ OBV上升+价涨=量价配合，趋势健康\n✅ OBV背离价格=趋势衰竭信号\n🔄 组合：OBV+MACD确认突破有效性',
  macd: '趋势+动量指标 | MACD(12,26,9)\n✅ 零轴上金叉=强势买入，零轴下死叉=弱势卖出\n✅ 顶背离(价新高MACD不新高)=见顶\n✅ 底背离(价新低MACD不新低)=见底\n🔄 组合：MACD+RSI双重确认，MACD+BOLL看转折',
  kdj: '震荡指标 | KDJ(9)\n✅ K上穿D金叉买入(20以下最准)\n✅ K下穿D死叉卖出(80以上最准)\n⚠️ 主升浪中KDJ长期超买别盲目卖\n🔄 组合：KDJ+RSI超买超卖共振，KDJ+BOLL触轨确认',
  rsi: '动量指标 | RSI(14)\n✅ RSI>70超买警惕回调，RSI<30超卖关注反弹\n✅ RSI>50多头占优，<50空头占优\n✅ 顶背离=诱多陷阱，坚决不追\n🔄 组合：RSI+MFI量价共振，RSI+MACD拐点确认',
  atr: '波动率指标 | ATR(14)\n✅ ATR大=波动剧烈，止损设宽\n✅ ATR小=波动平缓，止损设窄\n✅ 止损位=入场价±N×ATR(常用2倍)\n🔄 组合：ATR+Supertrend趋势跟踪，ATR+Keltner通道',
  vwap: '量价指标 | VWAP(20)\n✅ 价格在VWAP上方=多头主导\n✅ 价格在VWAP下方=空头主导\n✅ 机构常用VWAP做日内交易锚定价\n🔄 组合：VWAP+MFI确认资金方向',
  mfi: '量价指标 | MFI(14)\n✅ MFI>80超买，<20超卖\n✅ 量价版RSI，比RSI多考虑成交量\n✅ 背离时比RSI更早发出信号\n🔄 组合：MFI+RSI量价共振，MFI+VWAP主力资金追踪',
  kama: '自适应均线 | KAMA(10,2,30)\n✅ 趋势市自动变快，震荡市自动变慢\n✅ 解决MA在震荡市频繁假信号的问题\n✅ 价格突破KAMA=趋势启动信号\n🔄 组合：KAMA+ATR自适应止损',
  keltner: '波动率通道 | Keltner(20,10,1.5)\n✅ 价格突破上轨=强势，突破下轨=弱势\n✅ 与BOLL配合：BOLL收窄到Keltner内=挤压\n🔄 组合：Keltner+BOLL=TTM Squeeze挤压策略',
  supertrend: '趋势跟踪指标 | Supertrend(10,3)\n✅ 红线持股(看多)，绿线观望(看空)，最简洁的趋势信号\n✅ 可做移动止损：价格跌破红线止盈\n✅ A股短线波段神器，日线级别最准\n🔄 组合：Supertrend+EMA确认方向，Supertrend+ATR动态止损',
  ema: '趋势指标 | EMA(12,21)\n✅ EMA12上穿EMA21=金叉买入\n✅ EMA12下穿EMA21=死叉卖出\n✅ 比SMA更灵敏，短线操盘手偏爱\n🔄 组合：EMA+Supertrend趋势确认，EMA+MACD多级过滤',
  ichimoku: '综合指标 | 一目均衡表(9,26,52)\n✅ 价格在云层上方=多头，下方=空头\n✅ 转换线上穿基准线=金叉买入\n✅ 云层=支撑阻力区，厚云层阻力更大\n✅ 迟行线在价格上方=长期看多\n🔄 组合：Ichimoku+MACD多重趋势确认',
  cci: '震荡+趋势指标 | CCI(20)\n✅ CCI>+100=强势追涨，<-100=弱势关注\n✅ CCI回穿±100=趋势确认信号\n✅ 比KDJ/RSI更灵活，不受0-100限制\n🔄 组合：CCI+MACD趋势启动，CCI+BOLL超买超卖',
  ttmSqueeze: '波动率+动量指标 | TTM Squeeze\n✅ 黄色圆点=波动挤压中(即将爆发)\n✅ 绿色圆点=挤压释放(趋势已启动)\n✅ 柱状图正值=多头动量，负值=空头动量\n✅ BOLL收窄到Keltner内=挤压，扩张=释放\n🔄 组合：TTM Squeeze+Supertrend突破确认',
  sar: '趋势跟踪指标 | Parabolic SAR(0.02,0.2)\n✅ 价格下方出现红点=多头持股\n✅ 价格上方出现绿点=空头观望\n✅ 点位即为移动止损位，跌破即走\n⚠️ 震荡市频繁翻转，配合ADX过滤\n🔄 组合：SAR+ADX趋势确认，SAR+Supertrend双重止损',
  donchian: '突破通道指标 | Donchian(20)\n✅ 价格突破上轨=创新高买入\n✅ 价格跌破下轨=创新低卖出\n✅ 海龟交易法核心指标\n✅ 中轨=(最高+最低)/2，可做均值回归\n🔄 组合：Donchian+ATR仓位管理，Donchian+SAR止损',
  adx: '趋势强度指标 | ADX(14)\n✅ ADX>25=趋势行情，顺势操作\n✅ ADX<20=震荡行情，不做趋势交易\n✅ ADX从下上穿25=趋势正在形成\n✅ +DI>-DI=多头力量，-DI>+DI=空头力量\n🔄 组合：ADX+MACD过滤假信号，ADX+Supertrend趋势确认',
  williamsR: '超买超卖指标 | Williams %R(14)\n✅ %R>-20超买，<-80超卖\n✅ 比KDJ更敏感，信号更早\n✅ 超买区拐头向下=卖出信号\n✅ 超卖区拐头向上=买入信号\n🔄 组合：WilliamsR+RSI超买超卖共振',
  stochRsi: '灵敏度增强指标 | StochRSI(14,14,3,3)\n✅ RSI的随机化，灵敏度远超普通RSI\n✅ 值>80超买，<20超卖\n✅ 适合短线1-3天拐点捕捉\n⚠️ 信号多但噪声也多，需配合趋势指标\n🔄 组合：StochRSI+MACD过滤假信号，StochRSI+BOLL辅助判断',
  cmf: '资金流向指标 | CMF(20)\n✅ CMF>0=资金流入，多头占优\n✅ CMF<0=资金流出，空头占优\n✅ CMF背离价格=主力资金异动\n✅ 比OBV更精确的量价分析\n🔄 组合：CMF+MFI资金确认，CMF+MACD趋势+资金共振',
  aroon: '趋势启停指标 | Aroon(25)\n✅ Aroon Up=100=刚创新高，Down=100=刚创新低\n✅ Up>70且Down<30=上升趋势启动\n✅ Down>70且Up<30=下降趋势启动\n✅ 两线交叉=趋势方向切换信号\n🔄 组合：Aroon+ADX确认趋势形成，Aroon+Supertrend趋势跟踪',
  cmo: '纯动量指标 | CMO(14)\n✅ CMO>0=多头动量，<0=空头动量\n✅ |CMO|>50=极端动量，可能反转\n✅ 去除价格绝对值影响，只看动量变化\n✅ 比RSI更纯粹的动量衡量\n🔄 组合：CMO+MACD动量确认，CMO+ADX趋势+动量',
  forceIndex: '量价动量指标 | Force Index(13)\n✅ FI>0=多头力量占优，<0=空头力量\n✅ FI创高新=多头力量增强\n✅ FI与价格背离=趋势衰竭\n✅ Elder三重过滤法核心指标\n🔄 组合：ForceIndex+EMA趋势过滤，ForceIndex+MACD背离确认',
  pivot: '支撑阻力指标 | Pivot Points\n✅ PP=前日(H+L+C)/3，多空分水岭\n✅ R1/R2=阻力位，触及易回落\n✅ S1/S2=支撑位，触及易反弹\n✅ 日内交易者开盘必看\n🔄 组合：Pivot+BOLL双重支撑阻力，Pivot+VWAP日内锚定',
  dema: '快速均线指标 | DEMA(21)\n✅ 双重指数均线，比EMA更快速\n✅ 金叉/死叉信号比EMA更早\n✅ 减少均线滞后性的利器\n✅ 短线1-3天交易首选均线\n🔄 组合：DEMA+EMA21双线系统，DEMA+SAR快速止损',
  zigzag: '波段结构指标 | ZigZag(5%)\n✅ 自动标注波段高低点\n✅ 一目了然看清趋势结构\n✅ 忽略小波动，只看大趋势\n✅ 波段高低点是画趋势线的起点\n🔄 组合：ZigZag+Fibonacci回撤，ZigZag+MACD波段确认',
  sats: '自适应趋势系统 | SATS(自感知趋势)\n✅ Supertrend升级版：带宽根据TQI动态调整\n✅ 红线=持股看多，绿线=观望看空(A股红涨绿跌)\n✅ TQI趋势质量指数(0~1)：高=强趋势窄通道，低=弱趋势宽通道\n✅ 非对称带宽：趋势方向收紧，反方向放宽(棘轮效应)\n✅ Character-Flip：TQI急剧下降时提前翻转，不等价格突破\n✅ 效率加权ATR：趋势行情ATR全重，震荡行情折半\n✅ 上/下轨线可视：主动侧收紧=趋势确认，被动侧放宽=保护利润\n🔄 组合：SATS+MACD趋势确认，SATS+ATR动态止损',
}

function removeSeriesSafe(api) {
  if (!api || !chart) return null
  try {
    chart.removeSeries(api)
  } catch {
    /* ignore */
  }
  return null
}



function extractOHLCV(rows) {
  const sorted = [...(rows || [])].sort((a, b) => sortKey(a.day) - sortKey(b.day))
  const times = []
  const closes = []
  const highs = []
  const lows = []
  const vols = []
  for (const r of sorted) {
    const t = toChartTime(r.day)
    if (t === null) continue
    const o = Number(r.open)
    const h = Number(r.high)
    const l = Number(r.low)
    const c = Number(r.close)
    const v = Number(r.volume)
    if (![o, h, l, c].every(Number.isFinite)) continue
    times.push(t)
    closes.push(c)
    highs.push(h)
    lows.push(l)
    vols.push(Number.isFinite(v) ? v : 0)
  }
  return { times, closes, highs, lows, vols }
}

function smaValues(closes, period) {
  const out = []
  for (let i = 0; i < closes.length; i++) {
    if (i < period - 1) {
      out.push(null)
      continue
    }
    let s = 0
    for (let j = 0; j < period; j++) s += closes[i - j]
    out.push(s / period)
  }
  return out
}

function bollingerBands(closes, period, mult) {
  const mid = smaValues(closes, period)
  const upper = []
  const lower = []
  for (let i = 0; i < closes.length; i++) {
    if (i < period - 1) {
      upper.push(null)
      lower.push(null)
      continue
    }
    const m = mid[i]
    let sumSq = 0
    for (let j = 0; j < period; j++) {
      const d = closes[i - j] - m
      sumSq += d * d
    }
    const std = Math.sqrt(sumSq / period)
    upper.push(m + mult * std)
    lower.push(m - mult * std)
  }
  return { upper, mid, lower }
}

function obvValues(closes, vols) {
  if (!closes.length) return []
  const out = []
  let obv = vols[0] || 0
  out.push(obv)
  for (let i = 1; i < closes.length; i++) {
    const ch = closes[i] - closes[i - 1]
    if (ch > 0) obv += vols[i] || 0
    else if (ch < 0) obv -= vols[i] || 0
    out.push(obv)
  }
  return out
}

function toLineData(times, values) {
  const arr = []
  for (let i = 0; i < times.length; i++) {
    const v = values[i]
    if (v != null && Number.isFinite(v)) arr.push({ time: times[i], value: v })
  }
  return arr
}

/** 单根 K 的近似「成本中枢」：优先日 VWAP（成交额/量），否则典型价，夹在 [L,H] */
function chipBarCostCenter(r) {
  const h = Number(r.high)
  const l = Number(r.low)
  const c = Number(r.close)
  const o = Number(r.open)
  const vol = Number(r.volume)
  const amt = Number(r.amount)
  const hlOk = Number.isFinite(h) && Number.isFinite(l) && h > 0 && l > 0 && h >= l
  if (!hlOk) return null
  if (Number.isFinite(amt) && amt > 0 && Number.isFinite(vol) && vol > 0) {
    const vwap = amt / vol
    if (Number.isFinite(vwap) && vwap > 0) return Math.min(h, Math.max(l, vwap))
  }
  if ([h, l, c].every(Number.isFinite) && c > 0) {
    const tp = (h + l + c) / 3
    if (Number.isFinite(tp)) return Math.min(h, Math.max(l, tp))
  }
  if ([h, l, o, c].every(Number.isFinite) && o > 0 && c > 0) {
    const tp = (h + l + o + c) / 4
    if (Number.isFinite(tp)) return Math.min(h, Math.max(l, tp))
  }
  return (h + l) / 2
}

/**
 * 将成交量按高斯核落在 [low,high] 与各 bin 的交集上（核中心为成本中枢），
 * 比均匀铺满当日高低区间更接近「筹码集中在成交密集价」的经验事实。
 */
function addChipVolumeKernel(dist, bins, minP, width, low, high, vol, center) {
  if (vol <= 0 || low <= 0 || high <= 0) return
  let lo = low
  let hi = high
  if (hi < lo) [lo, hi] = [hi, lo]
  const span = hi - lo
  const loIdx = Math.max(0, Math.min(bins - 1, Math.floor((lo - minP) / width)))
  const hiIdx = Math.max(0, Math.min(bins - 1, Math.floor((hi - minP) / width)))
  if (hiIdx < loIdx) return
  if (span < 1e-9 * Math.max(1, hi)) {
    const i = Math.max(0, Math.min(bins - 1, Math.floor(((lo + hi) / 2 - minP) / width)))
    dist[i] += vol
    return
  }
  let m = center
  if (!Number.isFinite(m)) m = (lo + hi) / 2
  m = Math.min(hi, Math.max(lo, m))
  const sigma = Math.max(span * 0.18, hi * 1e-6, 1e-6)
  let wsum = 0
  for (let i = loIdx; i <= hiIdx; i++) {
    const bc = minP + (i + 0.5) * width
    if (bc < lo || bc > hi) continue
    const d = (bc - m) / sigma
    wsum += Math.exp(-0.5 * d * d)
  }
  if (wsum <= 0) {
    const cnt = hiIdx - loIdx + 1
    const add = vol / cnt
    for (let i = loIdx; i <= hiIdx; i++) dist[i] += add
    return
  }
  for (let i = loIdx; i <= hiIdx; i++) {
    const bc = minP + (i + 0.5) * width
    if (bc < lo || bc > hi) continue
    const d = (bc - m) / sigma
    const w = Math.exp(-0.5 * d * d)
    dist[i] += (vol * w) / wsum
  }
}

function calcChipDistribution(rows, bins) {
  if (!rows?.length || bins <= 0) return { items: [], avgCost: 0, profitRatio: 0, current: 0 }
  let minP = Infinity, maxP = 0
  for (const r of rows) {
    const lo = Number(r.low) || 0
    const hi = Number(r.high) || 0
    if (lo > 0 && lo < minP) minP = lo
    if (hi > 0 && hi > maxP) maxP = hi
  }
  if (minP <= 0 || maxP <= 0 || maxP < minP) return { items: [], avgCost: 0, profitRatio: 0, current: 0 }
  if (maxP === minP) maxP = minP * 1.001
  const width = (maxP - minP) / bins
  if (width <= 0) return { items: [], avgCost: 0, profitRatio: 0, current: 0 }
  const dist = new Float64Array(bins)
  for (const r of rows) {
    let turn = parseFloatPct(r.turnoverRate)
    if (turn < 0) turn = 0
    if (turn > 0.98) turn = 0.98
    const remain = 1.0 - turn
    for (let i = 0; i < bins; i++) dist[i] *= remain
    const low = Number(r.low) || 0
    const high = Number(r.high) || 0
    const vol = Number(r.volume) || 0
    if (vol <= 0 || low <= 0 || high <= 0) continue
    const center = chipBarCostCenter(r)
    addChipVolumeKernel(dist, bins, minP, width, low, high, vol, center)
  }
  let sum = 0
  for (let i = 0; i < bins; i++) sum += dist[i]
  const cur = Number(rows[rows.length - 1].close) || Number(rows[rows.length - 1].high) || 0
  const items = []
  let avgCost = 0, profitVol = 0
  for (let i = 0; i < bins; i++) {
    const center = minP + (i + 0.5) * width
    const v = dist[i]
    const ratio = sum > 0 ? v / sum : 0
    items.push({ price: Math.round(center * 10000) / 10000, vol: Math.round(v * 10000) / 10000, ratio: Math.round(ratio * 1e6) / 1e6 })
    avgCost += v * center
    if (center <= cur) profitVol += v
  }
  if (sum > 0) avgCost /= sum
  const profitRatio = sum > 0 ? profitVol / sum : 0
  return { items, avgCost: Math.round(avgCost * 10000) / 10000, profitRatio: Math.round(profitRatio * 1e6) / 1e6, current: Math.round(cur * 10000) / 10000, minPrice: minP, maxPrice: maxP }
}

function parseFloatPct(s) {
  const v = parseFloat(String(s ?? '').replace(/%/g, '').trim())
  return Number.isFinite(v) ? v / 100 : 0
}

function drawChipCanvas() {
  const canvas = chipCanvasRef.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  const dpr = window.devicePixelRatio || 1
  const rect = canvas.getBoundingClientRect()
  const w = rect.width
  const h = rect.height
  canvas.width = w * dpr
  canvas.height = h * dpr
  ctx.scale(dpr, dpr)
  ctx.clearRect(0, 0, w, h)
  const isDark = props.darkTheme
  ctx.fillStyle = isDark ? '#141414' : '#ffffff'
  ctx.fillRect(0, 0, w, h)
  const items = chipItems.value
  if (!items.length) return
  const maxRatio = Math.max(...items.map((it) => it.ratio || 0), 1e-9)
  const barMaxW = w - 4
  const barH = Math.max(1, h / items.length)
  const cur = chipMeta.value.current || 0
  for (let i = 0; i < items.length; i++) {
    const it = items[i]
    const y = i * barH
    const bw = Math.max(0, (it.ratio / maxRatio) * barMaxW)
    const isProfit = it.price <= cur
    if (isProfit) {
      ctx.fillStyle = isDark ? 'rgba(239, 83, 80, 0.7)' : 'rgba(239, 83, 80, 0.6)'
    } else {
      ctx.fillStyle = isDark ? 'rgba(38, 166, 154, 0.7)' : 'rgba(38, 166, 154, 0.6)'
    }
    ctx.fillRect(w - bw, y, bw, barH - 0.5)
  }
  if (cur > 0) {
    const minP = chipMeta.value.minPrice || 0
    const maxP = chipMeta.value.maxPrice || 0
    if (maxP > minP && cur >= minP && cur <= maxP) {
      const curY = ((cur - minP) / (maxP - minP)) * h
      ctx.strokeStyle = isDark ? '#fbbf24' : '#d97706'
      ctx.lineWidth = 1
      ctx.setLineDash([4, 3])
      ctx.beginPath()
      ctx.moveTo(0, curY)
      ctx.lineTo(w, curY)
      ctx.stroke()
      ctx.setLineDash([])
    }
  }
}

function emaFinite(values, period) {
  const out = []
  const k = 2 / (period + 1)
  let ema = null
  for (let i = 0; i < values.length; i++) {
    const v = values[i]
    if (!Number.isFinite(v)) {
      out.push(null)
      continue
    }
    if (ema === null) {
      if (i < period - 1) {
        out.push(null)
        continue
      }
      let s = 0
      let ok = true
      for (let j = i - period + 1; j <= i; j++) {
        if (!Number.isFinite(values[j])) {
          ok = false
          break
        }
        s += values[j]
      }
      if (!ok) {
        out.push(null)
        continue
      }
      ema = s / period
      out.push(ema)
    } else {
      ema = v * k + ema * (1 - k)
      out.push(ema)
    }
  }
  return out
}

/** DIF 序列前段为 null，从首个有效值起做 EMA（用于 MACD 信号线） */
function emaLeadingNull(series, period) {
  const out = series.map(() => null)
  const k = 2 / (period + 1)
  let ema = null
  let sum = 0
  let cnt = 0
  for (let i = 0; i < series.length; i++) {
    const v = series[i]
    if (v == null || !Number.isFinite(v)) {
      out[i] = null
      continue
    }
    if (ema === null) {
      sum += v
      cnt++
      if (cnt < period) {
        out[i] = null
        continue
      }
      if (cnt === period) {
        ema = sum / period
        out[i] = ema
      }
    } else {
      ema = v * k + ema * (1 - k)
      out[i] = ema
    }
  }
  return out
}

function macdBundle(closes) {
  const ema12 = emaFinite(closes, 12)
  const ema26 = emaFinite(closes, 26)
  const dif = closes.map((_, i) =>
    ema12[i] != null && ema26[i] != null ? ema12[i] - ema26[i] : null,
  )
  const dea = emaLeadingNull(dif, 9)
  const hist = dif.map((d, i) =>
    d != null && dea[i] != null ? 2 * (d - dea[i]) : null,
  )
  return { dif, dea, hist }
}

function kdjBundle(highs, lows, closes, n = 9) {
  const len = closes.length
  const rsv = new Array(len).fill(null)
  for (let i = n - 1; i < len; i++) {
    let hn = -Infinity
    let ln = Infinity
    for (let j = 0; j < n; j++) {
      hn = Math.max(hn, highs[i - j])
      ln = Math.min(ln, lows[i - j])
    }
    const c = closes[i]
    rsv[i] = hn === ln ? 50 : ((c - ln) / (hn - ln)) * 100
  }
  const K = new Array(len).fill(null)
  const D = new Array(len).fill(null)
  const J = new Array(len).fill(null)
  let pk = 50
  let pd = 50
  for (let i = 0; i < len; i++) {
    const r = rsv[i]
    if (r == null) continue
    pk = (2 * pk + r) / 3
    pd = (2 * pd + pk) / 3
    K[i] = pk
    D[i] = pd
    J[i] = 3 * pk - 2 * pd
  }
  return { K, D, J }
}

function rsiBundle(closes, period = 14) {
  const out = new Array(closes.length).fill(null)
  for (let i = period; i < closes.length; i++) {
    let gain = 0
    let loss = 0
    for (let j = 0; j < period; j++) {
      const ch = closes[i - j] - closes[i - j - 1]
      if (ch >= 0) gain += ch
      else loss -= ch
    }
    const ag = gain / period
    const al = loss / period
    out[i] = al === 0 ? 100 : 100 - 100 / (1 + ag / al)
  }
  return out
}

function atrValues(highs, lows, closes, period = 14) {
  const len = closes.length
  if (len < 2) return new Array(len).fill(null)
  const tr = new Array(len).fill(null)
  tr[0] = highs[0] - lows[0]
  for (let i = 1; i < len; i++) {
    tr[i] = Math.max(
      highs[i] - lows[i],
      Math.abs(highs[i] - closes[i - 1]),
      Math.abs(lows[i] - closes[i - 1]),
    )
  }
  const out = new Array(len).fill(null)
  let sum = 0
  for (let i = 0; i < period && i < len; i++) {
    sum += tr[i]
  }
  if (len >= period) {
    out[period - 1] = sum / period
    for (let i = period; i < len; i++) {
      out[i] = (out[i - 1] * (period - 1) + tr[i]) / period
    }
  }
  return out
}

function vwapValues(highs, lows, closes, vols, period = 20) {
  const len = closes.length
  const out = new Array(len).fill(null)
  for (let i = period - 1; i < len; i++) {
    let sumPV = 0
    let sumV = 0
    for (let j = 0; j < period; j++) {
      const tp = (highs[i - j] + lows[i - j] + closes[i - j]) / 3
      sumPV += tp * vols[i - j]
      sumV += vols[i - j]
    }
    out[i] = sumV > 0 ? sumPV / sumV : null
  }
  return out
}

function mfiValues(highs, lows, closes, vols, period = 14) {
  const len = closes.length
  if (len < 2) return new Array(len).fill(null)
  const tp = closes.map((_, i) => (highs[i] + lows[i] + closes[i]) / 3)
  const mf = tp.map((t, i) => t * vols[i])
  const out = new Array(len).fill(null)
  for (let i = period; i < len; i++) {
    let posMF = 0
    let negMF = 0
    for (let j = 0; j < period; j++) {
      const idx = i - j
      if (tp[idx] > tp[idx - 1]) posMF += mf[idx]
      else if (tp[idx] < tp[idx - 1]) negMF += mf[idx]
    }
    out[i] = negMF === 0 ? 100 : 100 - 100 / (1 + posMF / negMF)
  }
  return out
}

function kamaValues(closes, period = 10, fastPeriod = 2, slowPeriod = 30) {
  const len = closes.length
  const out = new Array(len).fill(null)
  if (len < period + 1) return out
  const fastSC = 2 / (fastPeriod + 1)
  const slowSC = 2 / (slowPeriod + 1)
  let kama = closes[period]
  out[period] = kama
  for (let i = period + 1; i < len; i++) {
    const direction = Math.abs(closes[i] - closes[i - period])
    let volatility = 0
    for (let j = 0; j < period; j++) {
      volatility += Math.abs(closes[i - j] - closes[i - j - 1])
    }
    const er = volatility > 0 ? direction / volatility : 0
    const sc = (er * (fastSC - slowSC) + slowSC) ** 2
    kama = kama + sc * (closes[i] - kama)
    out[i] = kama
  }
  return out
}

function keltnerChannelValues(highs, lows, closes, emaPeriod = 20, atrPeriod = 10, mult = 1.5) {
  const mid = emaFinite(closes, emaPeriod)
  const atr = atrValues(highs, lows, closes, atrPeriod)
  const upper = []
  const lower = []
  for (let i = 0; i < closes.length; i++) {
    if (mid[i] != null && atr[i] != null) {
      upper.push(mid[i] + mult * atr[i])
      lower.push(mid[i] - mult * atr[i])
    } else {
      upper.push(null)
      lower.push(null)
    }
  }
  return { upper, mid, lower }
}

function supertrendValues(highs, lows, closes, atrPeriod = 10, multiplier = 3) {
  const len = closes.length
  const atr = atrValues(highs, lows, closes, atrPeriod)
  const supertrend = new Array(len).fill(null)
  const direction = new Array(len).fill(0)
  let upperBand = null
  let lowerBand = null
  let prevUpper = null
  let prevLower = null
  let prevDir = 0
  for (let i = 0; i < len; i++) {
    if (atr[i] == null) continue
    const hl2 = (highs[i] + lows[i]) / 2
    let rawUpper = hl2 + multiplier * atr[i]
    let rawLower = hl2 - multiplier * atr[i]
    if (prevUpper != null && rawUpper >= prevUpper && closes[i - 1] <= prevUpper) {
      rawUpper = prevUpper
    }
    if (prevLower != null && rawLower <= prevLower && closes[i - 1] >= prevLower) {
      rawLower = prevLower
    }
    let dir
    if (prevDir === 0) {
      dir = 1
    } else if (prevDir === 1) {
      dir = closes[i] < rawLower ? -1 : 1
    } else {
      dir = closes[i] > rawUpper ? 1 : -1
    }
    upperBand = rawUpper
    lowerBand = rawLower
    supertrend[i] = dir === 1 ? lowerBand : upperBand
    direction[i] = dir
    prevUpper = upperBand
    prevLower = lowerBand
    prevDir = dir
  }
  return { supertrend, direction }
}

function ichimokuValues(highs, lows, closes, tenkanP = 9, kijunP = 26, senkouBP = 52) {
  const len = closes.length
  function periodHL(h, l, p) {
    const out = new Array(len).fill(null)
    for (let i = p - 1; i < len; i++) {
      let hi = -Infinity
      let lo = Infinity
      for (let j = 0; j < p; j++) {
        hi = Math.max(hi, h[i - j])
        lo = Math.min(lo, l[i - j])
      }
      out[i] = (hi + lo) / 2
    }
    return out
  }
  const tenkan = periodHL(highs, lows, tenkanP)
  const kijun = periodHL(highs, lows, kijunP)
  const senkouB = periodHL(highs, lows, senkouBP)
  const spanA = new Array(len).fill(null)
  const chikou = new Array(len).fill(null)
  for (let i = 0; i < len; i++) {
    if (tenkan[i] != null && kijun[i] != null) {
      spanA[i] = (tenkan[i] + kijun[i]) / 2
    }
    if (i + kijunP < len) {
      chikou[i] = closes[i + kijunP]
    }
  }
  return { tenkan, kijun, spanA, senkouB, chikou }
}

function cciValues(highs, lows, closes, period = 20) {
  const len = closes.length
  const tp = closes.map((_, i) => (highs[i] + lows[i] + closes[i]) / 3)
  const out = new Array(len).fill(null)
  for (let i = period - 1; i < len; i++) {
    let sum = 0
    for (let j = 0; j < period; j++) sum += tp[i - j]
    const mean = sum / period
    let meanDev = 0
    for (let j = 0; j < period; j++) meanDev += Math.abs(tp[i - j] - mean)
    meanDev /= period
    out[i] = meanDev > 0 ? (tp[i] - mean) / (0.015 * meanDev) : null
  }
  return out
}

function ttmSqueezeValues(highs, lows, closes, bollPeriod = 20, bollMult = 2, keltnerPeriod = 20, keltnerAtrPeriod = 10, keltnerMult = 1.5) {
  const boll = bollingerBands(closes, bollPeriod, bollMult)
  const keltner = keltnerChannelValues(highs, lows, closes, keltnerPeriod, keltnerAtrPeriod, keltnerMult)
  const len = closes.length
  const squeeze = new Array(len).fill(false)
  for (let i = 0; i < len; i++) {
    if (boll.lower[i] == null || keltner.lower[i] == null) continue
    squeeze[i] = boll.lower[i] >= keltner.lower[i] && boll.upper[i] <= keltner.upper[i]
  }
  const momentum = new Array(len).fill(null)
  const tp = closes.map((_, i) => (highs[i] + lows[i] + closes[i]) / 3)
  const emaTp = emaFinite(tp, bollPeriod)
  for (let i = 0; i < len; i++) {
    if (emaTp[i] != null) {
      momentum[i] = tp[i] - emaTp[i]
    }
  }
  return { squeeze, momentum }
}

function sarValues(highs, lows, closes, step = 0.02, maxStep = 0.2) {
  const len = closes.length
  if (len < 2) return { sar: new Array(len).fill(null), direction: new Array(len).fill(0) }
  const sar = new Array(len).fill(null)
  const direction = new Array(len).fill(0)
  let isLong = closes[1] > closes[0]
  let af = step
  let ep = isLong ? highs[1] : lows[1]
  let prevSar = isLong ? lows[0] : highs[0]
  sar[0] = null
  sar[1] = prevSar
  direction[1] = isLong ? 1 : -1
  for (let i = 2; i < len; i++) {
    let curSar = prevSar + af * (ep - prevSar)
    if (isLong) {
      curSar = Math.min(curSar, lows[i - 1], lows[i - 2])
      if (lows[i] < curSar) {
        isLong = false
        curSar = ep
        ep = lows[i]
        af = step
      } else {
        if (highs[i] > ep) {
          ep = highs[i]
          af = Math.min(af + step, maxStep)
        }
      }
    } else {
      curSar = Math.max(curSar, highs[i - 1], highs[i - 2])
      if (highs[i] > curSar) {
        isLong = true
        curSar = ep
        ep = highs[i]
        af = step
      } else {
        if (lows[i] < ep) {
          ep = lows[i]
          af = Math.min(af + step, maxStep)
        }
      }
    }
    sar[i] = curSar
    direction[i] = isLong ? 1 : -1
    prevSar = curSar
  }
  return { sar, direction }
}

function donchianChannelValues(highs, lows, period = 20) {
  const len = highs.length
  const upper = new Array(len).fill(null)
  const lower = new Array(len).fill(null)
  const mid = new Array(len).fill(null)
  for (let i = period - 1; i < len; i++) {
    let hi = -Infinity
    let lo = Infinity
    for (let j = 0; j < period; j++) {
      hi = Math.max(hi, highs[i - j])
      lo = Math.min(lo, lows[i - j])
    }
    upper[i] = hi
    lower[i] = lo
    mid[i] = (hi + lo) / 2
  }
  return { upper, mid, lower }
}

function adxValues(highs, lows, closes, period = 14) {
  const len = closes.length
  if (len < 2) return { adx: new Array(len).fill(null), diP: new Array(len).fill(null), diM: new Array(len).fill(null) }
  const tr = new Array(len).fill(0)
  const plusDM = new Array(len).fill(0)
  const minusDM = new Array(len).fill(0)
  tr[0] = highs[0] - lows[0]
  for (let i = 1; i < len; i++) {
    tr[i] = Math.max(highs[i] - lows[i], Math.abs(highs[i] - closes[i - 1]), Math.abs(lows[i] - closes[i - 1]))
    const upMove = highs[i] - highs[i - 1]
    const downMove = lows[i - 1] - lows[i]
    plusDM[i] = upMove > downMove && upMove > 0 ? upMove : 0
    minusDM[i] = downMove > upMove && downMove > 0 ? downMove : 0
  }
  const smoothTR = new Array(len).fill(null)
  const smoothPDM = new Array(len).fill(null)
  const smoothMDM = new Array(len).fill(null)
  let sTR = 0, sPDM = 0, sMDM = 0
  for (let i = 0; i < period && i < len; i++) {
    sTR += tr[i]; sPDM += plusDM[i]; sMDM += minusDM[i]
  }
  if (len >= period) {
    smoothTR[period - 1] = sTR
    smoothPDM[period - 1] = sPDM
    smoothMDM[period - 1] = sMDM
    for (let i = period; i < len; i++) {
      smoothTR[i] = smoothTR[i - 1] - smoothTR[i - 1] / period + tr[i]
      smoothPDM[i] = smoothPDM[i - 1] - smoothPDM[i - 1] / period + plusDM[i]
      smoothMDM[i] = smoothMDM[i - 1] - smoothMDM[i - 1] / period + minusDM[i]
    }
  }
  const diP = new Array(len).fill(null)
  const diM = new Array(len).fill(null)
  const dx = new Array(len).fill(null)
  for (let i = 0; i < len; i++) {
    if (smoothTR[i] != null && smoothTR[i] > 0) {
      diP[i] = 100 * smoothPDM[i] / smoothTR[i]
      diM[i] = 100 * smoothMDM[i] / smoothTR[i]
      const sum = diP[i] + diM[i]
      dx[i] = sum > 0 ? 100 * Math.abs(diP[i] - diM[i]) / sum : 0
    }
  }
  const adx = new Array(len).fill(null)
  if (len >= period * 2 - 1) {
    let sumDx = 0
    for (let i = period - 1; i < period * 2 - 1 && i < len; i++) {
      sumDx += dx[i] || 0
    }
    adx[period * 2 - 2] = sumDx / period
    for (let i = period * 2 - 1; i < len; i++) {
      adx[i] = (adx[i - 1] * (period - 1) + (dx[i] || 0)) / period
    }
  }
  return { adx, diP, diM }
}

function williamsRValues(highs, lows, closes, period = 14) {
  const len = closes.length
  const out = new Array(len).fill(null)
  for (let i = period - 1; i < len; i++) {
    let hi = -Infinity
    let lo = Infinity
    for (let j = 0; j < period; j++) {
      hi = Math.max(hi, highs[i - j])
      lo = Math.min(lo, lows[i - j])
    }
    const range = hi - lo
    out[i] = range > 0 ? ((hi - closes[i]) / range) * -100 : null
  }
  return out
}

function stochRsiValues(closes, rsiPeriod = 14, stochPeriod = 14, kSmooth = 3, dSmooth = 3) {
  const rsi = rsiBundle(closes, rsiPeriod)
  const len = closes.length
  const stochRsi = new Array(len).fill(null)
  for (let i = stochPeriod - 1; i < len; i++) {
    let minRsi = Infinity
    let maxRsi = -Infinity
    let valid = true
    for (let j = 0; j < stochPeriod; j++) {
      if (rsi[i - j] == null) { valid = false; break }
      minRsi = Math.min(minRsi, rsi[i - j])
      maxRsi = Math.max(maxRsi, rsi[i - j])
    }
    if (!valid) continue
    stochRsi[i] = maxRsi !== minRsi ? ((rsi[i] - minRsi) / (maxRsi - minRsi)) * 100 : 0
  }
  const k = new Array(len).fill(null)
  const d = new Array(len).fill(null)
  for (let i = 0; i < len; i++) {
    if (stochRsi[i] == null) continue
    let kSum = 0
    let kCnt = 0
    for (let j = 0; j < kSmooth && i - j >= 0; j++) {
      if (stochRsi[i - j] != null) { kSum += stochRsi[i - j]; kCnt++ }
    }
    if (kCnt === kSmooth) k[i] = kSum / kCnt
  }
  for (let i = 0; i < len; i++) {
    if (k[i] == null) continue
    let dSum = 0
    let dCnt = 0
    for (let j = 0; j < dSmooth && i - j >= 0; j++) {
      if (k[i - j] != null) { dSum += k[i - j]; dCnt++ }
    }
    if (dCnt === dSmooth) d[i] = dSum / dCnt
  }
  return { k, d }
}

function cmfValues(highs, lows, closes, vols, period = 20) {
  const len = closes.length
  const out = new Array(len).fill(null)
  for (let i = period - 1; i < len; i++) {
    let sumMFV = 0
    let sumVol = 0
    for (let j = 0; j < period; j++) {
      const idx = i - j
      const range = highs[idx] - lows[idx]
      const mfv = range > 0 ? ((closes[idx] - lows[idx]) - (highs[idx] - closes[idx])) / range * vols[idx] : 0
      sumMFV += mfv
      sumVol += vols[idx]
    }
    out[i] = sumVol > 0 ? sumMFV / sumVol : null
  }
  return out
}

function aroonValues(highs, lows, period = 25) {
  const len = highs.length
  const up = new Array(len).fill(null)
  const down = new Array(len).fill(null)
  for (let i = period - 1; i < len; i++) {
    let highIdx = 0
    let lowIdx = 0
    for (let j = 1; j < period; j++) {
      if (highs[i - j] > highs[i - highIdx]) highIdx = j
      if (lows[i - j] < lows[i - lowIdx]) lowIdx = j
    }
    up[i] = ((period - 1 - highIdx) / (period - 1)) * 100
    down[i] = ((period - 1 - lowIdx) / (period - 1)) * 100
  }
  return { up, down }
}

function cmoValues(closes, period = 14) {
  const len = closes.length
  const out = new Array(len).fill(null)
  for (let i = period; i < len; i++) {
    let sumUp = 0
    let sumDown = 0
    for (let j = 0; j < period; j++) {
      const diff = closes[i - j] - closes[i - j - 1]
      if (diff > 0) sumUp += diff
      else sumDown -= diff
    }
    out[i] = sumUp + sumDown > 0 ? ((sumUp - sumDown) / (sumUp + sumDown)) * 100 : 0
  }
  return out
}

function forceIndexValues(closes, vols, period = 13) {
  const len = closes.length
  if (len < 2) return new Array(len).fill(null)
  const raw = new Array(len).fill(null)
  raw[0] = 0
  for (let i = 1; i < len; i++) {
    raw[i] = (closes[i] - closes[i - 1]) * vols[i]
  }
  const out = emaFinite(raw, period)
  return out
}

function pivotPointsValues(highs, lows, closes) {
  const len = closes.length
  const pp = new Array(len).fill(null)
  const s1 = new Array(len).fill(null)
  const s2 = new Array(len).fill(null)
  const r1 = new Array(len).fill(null)
  const r2 = new Array(len).fill(null)
  for (let i = 1; i < len; i++) {
    const h = highs[i - 1]
    const l = lows[i - 1]
    const c = closes[i - 1]
    const p = (h + l + c) / 3
    pp[i] = p
    r1[i] = 2 * p - l
    s1[i] = 2 * p - h
    r2[i] = p + (h - l)
    s2[i] = p - (h - l)
  }
  return { pp, s1, s2, r1, r2 }
}

function demaValues(closes, period = 21) {
  const len = closes.length
  const e1 = emaFinite(closes, period)
  const e1Arr = e1.map(v => v ?? 0)
  const e2 = emaFinite(e1Arr, period)
  const out = new Array(len).fill(null)
  for (let i = 0; i < len; i++) {
    if (e1[i] != null && e2[i] != null) {
      out[i] = 2 * e1[i] - e2[i]
    }
  }
  return out
}

function zigzagValues(highs, lows, closes, threshold = 5) {
  const len = closes.length
  if (len < 3) return { zigzag: new Array(len).fill(null), directions: new Array(len).fill(0) }
  const points = []
  points.push({ idx: 0, price: highs[0], isHigh: true })
  let lastHigh = { idx: 0, price: highs[0] }
  let lastLow = { idx: 0, price: lows[0] }
  let lookingFor = 'high'
  for (let i = 1; i < len; i++) {
    const chgPct = threshold
    if (lookingFor === 'high') {
      if (highs[i] >= lastHigh.price) {
        lastHigh = { idx: i, price: highs[i] }
        if (points.length > 0) points[points.length - 1] = { idx: i, price: highs[i], isHigh: true }
      } else if (lastHigh.price - lows[i] >= lastHigh.price * chgPct / 100) {
        points.push({ idx: lastHigh.idx, price: lastHigh.price, isHigh: true })
        lastLow = { idx: i, price: lows[i] }
        lookingFor = 'low'
      }
    } else {
      if (lows[i] <= lastLow.price) {
        lastLow = { idx: i, price: lows[i] }
        if (points.length > 0) points[points.length - 1] = { idx: i, price: lows[i], isHigh: false }
      } else if (highs[i] - lastLow.price >= lastLow.price * chgPct / 100) {
        points.push({ idx: lastLow.idx, price: lastLow.price, isHigh: false })
        lastHigh = { idx: i, price: highs[i] }
        lookingFor = 'high'
      }
    }
  }
  const zigzag = new Array(len).fill(null)
  const directions = new Array(len).fill(0)
  for (let p = 0; p < points.length; p++) {
    const pt = points[p]
    zigzag[pt.idx] = pt.price
    directions[pt.idx] = pt.isHigh ? 1 : -1
  }
  return { zigzag, directions }
}

function satsValues(highs, lows, closes, vols, {
  atrLen = 14,
  baseMult = 2.0,
  erLen = 20,
  adaptStrength = 0.5,
  atrBaselineLen = 100,
  useAdaptive = true,
  useTqi = true,
  qualityStrength = 0.4,
  qualityCurve = 1.5,
  smoothMult = true,
  useAsymBands = true,
  asymStrength = 0.5,
  useEffAtr = true,
  useCharFlip = true,
  charFlipMinAge = 5,
  charFlipHigh = 0.55,
  charFlipLow = 0.25,
  tqiWeightEr = 0.35,
  tqiWeightVol = 0.20,
  tqiWeightStruct = 0.25,
  tqiWeightMom = 0.20,
  tqiStructLen = 20,
  tqiMomLen = 10,
  volLen = 20,
  multSmoothAlpha = 0.15,
} = {}) {
  const len = closes.length
  const rawAtr = atrValues(highs, lows, closes, atrLen)
  const atrBase = smaValues(rawAtr, atrBaselineLen)
  const outStLine = new Array(len).fill(null)
  const outUpper = new Array(len).fill(null)
  const outLower = new Array(len).fill(null)
  const outDirection = new Array(len).fill(0)
  const outTqi = new Array(len).fill(0)

  let prevLowerBand = null
  let prevUpperBand = null
  let prevDir = 0
  let prevActiveMultSm = null
  let prevPassiveMultSm = null
  let trendStartBar = 0

  const tqiWeightSum = tqiWeightEr + tqiWeightVol + tqiWeightStruct + tqiWeightMom
  const tqiWeightDenom = tqiWeightSum > 0 ? tqiWeightSum : 1

  for (let i = 0; i < len; i++) {
    if (rawAtr[i] == null || atrBase[i] == null) continue
    const atrVal = rawAtr[i]
    const volRatio = atrBase[i] !== 0 ? atrVal / atrBase[i] : 1

    let erValue = 0
    if (i >= erLen) {
      const change = Math.abs(closes[i] - closes[i - erLen])
      let volatility = 0
      for (let j = 0; j < erLen; j++) {
        volatility += Math.abs(closes[i - j] - closes[i - j - 1])
      }
      erValue = volatility !== 0 ? change / volatility : 0
    }

    const effAtr = useEffAtr ? atrVal * (0.5 + 0.5 * erValue) : atrVal

    const tqiEr = Math.max(0, Math.min(1, erValue))

    let tqiVol = 0.5
    if (vols[i] > 0 && i >= volLen) {
      let vMean = 0
      for (let j = 0; j < volLen; j++) vMean += vols[i - j]
      vMean /= volLen
      let vStdSq = 0
      for (let j = 0; j < volLen; j++) {
        const d = vols[i - j] - vMean
        vStdSq += d * d
      }
      const vStd = Math.sqrt(vStdSq / volLen)
      const volZ = vStd !== 0 ? (vols[i] - vMean) / vStd : 0
      const t = Math.max(0, Math.min(1, (volZ - (-1)) / (2 - (-1))))
      tqiVol = t
    } else {
      const t = Math.max(0, Math.min(1, (volRatio - 0.6) / (1.8 - 0.6)))
      tqiVol = t
    }

    let tqiStruct = 0
    if (i >= tqiStructLen) {
      let structHi = -Infinity
      let structLo = Infinity
      for (let j = 0; j < tqiStructLen; j++) {
        structHi = Math.max(structHi, highs[i - j])
        structLo = Math.min(structLo, lows[i - j])
      }
      const structRange = structHi - structLo
      const pricePos = structRange !== 0 ? (closes[i] - structLo) / structRange : 0.5
      tqiStruct = Math.max(0, Math.min(1, Math.abs(pricePos - 0.5) * 2))
    }

    let tqiMom = 0
    if (i >= tqiMomLen) {
      const windowChange = closes[i] - closes[i - tqiMomLen]
      let alignedBars = 0
      for (let j = 0; j < tqiMomLen; j++) {
        const barChange = closes[i - j] - closes[i - j - 1]
        if ((windowChange > 0 && barChange > 0) || (windowChange < 0 && barChange < 0)) {
          alignedBars++
        }
      }
      tqiMom = alignedBars / tqiMomLen
    }

    const tqiRaw = useTqi
      ? (tqiEr * tqiWeightEr + tqiVol * tqiWeightVol + tqiStruct * tqiWeightStruct + tqiMom * tqiWeightMom) / tqiWeightDenom
      : 0.5
    const tqi = Math.max(0, Math.min(1, tqiRaw))
    outTqi[i] = tqi

    const legacyAdaptFactor = useAdaptive ? (1 + adaptStrength * (0.5 - erValue)) : 1
    const qualityDeviation = useTqi ? Math.pow(1 - tqi, qualityCurve) : 0.5
    const tqiMult = 1 - qualityStrength + qualityStrength * (0.6 + 0.8 * qualityDeviation)
    const symMult = baseMult * legacyAdaptFactor * tqiMult

    let activeMultRaw = symMult
    let passiveMultRaw = symMult
    if (useTqi && useAsymBands) {
      const asymTighten = 1 - asymStrength * tqi * 0.3
      const asymWiden = 1 + asymStrength * tqi * 0.4
      activeMultRaw = symMult * asymTighten
      passiveMultRaw = symMult * asymWiden
    }

    const activeMultSm = prevActiveMultSm == null
      ? activeMultRaw
      : (smoothMult ? prevActiveMultSm * (1 - multSmoothAlpha) + activeMultRaw * multSmoothAlpha : activeMultRaw)
    const passiveMultSm = prevPassiveMultSm == null
      ? passiveMultRaw
      : (smoothMult ? prevPassiveMultSm * (1 - multSmoothAlpha) + passiveMultRaw * multSmoothAlpha : passiveMultRaw)
    prevActiveMultSm = activeMultSm
    prevPassiveMultSm = passiveMultSm

    const activeMult = activeMultSm
    const passiveMult = passiveMultSm

    const curPrevDir = prevDir === 0 ? 1 : prevDir
    const lowerMult = curPrevDir === 1 ? activeMult : passiveMult
    const upperMult = curPrevDir === 1 ? passiveMult : activeMult

    const hl2 = (highs[i] + lows[i]) / 2
    const lowerBandRaw = hl2 - lowerMult * effAtr
    const upperBandRaw = hl2 + upperMult * effAtr

    let lowerBand = prevLowerBand == null
      ? lowerBandRaw
      : (closes[i - 1] > prevLowerBand ? Math.max(lowerBandRaw, prevLowerBand) : lowerBandRaw)
    let upperBand = prevUpperBand == null
      ? upperBandRaw
      : (closes[i - 1] < prevUpperBand ? Math.min(upperBandRaw, prevUpperBand) : upperBandRaw)

    const priceFlipUp = prevDir === -1 && prevUpperBand != null && closes[i] > prevUpperBand
    const priceFlipDown = prevDir === 1 && prevLowerBand != null && closes[i] < prevLowerBand

    const trendAge = i - trendStartBar
    const prevTqi = i > 0 ? outTqi[i - 1] : 0.5
    const charFlipCondBase = useCharFlip && useTqi && prevTqi > charFlipHigh && tqi < charFlipLow && trendAge >= charFlipMinAge
    const charFlipDown = charFlipCondBase && curPrevDir === 1 && i > 0 && closes[i] < closes[i - 1]
    const charFlipUp = charFlipCondBase && curPrevDir === -1 && i > 0 && closes[i] > closes[i - 1]

    const finalFlipUp = priceFlipUp || charFlipUp
    const finalFlipDown = priceFlipDown || charFlipDown

    let dir = prevDir === 0 ? 1 : (finalFlipUp ? 1 : (finalFlipDown ? -1 : curPrevDir))
    if (dir !== curPrevDir) trendStartBar = i

    prevLowerBand = lowerBand
    prevUpperBand = upperBand
    prevDir = dir

    outStLine[i] = dir === 1 ? lowerBand : upperBand
    outUpper[i] = upperBand
    outLower[i] = lowerBand
    outDirection[i] = dir
  }

  return { stLine: outStLine, upper: outUpper, lower: outLower, direction: outDirection, tqi: outTqi }
}

function tearDownAllSubPanes() {
  if (!chart) return
  ind.obv = removeSeriesSafe(ind.obv)
  ind.macdHist = removeSeriesSafe(ind.macdHist)
  ind.macdDif = removeSeriesSafe(ind.macdDif)
  ind.macdDea = removeSeriesSafe(ind.macdDea)
  ind.kdjK = removeSeriesSafe(ind.kdjK)
  ind.kdjD = removeSeriesSafe(ind.kdjD)
  ind.kdjJ = removeSeriesSafe(ind.kdjJ)
  ind.rsi = removeSeriesSafe(ind.rsi)
  ind.atr = removeSeriesSafe(ind.atr)
  ind.mfi = removeSeriesSafe(ind.mfi)
  ind.cci = removeSeriesSafe(ind.cci)
  ind.ttmHist = removeSeriesSafe(ind.ttmHist)
  ind.ttmDots = removeSeriesSafe(ind.ttmDots)
  ind.adx = removeSeriesSafe(ind.adx)
  ind.adxDiP = removeSeriesSafe(ind.adxDiP)
  ind.adxDiM = removeSeriesSafe(ind.adxDiM)
  ind.williamsR = removeSeriesSafe(ind.williamsR)
  ind.stochRsi = removeSeriesSafe(ind.stochRsi)
  ind.stochRsiD = removeSeriesSafe(ind.stochRsiD)
  ind.cmf = removeSeriesSafe(ind.cmf)
  ind.aroonUp = removeSeriesSafe(ind.aroonUp)
  ind.aroonDown = removeSeriesSafe(ind.aroonDown)
  ind.cmo = removeSeriesSafe(ind.cmo)
  ind.forceIndex = removeSeriesSafe(ind.forceIndex)
  while (chart.panes().length > 1) {
    chart.removePane(chart.panes().length - 1)
  }
  chart.panes()[0]?.setStretchFactor(1)
}

const subLineOpts = {
  lineWidth: 1,
  lastValueVisible: false,
  priceLineVisible: false,
}

function syncSubPaneIndicators(times, closes, highs, lows, vols) {
  if (!chart) return
  tearDownAllSubPanes()

  const subs = []
  if (showOBV.value) subs.push('obv')
  if (showMACD.value) subs.push('macd')
  if (showKDJ.value) subs.push('kdj')
  if (showRSI.value) subs.push('rsi')
  if (showATR.value) subs.push('atr')
  if (showMFI.value) subs.push('mfi')
  if (showCCI.value) subs.push('cci')
  if (showTTMSqueeze.value) subs.push('ttmSqueeze')
  if (showADX.value) subs.push('adx')
  if (showWilliamsR.value) subs.push('williamsR')
  if (showStochRSI.value) subs.push('stochRsi')
  if (showCMF.value) subs.push('cmf')
  if (showAroon.value) subs.push('aroon')
  if (showCMO.value) subs.push('cmo')
  if (showForceIndex.value) subs.push('forceIndex')
  if (subs.length === 0) return

  chart.panes()[0]?.setStretchFactor(3)

  let paneIdx = 1
  for (const key of subs) {
    if (key === 'obv') {
      const obv = obvValues(closes, vols)
      ind.obv = chart.addSeries(
        LineSeries,
        {
          color: '#22c55e',
          lineWidth: 1,
          title: 'OBV',
          lastValueVisible: true,
          priceLineVisible: false,
          priceFormat: { type: 'price', precision: 0, minMove: 1 },
        },
        paneIdx,
      )
      ind.obv.setData(toLineData(times, obv))
    } else if (key === 'macd') {
      const { dif, dea, hist } = macdBundle(closes)
      ind.macdHist = chart.addSeries(
        HistogramSeries,
        {
          priceFormat: { type: 'price', precision: 4, minMove: 0.0001 },
          priceScaleId: 'macd',
        },
        paneIdx,
      )
      ind.macdDif = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#f59e0b',
          title: 'DIF',
          priceScaleId: 'macd',
        },
        paneIdx,
      )
      ind.macdDea = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#6366f1',
          title: 'DEA',
          priceScaleId: 'macd',
        },
        paneIdx,
      )
      const histData = []
      for (let i = 0; i < times.length; i++) {
        const hv = hist[i]
        if (hv == null || !Number.isFinite(hv)) continue
        histData.push({
          time: times[i],
          value: hv,
          color:
            hv >= 0
              ? 'rgba(239, 83, 80, 0.55)'
              : 'rgba(38, 166, 154, 0.55)',
        })
      }
      ind.macdHist.setData(histData)
      ind.macdDif.setData(toLineData(times, dif))
      ind.macdDea.setData(toLineData(times, dea))
    } else if (key === 'kdj') {
      const { K, D, J } = kdjBundle(highs, lows, closes, 9)
      ind.kdjK = chart.addSeries(
        LineSeries,
        { ...subLineOpts, color: '#f59e0b', title: 'K' },
        paneIdx,
      )
      ind.kdjD = chart.addSeries(
        LineSeries,
        { ...subLineOpts, color: '#3b82f6', title: 'D' },
        paneIdx,
      )
      ind.kdjJ = chart.addSeries(
        LineSeries,
        { ...subLineOpts, color: '#a855f7', title: 'J' },
        paneIdx,
      )
      ind.kdjK.setData(toLineData(times, K))
      ind.kdjD.setData(toLineData(times, D))
      ind.kdjJ.setData(toLineData(times, J))
    } else if (key === 'rsi') {
      const rsi = rsiBundle(closes, 14)
      ind.rsi = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#d946ef',
          title: 'RSI14',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.rsi.setData(toLineData(times, rsi))
    } else if (key === 'atr') {
      const atr = atrValues(highs, lows, closes, 14)
      ind.atr = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#06b6d4',
          title: 'ATR14',
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      ind.atr.setData(toLineData(times, atr))
    } else if (key === 'mfi') {
      const mfi = mfiValues(highs, lows, closes, vols, 14)
      ind.mfi = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#f97316',
          title: 'MFI14',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.mfi.setData(toLineData(times, mfi))
    } else if (key === 'cci') {
      const cci = cciValues(highs, lows, closes, 20)
      ind.cci = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#eab308',
          title: 'CCI20',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.cci.setData(toLineData(times, cci))
    } else if (key === 'ttmSqueeze') {
      const { squeeze, momentum } = ttmSqueezeValues(highs, lows, closes)
      ind.ttmHist = chart.addSeries(
        HistogramSeries,
        {
          priceLineVisible: false,
          lastValueVisible: false,
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      ind.ttmDots = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#6366f1',
          lineWidth: 0,
          pointMarkersVisible: true,
          pointMarkersRadius: 2,
          title: 'SQZ',
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      const histData = []
      const dotData = []
      for (let i = 0; i < times.length; i++) {
        const mv = momentum[i]
        if (mv != null && Number.isFinite(mv)) {
          histData.push({
            time: times[i],
            value: mv,
            color: mv >= 0
              ? (squeeze[i] ? 'rgba(239, 83, 80, 0.7)' : 'rgba(239, 83, 80, 0.4)')
              : (squeeze[i] ? 'rgba(38, 166, 154, 0.7)' : 'rgba(38, 166, 154, 0.4)'),
          })
          dotData.push({
            time: times[i],
            value: 0,
            color: squeeze[i] ? '#eab308' : '#22c55e',
          })
        }
      }
      ind.ttmHist.setData(histData)
      ind.ttmDots.setData(dotData)
    } else if (key === 'adx') {
      const { adx, diP, diM } = adxValues(highs, lows, closes, 14)
      ind.adxDiP = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#22c55e',
          lineWidth: 1,
          title: '+DI',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.adxDiM = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#ef4444',
          lineWidth: 1,
          title: '-DI',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.adx = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#3b82f6',
          lineWidth: 2,
          title: 'ADX',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.adxDiP.setData(toLineData(times, diP))
      ind.adxDiM.setData(toLineData(times, diM))
      ind.adx.setData(toLineData(times, adx))
    } else if (key === 'williamsR') {
      const wr = williamsRValues(highs, lows, closes, 14)
      ind.williamsR = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#8b5cf6',
          title: 'W%R14',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.williamsR.setData(toLineData(times, wr))
    } else if (key === 'stochRsi') {
      const { k, d } = stochRsiValues(closes, 14, 14, 3, 3)
      ind.stochRsi = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#06b6d4',
          title: 'StochRSI K',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.stochRsiD = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#f59e0b',
          title: 'StochRSI D',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.stochRsi.setData(toLineData(times, k))
      ind.stochRsiD.setData(toLineData(times, d))
    } else if (key === 'cmf') {
      const cmf = cmfValues(highs, lows, closes, vols, 20)
      ind.cmf = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#14b8a6',
          title: 'CMF20',
          priceFormat: { type: 'price', precision: 3, minMove: 0.001 },
        },
        paneIdx,
      )
      ind.cmf.setData(toLineData(times, cmf))
    } else if (key === 'aroon') {
      const { up, down } = aroonValues(highs, lows, 25)
      ind.aroonUp = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#22c55e',
          title: 'Aroon Up',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.aroonDown = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#ef4444',
          title: 'Aroon Down',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.aroonUp.setData(toLineData(times, up))
      ind.aroonDown.setData(toLineData(times, down))
    } else if (key === 'cmo') {
      const cmo = cmoValues(closes, 14)
      ind.cmo = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#8b5cf6',
          title: 'CMO14',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.cmo.setData(toLineData(times, cmo))
    } else if (key === 'forceIndex') {
      const fi = forceIndexValues(closes, vols, 13)
      ind.forceIndex = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#f97316',
          title: 'FI13',
          priceFormat: { type: 'price', precision: 0, minMove: 1 },
        },
        paneIdx,
      )
      ind.forceIndex.setData(toLineData(times, fi))
    }
    paneIdx++
  }

  for (let i = 1; i < chart.panes().length; i++) {
    chart.panes()[i].setStretchFactor(1)
  }
}

function syncIndicators() {
  if (!chart || !candleSeries) return

  const { times, closes, highs, lows, vols } = extractOHLCV(mergedRawRows)
  if (!times.length) {
    ind.ma5 = removeSeriesSafe(ind.ma5)
    ind.ma10 = removeSeriesSafe(ind.ma10)
    ind.ma20 = removeSeriesSafe(ind.ma20)
    ind.ma60 = removeSeriesSafe(ind.ma60)
    ind.bollU = removeSeriesSafe(ind.bollU)
    ind.bollM = removeSeriesSafe(ind.bollM)
    ind.bollL = removeSeriesSafe(ind.bollL)
    ind.vwap = removeSeriesSafe(ind.vwap)
    ind.kama = removeSeriesSafe(ind.kama)
    ind.keltnerU = removeSeriesSafe(ind.keltnerU)
    ind.keltnerM = removeSeriesSafe(ind.keltnerM)
    ind.keltnerL = removeSeriesSafe(ind.keltnerL)
    ind.supertrend = removeSeriesSafe(ind.supertrend)
    ind.ema12 = removeSeriesSafe(ind.ema12)
    ind.ema21 = removeSeriesSafe(ind.ema21)
    ind.ichTenkan = removeSeriesSafe(ind.ichTenkan)
    ind.ichKijun = removeSeriesSafe(ind.ichKijun)
    ind.ichSpanA = removeSeriesSafe(ind.ichSpanA)
    ind.ichSpanB = removeSeriesSafe(ind.ichSpanB)
    ind.ichChikou = removeSeriesSafe(ind.ichChikou)
    ind.supertrend = removeSeriesSafe(ind.supertrend)
    ind.ema12 = removeSeriesSafe(ind.ema12)
    ind.ema21 = removeSeriesSafe(ind.ema21)
    ind.sar = removeSeriesSafe(ind.sar)
    ind.donchianU = removeSeriesSafe(ind.donchianU)
    ind.donchianM = removeSeriesSafe(ind.donchianM)
    ind.donchianL = removeSeriesSafe(ind.donchianL)
    ind.pivotPP = removeSeriesSafe(ind.pivotPP)
    ind.pivotS1 = removeSeriesSafe(ind.pivotS1)
    ind.pivotS2 = removeSeriesSafe(ind.pivotS2)
    ind.pivotR1 = removeSeriesSafe(ind.pivotR1)
    ind.pivotR2 = removeSeriesSafe(ind.pivotR2)
    ind.dema = removeSeriesSafe(ind.dema)
    ind.zigzag = removeSeriesSafe(ind.zigzag)
    ind.satsLine = removeSeriesSafe(ind.satsLine)
    ind.satsUpper = removeSeriesSafe(ind.satsUpper)
    ind.satsLower = removeSeriesSafe(ind.satsLower)
    tearDownAllSubPanes()
    return
  }

  const lineCommon = {
    lineWidth: 1,
    lastValueVisible: false,
    priceLineVisible: false,
  }

  if (showMA.value) {
    const m5 = smaValues(closes, 5)
    const m10 = smaValues(closes, 10)
    const m20 = smaValues(closes, 20)
    const m60 = smaValues(closes, 60)
    if (!ind.ma5) {
      ind.ma5 = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#f59e0b', title: 'MA5' },
        0,
      )
    }
    if (!ind.ma10) {
      ind.ma10 = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#3b82f6', title: 'MA10' },
        0,
      )
    }
    if (!ind.ma20) {
      ind.ma20 = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#a855f7', title: 'MA20' },
        0,
      )
    }
    if (!ind.ma60) {
      ind.ma60 = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#64748b', title: 'MA60' },
        0,
      )
    }
    ind.ma5.setData(toLineData(times, m5))
    ind.ma10.setData(toLineData(times, m10))
    ind.ma20.setData(toLineData(times, m20))
    ind.ma60.setData(toLineData(times, m60))
  } else {
    ind.ma5 = removeSeriesSafe(ind.ma5)
    ind.ma10 = removeSeriesSafe(ind.ma10)
    ind.ma20 = removeSeriesSafe(ind.ma20)
    ind.ma60 = removeSeriesSafe(ind.ma60)
  }

  if (showBOLL.value) {
    const { upper, mid, lower } = bollingerBands(closes, 20, 2)
    if (!ind.bollU) {
      ind.bollU = chart.addSeries(
        LineSeries,
        {
          ...lineCommon,
          color: '#94a3b8',
          lineStyle: LineStyle.Dashed,
          title: 'BOLL上',
        },
        0,
      )
    }
    if (!ind.bollM) {
      ind.bollM = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#0ea5e9', title: 'BOLL中' },
        0,
      )
    }
    if (!ind.bollL) {
      ind.bollL = chart.addSeries(
        LineSeries,
        {
          ...lineCommon,
          color: '#94a3b8',
          lineStyle: LineStyle.Dashed,
          title: 'BOLL下',
        },
        0,
      )
    }
    ind.bollU.setData(toLineData(times, upper))
    ind.bollM.setData(toLineData(times, mid))
    ind.bollL.setData(toLineData(times, lower))
  } else {
    ind.bollU = removeSeriesSafe(ind.bollU)
    ind.bollM = removeSeriesSafe(ind.bollM)
    ind.bollL = removeSeriesSafe(ind.bollL)
  }

  if (showVWAP.value) {
    const vwap = vwapValues(highs, lows, closes, vols, 20)
    if (!ind.vwap) {
      ind.vwap = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#ec4899', title: 'VWAP20' },
        0,
      )
    }
    ind.vwap.setData(toLineData(times, vwap))
  } else {
    ind.vwap = removeSeriesSafe(ind.vwap)
  }

  if (showKAMA.value) {
    const kama = kamaValues(closes, 10, 2, 30)
    if (!ind.kama) {
      ind.kama = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#14b8a6', title: 'KAMA10' },
        0,
      )
    }
    ind.kama.setData(toLineData(times, kama))
  } else {
    ind.kama = removeSeriesSafe(ind.kama)
  }

  if (showKeltner.value) {
    const { upper: kU, mid: kM, lower: kL } = keltnerChannelValues(highs, lows, closes, 20, 10, 1.5)
    if (!ind.keltnerU) {
      ind.keltnerU = chart.addSeries(
        LineSeries,
        {
          ...lineCommon,
          color: '#a78bfa',
          lineStyle: LineStyle.Dashed,
          title: 'Kelt上',
        },
        0,
      )
    }
    if (!ind.keltnerM) {
      ind.keltnerM = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#8b5cf6', title: 'Kelt中' },
        0,
      )
    }
    if (!ind.keltnerL) {
      ind.keltnerL = chart.addSeries(
        LineSeries,
        {
          ...lineCommon,
          color: '#a78bfa',
          lineStyle: LineStyle.Dashed,
          title: 'Kelt下',
        },
        0,
      )
    }
    ind.keltnerU.setData(toLineData(times, kU))
    ind.keltnerM.setData(toLineData(times, kM))
    ind.keltnerL.setData(toLineData(times, kL))
  } else {
    ind.keltnerU = removeSeriesSafe(ind.keltnerU)
    ind.keltnerM = removeSeriesSafe(ind.keltnerM)
    ind.keltnerL = removeSeriesSafe(ind.keltnerL)
  }

  if (showSupertrend.value) {
    const { supertrend: stVal, direction } = supertrendValues(highs, lows, closes, 10, 3)
    if (!ind.supertrend) {
      ind.supertrend = chart.addSeries(
        LineSeries,
        { ...lineCommon, lineWidth: 2, title: 'ST(10,3)' },
        0,
      )
    }
    const stData = []
    for (let i = 0; i < times.length; i++) {
      if (stVal[i] != null) {
        stData.push({
          time: times[i],
          value: stVal[i],
          color: direction[i] === 1 ? '#ef4444' : '#22c55e',
        })
      }
    }
    ind.supertrend.setData(stData)
  } else {
    ind.supertrend = removeSeriesSafe(ind.supertrend)
  }

  if (showEMA.value) {
    const e12 = emaFinite(closes, 12)
    const e21 = emaFinite(closes, 21)
    if (!ind.ema12) {
      ind.ema12 = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#f59e0b', title: 'EMA12' },
        0,
      )
    }
    if (!ind.ema21) {
      ind.ema21 = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#3b82f6', title: 'EMA21' },
        0,
      )
    }
    ind.ema12.setData(toLineData(times, e12))
    ind.ema21.setData(toLineData(times, e21))
  } else {
    ind.ema12 = removeSeriesSafe(ind.ema12)
    ind.ema21 = removeSeriesSafe(ind.ema21)
  }

  if (showIchimoku.value) {
    const { tenkan, kijun, spanA, senkouB, chikou } = ichimokuValues(highs, lows, closes)
    if (!ind.ichTenkan) {
      ind.ichTenkan = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#ef4444', title: '转换' },
        0,
      )
    }
    if (!ind.ichKijun) {
      ind.ichKijun = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#3b82f6', title: '基准' },
        0,
      )
    }
    if (!ind.ichSpanA) {
      ind.ichSpanA = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#22c55e', lineStyle: LineStyle.Dashed, title: '先行A' },
        0,
      )
    }
    if (!ind.ichSpanB) {
      ind.ichSpanB = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#ef4444', lineStyle: LineStyle.Dashed, title: '先行B' },
        0,
      )
    }
    if (!ind.ichChikou) {
      ind.ichChikou = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#a855f7', lineWidth: 1, lineStyle: LineStyle.Dotted, title: '迟行' },
        0,
      )
    }
    ind.ichTenkan.setData(toLineData(times, tenkan))
    ind.ichKijun.setData(toLineData(times, kijun))
    ind.ichSpanA.setData(toLineData(times, spanA))
    ind.ichSpanB.setData(toLineData(times, senkouB))
    ind.ichChikou.setData(toLineData(times, chikou))
  } else {
    ind.ichTenkan = removeSeriesSafe(ind.ichTenkan)
    ind.ichKijun = removeSeriesSafe(ind.ichKijun)
    ind.ichSpanA = removeSeriesSafe(ind.ichSpanA)
    ind.ichSpanB = removeSeriesSafe(ind.ichSpanB)
    ind.ichChikou = removeSeriesSafe(ind.ichChikou)
  }

  if (showSAR.value) {
    const { sar, direction } = sarValues(highs, lows, closes, 0.02, 0.2)
    if (!ind.sar) {
      ind.sar = chart.addSeries(
        LineSeries,
        {
          ...lineCommon,
          lineWidth: 0,
          pointMarkersVisible: true,
          pointMarkersRadius: 3,
          title: 'SAR',
        },
        0,
      )
    }
    const sarData = []
    for (let i = 0; i < times.length; i++) {
      if (sar[i] != null) {
        sarData.push({
          time: times[i],
          value: sar[i],
          color: direction[i] === 1 ? '#ef4444' : '#22c55e',
        })
      }
    }
    ind.sar.setData(sarData)
  } else {
    ind.sar = removeSeriesSafe(ind.sar)
  }

  if (showDonchian.value) {
    const { upper, mid, lower } = donchianChannelValues(highs, lows, 20)
    if (!ind.donchianU) {
      ind.donchianU = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#f97316', lineStyle: LineStyle.Dashed, title: 'DC上' },
        0,
      )
    }
    if (!ind.donchianM) {
      ind.donchianM = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#fb923c', lineStyle: LineStyle.Dotted, title: 'DC中' },
        0,
      )
    }
    if (!ind.donchianL) {
      ind.donchianL = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#f97316', lineStyle: LineStyle.Dashed, title: 'DC下' },
        0,
      )
    }
    ind.donchianU.setData(toLineData(times, upper))
    ind.donchianM.setData(toLineData(times, mid))
    ind.donchianL.setData(toLineData(times, lower))
  } else {
    ind.donchianU = removeSeriesSafe(ind.donchianU)
    ind.donchianM = removeSeriesSafe(ind.donchianM)
    ind.donchianL = removeSeriesSafe(ind.donchianL)
  }

  if (showPivot.value) {
    const { pp, s1, s2, r1, r2 } = pivotPointsValues(highs, lows, closes)
    if (!ind.pivotPP) {
      ind.pivotPP = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#a3a3a3', lineStyle: LineStyle.Dotted, title: 'PP' },
        0,
      )
    }
    if (!ind.pivotS1) {
      ind.pivotS1 = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#22c55e', lineStyle: LineStyle.Dashed, title: 'S1' },
        0,
      )
    }
    if (!ind.pivotS2) {
      ind.pivotS2 = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#16a34a', lineStyle: LineStyle.Dashed, title: 'S2' },
        0,
      )
    }
    if (!ind.pivotR1) {
      ind.pivotR1 = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#ef4444', lineStyle: LineStyle.Dashed, title: 'R1' },
        0,
      )
    }
    if (!ind.pivotR2) {
      ind.pivotR2 = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#dc2626', lineStyle: LineStyle.Dashed, title: 'R2' },
        0,
      )
    }
    ind.pivotPP.setData(toLineData(times, pp))
    ind.pivotS1.setData(toLineData(times, s1))
    ind.pivotS2.setData(toLineData(times, s2))
    ind.pivotR1.setData(toLineData(times, r1))
    ind.pivotR2.setData(toLineData(times, r2))
  } else {
    ind.pivotPP = removeSeriesSafe(ind.pivotPP)
    ind.pivotS1 = removeSeriesSafe(ind.pivotS1)
    ind.pivotS2 = removeSeriesSafe(ind.pivotS2)
    ind.pivotR1 = removeSeriesSafe(ind.pivotR1)
    ind.pivotR2 = removeSeriesSafe(ind.pivotR2)
  }

  if (showDEMA.value) {
    const d = demaValues(closes, 21)
    if (!ind.dema) {
      ind.dema = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#ec4899', title: 'DEMA21' },
        0,
      )
    }
    ind.dema.setData(toLineData(times, d))
  } else {
    ind.dema = removeSeriesSafe(ind.dema)
  }

  if (showZigZag.value) {
    const { zigzag, directions } = zigzagValues(highs, lows, closes, 5)
    if (!ind.zigzag) {
      ind.zigzag = chart.addSeries(
        LineSeries,
        {
          ...lineCommon,
          lineWidth: 2,
          lineStyle: LineStyle.Dashed,
          color: '#f59e0b',
          pointMarkersVisible: true,
          pointMarkersRadius: 4,
          title: 'ZigZag',
        },
        0,
      )
    }
    const zzData = []
    for (let i = 0; i < times.length; i++) {
      if (zigzag[i] != null) {
        zzData.push({
          time: times[i],
          value: zigzag[i],
          color: directions[i] === 1 ? '#ef4444' : '#22c55e',
        })
      }
    }
    ind.zigzag.setData(zzData)
  } else {
    ind.zigzag = removeSeriesSafe(ind.zigzag)
  }

  if (showSATS.value) {
    const { stLine, upper, lower, direction, tqi } = satsValues(highs, lows, closes, vols)
    if (!ind.satsLine) {
      ind.satsLine = chart.addSeries(
        LineSeries,
        { ...lineCommon, lineWidth: 2, title: 'SATS' },
        0,
      )
    }
    const satsData = []
    for (let i = 0; i < times.length; i++) {
      if (stLine[i] != null) {
        satsData.push({
          time: times[i],
          value: stLine[i],
          color: direction[i] === 1 ? '#ef4444' : '#22c55e',
        })
      }
    }
    ind.satsLine.setData(satsData)
    if (!ind.satsUpper) {
      ind.satsUpper = chart.addSeries(
        LineSeries,
        { ...lineCommon, lineWidth: 1, lineStyle: LineStyle.Dashed, color: 'rgba(148,163,184,0.35)', title: 'SATS上' },
        0,
      )
    }
    if (!ind.satsLower) {
      ind.satsLower = chart.addSeries(
        LineSeries,
        { ...lineCommon, lineWidth: 1, lineStyle: LineStyle.Dashed, color: 'rgba(148,163,184,0.35)', title: 'SATS下' },
        0,
      )
    }
    ind.satsUpper.setData(toLineData(times, upper))
    ind.satsLower.setData(toLineData(times, lower))
  } else {
    ind.satsLine = removeSeriesSafe(ind.satsLine)
    ind.satsUpper = removeSeriesSafe(ind.satsUpper)
    ind.satsLower = removeSeriesSafe(ind.satsLower)
  }

  syncSubPaneIndicators(times, closes, highs, lows, vols)
}

/**
 * 东方财富 K 线时间按中国内地交易所视为北京时间（无 Z 后缀时补 +08:00）。
 * 纯日期 YYYY-MM-DD 返回 null，由调用方用字符串传给图表。
 */
function eastMoneyDayToUnixSeconds(dayStr) {
  const t = String(dayStr || '').trim().replace(/\//g, '-')
  if (!t || /^\d{4}-\d{2}-\d{2}$/.test(t)) return null
  let iso = t
  if (!t.includes('T')) {
    iso = t.replace(/^(\d{4}-\d{2}-\d{2})\s+/, '$1T')
  }
  if (!/[zZ]|[+-]\d{2}:?\d{2}$/.test(iso)) {
    iso += '+08:00'
  }
  const ms = Date.parse(iso)
  if (!Number.isFinite(ms)) return null
  return Math.floor(ms / 1000)
}

/** 东财 f51 常见形态：带时间的字符串、纯日期、14 位 YYYYMMDDHHmmss（用于分页 end） */
function eastMoneyKlineFieldToUnixSeconds(s) {
  let sec = eastMoneyDayToUnixSeconds(s)
  if (sec != null) return sec
  const t = String(s || '').trim().replace(/\//g, '-')
  const dm = t.match(/^(\d{4}-\d{2}-\d{2})$/)
  if (dm) {
    const ms = Date.parse(`${dm[1]}T12:00:00+08:00`)
    return Number.isFinite(ms) ? Math.floor(ms / 1000) : null
  }
  const c14 = t.match(/^(\d{4})(\d{2})(\d{2})(\d{2})(\d{2})(\d{2})$/)
  if (c14) {
    const ms = Date.parse(
      `${c14[1]}-${c14[2]}-${c14[3]}T${c14[4]}:${c14[5]}:${c14[6]}+08:00`,
    )
    return Number.isFinite(ms) ? Math.floor(ms / 1000) : null
  }
  const c12 = t.match(/^(\d{4})(\d{2})(\d{2})(\d{2})(\d{2})$/)
  if (c12) {
    const ms = Date.parse(
      `${c12[1]}-${c12[2]}-${c12[3]}T${c12[4]}:${c12[5]}:00+08:00`,
    )
    return Number.isFinite(ms) ? Math.floor(ms / 1000) : null
  }
  return null
}

/** lightweight-charts 的 Time → UTC 毫秒，用于按 Asia/Shanghai 格式化 */
function chartTimeToUtcMs(time) {
  if (typeof time === 'number') return time * 1000
  if (typeof time === 'string') {
    if (/^\d{4}-\d{2}-\d{2}$/.test(time)) {
      return Date.parse(`${time}T12:00:00+08:00`)
    }
    return Date.parse(time)
  }
  if (time && typeof time === 'object' && 'year' in time && 'month' in time && 'day' in time) {
    const { year, month, day } = time
    const mm = String(month).padStart(2, '0')
    const dd = String(day).padStart(2, '0')
    return Date.parse(`${year}-${mm}-${dd}T12:00:00+08:00`)
  }
  return NaN
}

function formatTickTime(time, tickMarkType) {
  const ms = chartTimeToUtcMs(time)
  if (!Number.isFinite(ms)) return null
  const d = new Date(ms)
  const loc = 'zh-CN'
  if (tickMarkType === TickMarkType.Year) {
    return new Intl.DateTimeFormat(loc, { timeZone: CN_TZ, year: 'numeric' }).format(d)
  }
  if (tickMarkType === TickMarkType.Month) {
    return new Intl.DateTimeFormat(loc, { timeZone: CN_TZ, year: 'numeric', month: '2-digit' }).format(d)
  }
  if (tickMarkType === TickMarkType.DayOfMonth) {
    return new Intl.DateTimeFormat(loc, { timeZone: CN_TZ, month: '2-digit', day: '2-digit' }).format(d)
  }
  return new Intl.DateTimeFormat(loc, {
    timeZone: CN_TZ,
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(d)
}

function formatCrosshairTime(time) {
  const ms = chartTimeToUtcMs(time)
  if (!Number.isFinite(ms)) return ''
  const d = new Date(ms)
  const loc = 'zh-CN'
  const minuteLike = !DAILY_LIKE_KLT.has(activeKlt.value)
  if (minuteLike) {
    return new Intl.DateTimeFormat(loc, {
      timeZone: CN_TZ,
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
    }).format(d)
  }
  return new Intl.DateTimeFormat(loc, {
    timeZone: CN_TZ,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(d)
}

function findRawRowByChartTime(t) {
  if (t === undefined || t === null) return null
  const targetMs = chartTimeToUtcMs(t)
  if (Number.isFinite(targetMs)) {
    for (const r of mergedRawRows) {
      const ct = toChartTime(r.day)
      const ms = chartTimeToUtcMs(ct)
      if (Number.isFinite(ms) && ms === targetMs) return r
    }
  }
  for (const r of mergedRawRows) {
    const ct = toChartTime(r.day)
    if (ct === t) return r
  }
  return null
}

/** mergedRawRows 按时间升序，取最后一根作为面板默认展示 */
function syncDefaultLatestPanelRow() {
  const rows = mergedRawRows
  if (!rows.length) {
    defaultLatestRawRow.value = null
    return
  }
  defaultLatestRawRow.value = rows[rows.length - 1]
}

function parseNumStr(s) {
  const n = Number(String(s ?? '').replace(/,/g, '').replace(/%/g, '').trim())
  return Number.isFinite(n) ? n : NaN
}

function formatPrice2(s) {
  const n = parseNumStr(s)
  return Number.isFinite(n) ? n.toFixed(2) : '--'
}

/** 成交量：东财为手，按万/亿缩写 */
function formatVolumeCn(s) {
  const n = parseNumStr(s)
  if (!Number.isFinite(n)) return '--'
  if (n >= 1e8) return `${(n / 1e8).toFixed(2)}亿`
  if (n >= 1e4) return `${(n / 1e4).toFixed(2)}万`
  return String(Math.round(n))
}

/** 成交额：元 → 亿 */
function formatAmountCn(s) {
  const n = parseNumStr(s)
  if (!Number.isFinite(n)) return '--'
  return `${(n / 1e8).toFixed(2)}亿`
}

function formatPctField(s) {
  const n = parseNumStr(s)
  if (!Number.isFinite(n)) return '--'
  return `${n.toFixed(2)}%`
}

function formatSigned2(s) {
  const n = parseNumStr(s)
  if (!Number.isFinite(n)) return '--'
  const t = n.toFixed(2)
  return n > 0 ? `+${t}` : t
}

function formatPanelTitleDay(r) {
  const dailyLike = DAILY_LIKE_KLT.has(activeKlt.value)
  if (dailyLike) {
    const ymd = extractYmdDatePart(String(r.day || '').replace(/\//g, '-'))
    if (/^\d{4}-\d{2}-\d{2}$/.test(ymd)) return ymd
  }
  const t = toChartTime(r.day)
  if (t == null) return String(r.day || '').trim() || '--'
  return formatCrosshairTime(t)
}

const crosshairPanel = computed(() => {
  const r = hoverRawRow.value ?? defaultLatestRawRow.value
  if (!r) return null
  const chgPct = parseNumStr(r.changePercent)
  const sign = chgPct > 0 ? 1 : chgPct < 0 ? -1 : 0
  const neu = props.darkTheme ? '#94a3b8' : '#64748b'
  const ohlcC = sign > 0 ? CLR_RISE : sign < 0 ? CLR_FALL : neu
  const chgC = sign > 0 ? CLR_RISE : sign < 0 ? CLR_FALL : neu
  const showLatestTag = !hoverRawRow.value && defaultLatestRawRow.value
  const titleDay = formatPanelTitleDay(r)
  return {
    title: showLatestTag ? `${titleDay} · 最新` : titleDay,
    open: formatPrice2(r.open),
    close: formatPrice2(r.close),
    high: formatPrice2(r.high),
    low: formatPrice2(r.low),
    changePercent: formatPctField(r.changePercent),
    changeValue: formatSigned2(r.changeValue),
    volume: formatVolumeCn(r.volume),
    amount: formatAmountCn(r.amount),
    amplitude: formatPctField(r.amplitude),
    turnoverRate: formatPctField(r.turnoverRate),
    cOpenClose: ohlcC,
    cHigh: CLR_RISE,
    cLow: CLR_FALL,
    cChg: chgC,
    cNeu: neu,
  }
})

function clearLongPositionPriceLines() {
  longLineByKind = { entry: null, stop: null, takeProfit: null }
  if (!candleSeries) {
    longPositionPriceLines = []
    return
  }
  for (const pl of longPositionPriceLines) {
    try {
      candleSeries.removePriceLine(pl)
    } catch {
      /* ignore */
    }
  }
  longPositionPriceLines = []
}

function syncLongPositionPriceLines() {
  console.log('[DEBUG syncLongPositionPriceLines] called, showLongPosition:', showLongPosition.value, 'candleSeries:', !!candleSeries)
  clearLongPositionPriceLines()
  if (!showLongPosition.value || !candleSeries) {
    console.log('[DEBUG syncLongPositionPriceLines] early return, showLongPosition:', showLongPosition.value, 'candleSeries:', !!candleSeries)
    return
  }
  const entry = parseNumStr(longEntryStr.value)
  const stop = parseNumStr(longStopStr.value)
  const tp = parseNumStr(longTakeProfitStr.value)
  const cost = parseNumStr(longCostStr.value)
  console.log('[DEBUG syncLongPositionPriceLines] entry:', entry, 'stop:', stop, 'tp:', tp, 'cost:', cost)

  // 如果没有任何价格信息，直接返回
  if (!Number.isFinite(entry) && !Number.isFinite(cost)) {
    console.log('[DEBUG syncLongPositionPriceLines] no valid price, returning')
    return
  }

  const pushLine = (price, kind, partial) => {
    const pl = candleSeries.createPriceLine({
      price,
      lineWidth: 2,
      axisLabelVisible: true,
      ...partial,
    })
    longPositionPriceLines.push(pl)
    longLineByKind[kind] = pl
  }
  if (Number.isFinite(entry)) {
    pushLine(entry, 'entry', {
      color: '#3b82f6',
      lineStyle: LineStyle.Solid,
      title: '开仓',
    })
  }
  if (Number.isFinite(cost)) {
    pushLine(cost, 'cost', {
      color: '#f59e0b',
      lineStyle: LineStyle.Dashed,
      title: '成本',
    })
  }
  if (Number.isFinite(stop)) {
    pushLine(stop, 'stop', {
      color: CLR_FALL,
      lineStyle: LineStyle.Dashed,
      title: '止损',
    })
  }
  if (Number.isFinite(tp)) {
    pushLine(tp, 'takeProfit', {
      color: CLR_RISE,
      lineStyle: LineStyle.Dashed,
      title: '止盈',
    })
  }
  console.log('[DEBUG syncLongPositionPriceLines] done, created', longPositionPriceLines.length, 'lines')
}

function getLongDragPaneElement() {
  if (!chart) return chartContainerRef.value
  return chart.panes()[0]?.getHTMLElement() ?? chartContainerRef.value
}

function longPaneLocalYFromClient(clientY) {
  const el = getLongDragPaneElement()
  if (!el) return null
  const r = el.getBoundingClientRect()
  const y = clientY - r.top
  return Number.isFinite(y) ? y : null
}

function refreshLongPriceLineCursorFromCrosshair(param) {
  const paneEl = getLongDragPaneElement()
  if (!paneEl) return
  if (longPositionDragActive) {
    paneEl.style.cursor = 'grabbing'
    return
  }
  if (
    !showLongPosition.value ||
    !longLineByKind.entry ||
    param.point === undefined ||
    (param.paneIndex != null && param.paneIndex !== 0)
  ) {
    paneEl.style.cursor = ''
    return
  }
  const kind = hitTestLongPriceLineKind(param.point.y)
  paneEl.style.cursor = kind ? 'grab' : ''
}

function clearLongPriceLinePaneCursor() {
  if (longPositionDragActive) return
  const paneEl = getLongDragPaneElement()
  if (paneEl) paneEl.style.cursor = ''
}

/** @returns {'entry'|'stop'|'takeProfit'|null} */
function hitTestLongPriceLineKind(localY) {
  if (!candleSeries || !showLongPosition.value || localY == null) return null
  let best = null
  let bestDist = LONG_PRICE_LINE_HIT_PX + 1
  for (const kind of ['entry', 'stop', 'takeProfit']) {
    const line = longLineByKind[kind]
    if (!line) continue
    const p = Number(line.options().price)
    if (!Number.isFinite(p)) continue
    const ly = candleSeries.priceToCoordinate(p)
    if (ly == null) continue
    const cy = Number(ly)
    if (!Number.isFinite(cy)) continue
    const d = Math.abs(cy - localY)
    if (d <= LONG_PRICE_LINE_HIT_PX && d < bestDist) {
      bestDist = d
      best = kind
    }
  }
  return best
}

function detachLongDragWindowListeners() {
  if (!longDragWindowListenersOn) return
  longDragWindowListenersOn = false
  window.removeEventListener('pointermove', onLongDragWindowMove, true)
  window.removeEventListener('pointerup', onLongDragWindowUp, true)
  window.removeEventListener('pointercancel', onLongDragWindowUp, true)
}

function onLongDragWindowMove(e) {
  if (!longPositionDragActive || !longDragKind || !candleSeries) return
  longLastPointerClientY = e.clientY
  const y = longPaneLocalYFromClient(e.clientY)
  if (y == null) return
  const raw = candleSeries.coordinateToPrice(y)
  const price = raw == null ? NaN : Number(raw)
  if (!Number.isFinite(price)) return
  const s = price.toFixed(2)
  const line = longLineByKind[longDragKind]
  if (line) {
    try {
      line.applyOptions({ price })
    } catch {
      /* ignore */
    }
  }
  if (longDragKind === 'entry') longEntryStr.value = s
  else if (longDragKind === 'stop') longStopStr.value = s
  else longTakeProfitStr.value = s
}

function onLongDragWindowUp() {
  if (!longDragWindowListenersOn) return
  const was = longPositionDragActive
  detachLongDragWindowListeners()
  longPositionDragActive = false
  longDragKind = null
  if (was) syncLongPositionPriceLines()
  const paneEl = getLongDragPaneElement()
  if (paneEl) {
    if (
      longLastPointerClientY != null &&
      showLongPosition.value &&
      longLineByKind.entry
    ) {
      const ly = longPaneLocalYFromClient(longLastPointerClientY)
      const kind = ly != null ? hitTestLongPriceLineKind(ly) : null
      paneEl.style.cursor = kind ? 'grab' : ''
    } else {
      paneEl.style.cursor = ''
    }
  }
  longLastPointerClientY = null
  setTimeout(() => {
    longSuppressChartClick = false
  }, 0)
}

function onLongPriceLinePointerDownCapture(e) {
  if (!showLongPosition.value || !candleSeries) return
  if (e.pointerType === 'mouse' && e.button !== 0) return
  const y = longPaneLocalYFromClient(e.clientY)
  if (y == null) return
  const kind = hitTestLongPriceLineKind(y)
  if (!kind) return
  longLastPointerClientY = e.clientY
  longSuppressChartClick = true
  longPositionDragActive = true
  longDragKind = kind
  const paneElGrab = getLongDragPaneElement()
  if (paneElGrab) paneElGrab.style.cursor = 'grabbing'
  try {
    e.preventDefault()
  } catch {
    /* ignore */
  }
  e.stopPropagation()
  detachLongDragWindowListeners()
  longDragWindowListenersOn = true
  window.addEventListener('pointermove', onLongDragWindowMove, true)
  window.addEventListener('pointerup', onLongDragWindowUp, true)
  window.addEventListener('pointercancel', onLongDragWindowUp, true)
}

function attachLongPriceLineDragListeners() {
  detachLongPriceLinePaneListener()
  longPaneDragEl = getLongDragPaneElement()
  if (longPaneDragEl) {
    longPaneDragEl.addEventListener('pointerdown', onLongPriceLinePointerDownCapture, true)
  }
}

function detachLongPriceLinePaneListener() {
  if (longPaneDragEl) {
    longPaneDragEl.removeEventListener('pointerdown', onLongPriceLinePointerDownCapture, true)
    longPaneDragEl = null
  }
}

const longPositionStats = computed(() => {
  const entry = parseNumStr(longEntryStr.value)
  const stop = parseNumStr(longStopStr.value)
  const tp = parseNumStr(longTakeProfitStr.value)
  if (!Number.isFinite(entry)) return null
  const risk = Number.isFinite(stop) ? entry - stop : NaN
  const reward = Number.isFinite(tp) ? tp - entry : NaN
  const rr =
    Number.isFinite(risk) && risk > 0 && Number.isFinite(reward)
      ? reward / risk
      : NaN
  return {
    riskPts: risk,
    rewardPts: reward,
    riskRr: rr,
    riskPct: Number.isFinite(risk) && entry !== 0 ? (risk / entry) * 100 : NaN,
    rewardPct: Number.isFinite(reward) && entry !== 0 ? (reward / entry) * 100 : NaN,
  }
})

const longPositionHint = computed(() => {
  if (!showLongPosition.value) return ''
  if (!Number.isFinite(parseNumStr(longEntryStr.value))) {
    return '输入开仓价后显示线；止损低于开仓、止盈高于开仓为典型多单'
  }
  const s = longPositionStats.value
  if (!s) return ''
  const parts = []
  if (Number.isFinite(s.riskPts)) parts.push(`风险幅度 ${s.riskPts.toFixed(2)}`)
  if (Number.isFinite(s.rewardPts)) parts.push(`目标幅度 ${s.rewardPts.toFixed(2)}`)
  if (Number.isFinite(s.riskRr) && s.riskRr > 0) parts.push(`盈亏比 1 : ${s.riskRr.toFixed(2)}`)
  parts.push('单击线条可拖动改价')
  return parts.join(' · ')
})

function toggleLongPosition() {
  showLongPosition.value = !showLongPosition.value
  if (!showLongPosition.value) {
    longClickPickEnabled.value = false
    clearLongFocusedPriceField()
    clearLongPriceLinePaneCursor()
  }
  syncLongPositionPriceLines()
}

function fillLongEntryFromLatestClose() {
  const r = defaultLatestRawRow.value
  if (!r) return
  const c = parseNumStr(r.close)
  if (!Number.isFinite(c)) return
  longEntryStr.value = c.toFixed(2)
  showLongPosition.value = true
  syncLongPositionPriceLines()
}

const longClickNextLabel = computed(() => {
  const m = { entry: '开仓价', stop: '止损价', takeProfit: '止盈价' }
  return m[longClickNextField.value] || '开仓价'
})

const longFocusChartHint = computed(() => {
  const k = longFocusedPriceField.value
  if (!k) return ''
  const m = { entry: '开仓', stop: '止损', takeProfit: '止盈' }
  return `已选「${m[k] || ''}」：请在 K 线主图（非成交量）点击纵轴位置写入价格`
})

function cancelLongFocusBlurTimer() {
  if (longFocusBlurTimer != null) {
    clearTimeout(longFocusBlurTimer)
    longFocusBlurTimer = null
  }
}

function onLongPriceInputFocus(kind) {
  cancelLongFocusBlurTimer()
  longFocusedPriceField.value = kind
}

function onLongPriceInputBlur() {
  cancelLongFocusBlurTimer()
  longFocusBlurTimer = setTimeout(() => {
    longFocusBlurTimer = null
    longFocusedPriceField.value = null
  }, LONG_FOCUS_BLUR_CLEAR_MS)
}

function clearLongFocusedPriceField() {
  cancelLongFocusBlurTimer()
  longFocusedPriceField.value = null
}

function applyLongPriceFromChartByField(kind, price) {
  if (!Number.isFinite(price)) return
  const s = price.toFixed(2)
  showLongPosition.value = true
  if (kind === 'entry') longEntryStr.value = s
  else if (kind === 'stop') longStopStr.value = s
  else if (kind === 'takeProfit') longTakeProfitStr.value = s
  clearLongFocusedPriceField()
  syncLongPositionPriceLines()
}

function toggleLongClickPick() {
  longClickPickEnabled.value = !longClickPickEnabled.value
  if (longClickPickEnabled.value) {
    showLongPosition.value = true
    longClickNextField.value = 'entry'
  }
  syncLongPositionPriceLines()
}

function resetLongClickSequence() {
  longClickNextField.value = 'entry'
}

function applyLongClickPrice(price) {
  if (!Number.isFinite(price)) return
  const s = price.toFixed(2)
  const step = longClickNextField.value
  if (step === 'entry') {
    longEntryStr.value = s
    longClickNextField.value = 'stop'
  } else if (step === 'stop') {
    longStopStr.value = s
    longClickNextField.value = 'takeProfit'
  } else {
    longTakeProfitStr.value = s
    longClickNextField.value = 'entry'
  }
  syncLongPositionPriceLines()
}

function chartThemeOptions(isDark) {
  const minuteLike = !DAILY_LIKE_KLT.has(activeKlt.value)
  return {
    layout: {
      background: { type: 'solid', color: isDark ? '#141414' : '#ffffff' },
      textColor: isDark ? '#cbd5e1' : '#334155',
    },
    grid: {
      vertLines: { color: isDark ? '#27272a' : '#f1f5f9' },
      horzLines: { color: isDark ? '#27272a' : '#f1f5f9' },
    },
    crosshair: { mode: 1 },
    rightPriceScale: { borderColor: isDark ? '#3f3f46' : '#e2e8f0' },
    localization: {
      locale: 'zh-CN',
      dateFormat: 'yyyy-MM-dd',
      timeFormatter: (t) => formatCrosshairTime(t),
    },
    timeScale: {
      borderColor: isDark ? '#3f3f46' : '#e2e8f0',
      timeVisible: minuteLike,
      secondsVisible: false,
      tickMarkFormatter: (t, tickMarkType) => formatTickTime(t, tickMarkType),
    },
  }
}

function sortKey(dayStr) {
  const sec = eastMoneyKlineFieldToUnixSeconds(dayStr)
  if (sec != null) return sec * 1000
  const s = String(dayStr || '').trim()
  const m = s.match(/^(\d{4})-(\d{2})-(\d{2})/)
  if (m) {
    return Date.UTC(Number(m[1]), Number(m[2]) - 1, Number(m[3]))
  }
  return 0
}

/** @returns {number|string|null} lightweight-charts Time */
function toChartTime(dayStr) {
  const s = String(dayStr || '').trim()
  if (!s) return null
  const sec = eastMoneyDayToUnixSeconds(dayStr)
  if (sec != null) return sec
  if (/^\d{4}-\d{2}-\d{2}$/.test(s)) return s
  const sec2 = eastMoneyKlineFieldToUnixSeconds(s)
  if (sec2 != null) return sec2
  return s
}

function mergeKlineRows(existing, incoming) {
  const map = new Map()
  for (const r of existing) {
    if (r?.day) map.set(r.day, r)
  }
  for (const r of incoming) {
    if (r?.day && !map.has(r.day)) map.set(r.day, r)
  }
  return Array.from(map.values()).sort((a, b) => sortKey(a.day) - sortKey(b.day))
}

/** 定时刷新：保留已向左加载的更久历史，只与最新一段合并 */
function mergeRefreshWithLatest(existingSorted, latestChunk) {
  const list = Array.isArray(latestChunk) ? latestChunk : []
  if (!list.length) return existingSorted.length ? existingSorted : []
  const sortedLatest = [...list].sort((a, b) => sortKey(a.day) - sortKey(b.day))
  const cutoff = sortKey(sortedLatest[0].day)
  const kept = existingSorted.filter((r) => sortKey(r.day) < cutoff)
  const map = new Map()
  for (const r of kept) {
    if (r?.day) map.set(r.day, r)
  }
  for (const r of sortedLatest) {
    if (r?.day) map.set(r.day, r)
  }
  return Array.from(map.values()).sort((a, b) => sortKey(a.day) - sortKey(b.day))
}

function formatYmdCompactShanghai(ms) {
  return new Date(ms)
    .toLocaleString('sv-SE', { timeZone: CN_TZ })
    .slice(0, 10)
    .replace(/-/g, '')
}

function formatYmdHmsCompactShanghai(ms) {
  return new Date(ms)
    .toLocaleString('sv-SE', { timeZone: CN_TZ })
    .replace(/[- :\s]/g, '')
    .slice(0, 14)
}

/** 从东财 day 字段取出日历日期 YYYY-MM-DD（支持 2024-01-15、20240115…） */
function extractYmdDatePart(s) {
  const t = String(s || '').trim()
  const mDash = t.match(/^(\d{4}-\d{2}-\d{2})/)
  if (mDash) return mDash[1]
  const m8 = t.match(/^(\d{4})(\d{2})(\d{2})/)
  if (m8) return `${m8[1]}-${m8[2]}-${m8[3]}`
  return ''
}

/** 分钟 K 的 klt 与单根 K 线时长（秒）；用于 end 回推，避免只减 60s 与高周期重叠导致分页无增量 */
function barSecondsForMinuteKlt(klt) {
  const n = Number.parseInt(String(klt), 10)
  if (Number.isFinite(n) && n > 0) return n * 60
  return 60
}

/** 根据当前最旧一根 K 线的 day 字段生成东财 end 参数 */
function formatEastMoneyEndFromOldest(oldestDayField, klt) {
  const s = String(oldestDayField || '').trim()
  const dailyOnly = DAILY_LIKE_KLT.has(klt)
  if (dailyOnly) {
    const dateStr = extractYmdDatePart(s.replace(/\//g, '-'))
    if (!/^\d{4}-\d{2}-\d{2}$/.test(dateStr)) return ''
    const noon = Date.parse(`${dateStr}T12:00:00+08:00`)
    if (!Number.isFinite(noon)) return ''
    return formatYmdCompactShanghai(noon - 86400000)
  }
  const sec = eastMoneyKlineFieldToUnixSeconds(s)
  if (sec == null) return ''
  const back = barSecondsForMinuteKlt(klt)
  return formatYmdHmsCompactShanghai((sec - back) * 1000)
}

function applySeriesFromRaw() {
  if (!candleSeries || !volSeries) return
  const { candles, volumes } = toSeriesData(mergedRawRows)
  candleSeries.setData(candles)
  volSeries.setData(volumes)
  syncIndicators()
  syncLongPositionPriceLines()
}

function withProgrammaticTimeRange(fn) {
  programmaticRangeDepth++
  try {
    return fn()
  } finally {
    programmaticRangeDepth--
  }
}

/** 默认只展开最近若干根，避免 fitContent 全量挤在一起；仍可向左拖看更早 */
function applyDefaultVisibleRange() {
  if (!chart || !mergedRawRows.length) return
  const n = mergedRawRows.length
  const vis = Math.min(DEFAULT_VISIBLE_BARS, n)
  const from = Math.max(0, n - vis)
  const to = n - 1 + DEFAULT_RIGHT_LOGICAL_GAP
  chart.timeScale().setVisibleLogicalRange({ from, to })
}

function toSeriesData(rows) {
  const candles = []
  const volumes = []
  if (!rows?.length) return { candles, volumes }
  const sorted = [...rows].sort((a, b) => sortKey(a.day) - sortKey(b.day))
  for (const r of sorted) {
    const t = toChartTime(r.day)
    if (t === null) continue
    const o = Number(r.open)
    const h = Number(r.high)
    const l = Number(r.low)
    const c = Number(r.close)
    const v = Number(r.volume)
    if (![o, h, l, c].every(Number.isFinite)) continue
    candles.push({ time: t, open: o, high: h, low: l, close: c })
    const up = c >= o
    volumes.push({
      time: t,
      value: Number.isFinite(v) ? v : 0,
      color: up ? 'rgba(239, 83, 80, 0.45)' : 'rgba(38, 166, 154, 0.45)',
    })
  }
  return { candles, volumes }
}

function clearPoll() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function setupPoll() {
  clearPoll()
  if (props.realtimeIntervalMs > 0 && props.code) {
    pollTimer = setInterval(refreshLatestPoll, props.realtimeIntervalMs)
  }
}

function disposeChart() {
  clearPoll()
  if (loadOlderDebounceTimer) {
    clearTimeout(loadOlderDebounceTimer)
    loadOlderDebounceTimer = null
  }
  stopHistoryVisiblePoll()
  detachLongDragWindowListeners()
  detachLongPriceLinePaneListener()
  cancelLongFocusBlurTimer()
  longFocusedPriceField.value = null
  longPositionDragActive = false
  longDragKind = null
  longSuppressChartClick = false
  if (chart && logicalRangeHandler) {
    chart.timeScale().unsubscribeVisibleLogicalRangeChange(logicalRangeHandler)
    logicalRangeHandler = null
  }
  if (chart && visibleTimeRangeHandler) {
    chart.timeScale().unsubscribeVisibleTimeRangeChange(visibleTimeRangeHandler)
    visibleTimeRangeHandler = null
  }
  if (chart && crosshairMoveHandler) {
    chart.unsubscribeCrosshairMove(crosshairMoveHandler)
    crosshairMoveHandler = null
  }
  if (chart && chartClickHandler) {
    chart.unsubscribeClick(chartClickHandler)
    chartClickHandler = null
  }
  hoverRawRow.value = null
  defaultLatestRawRow.value = null
  if (chart) {
    try {
      const pe = chart.panes()[0]?.getHTMLElement()
      if (pe?.style) pe.style.cursor = ''
    } catch {
      /* ignore */
    }
    clearLongPositionPriceLines()
    chart.remove()
    chart = null
    candleSeries = null
    volSeries = null
  }
  ind.ma5 = ind.ma10 = ind.ma20 = ind.ma60 = null
  ind.bollU = ind.bollM = ind.bollL = null
  ind.obv = null
  ind.macdHist = ind.macdDif = ind.macdDea = null
  ind.kdjK = ind.kdjD = ind.kdjJ = null
  ind.rsi = null
  ind.atr = null
  ind.vwap = null
  ind.mfi = null
  ind.kama = null
  ind.keltnerU = ind.keltnerM = ind.keltnerL = null
  ind.supertrend = null
  ind.ema12 = ind.ema21 = null
  ind.ichTenkan = ind.ichKijun = ind.ichSpanA = ind.ichSpanB = ind.ichChikou = null
  ind.cci = null
  ind.ttmHist = ind.ttmDots = null
  ind.sar = null
  ind.donchianU = ind.donchianM = ind.donchianL = null
  ind.adx = ind.adxDiP = ind.adxDiM = null
  ind.williamsR = null
  ind.stochRsi = null
  ind.stochRsiD = null
  ind.cmf = null
  ind.aroonUp = ind.aroonDown = null
  ind.cmo = null
  ind.forceIndex = null
  ind.pivotPP = ind.pivotS1 = ind.pivotS2 = ind.pivotR1 = ind.pivotR2 = null
  ind.dema = null
  ind.zigzag = null
  ind.satsLine = null
  ind.satsUpper = null
  ind.satsLower = null
}

function scheduleLoadOlderDebounced() {
  if (loadOlderDebounceTimer) clearTimeout(loadOlderDebounceTimer)
  loadOlderDebounceTimer = setTimeout(() => {
    loadOlderDebounceTimer = null
    loadOlderHistory()
  }, 280)
}

/** 根据当前可见逻辑区间判断是否需要加载更早 K 线（与 subscribe 共用 + 轮询兜底） */
function tryScheduleLoadOlderFromVisibleRange(range) {
  if (!chart || !candleSeries) return
  if (programmaticRangeDepth > 0 || loadingHistory.value || !hasMoreOlder.value) return
  const lr = range ?? chart?.timeScale().getVisibleLogicalRange()
  if (!lr) return
  const info = candleSeries.barsInLogicalRange(lr)
  if (!info || typeof info.barsBefore !== 'number') return
  if (info.barsBefore < BARS_BEFORE_LOAD_MORE) {
    scheduleLoadOlderDebounced()
  }
}

function onVisibleLogicalRangeChanged(range) {
  if (!range || !candleSeries) return
  if (programmaticRangeDepth > 0) return
  tryScheduleLoadOlderFromVisibleRange(range)
}

/** 分钟线等：部分 WebView 下逻辑区间回调稀疏，时间区间在拖动时更可靠 */
function onVisibleTimeRangeChanged() {
  if (programmaticRangeDepth > 0 || !candleSeries) return
  tryScheduleLoadOlderFromVisibleRange(null)
}

function startHistoryVisiblePoll() {
  stopHistoryVisiblePoll()
  historyVisiblePollTimer = setInterval(() => {
    tryScheduleLoadOlderFromVisibleRange(null)
  }, 400)
}

function stopHistoryVisiblePoll() {
  if (historyVisiblePollTimer) {
    clearInterval(historyVisiblePollTimer)
    historyVisiblePollTimer = null
  }
}

async function loadOlderHistory() {
  if (
    loadingHistory.value ||
    !hasMoreOlder.value ||
    !mergedRawRows.length ||
    !props.code ||
    !chart ||
    !candleSeries
  ) {
    return
  }
  const kltSnap = activeKlt.value
  const codeSnap = props.code
  const oldest = mergedRawRows[0]
  const end = formatEastMoneyEndFromOldest(oldest.day, kltSnap)
  if (!end) {
    hasMoreOlder.value = false
    return
  }
  loadingHistory.value = true
  const logical = chart.timeScale().getVisibleLogicalRange()
  const beforeCount = mergedRawRows.length
  try {
    const result = await GetStockKLinePageWithFallback(
      codeSnap,
      props.stockName || '',
      kltSnap,
      HISTORY_PAGE_SIZE,
      end,
    )
    if (kltSnap !== activeKlt.value || codeSnap !== props.code) return
    const src = result?.source || ''
    if (src) activeDataSource.value = src
    const raw = result?.data
    const inc = Array.isArray(raw) ? raw : []
    if (!inc.length) {
      hasMoreOlder.value = false
      lastOlderHistoryEndTried = ''
      return
    }
    const merged = mergeKlineRows(mergedRawRows, inc)
    const added = merged.length - beforeCount
    if (added <= 0) {
      if (end === lastOlderHistoryEndTried) {
        hasMoreOlder.value = false
      } else {
        lastOlderHistoryEndTried = end
      }
      return
    }
    lastOlderHistoryEndTried = ''
    mergedRawRows = merged
    syncDefaultLatestPanelRow()
    withProgrammaticTimeRange(() => {
      applySeriesFromRaw()
      if (logical) {
        chart.timeScale().setVisibleLogicalRange({
          from: logical.from + added,
          to: logical.to + added,
        })
      }
    })
  } catch {
    /* 网络/桥接异常：保留 hasMoreOlder，用户可继续拖动重试 */
  } finally {
    loadingHistory.value = false
  }
}

async function refreshLatestPoll() {
  if (!props.code || !candleSeries) return
  const kltSnap = activeKlt.value
  const codeSnap = props.code
  try {
    const meta = INTERVALS.find((x) => x.klt === kltSnap) || INTERVALS[0]
    const result = await GetStockKLineWithFallback(
      codeSnap,
      props.stockName || '',
      meta.klt,
      meta.limit,
    )
    if (codeSnap !== props.code || activeKlt.value !== kltSnap) return
    const src = result?.source || ''
    if (src) activeDataSource.value = src
    const raw = result?.data
    const list = Array.isArray(raw) ? raw : []
    if (!list.length) return
    mergedRawRows = mergeRefreshWithLatest(mergedRawRows, list)
    syncDefaultLatestPanelRow()
    withProgrammaticTimeRange(() => applySeriesFromRaw())
  } catch {
    /* 静默，避免打断看盘 */
  }
}

function ensureChart() {
  if (!chartContainerRef.value || chart) return
  chart = createChart(chartContainerRef.value, {
    autoSize: true,
    height: props.chartHeight,
    ...chartThemeOptions(props.darkTheme),
  })
  candleSeries = chart.addSeries(CandlestickSeries, {
    upColor: '#ef5350',
    downColor: '#26a69a',
    borderVisible: false,
    wickUpColor: '#ef5350',
    wickDownColor: '#26a69a',
  })
  volSeries = chart.addSeries(
    HistogramSeries,
    {
      priceFormat: { type: 'volume' },
      priceScaleId: 'vol',
      color: 'rgba(38, 166, 154, 0.35)',
    },
    0,
  )
  chart.priceScale('vol').applyOptions({
    scaleMargins: { top: 0.82, bottom: 0 },
  })
  candleSeries.priceScale().applyOptions({
    scaleMargins: { top: 0.06, bottom: 0.22 },
  })
  logicalRangeHandler = onVisibleLogicalRangeChanged
  chart.timeScale().subscribeVisibleLogicalRangeChange(logicalRangeHandler)
  visibleTimeRangeHandler = onVisibleTimeRangeChanged
  chart.timeScale().subscribeVisibleTimeRangeChange(visibleTimeRangeHandler)
  startHistoryVisiblePoll()
  crosshairMoveHandler = (param) => {
    if (param.point === undefined) {
      hoverRawRow.value = null
      clearLongPriceLinePaneCursor()
      if (showChip.value) updateChipFromHover()
      return
    }
    refreshLongPriceLineCursorFromCrosshair(param)
    if (param.time === undefined) {
      hoverRawRow.value = null
      if (showChip.value) updateChipFromHover()
      return
    }
    const bar = param.seriesData.get(candleSeries)
    if (!bar) {
      hoverRawRow.value = null
      if (showChip.value) updateChipFromHover()
      return
    }
    hoverRawRow.value = findRawRowByChartTime(param.time)
    if (showChip.value) updateChipFromHover()
  }
  chart.subscribeCrosshairMove(crosshairMoveHandler)
  chartClickHandler = (param) => {
    if (longSuppressChartClick) return
    if (!candleSeries || !param.point) return
    if (param.paneIndex != null && param.paneIndex !== 0) return
    if (param.hoveredSeries && param.hoveredSeries !== candleSeries) return
    const y = param.point.y
    const raw = candleSeries.coordinateToPrice(y)
    const price = raw == null ? NaN : Number(raw)
    if (!Number.isFinite(price)) return
    const focusKind = longFocusedPriceField.value
    if (focusKind === 'entry' || focusKind === 'stop' || focusKind === 'takeProfit') {
      applyLongPriceFromChartByField(focusKind, price)
      return
    }
    if (!longClickPickEnabled.value || !showLongPosition.value) return
    applyLongClickPrice(price)
  }
  chart.subscribeClick(chartClickHandler)
  syncLongPositionPriceLines()
  nextTick(() => attachLongPriceLineDragListeners())
}

async function loadData() {
  if (!props.code) {
    errorText.value = '未设置股票代码'
    mergedRawRows = []
    syncDefaultLatestPanelRow()
    hasMoreOlder.value = true
    lastOlderHistoryEndTried = ''
    candleSeries?.setData([])
    volSeries?.setData([])
    syncLongPositionPriceLines()
    chipItems.value = []
    chipMeta.value = { avgCost: 0, profitRatio: 0, current: 0, hoverDate: '', minPrice: 0, maxPrice: 0 }
    return
  }
  loading.value = true
  errorText.value = ''
  mergedRawRows = []
  syncDefaultLatestPanelRow()
  hasMoreOlder.value = true
  lastOlderHistoryEndTried = ''
  try {
    const meta = INTERVALS.find((x) => x.klt === activeKlt.value) || INTERVALS[0]
    const result = await GetStockKLineWithFallback(
      props.code,
      props.stockName || '',
      meta.klt,
      meta.limit,
    )
    const src = result?.source || ''
    activeDataSource.value = src
    const raw = result?.data
    const list = Array.isArray(raw) ? raw : []
    ensureChart()
    mergedRawRows = mergeKlineRows([], list)
    syncDefaultLatestPanelRow()
    const { candles } = toSeriesData(mergedRawRows)
    if (!candles.length) {
      errorText.value =
        '暂无 K 线数据（需东方财富或新浪支持的代码，如 600519.SH、000001.SZ）'
      candleSeries?.setData([])
      volSeries?.setData([])
      syncIndicators()
      syncLongPositionPriceLines()
      return
    }
    withProgrammaticTimeRange(() => {
      applySeriesFromRaw()
      applyDefaultVisibleRange()
    })

    if (showChip.value) {
      updateChipFromHover()
    } else {
      chipItems.value = []
    }
  } catch (e) {
    errorText.value = String(e?.message || e)
  } finally {
    loading.value = false
  }
}

function onSelectKlt(klt) {
  activeKlt.value = klt
}

function toggleMA() {
  showMA.value = !showMA.value
  syncIndicators()
}
function toggleBOLL() {
  showBOLL.value = !showBOLL.value
  syncIndicators()
}
function toggleOBV() {
  showOBV.value = !showOBV.value
  syncIndicators()
}
function toggleMACD() {
  showMACD.value = !showMACD.value
  syncIndicators()
}
function toggleKDJ() {
  showKDJ.value = !showKDJ.value
  syncIndicators()
}
function toggleRSI() {
  showRSI.value = !showRSI.value
  syncIndicators()
}
function toggleATR() {
  showATR.value = !showATR.value
  syncIndicators()
}
function toggleVWAP() {
  showVWAP.value = !showVWAP.value
  syncIndicators()
}
function toggleMFI() {
  showMFI.value = !showMFI.value
  syncIndicators()
}
function toggleKAMA() {
  showKAMA.value = !showKAMA.value
  syncIndicators()
}
function toggleKeltner() {
  showKeltner.value = !showKeltner.value
  syncIndicators()
}
function toggleSupertrend() {
  showSupertrend.value = !showSupertrend.value
  syncIndicators()
}
function toggleEMA() {
  showEMA.value = !showEMA.value
  syncIndicators()
}
function toggleIchimoku() {
  showIchimoku.value = !showIchimoku.value
  syncIndicators()
}
function toggleCCI() {
  showCCI.value = !showCCI.value
  syncIndicators()
}
function toggleTTMSqueeze() {
  showTTMSqueeze.value = !showTTMSqueeze.value
  syncIndicators()
}
function toggleSAR() {
  showSAR.value = !showSAR.value
  syncIndicators()
}
function toggleDonchian() {
  showDonchian.value = !showDonchian.value
  syncIndicators()
}
function toggleADX() {
  showADX.value = !showADX.value
  syncIndicators()
}
function toggleWilliamsR() {
  showWilliamsR.value = !showWilliamsR.value
  syncIndicators()
}
function toggleStochRSI() {
  showStochRSI.value = !showStochRSI.value
  syncIndicators()
}
function toggleCMF() {
  showCMF.value = !showCMF.value
  syncIndicators()
}
function toggleAroon() {
  showAroon.value = !showAroon.value
  syncIndicators()
}
function toggleCMO() {
  showCMO.value = !showCMO.value
  syncIndicators()
}
function toggleForceIndex() {
  showForceIndex.value = !showForceIndex.value
  syncIndicators()
}
function togglePivot() {
  showPivot.value = !showPivot.value
  syncIndicators()
}
function toggleDEMA() {
  showDEMA.value = !showDEMA.value
  syncIndicators()
}
function toggleZigZag() {
  showZigZag.value = !showZigZag.value
  syncIndicators()
}
function toggleSATS() {
  showSATS.value = !showSATS.value
  syncIndicators()
}

let chipUpdateTimer = null

function toggleChip() {
  showChip.value = !showChip.value
  if (showChip.value) {
    updateChipFromHover()
  }
}

function updateChipFromHover() {
  if (!showChip.value || !mergedRawRows.length) {
    chipItems.value = []
    return
  }
  if (chipUpdateTimer) return
  chipUpdateTimer = setTimeout(() => {
    chipUpdateTimer = null
    doUpdateChip()
  }, 30)
}

function doUpdateChip() {
  if (!showChip.value || !mergedRawRows.length) {
    chipItems.value = []
    return
  }
  const r = hoverRawRow.value ?? defaultLatestRawRow.value
  let rows = mergedRawRows
  if (r) {
    const sk = sortKey(r.day)
    let hi = mergedRawRows.length
    for (let i = 0; i < mergedRawRows.length; i++) {
      if (sortKey(mergedRawRows[i].day) > sk) { hi = i; break }
    }
    rows = hi > 0 ? mergedRawRows.slice(0, hi) : mergedRawRows
  }
  if (!rows.length) {
    chipItems.value = []
    return
  }
  const result = calcChipDistribution(rows, chipBins.value)
  chipItems.value = result.items
  chipMeta.value = {
    avgCost: result.avgCost,
    profitRatio: result.profitRatio,
    current: result.current,
    hoverDate: r ? extractYmdDatePart(String(r.day || '').replace(/\//g, '-')) : '',
    minPrice: result.minPrice || 0,
    maxPrice: result.maxPrice || 0,
  }
  nextTick(() => drawChipCanvas())
}

function pricePropToInputStr(v) {
  if (v == null) return ''
  return String(v).trim()
}

watch(
  () => [props.longEntryPrice, props.longStopLossPrice, props.longTakeProfitPrice, props.costPrice],
  ([e, s, t, c]) => {
    console.log('[DEBUG props watch] triggered, e:', e, 's:', s, 't:', t, 'c:', c, 'candleSeries:', !!candleSeries)
    let needReleaseSuppress = false
    if (e !== undefined) {
      const se = pricePropToInputStr(e)
      console.log('[DEBUG props watch] entry: propVal=', e, 'converted=', se, 'current longEntryStr=', longEntryStr.value)
      if (se !== longEntryStr.value) {
        suppressLongPriceEmit.value = true
        needReleaseSuppress = true
        longEntryStr.value = se
      }
    }
    if (s !== undefined) {
      const ss = pricePropToInputStr(s)
      if (ss !== longStopStr.value) {
        suppressLongPriceEmit.value = true
        needReleaseSuppress = true
        longStopStr.value = ss
      }
    }
    if (t !== undefined) {
      const st = pricePropToInputStr(t)
      if (st !== longTakeProfitStr.value) {
        suppressLongPriceEmit.value = true
        needReleaseSuppress = true
        longTakeProfitStr.value = st
      }
    }
    if (c !== undefined) {
      const sc = pricePropToInputStr(c)
      if (sc !== longCostStr.value) {
        suppressLongPriceEmit.value = true
        needReleaseSuppress = true
        longCostStr.value = sc
      }
    }
    if (needReleaseSuppress) {
      nextTick(() => {
        suppressLongPriceEmit.value = false
      })
    }
    const hasPrice = Number.isFinite(parseNumStr(longEntryStr.value)) || Number.isFinite(parseNumStr(longStopStr.value)) || Number.isFinite(parseNumStr(longTakeProfitStr.value)) || Number.isFinite(parseNumStr(longCostStr.value))
    console.log('[DEBUG props watch] hasPrice:', hasPrice, 'showLongPosition:', showLongPosition.value)
    if (hasPrice) {
      showLongPosition.value = true
      nextTick(() => {
        console.log('[DEBUG props watch] nextTick: calling syncLongPositionPriceLines, candleSeries:', !!candleSeries)
        syncLongPositionPriceLines()
      })
    }
  },
  { immediate: true },
)

watch(longEntryStr, (v) => {
  console.log('[DEBUG longEntryStr watch] v:', v, 'suppress:', suppressLongPriceEmit.value, 'props.longEntryPrice:', props.longEntryPrice)
  if (suppressLongPriceEmit.value) return
  emit('update:longEntryPrice', v)
})
watch(longStopStr, (v) => {
  console.log('[DEBUG longStopStr watch] v:', v, 'suppress:', suppressLongPriceEmit.value, 'props.longStopLossPrice:', props.longStopLossPrice)
  if (suppressLongPriceEmit.value) return
  emit('update:longStopLossPrice', v)
})
watch(longTakeProfitStr, (v) => {
  console.log('[DEBUG longTakeProfitStr watch] v:', v, 'suppress:', suppressLongPriceEmit.value, 'props.longTakeProfitPrice:', props.longTakeProfitPrice)
  if (suppressLongPriceEmit.value) return
  emit('update:longTakeProfitPrice', v)
})
watch(longCostStr, (v) => {
  console.log('[DEBUG longCostStr watch] v:', v, 'suppress:', suppressLongPriceEmit.value, 'props.costPrice:', props.costPrice)
  if (suppressLongPriceEmit.value) return
  emit('update:costPrice', v)
})

onMounted(() => {
  console.log('[DEBUG onMounted] starting')
  nextTick(() => {
    console.log('[DEBUG onMounted] nextTick callback')
    console.log('[DEBUG onMounted] current longEntryStr:', longEntryStr.value, 'showLongPosition:', showLongPosition.value)
    ensureChart()
    console.log('[DEBUG onMounted] after ensureChart, candleSeries:', !!candleSeries)
    loadData()
    console.log('[DEBUG onMounted] after loadData call')
    setupPoll()
  })
})

onBeforeUnmount(() => {
  disposeChart()
})

watch(
  () => props.code,
  () => {
    hoverRawRow.value = null
    loadData()
    setupPoll()
  },
)

watch(activeKlt, () => {
  hoverRawRow.value = null
  chart?.applyOptions(chartThemeOptions(props.darkTheme))
  loadData()
  setupPoll()
})

watch(
  () => props.darkTheme,
  (d) => {
    chart?.applyOptions(chartThemeOptions(d))
    if (showChip.value) nextTick(() => drawChipCanvas())
  },
)

watch(
  () => props.chartHeight,
  (h) => {
    chart?.applyOptions({ height: h })
    if (showChip.value) nextTick(() => drawChipCanvas())
  },
)

watch(
  () => props.realtimeIntervalMs,
  () => setupPoll(),
)

watch(
  [showLongPosition, longEntryStr, longStopStr, longTakeProfitStr],
  () => {
    console.log('[DEBUG priceLines watch] triggered, showLongPosition:', showLongPosition.value, 'longEntryStr:', longEntryStr.value)
    if (longPositionDragActive) return
    syncLongPositionPriceLines()
  },
)

watch(showLongPosition, (newVal) => {
  console.log('[DEBUG showLongPosition watch] changed to:', newVal)
  if (newVal && candleSeries) {
    nextTick(() => syncLongPositionPriceLines())
  }
})

</script>

<template>
  <div class="lw-kline-root" :class="{ 'lw-kline--dark': darkTheme }">
    <div class="lw-kline-body">
      <div class="lw-kline-sidebar">
        <div class="lw-kline-sidebar__inner">
          <NFlex vertical :size="6">
            <div class="lw-kline-sidebar__section">
              <NText depth="3" style="font-size: 13px; font-weight: 700; display: block; margin-bottom: 4px; padding: 2px 6px; background: rgba(239,68,68,0.08); border-radius: 4px; border-left: 3px solid #ef4444; color: #ef4444">📈趋势</NText>
              <NFlex :size="4" wrap style="row-gap: 4px">
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showMA ? 'primary' : 'default'" :secondary="!showMA" @click="toggleMA">MA</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.ma }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showEMA ? 'primary' : 'default'" :secondary="!showEMA" @click="toggleEMA">EMA</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.ema }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showKAMA ? 'primary' : 'default'" :secondary="!showKAMA" @click="toggleKAMA">KAMA</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.kama }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showSupertrend ? 'primary' : 'default'" :secondary="!showSupertrend" @click="toggleSupertrend">STrend</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.supertrend }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showSAR ? 'primary' : 'default'" :secondary="!showSAR" @click="toggleSAR">SAR</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.sar }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showIchimoku ? 'primary' : 'default'" :secondary="!showIchimoku" @click="toggleIchimoku">Ichi</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.ichimoku }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showAroon ? 'primary' : 'default'" :secondary="!showAroon" @click="toggleAroon">Aroon</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.aroon }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showDEMA ? 'primary' : 'default'" :secondary="!showDEMA" @click="toggleDEMA">DEMA</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.dema }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showSATS ? 'primary' : 'default'" :secondary="!showSATS" @click="toggleSATS">SATS</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.sats }}</span>
                </NTooltip>
              </NFlex>
            </div>
            <div class="lw-kline-sidebar__section">
              <NText depth="3" style="font-size: 13px; font-weight: 700; display: block; margin-bottom: 4px; padding: 2px 6px; background: rgba(245,158,11,0.08); border-radius: 4px; border-left: 3px solid #f59e0b; color: #d97706">🎢波动</NText>
              <NFlex :size="4" wrap style="row-gap: 4px">
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showBOLL ? 'primary' : 'default'" :secondary="!showBOLL" @click="toggleBOLL">BOLL</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.boll }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showKeltner ? 'primary' : 'default'" :secondary="!showKeltner" @click="toggleKeltner">Kelt</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.keltner }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showDonchian ? 'primary' : 'default'" :secondary="!showDonchian" @click="toggleDonchian">Donch</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.donchian }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showATR ? 'primary' : 'default'" :secondary="!showATR" @click="toggleATR">ATR</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.atr }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showTTMSqueeze ? 'primary' : 'default'" :secondary="!showTTMSqueeze" @click="toggleTTMSqueeze">TTM</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.ttmSqueeze }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showZigZag ? 'primary' : 'default'" :secondary="!showZigZag" @click="toggleZigZag">ZigZag</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.zigzag }}</span>
                </NTooltip>
              </NFlex>
            </div>
            <div class="lw-kline-sidebar__section">
              <NText depth="3" style="font-size: 13px; font-weight: 700; display: block; margin-bottom: 4px; padding: 2px 6px; background: rgba(59,130,246,0.08); border-radius: 4px; border-left: 3px solid #3b82f6; color: #2563eb">💫动量</NText>
              <NFlex :size="4" wrap style="row-gap: 4px">
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showMACD ? 'primary' : 'default'" :secondary="!showMACD" @click="toggleMACD">MACD</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.macd }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showKDJ ? 'primary' : 'default'" :secondary="!showKDJ" @click="toggleKDJ">KDJ</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.kdj }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showRSI ? 'primary' : 'default'" :secondary="!showRSI" @click="toggleRSI">RSI</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.rsi }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showCCI ? 'primary' : 'default'" :secondary="!showCCI" @click="toggleCCI">CCI</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.cci }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showWilliamsR ? 'primary' : 'default'" :secondary="!showWilliamsR" @click="toggleWilliamsR">W%R</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.williamsR }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showStochRSI ? 'primary' : 'default'" :secondary="!showStochRSI" @click="toggleStochRSI">SRSI</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.stochRsi }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showCMO ? 'primary' : 'default'" :secondary="!showCMO" @click="toggleCMO">CMO</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.cmo }}</span>
                </NTooltip>
              </NFlex>
            </div>
            <div class="lw-kline-sidebar__section">
              <NText depth="3" style="font-size: 13px; font-weight: 700; display: block; margin-bottom: 4px; padding: 2px 6px; background: rgba(16,185,129,0.08); border-radius: 4px; border-left: 3px solid #10b981; color: #059669">📊量价</NText>
              <NFlex :size="4" wrap style="row-gap: 4px">
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showOBV ? 'primary' : 'default'" :secondary="!showOBV" @click="toggleOBV">OBV</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.obv }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showVWAP ? 'primary' : 'default'" :secondary="!showVWAP" @click="toggleVWAP">VWAP</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.vwap }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showMFI ? 'primary' : 'default'" :secondary="!showMFI" @click="toggleMFI">MFI</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.mfi }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showCMF ? 'primary' : 'default'" :secondary="!showCMF" @click="toggleCMF">CMF</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.cmf }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showForceIndex ? 'primary' : 'default'" :secondary="!showForceIndex" @click="toggleForceIndex">FI</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.forceIndex }}</span>
                </NTooltip>
              </NFlex>
            </div>
            <div class="lw-kline-sidebar__section">
              <NText depth="3" style="font-size: 13px; font-weight: 700; display: block; margin-bottom: 4px; padding: 2px 6px; background: rgba(139,92,246,0.08); border-radius: 4px; border-left: 3px solid #8b5cf6; color: #7c3aed">📏强度</NText>
              <NFlex :size="4" wrap style="row-gap: 4px">
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showADX ? 'primary' : 'default'" :secondary="!showADX" @click="toggleADX">ADX</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.adx }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showPivot ? 'primary' : 'default'" :secondary="!showPivot" @click="togglePivot">Pivot</NButton>
                  </template>
                  <span style="white-space: pre-line">{{ indicatorTips.pivot }}</span>
                </NTooltip>
                <NButton
                  v-if="SHOW_CHIP_TOOLBAR_BUTTON"
                  size="tiny"
                  :type="showChip ? 'primary' : 'default'"
                  :secondary="!showChip"
                  @click="toggleChip"
                >
                  筹码
                </NButton>
              </NFlex>
            </div>
          </NFlex>
        </div>
      </div>
      <div class="lw-kline-main">
        <NFlex :size="6" wrap style="row-gap: 4px; align-items: center">
          <NText depth="3" style="font-size: 12px; margin-right: 2px">周期</NText>
          <NButton
            v-for="it in INTERVALS"
            :key="it.klt"
            size="tiny"
            :type="activeKlt === it.klt ? 'primary' : 'default'"
            :secondary="activeKlt !== it.klt"
            @click="onSelectKlt(it.klt)"
          >
            {{ it.label }}
          </NButton>
          <span style="width: 12px" />
          <NText depth="3" style="font-size: 12px; margin-right: 2px">多单</NText>
          <NButton
            size="tiny"
            :type="showLongPosition ? 'primary' : 'default'"
            :secondary="!showLongPosition"
            @click="toggleLongPosition"
          >
            价位线
          </NButton>
          <NInput
            v-model:value="longEntryStr"
            size="tiny"
            placeholder="开仓"
            style="width: 80px"
            clearable
            @focus="onLongPriceInputFocus('entry')"
            @blur="onLongPriceInputBlur"
          />
          <NInput
            v-model:value="longStopStr"
            size="tiny"
            placeholder="止损"
            style="width: 80px"
            clearable
            @focus="onLongPriceInputFocus('stop')"
            @blur="onLongPriceInputBlur"
          />
          <NInput
            v-model:value="longTakeProfitStr"
            size="tiny"
            placeholder="止盈"
            style="width: 80px"
            clearable
            @focus="onLongPriceInputFocus('takeProfit')"
            @blur="onLongPriceInputBlur"
          />
          <NButton size="tiny" secondary @click="fillLongEntryFromLatestClose">
            最新收盘
          </NButton>
          <NButton
            size="tiny"
            :type="longClickPickEnabled ? 'primary' : 'default'"
            :secondary="!longClickPickEnabled"
            @click="toggleLongClickPick"
          >
            设置价位线
          </NButton>
          <NButton
            v-if="longClickPickEnabled"
            size="tiny"
            quaternary
            @click="resetLongClickSequence"
          >
            重置
          </NButton>
          <NText
            v-if="longFocusChartHint"
            depth="3"
            class="lw-kline-longpos-focus-hint"
          >
            {{ longFocusChartHint }}
          </NText>
          <NText
            v-if="longClickPickEnabled && showLongPosition"
            depth="3"
            class="lw-kline-longpos-click-hint"
          >
            点击K线设置{{ longClickNextLabel }}
          </NText>
          <NText v-if="longPositionHint" depth="3" class="lw-kline-longpos-hint">
            {{ longPositionHint }}
          </NText>
        </NFlex>
        <div
          class="lw-kline-crosshair-strip"
          :class="{ 'lw-kline-crosshair-strip--dark': darkTheme }"
        >
          <template v-if="crosshairPanel">
            <div class="lw-kline-crosshair-strip__head">
              <span class="lw-kline-crosshair-strip__date">{{ crosshairPanel.title }}</span>
            </div>
            <div class="lw-kline-crosshair-strip__grid">
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">开盘</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cOpenClose }">{{
                  crosshairPanel.open
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">收盘</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cOpenClose }">{{
                  crosshairPanel.close
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">最高</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cHigh }">{{
                  crosshairPanel.high
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">最低</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cLow }">{{
                  crosshairPanel.low
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">涨跌幅</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cChg }">{{
                  crosshairPanel.changePercent
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">涨跌额</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cChg }">{{
                  crosshairPanel.changeValue
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">成交量</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cNeu }">{{
                  crosshairPanel.volume
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">成交额</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cNeu }">{{
                  crosshairPanel.amount
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">振幅</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cNeu }">{{
                  crosshairPanel.amplitude
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">换手率</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cNeu }">{{
                  crosshairPanel.turnoverRate
                }}</span>
              </span>
            </div>
          </template>
          <NText v-else depth="3" style="font-size: 11px; line-height: 1.5">
            {{ loading ? '加载中…' : '暂无 K 线数据' }}
          </NText>
        </div>
        <NText v-if="errorText" type="error" style="font-size: 12px">{{ errorText }}</NText>
        <div class="lw-kline-chart-wrap">
          <div
            ref="chartContainerRef"
            class="lw-kline-chart"
            :style="{ height: chartHeight + 'px', minHeight: chartHeight + 'px' }"
          />
          <div
            v-if="showChip"
            class="lw-chip"
            :class="{ 'lw-chip--dark': darkTheme }"
            :style="{ height: chartHeight + 'px', minHeight: chartHeight + 'px' }"
          >
            <div class="lw-chip__head">
              <span class="lw-chip__title">筹码分布</span>
              <span v-if="chipMeta.hoverDate" class="lw-chip__meta">
                {{ chipMeta.hoverDate }}
              </span>
              <span v-if="chipItems.length" class="lw-chip__meta">
                均成本 {{ chipMeta.avgCost.toFixed(2) }} · 获利
                {{ (chipMeta.profitRatio * 100).toFixed(1) }}%
              </span>
            </div>
            <div v-if="!chipItems.length" class="lw-chip__empty">
              {{ mergedRawRows.length ? '移动鼠标到K线查看' : '暂无K线数据' }}
            </div>
            <canvas
              v-show="chipItems.length"
              ref="chipCanvasRef"
              class="lw-chip__canvas"
            />
          </div>
        </div>
        <NFlex align="center" :size="8" class="lw-kline-hint-row">
          <NText depth="3" class="lw-kline-hint-text">
            {{ stockName || code }} ·
            {{ 
              realtimeIntervalMs > 0
                ? `每 ${Math.round(realtimeIntervalMs / 1000)} 秒刷新`
                : '切换周期后加载'
            }}
            · 按住拖动查看左侧历史时会自动加载更早 K 线
            <span v-if="activeDataSource" class="lw-kline-source-tag" :class="{ 'lw-kline-source-tag--fallback': activeDataSource !== 'eastmoney' }">
              {{ activeDataSource === 'eastmoney' ? '东方财富' : activeDataSource === 'sina' ? '新浪财经' : activeDataSource === 'tencent' ? '腾讯财经' : activeDataSource === 'tdx' ? '通达信' : activeDataSource }}
            </span>
          </NText>
          <NSpin v-if="loading || loadingHistory" size="small" />
        </NFlex>
      </div>
    </div>
  </div>
</template>

<style scoped>
.lw-kline-root {
  width: 100%;
  max-width: 100%;
  min-width: 0;
  box-sizing: border-box;
  --wails-draggable: no-drag;
}
.lw-kline-body {
  display: flex;
  width: 100%;
  gap: 8px;
  align-items: stretch;
}
.lw-kline-sidebar {
  flex: 0 0 auto;
  width: 140px;
  min-width: 120px;
}
.lw-kline--dark .lw-kline-sidebar {
  border-color: #3f3f46;
}
.lw-kline-sidebar__inner {
  min-width: 0;
  position: sticky;
  top: 0;
}
.lw-kline-sidebar__section {
  margin-bottom: 6px;
}
.lw-kline-main {
  flex: 1 1 0;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.lw-kline-hint-row {
  min-width: 0;
  max-width: 100%;
}
.lw-kline-hint-text {
  font-size: 12px;
  min-width: 0;
  flex: 1 1 auto;
  overflow-wrap: anywhere;
  word-break: break-word;
}
.lw-kline-longpos-hint {
  font-size: 11px;
  line-height: 1.4;
  min-width: 0;
  flex: 1 1 200px;
  overflow-wrap: anywhere;
}
.lw-kline-longpos-click-hint {
  font-size: 11px;
  line-height: 1.4;
  min-width: 0;
  flex: 1 1 220px;
  color: #0ea5e9;
  overflow-wrap: anywhere;
}
.lw-kline--dark .lw-kline-longpos-click-hint {
  color: #38bdf8;
}
.lw-kline-longpos-focus-hint {
  font-size: 11px;
  line-height: 1.4;
  min-width: 0;
  flex: 1 1 220px;
  color: #d97706;
  overflow-wrap: anywhere;
}
.lw-kline--dark .lw-kline-longpos-focus-hint {
  color: #fbbf24;
}
.lw-kline-crosshair-strip {
  width: 100%;
  max-width: 100%;
  min-width: 0;
  box-sizing: border-box;
  padding: 6px 8px;
  border-radius: 6px;
  border: 1px solid #e2e8f0;
  background: #f8fafc;
  overflow-x: auto;
  overflow-y: hidden;
}
.lw-kline-crosshair-strip--dark {
  border-color: #3f3f46;
  background: #18181b;
}
.lw-kline-crosshair-strip__head {
  margin-bottom: 4px;
}
.lw-kline-crosshair-strip__date {
  font-weight: 700;
  font-size: 12px;
  color: #0f172a;
  white-space: nowrap;
}
.lw-kline-crosshair-strip--dark .lw-kline-crosshair-strip__date {
  color: #f1f5f9;
}
.lw-kline-kv {
  display: inline-flex;
  align-items: baseline;
  gap: 4px;
  white-space: nowrap;
  flex-shrink: 0;
}
.lw-kline-crosshair-strip__grid {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  column-gap: 14px;
  row-gap: 4px;
  font-size: 11px;
  min-width: 0;
}
.lw-kline-crosshair-strip__k {
  color: #64748b;
  white-space: nowrap;
}
.lw-kline-crosshair-strip--dark .lw-kline-crosshair-strip__k {
  color: #94a3b8;
}
.lw-kline-crosshair-strip__v {
  font-variant-numeric: tabular-nums;
  min-width: 0;
}
.lw-kline-chart {
  width: 100%;
  max-width: 100%;
  min-width: 0;
  position: relative;
  touch-action: none;
  box-sizing: border-box;
}

.lw-kline-chart-wrap {
  width: 100%;
  display: flex;
  gap: 10px;
  align-items: stretch;
  min-width: 0;
}
.lw-kline--dark .lw-kline-chart {
  border-radius: 4px;
  border: 1px solid #27272a;
}
.lw-kline-root:not(.lw-kline--dark) .lw-kline-chart {
  border-radius: 4px;
  border: 1px solid #e2e8f0;
}

.lw-chip {
  width: 160px;
  flex: 0 0 160px;
  border-radius: 4px;
  border: 1px solid #e2e8f0;
  background: #ffffff;
  box-sizing: border-box;
  padding: 8px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.lw-chip--dark {
  border-color: #27272a;
  background: #141414;
}
.lw-chip__head {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  margin-bottom: 6px;
  flex-wrap: wrap;
}
.lw-chip__title {
  font-weight: 700;
  font-size: 12px;
  color: #0f172a;
  white-space: nowrap;
}
.lw-chip--dark .lw-chip__title {
  color: #f1f5f9;
}
.lw-chip__meta {
  font-size: 11px;
  color: #64748b;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.lw-chip--dark .lw-chip__meta {
  color: #94a3b8;
}
.lw-chip__empty {
  font-size: 11px;
  color: #64748b;
}
.lw-chip--dark .lw-chip__empty {
  color: #94a3b8;
}
.lw-chip__canvas {
  flex: 1 1 auto;
  width: 100%;
  min-height: 0;
  display: block;
}
.lw-kline-source-tag {
  display: inline-block;
  font-size: 10px;
  line-height: 1;
  padding: 2px 5px;
  border-radius: 3px;
  background: #e0f2fe;
  color: #0369a1;
  vertical-align: middle;
  margin-left: 4px;
}
.lw-kline--dark .lw-kline-source-tag {
  background: #1e3a5f;
  color: #7dd3fc;
}
.lw-kline-source-tag--fallback {
  background: #fef3c7;
  color: #b45309;
}
.lw-kline--dark .lw-kline-source-tag--fallback {
  background: #422006;
  color: #fbbf24;
}
</style>
