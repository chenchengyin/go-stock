<script setup>

import {AnalyzeSentimentWithFreqWeight,GlobalStockIndexes,GetTodayMarketStatistic,GetRecentDaysMarketStatistic,GetDailyChangeStats,GetChangeTypeDailyStats,GetChangeRank,GetDailyDimensionStats,GetTypeStatsByDate,IsTradingTime} from "../../wailsjs/go/main/App";
import * as echarts from "echarts";
import {onMounted,onUnmounted, ref, watch, nextTick} from "vue";
import _ from "lodash";
const { name,darkTheme,kDays ,chartHeight} = defineProps({
  name: {
    type: String,
    default: ''
  },
  kDays: {
    type: Number,
    default: 14
  },
  chartHeight: {
    type: Number,
    default: 500
  },
  darkTheme: {
    type: Boolean,
    default: false
  }
})
const common = ref([])
const america = ref([])
const europe = ref([])
const asia = ref([])
const mainIndex = ref([])
const chinaIndex = ref([])
const other = ref([])
const globalStockIndexes = ref(null)
const chartRef = ref(null);
const limitChartRef = ref(null);
const treemapRef = ref(null);
const dailyUpDownChartRef = ref(null);
const dailyLimitChartRef = ref(null);
const changeStatsChartRef = ref(null);
const changeTypeChartRef = ref(null);
const changeRankStockRef = ref(null);
const changeRankIndustryRef = ref(null);
const changeRankConceptRef = ref(null);
const showTreemap = ref(false);
const showDailyChart = ref(false);
const showChangeStats = ref(false);
const showChangeRank = ref(false);
const changeRankDays = ref(1);
const showBullBearRank = ref(false);
const bullBearDays = ref(1);
const bullBearStockUpRef = ref(null);
const bullBearStockDownRef = ref(null);
const bullBearIndustryUpRef = ref(null);
const bullBearIndustryDownRef = ref(null);
const bullBearConceptUpRef = ref(null);
const bullBearConceptDownRef = ref(null);
const showDimensionModal = ref(false);
const dimensionModalTitle = ref('');
const dimensionDetailChartRef = ref(null);
const triggerAreas=ref(["main","extra","arrow"])
let handleChartInterval=null
let handleIndexInterval=null
let treemapchart =null;

onMounted(() => {
  handleChart()
  handleTreemap()
  handleDailyChart()
  handleChangeRank()
  getIndex()
  handleChartInterval=setInterval(function () {
    handleChart()
  }, 1000 * 60)

  handleIndexInterval=setInterval(function () {
    getIndex()
    handleTreemap()
  }, 1000 * 10)
})

onUnmounted(()=>{
  clearInterval(handleChartInterval)
  clearInterval(handleIndexInterval)
})

watch(showTreemap, (newVal) => {
  if (newVal) {
    nextTick(() => {
      handleTreemap()
    })
  }
})

watch(showDailyChart, (newVal) => {
  if (newVal) {
    nextTick(() => {
      handleDailyChart()
    })
  }
})

watch(showChangeStats, (newVal) => {
  if (newVal) {
    nextTick(() => {
      handleChangeStats()
    })
  }
})

watch(showChangeRank, (newVal) => {
  if (newVal) {
    nextTick(() => {
      handleChangeRank()
    })
  }
})

watch(changeRankDays, () => {
  handleChangeRank()
})

watch(showBullBearRank, (newVal) => {
  if (newVal) {
    nextTick(() => {
      handleBullBearRank()
    })
  }
})

watch(bullBearDays, () => {
  handleBullBearRank()
})

watch(showDimensionModal, (newVal) => {
  if (newVal) {
    nextTick(() => {
      handleDimensionDetail()
    })
  }
})

function getIndex() {
  GlobalStockIndexes().then((res) => {
    globalStockIndexes.value = res
    common.value = res["common"]
    america.value = res["america"]
    europe.value = res["europe"]
    asia.value = res["asia"]
    other.value = res["other"]
    mainIndex.value=asia.value.filter(function (item) {
      return ['上海',"深圳","香港","台湾","北京","东京","首尔","纽约","纳斯达克"].includes(item.location)
    }).concat(america.value.filter(function (item) {
      return ['上海',"深圳","香港","台湾","北京","东京","首尔","纽约","纳斯达克"].includes(item.location)
    }))

    chinaIndex.value=asia.value.filter(function (item) {
      return ['上海',"深圳","香港","台湾","北京"].includes(item.location)
    })

  })
}

async function handleChart(){
  try {
    const data = await GetTodayMarketStatistic()
    if (data && data.length > 0) {
      renderUpDownChart(data)
      renderLimitChart(data)
    }
  } catch (error) {
    console.error('获取市场统计数据失败:', error)
  }
}

function renderUpDownChart(data) {
  if (!chartRef.value || !data || data.length === 0) return
  
  const chart = echarts.init(chartRef.value)
  
  const times = data.map(d => d.dataTime)
  const upCounts = data.map(d => d.upCount)
  const downCounts = data.map(d => d.downCount)
  const ratios = data.map(d => d.upRatio.toFixed(2))
  const upDownRatios = data.map(d => d.upDownRatio.toFixed(2))
  
  const option = {
    darkMode: darkTheme,
    title: {
      text: '涨跌家数比',
      left: 'center',
      textStyle: {
        color: darkTheme ? '#ccc' : '#333',
        fontSize: 14
      }
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: {
        type: 'cross'
      },
      formatter: function(params) {
        let result = params[0].axisValue + '<br/>'
        params.forEach(param => {
          result += param.marker + ' ' + param.seriesName + ': ' + param.value + '<br/>'
        })
        const idx = params[0].dataIndex
        if (idx < data.length) {
          const d = data[idx]
          result += `<span style="color:#666">红盘率: ${d.upRatio.toFixed(1)}%</span><br/>`
          result += `<span style="color:#666">情绪指标: ${d.upDownRatio.toFixed(2)} (${d.sentimentDesc || ''})</span>`
        }
        return result
      }
    },
    legend: {
      data: ['上涨家数', '下跌家数', '红盘率(%)', '情绪指标'],
      top: 25,
      textStyle: {
        color: darkTheme ? '#ccc' : '#333'
      }
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '3%',
      top: 60,
      containLabel: true
    },
    xAxis: {
      type: 'category',
      data: times,
      axisLabel: {
        color: darkTheme ? '#999' : '#666',
        rotate: 45
      },
      axisLine: {
        lineStyle: {
          color: darkTheme ? '#444' : '#ccc'
        }
      }
    },
    yAxis: [
      {
        type: 'value',
        name: '家数',
        position: 'left',
        axisLabel: {
          color: darkTheme ? '#999' : '#666'
        },
        axisLine: {
          lineStyle: {
            color: darkTheme ? '#444' : '#ccc'
          }
        },
        splitLine: {
          lineStyle: {
            color: darkTheme ? '#333' : '#eee'
          }
        }
      },
      {
        type: 'value',
        name: '红盘率(%)',
        position: 'right',
        min: 0,
        max: 100,
        axisLabel: {
          color: darkTheme ? '#999' : '#666',
          formatter: '{value}%'
        },
        axisLine: {
          lineStyle: {
            color: darkTheme ? '#444' : '#ccc'
          }
        },
        splitLine: {
          show: false
        }
      },
      {
        type: 'value',
        name: '情绪指标',
        position: 'right',
        offset: 60,
        axisLabel: {
          color: darkTheme ? '#999' : '#666'
        },
        axisLine: {
          lineStyle: {
            color: darkTheme ? '#444' : '#ccc'
          }
        },
        splitLine: {
          show: false
        }
      }
    ],
    series: [
      {
        name: '上涨家数',
        type: 'bar',
        stack: 'total',
        data: upCounts,
        itemStyle: {
          color: '#ef4444'
        }
      },
      {
        name: '下跌家数',
        type: 'bar',
        stack: 'total',
        data: downCounts,
        itemStyle: {
          color: '#22c55e'
        }
      },
      {
        name: '红盘率(%)',
        type: 'line',
        yAxisIndex: 1,
        data: ratios,
        smooth: true,
        lineStyle: {
          color: '#f59e0b',
          width: 2
        },
        itemStyle: {
          color: '#f59e0b'
        },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(245, 158, 11, 0.3)' },
            { offset: 1, color: 'rgba(245, 158, 11, 0.05)' }
          ])
        },
        markLine: {
          silent: true,
          data: [
            { yAxis: 50, name: '平衡线', lineStyle: { color: '#888', type: 'dashed' } }
          ]
        }
      },
      {
        name: '情绪指标',
        type: 'line',
        yAxisIndex: 2,
        data: upDownRatios,
        smooth: true,
        lineStyle: {
          color: '#8b5cf6',
          width: 2
        },
        itemStyle: {
          color: '#8b5cf6'
        },
        markLine: {
          silent: true,
          data: [
            { yAxis: 1, name: '平衡线', lineStyle: { color: '#8b5cf6', type: 'dashed' } },
            { yAxis: 2, name: '极强线', lineStyle: { color: '#ef4444', type: 'dotted' } },
            { yAxis: 0.5, name: '冰点线', lineStyle: { color: '#22c55e', type: 'dotted' } }
          ]
        }
      }
    ]
  }
  
  chart.setOption(option)
}

