<script setup>
import {onBeforeUnmount, onMounted, ref, watch, computed} from "vue";
import {GetBKFundFlowListByDate, GetBKFundFlowTopListByDate} from "../../wailsjs/go/main/App";
import * as echarts from "echarts";

const props = defineProps({
  darkTheme: {
    type: Boolean,
    default: false
  },
  chartHeight: {
    type: Number,
    default: 600
  }
})

const chartRef = ref(null)
const topList = ref([])
const loading = ref(false)
const refreshInterval = ref(null)
let chart = null

// 默认当天日期，格式 YYYY-MM-DD
const today = new Date()
const todayStr = today.getFullYear() + '-' +
  String(today.getMonth() + 1).padStart(2, '0') + '-' +
  String(today.getDate()).padStart(2, '0')
const selectedDate = ref(todayStr)

// 是否查看的是今天
const isToday = computed(() => selectedDate.value === todayStr)

// 流入前20
const inflowList = computed(() => topList.value.filter((item) => item.netInflow > 0).slice(0, 20))
// 流出前20
const outflowList = computed(() => topList.value.filter((item) => item.netInflow < 0).slice(-20).reverse())

onMounted(async () => {
  try {
    await loadAllData()
    // 交易时间每分钟刷新（仅当天）
    refreshInterval.value = setInterval(async () => {
      if (isToday.value && isTradingTime()) {
        await loadAllData()
      }
    }, 60000)
  } catch (e) {
    console.error('onMounted error:', e)
  }
})

onBeforeUnmount(() => {
  if (refreshInterval.value) {
    clearInterval(refreshInterval.value)
  }
  if (chart) {
    chart.dispose()
  }
})

function isTradingTime() {
  const now = new Date()
  const h = now.getHours()
  const m = now.getMinutes()
  const t = h * 60 + m
  // 9:30 - 11:30 或 13:00 - 15:00
  return (t >= 570 && t <= 690) || (t >= 780 && t <= 900)
}

// 流入前20用红色系，流出前20用绿色系
function getSeriesColor(index, isInflow) {
  const redShades = [
    '#ee6666', '#d14a61', '#fc8452', '#e8534e', '#c23531',
    '#f4755e', '#d4816a', '#e76f51', '#ef6c4a', '#d9534f',
    '#e74c3c', '#ff6b6b', '#ee5a24', '#f19066', '#e55039',
    '#eb4d4b', '#fc5c65', '#ff6348', '#e71d36', '#c0392b'
  ]
  const greenShades = [
    '#00da3c', '#3ba272', '#91cc75', '#2ecc71', '#27ae60',
    '#52be80', '#58d68d', '#45b39d', '#1abc9c', '#16a085',
    '#239b56', '#28b463', '#73c0de', '#48b8d0', '#87cefa',
    '#1dd1a1', '#10ac84', '#0abde3', '#01a3a4', '#00cec9'
  ]
  return isInflow ? redShades[index % redShades.length] : greenShades[index % greenShades.length]
}

// 日期禁用：不能选未来日期
function isDateDisabled(ts) {
  return ts > Date.now()
}

async function loadAllData() {
  loading.value = true
  try {
    const date = selectedDate.value
    // 按日期获取板块排名
    const res = await GetBKFundFlowTopListByDate(date, 500)
    if (!res || !Array.isArray(res) || res.length === 0) {
      topList.value = []
      return
    }
    topList.value = res

    // 流入前20 和 流出前20
    const inflowTop = res.filter((item) => item.netInflow > 0).slice(0, 20)
    const outflowTop = res.filter((item) => item.netInflow < 0).slice(-20).reverse()
    const sectors = [...inflowTop, ...outflowTop]

    const allData = await Promise.all(
      sectors.map(async (item) => {
        const points = await GetBKFundFlowListByDate(item.code, date)
        return {
          code: item.code,
          name: item.name,
          isInflow: item.netInflow > 0,
          points: points || []
        }
      })
    )

    renderChart(allData)
  } catch (e) {
    console.error('loadAllData error:', e)
  } finally {
    loading.value = false
  }
}

