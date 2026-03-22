<script setup>
import { GetStockEastMoneyKLine, GetStockEastMoneyKLinePage } from '../../wailsjs/go/main/App'
import {
  CandlestickSeries,
  createChart,
  HistogramSeries,
  LineSeries,
  LineStyle,
  TickMarkType,
} from 'lightweight-charts'
import { NButton, NFlex, NSpin, NText } from 'naive-ui'
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
  chartHeight: { type: Number, default: 460 },
  /** 定时拉取当前周期最新 K 线，毫秒；0 关闭；默认 60 秒 */
  realtimeIntervalMs: { type: Number, default: 1000*60 },
})

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
const loading = ref(false)
const loadingHistory = ref(false)
const errorText = ref('')

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
/** 上一次请求更早 K 线使用的 end，用于识别「重叠返回」避免误判无更多数据 */
let lastOlderHistoryEndTried = ''
/** >0 时表示由代码在改时间轴（fitContent / setData / setVisibleLogicalRange），不触发分页加载 */
let programmaticRangeDepth = 0

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
  if (subs.length === 0) return

  chart.panes()[0]?.setStretchFactor(8 + subs.length * 2)

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
  const to = n - 1
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
  hoverRawRow.value = null
  defaultLatestRawRow.value = null
  if (chart) {
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
    const raw = await GetStockEastMoneyKLinePage(
      codeSnap,
      props.stockName || '',
      kltSnap,
      HISTORY_PAGE_SIZE,
      end,
    )
    if (kltSnap !== activeKlt.value || codeSnap !== props.code) return
    const inc = Array.isArray(raw) ? raw : []
    if (!inc.length) {
      hasMoreOlder.value = false
      lastOlderHistoryEndTried = ''
      return
    }
    const merged = mergeKlineRows(mergedRawRows, inc)
    const added = merged.length - beforeCount
    if (added <= 0) {
      // 接口返回了数据但与本地 key 完全重叠：多为 end 步长与 klt 不匹配；勿直接关闭，避免分钟线拖不动后永不再请求
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
    const raw = await GetStockEastMoneyKLine(
      codeSnap,
      props.stockName || '',
      meta.klt,
      meta.limit,
    )
    if (codeSnap !== props.code || activeKlt.value !== kltSnap) return
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
      return
    }
    if (param.time === undefined) {
      hoverRawRow.value = null
      return
    }
    const bar = param.seriesData.get(candleSeries)
    if (!bar) {
      hoverRawRow.value = null
      return
    }
    hoverRawRow.value = findRawRowByChartTime(param.time)
  }
  chart.subscribeCrosshairMove(crosshairMoveHandler)
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
    const raw = await GetStockEastMoneyKLine(
      props.code,
      props.stockName || '',
      meta.klt,
      meta.limit,
    )
    const list = Array.isArray(raw) ? raw : []
    ensureChart()
    mergedRawRows = mergeKlineRows([], list)
    syncDefaultLatestPanelRow()
    const { candles } = toSeriesData(mergedRawRows)
    if (!candles.length) {
      errorText.value =
        '暂无 K 线数据（需东方财富支持的代码，如 600519.SH、000001.SZ）'
      candleSeries?.setData([])
      volSeries?.setData([])
      syncIndicators()
      return
    }
    withProgrammaticTimeRange(() => {
      applySeriesFromRaw()
      applyDefaultVisibleRange()
    })
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

onMounted(() => {
  nextTick(() => {
    ensureChart()
    loadData()
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
  },
)

watch(
  () => props.chartHeight,
  (h) => {
    chart?.applyOptions({ height: h })
  },
)

watch(
  () => props.realtimeIntervalMs,
  () => setupPoll(),
)

</script>

<template>
  <div class="lw-kline-root" :class="{ 'lw-kline--dark': darkTheme }">
    <NFlex vertical :size="8" class="lw-kline-stack">
      <div class="lw-kline-toolbar">
        <div class="lw-kline-toolbar__main">
          <NFlex vertical :size="8">
            <NFlex :size="6" wrap style="row-gap: 6px">
              <NText depth="3" style="font-size: 12px; margin-right: 4px">周期</NText>
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
            </NFlex>
            <NFlex :size="6" wrap style="row-gap: 6px; align-items: center">
              <NText depth="3" style="font-size: 12px; margin-right: 4px">指标</NText>
              <NButton
                size="tiny"
                :type="showMA ? 'primary' : 'default'"
                :secondary="!showMA"
                @click="toggleMA"
              >
                均线 MA5/10/20/60
              </NButton>
              <NButton
                size="tiny"
                :type="showBOLL ? 'primary' : 'default'"
                :secondary="!showBOLL"
                @click="toggleBOLL"
              >
                BOLL(20,2)
              </NButton>
              <NButton
                size="tiny"
                :type="showOBV ? 'primary' : 'default'"
                :secondary="!showOBV"
                @click="toggleOBV"
              >
                OBV
              </NButton>
              <NButton
                size="tiny"
                :type="showMACD ? 'primary' : 'default'"
                :secondary="!showMACD"
                @click="toggleMACD"
              >
                MACD(12,26,9)
              </NButton>
              <NButton
                size="tiny"
                :type="showKDJ ? 'primary' : 'default'"
                :secondary="!showKDJ"
                @click="toggleKDJ"
              >
                KDJ(9)
              </NButton>
              <NButton
                size="tiny"
                :type="showRSI ? 'primary' : 'default'"
                :secondary="!showRSI"
                @click="toggleRSI"
              >
                RSI(14)
              </NButton>
            </NFlex>
            <NFlex align="center" :size="8" class="lw-kline-hint-row">
              <NText depth="3" class="lw-kline-hint-text">
                {{ stockName || code }} ·
                {{
                  realtimeIntervalMs > 0
                    ? `每 ${Math.round(realtimeIntervalMs / 1000)} 秒刷新`
                    : '切换周期后加载'
                }}
                · 按住拖动查看左侧历史时会自动加载更早 K 线
              </NText>
              <NSpin v-if="loading || loadingHistory" size="small" />
            </NFlex>
          </NFlex>
        </div>
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
      </div>
      <NText v-if="errorText" type="error" style="font-size: 12px">{{ errorText }}</NText>
      <div
        ref="chartContainerRef"
        class="lw-kline-chart"
        :style="{ height: chartHeight + 'px', minHeight: chartHeight + 'px' }"
      />
    </NFlex>
  </div>
</template>

<style scoped>
.lw-kline-root {
  width: 100%;
  max-width: 100%;
  min-width: 0;
  box-sizing: border-box;
  overflow-x: hidden;
  --wails-draggable: no-drag;
}
.lw-kline-stack {
  width: 100%;
  max-width: 100%;
  min-width: 0;
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
/* 上下布局：避免右侧信息栏把弹窗顶高；宽度跟随弹窗不外扩 */
.lw-kline-toolbar {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 8px;
  min-width: 0;
  max-width: 100%;
}
.lw-kline-toolbar__main {
  min-width: 0;
  max-width: 100%;
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
.lw-kline--dark .lw-kline-chart {
  border-radius: 4px;
  border: 1px solid #27272a;
}
.lw-kline-root:not(.lw-kline--dark) .lw-kline-chart {
  border-radius: 4px;
  border: 1px solid #e2e8f0;
}
</style>