function renderLimitChart(data) {
  if (!limitChartRef.value || !data || data.length === 0) return
  
  const chart = echarts.init(limitChartRef.value)
  
  const times = data.map(d => d.dataTime)
  const limitUps = data.map(d => d.limitUp)
  const limitDowns = data.map(d => d.limitDown)
  const ratios = data.map(d => d.limitRatio.toFixed(2))
  
  const option = {
    darkMode: darkTheme,
    title: {
      text: '涨跌停家数比',
      left: 'center',
      textStyle: {
        color: darkTheme ? '#ccc' : '#333',
        fontSize: 14
      }
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: {
        type: 'cross'
      },
      formatter: function(params) {
        let result = params[0].axisValue + '<br/>'
        params.forEach(param => {
          result += param.marker + ' ' + param.seriesName + ': ' + param.value + '<br/>'
        })
        const idx = params[0].dataIndex
        if (idx < data.length) {
          const d = data[idx]
          result += `<span style="color:#666">涨跌停比: ${d.limitRatio.toFixed(2)}</span><br/>`
        }
        return result
      }
    },
    legend: {
      data: ['涨停家数', '跌停家数', '涨跌停比'],
      top: 25,
      textStyle: {
        color: darkTheme ? '#ccc' : '#333'
      }
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '3%',
      top: 60,
      containLabel: true
    },
    xAxis: {
      type: 'category',
      data: times,
      axisLabel: {
        color: darkTheme ? '#999' : '#666',
        rotate: 45
      },
      axisLine: {
        lineStyle: {
          color: darkTheme ? '#444' : '#ccc'
        }
      }
    },
    yAxis: [
      {
        type: 'value',
        name: '家数',
        position: 'left',
        axisLabel: {
          color: darkTheme ? '#999' : '#666'
        },
        axisLine: {
          lineStyle: {
            color: darkTheme ? '#444' : '#ccc'
          }
        },
        splitLine: {
          lineStyle: {
            color: darkTheme ? '#333' : '#eee'
          }
        }
      },
      {
        type: 'value',
        name: '涨跌停比',
        position: 'right',
        axisLabel: {
          color: darkTheme ? '#999' : '#666'
        },
        axisLine: {
          lineStyle: {
            color: darkTheme ? '#444' : '#ccc'
          }
        },
        splitLine: {
          show: false
        }
      }
    ],
    series: [
      {
        name: '涨停家数',
        type: 'bar',
        stack: 'total',
        data: limitUps,
        itemStyle: {
          color: '#ef4444'
        }
      },
      {
        name: '跌停家数',
        type: 'bar',
        stack: 'total',
        data: limitDowns,
        itemStyle: {
          color: '#22c55e'
        }
      },
      {
        name: '涨跌停比',
        type: 'line',
        yAxisIndex: 1,
        data: ratios,
        smooth: true,
        lineStyle: {
          color: '#f59e0b',
          width: 2
        },
        itemStyle: {
          color: '#f59e0b'
        },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(245, 158, 11, 0.3)' },
            { offset: 1, color: 'rgba(245, 158, 11, 0.05)' }
          ])
        },
        markLine: {
          silent: true,
          data: [
            { yAxis: 1, name: '平衡线', lineStyle: { color: '#888', type: 'dashed' } }
          ]
        }
      }
    ]
  }
  
  chart.setOption(option)
}

function aggregateByDate(data) {
  if (!data || data.length === 0) return []
  const grouped = {}
  data.forEach(d => {
    const date = d.dataDate
    if (!grouped[date] || d.dataTime >= grouped[date].dataTime) {
      grouped[date] = d
    }
  })
  return Object.keys(grouped).sort().map(date => grouped[date])
}

async function handleDailyChart() {
  try {
    const data = await GetRecentDaysMarketStatistic(30)
    if (data && data.length > 0) {
      const dailyData = aggregateByDate(data)
      renderDailyUpDownChart(dailyData)
      renderDailyLimitChart(dailyData)
    }
  } catch (error) {
    console.error('获取历史市场统计数据失败:', error)
  }
}