function renderChart(allData) {
  if (!allData || allData.length === 0) return

  // 收集所有时间点，去重并排序
  const timeSet = new Set()
  for (const sector of allData) {
    for (const pt of sector.points) {
      const t = extractTime(pt.snapTime)
      if (t) timeSet.add(t)
    }
  }
  const times = Array.from(timeSet).sort()

  // 分别统计流入/流出板块的索引，用于配色
  let inflowIdx = 0
  let outflowIdx = 0

  // 构建每个板块的 series 数据，对齐到统一时间轴
  const seriesList = []
  for (let i = 0; i < allData.length; i++) {
    const sector = allData[i]
    const dataMap = new Map()
    for (const pt of sector.points) {
      const t = extractTime(pt.snapTime)
      if (t) dataMap.set(t, pt.netInflow || 0)
    }

    // 对齐到统一时间轴，缺失的时间点用 null
    const values = times.map(t => dataMap.has(t) ? dataMap.get(t) : null)

    const color = getSeriesColor(sector.isInflow ? inflowIdx++ : outflowIdx++, sector.isInflow)

    seriesList.push({
      name: sector.name,
      type: 'line',
      data: values,
      smooth: true,
      showSymbol: false,
      lineStyle: {width: 2, color},
      itemStyle: {color},
      emphasis: {
        lineStyle: {width: 3},
        focus: 'series'
      },
      // 折线末尾显示名称
      endLabel: {
        show: true,
        formatter: '{a}',
        color,
        fontSize: 12,
        fontWeight: 'bold',
        distance: 8
      }
    })
  }

  if (!chart && chartRef.value) {
    chart = echarts.init(chartRef.value)
  }
  if (!chart) return

  const textColor = props.darkTheme ? '#aaa' : '#666'
  const bgColor = props.darkTheme ? '#1a1a2e' : '#fff'
  const dateLabel = selectedDate.value

  const option = {
    backgroundColor: bgColor,
    title: {
      text: `${dateLabel} 板块资金流向 - 多板块对比`,
      left: '20px',
      textStyle: {color: props.darkTheme ? '#ccc' : '#456', fontSize: 16}
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: {type: 'cross'},
      borderWidth: 1,
      borderColor: props.darkTheme ? '#456' : '#ddd',
      backgroundColor: props.darkTheme ? 'rgba(30,30,60,0.9)' : 'rgba(255,255,255,0.95)',
      padding: 10,
      textStyle: {color: props.darkTheme ? '#ccc' : '#333', fontSize: 12},
      formatter: (params) => {
        if (!Array.isArray(params)) return ''
        const inflowItems = params.filter((p) => p.value != null && p.value > 0)
          .sort((a, b) => (b.value || 0) - (a.value || 0))
        const outflowItems = params.filter((p) => p.value != null && p.value < 0)
          .sort((a, b) => (a.value || 0) - (b.value || 0))

        const renderList = (items) => items.map((p) => {
          const val = (p.value / 100000000).toFixed(2)
          const sign = p.value > 0 ? '+' : ''
          return `${p.marker} ${p.seriesName} <b>${sign}${val}</b>`
        }).join('<br/>')

        let html = `<b>${params[0].axisValue}</b><br/>`
        html += '<div style="display:flex;gap:20px;">'
        html += `<div><div style="color:#ee6666;font-weight:bold;margin-bottom:4px;">流入</div>${renderList(inflowItems) || '-'}</div>`
        html += `<div><div style="color:#00da3c;font-weight:bold;margin-bottom:4px;">流出</div>${renderList(outflowItems) || '-'}</div>`
        html += '</div>'
        return html
      }
    },
    grid: {
      left: '8%',
      right: '12%',
      top: 80,
      height: '58%'
    },
    xAxis: {
      type: 'category',
      data: times,
      boundaryGap: false,
      axisLine: {onZero: false, lineStyle: {color: props.darkTheme ? '#444' : '#ccc'}},
      splitLine: {show: false},
      axisLabel: {
        color: textColor,
        rotate: 30,
        fontSize: 11,
        interval: times.length <= 30 ? 0 : Math.floor(times.length / 12)
      },
      axisTick: {lineStyle: {color: props.darkTheme ? '#444' : '#ccc'}}
    },
    yAxis: {
      name: '净流入/亿元',
      type: 'value',
      nameTextStyle: {color: textColor, fontSize: 11},
      axisLine: {show: true, lineStyle: {color: props.darkTheme ? '#444' : '#ccc'}},
      splitLine: {lineStyle: {color: props.darkTheme ? '#333' : '#eee', type: 'dashed'}},
      axisLabel: {
        color: textColor,
        fontSize: 11,
        formatter: (v) => (v / 100000000).toFixed(2)
      }
    },
    series: seriesList,
    dataZoom: [
      {
        type: 'inside',
        xAxisIndex: [0],
        start: Math.max(0, 100 - Math.min(100, 120 / times.length * 100)),
        end: 100
      },
      {
        show: true,
        xAxisIndex: [0],
        type: 'slider',
        top: '88%',
        start: Math.max(0, 100 - Math.min(100, 120 / times.length * 100)),
        end: 100,
        borderColor: props.darkTheme ? '#444' : '#ccc',
        fillerColor: props.darkTheme ? 'rgba(100,100,200,0.2)' : 'rgba(100,100,200,0.15)',
        handleStyle: {color: props.darkTheme ? '#666' : '#999'},
        textStyle: {color: textColor}
      }
    ]
  }

  chart.setOption(option, true)
  chart.resize()
}