function renderDailyUpDownChart(data) {
  if (!dailyUpDownChartRef.value || !data || data.length === 0) return

  const chart = echarts.init(dailyUpDownChartRef.value)

  const dates = data.map(d => d.dataDate)
  const upCounts = data.map(d => d.upCount)
  const downCounts = data.map(d => d.downCount)
  const ratios = data.map(d => d.upRatio.toFixed(2))
  const upDownRatios = data.map(d => d.upDownRatio.toFixed(2))

  const option = {
    darkMode: darkTheme,
    title: {
      text: '近30日涨跌家数趋势',
      left: 'center',
      textStyle: {
        color: darkTheme ? '#ccc' : '#333',
        fontSize: 14
      }
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: {
        type: 'cross'
      },
      formatter: function(params) {
        let result = params[0].axisValue + '<br/>'
        params.forEach(param => {
          result += param.marker + ' ' + param.seriesName + ': ' + param.value + '<br/>'
        })
        const idx = params[0].dataIndex
        if (idx < data.length) {
          const d = data[idx]
          result += `<span style="color:#666">红盘率: ${d.upRatio.toFixed(1)}%</span><br/>`
          result += `<span style="color:#666">情绪指标: ${d.upDownRatio.toFixed(2)} (${d.sentimentDesc || ''})</span>`
        }
        return result
      }
    },
    legend: {
      data: ['上涨家数', '下跌家数', '红盘率(%)', '情绪指标'],
      top: 25,
      textStyle: {
        color: darkTheme ? '#ccc' : '#333'
      }
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '3%',
      top: 60,
      containLabel: true
    },
    xAxis: {
      type: 'category',
      data: dates,
      axisLabel: {
        color: darkTheme ? '#999' : '#666',
        rotate: 45
      },
      axisLine: {
        lineStyle: {
          color: darkTheme ? '#444' : '#ccc'
        }
      }
    },
    yAxis: [
      {
        type: 'value',
        name: '家数',
        position: 'left',
        axisLabel: {
          color: darkTheme ? '#999' : '#666'
        },
        axisLine: {
          lineStyle: {
            color: darkTheme ? '#444' : '#ccc'
          }
        },
        splitLine: {
          lineStyle: {
            color: darkTheme ? '#333' : '#eee'
          }
        }
      },
      {
        type: 'value',
        name: '红盘率(%)',
        position: 'right',
        min: 0,
        max: 100,
        axisLabel: {
          color: darkTheme ? '#999' : '#666',
          formatter: '{value}%'
        },
        axisLine: {
          lineStyle: {
            color: darkTheme ? '#444' : '#ccc'
          }
        },
        splitLine: {
          show: false
        }
      },
      {
        type: 'value',
        name: '情绪指标',
        position: 'right',
        offset: 60,
        axisLabel: {
          color: darkTheme ? '#999' : '#666'
        },
        axisLine: {
          lineStyle: {
            color: darkTheme ? '#444' : '#ccc'
          }
        },
        splitLine: {
          show: false
        }
      }
    ],
    series: [
      {
        name: '上涨家数',
        type: 'bar',
        data: upCounts,
        itemStyle: {
          color: '#ef4444'
        }
      },
      {
        name: '下跌家数',
        type: 'bar',
        data: downCounts,
        itemStyle: {
          color: '#22c55e'
        }
      },
      {
        name: '红盘率(%)',
        type: 'line',
        yAxisIndex: 1,
        data: ratios,
        smooth: true,
        lineStyle: {
          color: '#f59e0b',
          width: 2
        },
        itemStyle: {
          color: '#f59e0b'
        },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(245, 158, 11, 0.3)' },
            { offset: 1, color: 'rgba(245, 158, 11, 0.05)' }
          ])
        },
        markLine: {
          silent: true,
          data: [
            { yAxis: 50, name: '平衡线', lineStyle: { color: '#888', type: 'dashed' } }
          ]
        }
      },
      {
        name: '情绪指标',
        type: 'line',
        yAxisIndex: 2,
        data: upDownRatios,
        smooth: true,
        lineStyle: {
          color: '#8b5cf6',
          width: 2
        },
        itemStyle: {
          color: '#8b5cf6'
        },
        markLine: {
          silent: true,
          data: [
            { yAxis: 1, name: '平衡线', lineStyle: { color: '#8b5cf6', type: 'dashed' } },
            { yAxis: 2, name: '极强线', lineStyle: { color: '#ef4444', type: 'dotted' } },
            { yAxis: 0.5, name: '冰点线', lineStyle: { color: '#22c55e', type: 'dotted' } }
          ]
        }
      }
    ]
  }

  chart.setOption(option)
}

function renderDailyLimitChart(data) {
  if (!dailyLimitChartRef.value || !data || data.length === 0) return

  const chart = echarts.init(dailyLimitChartRef.value)

  const dates = data.map(d => d.dataDate)
  const limitUps = data.map(d => d.limitUp)
  const limitDowns = data.map(d => d.limitDown)
  const ratios = data.map(d => d.limitRatio.toFixed(2))

  const option = {
    darkMode: darkTheme,
    title: {
      text: '近30日涨跌停趋势',
      left: 'center',
      textStyle: {
        color: darkTheme ? '#ccc' : '#333',
        fontSize: 14
      }
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: {
        type: 'cross'
      },
      formatter: function(params) {
        let result = params[0].axisValue + '<br/>'
        params.forEach(param => {
          result += param.marker + ' ' + param.seriesName + ': ' + param.value + '<br/>'
        })
        const idx = params[0].dataIndex
        if (idx < data.length) {
          const d = data[idx]
          result += `<span style="color:#666">涨跌停比: ${d.limitRatio.toFixed(2)}</span><br/>`
          result += `<span style="color:#666">涨停: ${d.limitUp} 跌停: ${d.limitDown}</span>`
        }
        return result
      }
    },
    legend: {
      data: ['涨停家数', '跌停家数', '涨跌停比'],
      top: 25,
      textStyle: {
        color: darkTheme ? '#ccc' : '#333'
      }
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '3%',
      top: 60,
      containLabel: true
    },
    xAxis: {
      type: 'category',
      data: dates,
      axisLabel: {
        color: darkTheme ? '#999' : '#666',
        rotate: 45
      },
      axisLine: {
        lineStyle: {
          color: darkTheme ? '#444' : '#ccc'
        }
      }
    },
    yAxis: [
      {
        type: 'value',
        name: '家数',
        position: 'left',
        axisLabel: {
          color: darkTheme ? '#999' : '#666'
        },
        axisLine: {
          lineStyle: {
            color: darkTheme ? '#444' : '#ccc'
          }
        },
        splitLine: {
          lineStyle: {
            color: darkTheme ? '#333' : '#eee'
          }
        }
      },
      {
        type: 'value',
        name: '涨跌停比',
        position: 'right',
        axisLabel: {
          color: darkTheme ? '#999' : '#666'
        },
        axisLine: {
          lineStyle: {
            color: darkTheme ? '#444' : '#ccc'
          }
        },
        splitLine: {
          show: false
        }
      }
    ],
    series: [
      {
        name: '涨停家数',
        type: 'bar',
        data: limitUps,
        itemStyle: {
          color: '#ef4444'
        }
      },
      {
        name: '跌停家数',
        type: 'bar',
        data: limitDowns,
        itemStyle: {
          color: '#22c55e'
        }
      },
      {
        name: '涨跌停比',
        type: 'line',
        yAxisIndex: 1,
        data: ratios,
        smooth: true,
        lineStyle: {
          color: '#f59e0b',
          width: 2
        },
        itemStyle: {
          color: '#f59e0b'
        },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(245, 158, 11, 0.3)' },
            { offset: 1, color: 'rgba(245, 158, 11, 0.05)' }
          ])
        },
        markLine: {
          silent: true,
          data: [
            { yAxis: 1, name: '平衡线', lineStyle: { color: '#888', type: 'dashed' } }
          ]
        }
      }
    ]
  }

  chart.setOption(option)
}

async function handleChangeStats() {
  try {
    const [dailyStats, typeStats] = await Promise.all([
      GetDailyChangeStats(30),
      GetChangeTypeDailyStats(30)
    ])
    if (dailyStats && dailyStats.length > 0) {
      renderChangeStatsChart(dailyStats)
    }
    if (typeStats && typeStats.length > 0) {
      renderChangeTypeChart(typeStats)
    }
  } catch (error) {
    console.error('获取异动统计数据失败:', error)
  }
}

function renderChangeStatsChart(data) {
  if (!changeStatsChartRef.value || !data || data.length === 0) return

  const chart = echarts.init(changeStatsChartRef.value)

  const dates = data.map(d => d.changeDate)
  const totalCounts = data.map(d => d.totalCount)
  const upCounts = data.map(d => d.upCount)
  const downCounts = data.map(d => d.downCount)
  const limitUps = data.map(d => d.limitUp)
  const limitDowns = data.map(d => d.limitDown)

  const option = {
    darkMode: darkTheme,
    title: {
      text: '近30日异动统计趋势',
      left: 'center',
      textStyle: {
        color: darkTheme ? '#ccc' : '#333',
        fontSize: 14
      }
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: {
        type: 'cross'
      },
      formatter: function(params) {
        let result = params[0].axisValue + '<br/>'
        params.forEach(param => {
          result += param.marker + ' ' + param.seriesName + ': ' + param.value + '<br/>'
        })
        const idx = params[0].dataIndex
        if (idx < data.length) {
          const d = data[idx]
          result += `<span style="color:#666">封涨停: ${d.limitUp} 封跌停: ${d.limitDown}</span>`
        }
        return result
      }
    },
    legend: {
      data: ['上涨异动', '下跌异动', '封涨停', '封跌停', '总异动数'],
      top: 25,
      textStyle: {
        color: darkTheme ? '#ccc' : '#333'
      }
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '3%',
      top: 60,
      containLabel: true
    },
    xAxis: {
      type: 'category',
      data: dates,
      axisLabel: {
        color: darkTheme ? '#999' : '#666',
        rotate: 45
      },
      axisLine: {
        lineStyle: {
          color: darkTheme ? '#444' : '#ccc'
        }
      }
    },
    yAxis: [
      {
        type: 'value',
        name: '家数',
        position: 'left',
        axisLabel: {
          color: darkTheme ? '#999' : '#666'
        },
        axisLine: {
          lineStyle: {
            color: darkTheme ? '#444' : '#ccc'
          }
        },
        splitLine: {
          lineStyle: {
            color: darkTheme ? '#333' : '#eee'
          }
        }
      },
      {
        type: 'value',
        name: '总异动数',
        position: 'right',
        axisLabel: {
          color: darkTheme ? '#999' : '#666'
        },
        axisLine: {
          lineStyle: {
            color: darkTheme ? '#444' : '#ccc'
          }
        },
        splitLine: {
          show: false
        }
      }
    ],
    series: [
      {
        name: '上涨异动',
        type: 'bar',
        stack: 'direction',
        data: upCounts,
        itemStyle: {
          color: '#ef4444'
        }
      },
      {
        name: '下跌异动',
        type: 'bar',
        stack: 'direction',
        data: downCounts,
        itemStyle: {
          color: '#22c55e'
        }
      },
      {
        name: '封涨停',
        type: 'bar',
        data: limitUps,
        itemStyle: {
          color: '#f97316'
        }
      },
      {
        name: '封跌停',
        type: 'bar',
        data: limitDowns,
        itemStyle: {
          color: '#06b6d4'
        }
      },
      {
        name: '总异动数',
        type: 'line',
        yAxisIndex: 1,
        data: totalCounts,
        smooth: true,
        lineStyle: {
          color: '#8b5cf6',
          width: 2
        },
        itemStyle: {
          color: '#8b5cf6'
        },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(139, 92, 246, 0.3)' },
            { offset: 1, color: 'rgba(139, 92, 246, 0.05)' }
          ])
        }
      }
    ]
  }

  chart.setOption(option)
  chart.off('click')
  chart.on('click', function(params) {
    if (params.componentType === 'series' && params.name) {
      openDimensionDetail('date', params.name)
    }
  })
}

function renderChangeTypeChart(data) {
  if (!changeTypeChartRef.value || !data || data.length === 0) return

  const chart = echarts.init(changeTypeChartRef.value)

  const dateSet = [...new Set(data.map(d => d.changeDate))].sort()

  const upTypes = ['封涨停板', '打开涨停板', '火箭发射', '快速反弹', '大笔买入', '有大买盘', '竞价上涨', '高开5日线', '向上缺口', '60日新高', '60日大幅上涨']
  const downTypes = ['封跌停板', '打开跌停板', '高台跳水', '加速下跌', '大笔卖出', '有大卖盘', '竞价下跌', '低开5日线', '向下缺口', '60日新低', '60日大幅下跌']

  const typeColorMap = {
    '封涨停板': '#ef4444',
    '封跌停板': '#22c55e',
    '打开涨停板': '#f97316',
    '打开跌停板': '#06b6d4',
    '火箭发射': '#dc2626',
    '快速反弹': '#f59e0b',
    '高台跳水': '#10b981',
    '加速下跌': '#14b8a6',
    '大笔买入': '#e11d48',
    '大笔卖出': '#059669',
    '有大买盘': '#db2777',
    '有大卖盘': '#0d9488',
    '竞价上涨': '#f43f5e',
    '竞价下跌': '#0891b2',
    '高开5日线': '#fb923c',
    '低开5日线': '#2dd4bf',
    '向上缺口': '#f87171',
    '向下缺口': '#34d399',
    '60日新高': '#c026d3',
    '60日新低': '#0ea5e9',
    '60日大幅上涨': '#a855f7',
    '60日大幅下跌': '#38bdf8',
  }

  const upSeries = upTypes.filter(typeName => data.some(d => d.typeName === typeName)).map(typeName => {
    const typeData = dateSet.map(date => {
      const found = data.find(d => d.changeDate === date && d.typeName === typeName)
      return found ? found.count : 0
    })
    return {
      name: typeName,
      type: 'bar',
      stack: 'up',
      emphasis: { focus: 'series' },
      data: typeData,
      itemStyle: { color: typeColorMap[typeName] || '#ef4444' }
    }
  })

  const downSeries = downTypes.filter(typeName => data.some(d => d.typeName === typeName)).map(typeName => {
    const typeData = dateSet.map(date => {
      const found = data.find(d => d.changeDate === date && d.typeName === typeName)
      return found ? found.count : 0
    })
    return {
      name: typeName,
      type: 'bar',
      stack: 'down',
      emphasis: { focus: 'series' },
      data: typeData,
      itemStyle: { color: typeColorMap[typeName] || '#22c55e' }
    }
  })

  const series = [...upSeries, ...downSeries]

  const option = {
    darkMode: darkTheme,
    title: {
      text: '近30日异动类型分布(利好↑/利空↓)',
      left: 'center',
      textStyle: {
        color: darkTheme ? '#ccc' : '#333',
        fontSize: 14
      }
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: {
        type: 'shadow'
      }
    },
    legend: {
      type: 'scroll',
      top: 25,
      textStyle: {
        color: darkTheme ? '#ccc' : '#333',
        fontSize: 11
      },
      pageIconColor: darkTheme ? '#aaa' : '#333',
      pageTextStyle: {
        color: darkTheme ? '#aaa' : '#333'
      }
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '3%',
      top: 60,
      containLabel: true
    },
    xAxis: {
      type: 'category',
      data: dateSet,
      axisLabel: {
        color: darkTheme ? '#999' : '#666',
        rotate: 45
      },
      axisLine: {
        lineStyle: {
          color: darkTheme ? '#444' : '#ccc'
        }
      }
    },
    yAxis: {
      type: 'value',
      name: '次数',
      axisLabel: {
        color: darkTheme ? '#999' : '#666'
      },
      axisLine: {
        lineStyle: {
          color: darkTheme ? '#444' : '#ccc'
        }
      },
      splitLine: {
        lineStyle: {
          color: darkTheme ? '#333' : '#eee'
        }
      }
    },
    series: series
  }

  chart.setOption(option)
  chart.off('click')
  chart.on('click', function(params) {
    if (params.componentType === 'series' && params.seriesName) {
      openDimensionDetail('type', params.seriesName)
    }
  })
}