function extractTime(snapTime) {
  if (!snapTime || typeof snapTime !== 'string') return ''
  if (snapTime.length >= 16) return snapTime.substring(11, 16)
  return String(snapTime)
}

function onDateChange() {
  loadAllData()
}

watch(() => props.darkTheme, () => {
  loadAllData()
})

watch(() => props.chartHeight, () => {
  if (chart) chart.resize()
})
</script>

<template>
  <div style="width: 100%">
    <!-- 控制栏 -->
    <div style="display: flex; align-items: center; gap: 12px; margin-bottom: 12px; flex-wrap: wrap;">
      <n-tag :bordered="false" type="error" size="small">红色系 = 流入前20</n-tag>
      <n-tag :bordered="false" type="success" size="small">绿色系 = 流出前20</n-tag>
      <n-date-picker
          v-model:formatted-value="selectedDate"
          type="date"
          value-format="yyyy-MM-dd"
          :is-date-disabled="isDateDisabled"
          style="width: 150px"
          @update:formatted-value="onDateChange"
      />
      <n-button size="small" :loading="loading" @click="loadAllData">
        刷新
      </n-button>
      <n-text v-if="isToday" :depth="3" style="font-size: 12px; margin-left: auto;">
        交易时间自动每分钟刷新 (9:30-15:00)
      </n-text>
      <n-text v-else depth="3" style="font-size: 12px; margin-left: auto;">
        历史数据
      </n-text>
    </div>

    <!-- 折线图 -->
    <div ref="chartRef" style="width: 100%;" :style="{height: chartHeight + 'px'}"></div>

    <!-- 板块资金排名表格 - 并排展示 -->
    <div style="margin-top: 20px; display: flex; gap: 20px; align-items: flex-start;">
      <!-- 流入排名 -->
      <div style="flex: 1; min-width: 0;">
        <n-h3 :style="{color: '#ee6666'}">流入 Top 20</n-h3>
        <n-table striped size="small">
          <n-thead>
            <n-tr>
              <n-th>排名</n-th>
              <n-th>板块名称</n-th>
              <n-th>净流入/亿元</n-th>
            </n-tr>
          </n-thead>
          <n-tbody>
            <n-tr v-for="(item, idx) in inflowList" :key="item.code">
              <n-td>{{ idx + 1 }}</n-td>
              <n-td>
                <n-tag :bordered="false" type="error" size="small">{{ item.name }}</n-tag>
              </n-td>
              <n-td>
                <n-text type="error">+{{ (item.netInflow / 100000000).toFixed(2) }}</n-text>
              </n-td>
            </n-tr>
            <n-tr v-if="inflowList.length === 0">
              <n-td colspan="3" style="text-align: center; color: #999;">暂无数据</n-td>
            </n-tr>
          </n-tbody>
        </n-table>
      </div>
      <!-- 流出排名 -->
      <div style="flex: 1; min-width: 0;">
        <n-h3 :style="{color: '#00da3c'}">流出 Top 20</n-h3>
        <n-table striped size="small">
          <n-thead>
            <n-tr>
              <n-th>排名</n-th>
              <n-th>板块名称</n-th>
              <n-th>净流入/亿元</n-th>
            </n-tr>
          </n-thead>
          <n-tbody>
            <n-tr v-for="(item, idx) in outflowList" :key="item.code">
              <n-td>{{ idx + 1 }}</n-td>
              <n-td>
                <n-tag :bordered="false" type="success" size="small">{{ item.name }}</n-tag>
              </n-td>
              <n-td>
                <n-text type="success">{{ (item.netInflow / 100000000).toFixed(2) }}</n-text>
              </n-td>
            </n-tr>
            <n-tr v-if="outflowList.length === 0">
              <n-td colspan="3" style="text-align: center; color: #999;">暂无数据</n-td>
            </n-tr>
          </n-tbody>
        </n-table>
      </div>
    </div>
  </div>
</template>

<style scoped>
</style>