let currentDimension = ''
let currentDimensionName = ''

function openDimensionDetail(dimension, name) {
  currentDimension = dimension
  currentDimensionName = name
  const labels = { stock: '股票', industry: '行业', concept: '概念', type: '异动类型' }
  dimensionModalTitle.value = `${name} - 近30日${labels[dimension] || ''}异动趋势`
  showDimensionModal.value = true
}

async function handleDimensionDetail() {
  if (!currentDimension || !currentDimensionName) return
  try {
    if (currentDimension === 'date') {
      const data = await GetTypeStatsByDate(currentDimensionName)
      if (data && data.length > 0) {
        renderDateTypeChart(data)
      }
    } else {
      const data = await GetDailyDimensionStats(currentDimension, currentDimensionName, 30)
      if (data && data.length > 0) {
        renderDimensionDetailChart(data)
      }
    }
  } catch (error) {
    console.error('获取维度详情数据失败:', error)
  }
}

function renderDimensionDetailChart(data) {
  if (!dimensionDetailChartRef.value) return

  const chart = echarts.init(dimensionDetailChartRef.value)

  const dates = data.map(d => d.changeDate)
  const upCounts = data.map(d => d.upCount)
  const downCounts = data.map(d => d.downCount)
  const totalCounts = data.map(d => d.totalCount)

  const option = {
    darkMode: darkTheme,
    title: {
      text: dimensionModalTitle.value,
      left: 'center',
      textStyle: {
        color: darkTheme ? '#ccc' : '#333',
        fontSize: 14
      }
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'cross' },
      formatter: function(params) {
        let result = params[0].axisValue + '<br/>'
        params.forEach(param => {
          result += param.marker + ' ' + param.seriesName + ': ' + param.value + '<br/>'
        })
        return result
      }
    },
    legend: {
      data: ['利好异动', '利空异动', '总异动数'],
      top: 25,
      textStyle: { color: darkTheme ? '#ccc' : '#333' }
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '3%',
      top: 60,
      containLabel: true
    },
    xAxis: {
      type: 'category',
      data: dates,
      axisLabel: {
        color: darkTheme ? '#999' : '#666',
        rotate: 45
      },
      axisLine: { lineStyle: { color: darkTheme ? '#444' : '#ccc' } }
    },
    yAxis: [
      {
        type: 'value',
        name: '次数',
        position: 'left',
        axisLabel: { color: darkTheme ? '#999' : '#666' },
        axisLine: { lineStyle: { color: darkTheme ? '#444' : '#ccc' } },
        splitLine: { lineStyle: { color: darkTheme ? '#333' : '#eee' } }
      },
      {
        type: 'value',
        name: '总异动数',
        position: 'right',
        axisLabel: { color: darkTheme ? '#999' : '#666' },
        axisLine: { lineStyle: { color: darkTheme ? '#444' : '#ccc' } },
        splitLine: { show: false }
      }
    ],
    series: [
      {
        name: '利好异动',
        type: 'bar',
        stack: 'direction',
        data: upCounts,
        itemStyle: { color: '#ef4444' }
      },
      {
        name: '利空异动',
        type: 'bar',
        stack: 'direction',
        data: downCounts,
        itemStyle: { color: '#22c55e' }
      },
      {
        name: '总异动数',
        type: 'line',
        yAxisIndex: 1,
        data: totalCounts,
        smooth: true,
        lineStyle: { color: '#8b5cf6', width: 2 },
        itemStyle: { color: '#8b5cf6' },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(139, 92, 246, 0.3)' },
            { offset: 1, color: 'rgba(139, 92, 246, 0.05)' }
          ])
        }
      }
    ]
  }

  chart.setOption(option)
}

function renderDateTypeChart(data) {
  if (!dimensionDetailChartRef.value) return

  const chart = echarts.init(dimensionDetailChartRef.value)

  const typeNames = data.map(d => d.typeName).reverse()
  const upValues = data.map(d => d.upCount).reverse()
  const downValues = data.map(d => d.downCount).reverse()

  const option = {
    darkMode: darkTheme,
    title: {
      text: dimensionModalTitle.value,
      left: 'center',
      textStyle: {
        color: darkTheme ? '#ccc' : '#333',
        fontSize: 14
      }
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      formatter: function(params) {
        let result = params[0].axisValue + '<br/>'
        let total = 0
        params.forEach(param => {
          result += param.marker + ' ' + param.seriesName + ': ' + param.value + '<br/>'
          total += param.value
        })
        result += '<b>合计: ' + total + '</b>'
        return result
      }
    },
    legend: {
      data: ['利好异动', '利空异动'],
      top: 25,
      textStyle: { color: darkTheme ? '#ccc' : '#333' }
    },
    grid: {
      left: '3%',
      right: '8%',
      bottom: '3%',
      top: 55,
      containLabel: true
    },
    xAxis: {
      type: 'value',
      name: '次数',
      axisLabel: { color: darkTheme ? '#999' : '#666' },
      axisLine: { lineStyle: { color: darkTheme ? '#444' : '#ccc' } },
      splitLine: { lineStyle: { color: darkTheme ? '#333' : '#eee' } }
    },
    yAxis: {
      type: 'category',
      data: typeNames,
      axisLabel: {
        color: darkTheme ? '#999' : '#666',
        fontSize: 11,
        width: 100,
        overflow: 'truncate'
      },
      axisLine: { lineStyle: { color: darkTheme ? '#444' : '#ccc' } }
    },
    series: [
      {
        name: '利好异动',
        type: 'bar',
        stack: 'total',
        data: upValues,
        itemStyle: { color: '#ef4444' }
      },
      {
        name: '利空异动',
        type: 'bar',
        stack: 'total',
        data: downValues,
        itemStyle: { color: '#22c55e', borderRadius: [0, 4, 4, 0] },
        label: {
          show: true,
          position: 'right',
          color: darkTheme ? '#ccc' : '#333',
          fontSize: 10,
          formatter: function(params) {
            const total = upValues[params.dataIndex] + downValues[params.dataIndex]
            return total > 0 ? total : ''
          }
        }
      }
    ]
  }

  chart.setOption(option)
}

async function handleChangeRank() {
  try {
    const days = changeRankDays.value
    const result = await GetChangeRank(days, 20)
    if (result) {
      const hasData = (result.topStocks && result.topStocks.length > 0) ||
        (result.topIndustries && result.topIndustries.length > 0) ||
        (result.topConcepts && result.topConcepts.length > 0)
      if (days === 1 && !hasData) {
        const isTrading = await IsTradingTime()
        if (!isTrading) {
          changeRankDays.value = 3
          return
        }
      }
      const periodLabel = days === 1 ? '当日' : `近${days}日`
      if (result.topStocks && result.topStocks.length > 0) {
        renderRankChart(changeRankStockRef, `${periodLabel}异动次数最多的股票`, result.topStocks, 'stock')
      }
      if (result.topIndustries && result.topIndustries.length > 0) {
        renderRankChart(changeRankIndustryRef, `${periodLabel}异动次数最多的行业`, result.topIndustries, 'industry')
      }
      if (result.topConcepts && result.topConcepts.length > 0) {
        renderRankChart(changeRankConceptRef, `${periodLabel}异动次数最多的概念`, result.topConcepts, 'concept')
      }
    }
  } catch (error) {
    console.error('获取异动排行数据失败:', error)
  }
}

function renderRankChart(chartRef, title, items, dimension) {
  if (!chartRef.value || !items || items.length === 0) return

  const chart = echarts.init(chartRef.value)

  const names = items.map(d => d.name).reverse()
  const upValues = items.map(d => d.upCount).reverse()
  const downValues = items.map(d => d.downCount).reverse()

  const option = {
    darkMode: darkTheme,
    title: {
      text: title,
      left: 'center',
      textStyle: {
        color: darkTheme ? '#ccc' : '#333',
        fontSize: 14
      }
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: {
        type: 'shadow'
      },
      formatter: function(params) {
        let result = params[0].axisValue + '<br/>'
        let total = 0
        params.forEach(param => {
          result += param.marker + ' ' + param.seriesName + ': ' + param.value + '<br/>'
          total += param.value
        })
        result += '<b>合计: ' + total + '</b><br/><span style="color:#888">点击查看按天趋势</span>'
        return result
      }
    },
    legend: {
      data: ['利好异动', '利空异动'],
      top: 25,
      textStyle: {
        color: darkTheme ? '#ccc' : '#333'
      }
    },
    grid: {
      left: '3%',
      right: '8%',
      bottom: '3%',
      top: 55,
      containLabel: true
    },
    xAxis: {
      type: 'value',
      name: '异动次数',
      axisLabel: {
        color: darkTheme ? '#999' : '#666'
      },
      axisLine: {
        lineStyle: {
          color: darkTheme ? '#444' : '#ccc'
        }
      },
      splitLine: {
        lineStyle: {
          color: darkTheme ? '#333' : '#eee'
        }
      }
    },
    yAxis: {
      type: 'category',
      data: names,
      axisLabel: {
        color: darkTheme ? '#999' : '#666',
        fontSize: 11,
        width: 80,
        overflow: 'truncate'
      },
      axisLine: {
        lineStyle: {
          color: darkTheme ? '#444' : '#ccc'
        }
      }
    },
    series: [
      {
        name: '利好异动',
        type: 'bar',
        stack: 'total',
        data: upValues,
        itemStyle: {
          color: '#ef4444',
          borderRadius: [0, 0, 0, 0]
        },
        label: {
          show: true,
          position: 'insideRight',
          color: '#fff',
          fontSize: 10,
          formatter: function(params) {
            return params.value > 0 ? params.value : ''
          }
        }
      },
      {
        name: '利空异动',
        type: 'bar',
        stack: 'total',
        data: downValues,
        itemStyle: {
          color: '#22c55e',
          borderRadius: [0, 4, 4, 0]
        },
        label: {
          show: true,
          position: 'right',
          color: darkTheme ? '#ccc' : '#333',
          fontSize: 10,
          formatter: function(params) {
            const total = upValues[params.dataIndex] + downValues[params.dataIndex]
            return total > 0 ? total : ''
          }
        }
      }
    ]
  }

  chart.setOption(option)
  chart.off('click')
  chart.on('click', function(params) {
    if (params.componentType === 'series') {
      const clickedName = names[params.dataIndex]
      if (clickedName) {
        openDimensionDetail(dimension, clickedName)
      }
    }
  })
}

async function handleBullBearRank() {
  try {
    const days = bullBearDays.value
    const result = await GetChangeRank(days, 20)
    if (result) {
      const hasData = (result.topStocks && result.topStocks.length > 0) ||
        (result.topIndustries && result.topIndustries.length > 0) ||
        (result.topConcepts && result.topConcepts.length > 0)
      if (days === 1 && !hasData) {
        const isTrading = await IsTradingTime()
        if (!isTrading) {
          bullBearDays.value = 3
          return
        }
      }
      if (result.topStocks && result.topStocks.length > 0) {
        const upStocks = [...result.topStocks].sort((a, b) => b.upCount - a.upCount).slice(0, 15)
        const downStocks = [...result.topStocks].sort((a, b) => b.downCount - a.downCount).slice(0, 15)
        renderBullBearChart(bullBearStockUpRef, '利好异动最多的股票', upStocks, 'up', 'stock')
        renderBullBearChart(bullBearStockDownRef, '利空异动最多的股票', downStocks, 'down', 'stock')
      }
      if (result.topIndustries && result.topIndustries.length > 0) {
        const upIndustries = [...result.topIndustries].sort((a, b) => b.upCount - a.upCount).slice(0, 15)
        const downIndustries = [...result.topIndustries].sort((a, b) => b.downCount - a.downCount).slice(0, 15)
        renderBullBearChart(bullBearIndustryUpRef, '利好异动最多的行业', upIndustries, 'up', 'industry')
        renderBullBearChart(bullBearIndustryDownRef, '利空异动最多的行业', downIndustries, 'down', 'industry')
      }
      if (result.topConcepts && result.topConcepts.length > 0) {
        const upConcepts = [...result.topConcepts].sort((a, b) => b.upCount - a.upCount).slice(0, 15)
        const downConcepts = [...result.topConcepts].sort((a, b) => b.downCount - a.downCount).slice(0, 15)
        renderBullBearChart(bullBearConceptUpRef, '利好异动最多的概念', upConcepts, 'up', 'concept')
        renderBullBearChart(bullBearConceptDownRef, '利空异动最多的概念', downConcepts, 'down', 'concept')
      }
    }
  } catch (error) {
    console.error('获取利好利空排行数据失败:', error)
  }
}

function renderBullBearChart(chartRefVal, title, items, direction, dimension) {
  if (!chartRefVal.value || !items || items.length === 0) return

  const chart = echarts.init(chartRefVal.value)

  const names = items.map(d => d.name).reverse()
  const mainValues = direction === 'up'
    ? items.map(d => d.upCount).reverse()
    : items.map(d => d.downCount).reverse()
  const subValues = direction === 'up'
    ? items.map(d => d.downCount).reverse()
    : items.map(d => d.upCount).reverse()

  const mainColor = direction === 'up' ? '#ef4444' : '#22c55e'
  const subColor = direction === 'up' ? '#22c55e' : '#ef4444'
  const mainLabel = direction === 'up' ? '利好次数' : '利空次数'
  const subLabel = direction === 'up' ? '利空次数' : '利好次数'

  const option = {
    darkMode: darkTheme,
    title: {
      text: title,
      left: 'center',
      textStyle: {
        color: darkTheme ? '#ccc' : '#333',
        fontSize: 14
      }
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: {
        type: 'shadow'
      },
      formatter: function(params) {
        let result = params[0].axisValue + '<br/>'
        params.forEach(param => {
          result += param.marker + ' ' + param.seriesName + ': ' + param.value + '<br/>'
        })
        const idx = params[0].dataIndex
        const d = items[items.length - 1 - idx]
        if (d) {
          result += `<b>利好: ${d.upCount} 利空: ${d.downCount} 合计: ${d.count}</b><br/>`
          result += '<span style="color:#888">点击查看按天趋势</span>'
        }
        return result
      }
    },
    legend: {
      data: [mainLabel, subLabel],
      top: 25,
      textStyle: {
        color: darkTheme ? '#ccc' : '#333'
      }
    },
    grid: {
      left: '3%',
      right: '8%',
      bottom: '3%',
      top: 55,
      containLabel: true
    },
    xAxis: {
      type: 'value',
      name: '次数',
      axisLabel: {
        color: darkTheme ? '#999' : '#666'
      },
      axisLine: {
        lineStyle: {
          color: darkTheme ? '#444' : '#ccc'
        }
      },
      splitLine: {
        lineStyle: {
          color: darkTheme ? '#333' : '#eee'
        }
      }
    },
    yAxis: {
      type: 'category',
      data: names,
      axisLabel: {
        color: darkTheme ? '#999' : '#666',
        fontSize: 11,
        width: 80,
        overflow: 'truncate'
      },
      axisLine: {
        lineStyle: {
          color: darkTheme ? '#444' : '#ccc'
        }
      }
    },
    series: [
      {
        name: mainLabel,
        type: 'bar',
        data: mainValues,
        itemStyle: {
          color: mainColor,
          borderRadius: direction === 'up' ? [0, 4, 4, 0] : [0, 4, 4, 0]
        },
        label: {
          show: true,
          position: 'right',
          color: darkTheme ? '#ccc' : '#333',
          fontSize: 10,
          formatter: function(params) {
            return params.value > 0 ? params.value : ''
          }
        }
      },
      {
        name: subLabel,
        type: 'bar',
        data: subValues,
        itemStyle: {
          color: subColor,
          opacity: 0.4
        }
      }
    ]
  }

  chart.setOption(option)
  chart.off('click')
  chart.on('click', function(params) {
    if (params.componentType === 'series') {
      const clickedName = names[params.dataIndex]
      if (clickedName) {
        openDimensionDetail(dimension, clickedName)
      }
    }
  })
}

function handleTreemap() {
  const formatUtil = echarts.format;
  AnalyzeSentimentWithFreqWeight("").then((res) => {
    treemapchart = echarts.init(treemapRef.value);
    let data = res['frequencies'].map(item => ({
      name: item.Word,
      frequency: item.Frequency,
      weight: item.Weight,
      value: item.Score,
    }));

    let data2 = res['frequencies'].map(item => ({
      name: item.Word,
       value: item.Frequency,
      frequency: item.Frequency,
      weight: item.Weight,
    }));

    let data3 = res['frequencies'].map(item => ({
      name: item.Word,
       value: item.Weight,
      frequency: item.Frequency,
      weight: item.Weight,
    }));

    let option = {
      darkMode: darkTheme,
      title: {
        text:name,
        left: 'center',
        textStyle: {
          color: darkTheme?'#ccc':'#456'
        }
      },
      legend: {
        show: false
      },
      toolbox: {
        left: '20px',
        tooltip:{
          textStyle: {
            color: darkTheme?'#ccc':'#456'
          }
        },
        feature: {
          saveAsImage: {title: '保存图片'},
          restore: {
            title: '默认',
          },
          myTool2: {
            show: true,
            title: '按权重',
            icon:"path://M393.8816 148.1216a29.3376 29.3376 0 0 1-15.2576 38.0928c-43.776 17.152-81.92 43.8272-114.2784 76.2368A345.7536 345.7536 0 0 0 159.5392 512 352.8704 352.8704 0 0 0 512 864.4608a351.744 351.744 0 0 0 249.5488-102.912 353.536 353.536 0 0 0 76.2368-114.2784c5.6832-15.2576 22.8352-20.992 38.0928-15.2576 15.2576 5.7344 20.992 22.8864 15.2576 38.0928a421.2224 421.2224 0 0 1-89.6 133.376A412.6208 412.6208 0 0 1 512 921.6c-226.7136 0-409.6-182.8864-409.6-409.6 0-108.544 41.9328-211.456 120.0128-289.5872A421.2224 421.2224 0 0 1 355.84 132.864a29.3376 29.3376 0 0 1 38.0928 15.2576zM512 102.4c226.7136 0 409.6 182.8864 409.6 409.6 0 15.2576-13.312 28.5696-28.5696 28.5696H512A29.2864 29.2864 0 0 1 483.4304 512V130.9696c0-15.2576 13.312-28.5696 28.5696-28.5696z m28.5696 59.0336v321.9968h321.9968a350.976 350.976 0 0 0-321.9968-321.9968z",
            onclick: function (){
              treemapchart.setOption( {series:{
                  data: data3
                }})
            }
          },
          myTool1: {
            show: true,
            title: '按频次',
            icon:"path://M895.466667 476.8l-87.424-87.424v-123.626667a49.770667 49.770667 0 0 0-49.770667-49.770666h-123.626667L547.2 128.533333a49.792 49.792 0 0 0-70.4 0l-87.424 87.424h-123.626667a49.770667 49.770667 0 0 0-49.770666 49.770667v123.626667L128.533333 476.8a49.792 49.792 0 0 0 0 70.4l87.424 87.424v123.626667a49.770667 49.770667 0 0 0 49.770667 49.770666h123.626667l87.424 87.424a49.792 49.792 0 0 0 70.4 0l87.424-87.424h123.626666a49.770667 49.770667 0 0 0 49.770667-49.770666v-123.626667l87.424-87.424a49.749333 49.749333 0 0 0 0.042667-70.4z m-137.216 137.194667v144.256h-144.256L512 860.266667l-101.994667-101.994667h-144.256v-144.256L163.733333 512l101.994667-101.994667v-144.256h144.256L512 163.733333l101.994667 101.994667h144.256v144.256L860.266667 512l-102.016 101.994667z M414.378667 514.730667l28.672 10.922666c-18.090667 47.445333-38.229333 92.16-60.757334 133.802667l-30.037333-13.653333a1042.133333 1042.133333 0 0 0 62.122667-131.072zM381.952 367.616L355.669333 384c25.258667 26.282667 45.056 50.176 60.074667 72.021333l25.6-17.749333c-13.994667-20.48-33.792-44.032-59.392-70.656zM537.258667 455.338667c-0.682667 43.690667-6.144 79.189333-16.725334 106.837333-14.336 32.768-44.373333 60.416-89.429333 82.944l21.162667 25.941333c52.224-26.624 85.333333-60.074667 99.328-100.693333 1.706667-5.12 3.413333-10.24 4.778666-15.36 21.504 45.738667 52.906667 83.968 93.866667 115.370667l21.504-24.917334c-51.2-34.474667-86.357333-81.237333-105.813333-140.288 1.706667-15.701333 2.730667-32.085333 2.730666-49.834666h-31.402666z M508.586667 434.858667h115.712c-6.826667 25.258667-15.018667 47.786667-24.917334 66.901333l31.744 8.874667a627.008 627.008 0 0 0 27.989334-85.674667v-21.162667H517.12c3.413333-14.336 6.144-29.354667 8.874667-45.738666l-32.426667-5.12c-7.850667 59.392-25.6 105.813333-52.906667 139.264l26.965334 19.114666c16.725333-19.114667 30.378667-44.373333 40.96-76.458666z",
            onclick: function (){
              treemapchart.setOption( {series:{
                  data: data2
                }})
            }
          }
        }
      },
      tooltip: {
        formatter: function (info) {
          var value = info.value.toFixed(2);
          var frequency = info.data.frequency;
          var weight = info.data.weight;
          return [
            '<div class="tooltip-title">' + info.name+ '</div>',
            '热度: ' + formatUtil.addCommas(value) + '',
            '<div class="tooltip-title">频次: ' +  formatUtil.addCommas(frequency)+ '</div>',
            '<div class="tooltip-title">权重: ' +  formatUtil.addCommas(weight)+ '</div>',
          ].join('');
        }
      },
      series: [
        {
          type: 'treemap',
          breadcrumb:{show: false},
          left: '0',
          top: '40',
          right: '0',
          bottom: '0',
          tooltip: {
            show: true
          },
          data: data
        }
      ]
    };
    treemapchart.setOption(option);
  })
}
</script>

<template>
  <n-collapse :trigger-areas="triggerAreas" :default-expanded-names="['1']" display-directive="show">
    <n-collapse-item  name="1" >
      <template #header>
          <n-flex>
              <n-tag size="small" :bordered="false" v-for="(item, index) in mainIndex" :type="item.zdf>0?'error':'success'">
                <n-flex>
                  <n-image :width="20" :src="item.img" />
                  <n-text style="font-size: 14px" :type="item.zdf>0?'error':'success'">{{item.name}}&nbsp;{{item.zxj}}</n-text>
                  <n-number-animation :precision="2" :from="0" :to="item.zdf" style="font-size: 14px"/>
                  <n-text style="margin-left: -12px;font-size: 14px" :type="item.zdf>0?'error':'success'">%</n-text>
                </n-flex>
              </n-tag>
          </n-flex>
      </template>
      <template #header-extra>
        主要股指
      </template>
      <n-flex justify="end" style="margin-bottom: 4px">
        <n-button-group size="tiny">
          <n-button :type="changeRankDays===1?'primary':'default'" @click="changeRankDays=1">当日</n-button>
          <n-button :type="changeRankDays===3?'primary':'default'" @click="changeRankDays=3">近3日</n-button>
          <n-button :type="changeRankDays===5?'primary':'default'" @click="changeRankDays=5">近5日</n-button>
          <n-button :type="changeRankDays===10?'primary':'default'" @click="changeRankDays=10">近10日</n-button>
        </n-button-group>
      </n-flex>
      <n-grid :cols="24" :y-gap="0">
        <n-gi span="8">
          <div ref="chartRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
        </n-gi>
        <n-gi span="8">
          <div ref="limitChartRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
        </n-gi>
        <n-gi span="8">
          <div ref="changeRankConceptRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
        </n-gi>
      </n-grid>
      <n-flex justify="center" style="margin: 8px 0" :wrap="false">
        <n-button text @click="showTreemap = !showTreemap" :type="showTreemap?'primary':''">
          {{ showTreemap ? '隐藏热词' : '查看热词' }}
        </n-button>
        <n-divider vertical />
        <n-button text @click="showDailyChart = !showDailyChart" :type="showDailyChart?'primary':''">
          {{ showDailyChart ? '隐藏按天分析' : '按天涨跌/涨跌停分析' }}
        </n-button>
        <n-divider vertical />
        <n-button text @click="showChangeStats = !showChangeStats" :type="showChangeStats?'primary':''">
          {{ showChangeStats ? '隐藏异动分析' : '历史异动分析' }}
        </n-button>
        <n-divider vertical />
        <n-button text @click="showChangeRank = !showChangeRank" :type="showChangeRank?'primary':''">
          {{ showChangeRank ? '隐藏异动排行' : '异动排行' }}
        </n-button>
        <n-divider vertical />
        <n-button text @click="showBullBearRank = !showBullBearRank" :type="showBullBearRank?'primary':''">
          {{ showBullBearRank ? '隐藏利好/利空排行' : '利好/利空排行' }}
        </n-button>
      </n-flex>
      <n-collapse-transition :show="showTreemap">
        <div ref="treemapRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
      </n-collapse-transition>
      <n-collapse-transition :show="showDailyChart">
        <n-grid :cols="24" :y-gap="0">
          <n-gi span="12">
            <div ref="dailyUpDownChartRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
          </n-gi>
          <n-gi span="12">
            <div ref="dailyLimitChartRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
          </n-gi>
        </n-grid>
      </n-collapse-transition>
      <n-collapse-transition :show="showChangeStats">
        <n-grid :cols="24" :y-gap="0">
          <n-gi span="12">
            <div ref="changeStatsChartRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
          </n-gi>
          <n-gi span="12">
            <div ref="changeTypeChartRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
          </n-gi>
        </n-grid>
      </n-collapse-transition>
      <n-collapse-transition :show="showChangeRank">
        <n-grid :cols="24" :y-gap="0">
          <n-gi span="12">
            <div ref="changeRankStockRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
          </n-gi>
          <n-gi span="12">
            <div ref="changeRankIndustryRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
          </n-gi>
        </n-grid>
      </n-collapse-transition>
      <n-collapse-transition :show="showBullBearRank">
        <n-flex justify="end" style="margin-bottom: 4px">
          <n-button-group size="tiny">
            <n-button :type="bullBearDays===1?'primary':'default'" @click="bullBearDays=1">当日</n-button>
            <n-button :type="bullBearDays===3?'primary':'default'" @click="bullBearDays=3">近3日</n-button>
            <n-button :type="bullBearDays===5?'primary':'default'" @click="bullBearDays=5">近5日</n-button>
            <n-button :type="bullBearDays===10?'primary':'default'" @click="bullBearDays=10">近10日</n-button>
            <n-button :type="bullBearDays===30?'primary':'default'" @click="bullBearDays=30">近30日</n-button>
          </n-button-group>
        </n-flex>
        <n-grid :cols="24" :y-gap="0">
          <n-gi span="8">
            <div ref="bullBearStockUpRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
          </n-gi>
          <n-gi span="8">
            <div ref="bullBearIndustryUpRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
          </n-gi>
          <n-gi span="8">
            <div ref="bullBearConceptUpRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
          </n-gi>
        </n-grid>
        <n-grid :cols="24" :y-gap="0">
          <n-gi span="8">
            <div ref="bullBearStockDownRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
          </n-gi>
          <n-gi span="8">
            <div ref="bullBearIndustryDownRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
          </n-gi>
          <n-gi span="8">
            <div ref="bullBearConceptDownRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
          </n-gi>
        </n-grid>
      </n-collapse-transition>
    </n-collapse-item>
  </n-collapse>
  <n-modal v-model:show="showDimensionModal" preset="card" :title="dimensionModalTitle" style="width: 800px" :mask-closable="true">
    <div ref="dimensionDetailChartRef" style="width: 100%;height: 450px;--wails-draggable:no-drag"></div>
  </n-modal>
</template>

<style scoped>

</style>
