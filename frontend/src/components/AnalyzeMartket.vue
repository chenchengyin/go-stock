<script setup>

import {AnalyzeSentimentWithFreqWeight,GlobalStockIndexes,GetTodayMarketStatistic} from "../../wailsjs/go/main/App";
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
const showTreemap = ref(false);
const triggerAreas=ref(["main","extra","arrow"])
let handleChartInterval=null
let handleIndexInterval=null
let treemapchart =null;

onMounted(() => {
  handleChart()
  handleTreemap()
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
      <n-grid :cols="24" :y-gap="0">
        <n-gi span="12">
          <div ref="chartRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
        </n-gi>
        <n-gi span="12">
          <div ref="limitChartRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
        </n-gi>
      </n-grid>
      <n-divider style="margin: 8px 0">
        <n-button text @click="showTreemap = !showTreemap">
          <template #icon>
            <n-icon><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><path d="M11 7V4a1 1 0 0 1 1-1h8a1 1 0 0 1 1 1v8a1 1 0 0 1-1 1h-3v3a1 1 0 0 1-1 1H8a1 1 0 0 1-1-1v-8a1 1 0 0 1 1-1h3zm-2 4v6h6v-6H9zm8-4v6h2V5h-6v2h4z"></path></svg></n-icon>
          </template>
          {{ showTreemap ? '隐藏热词' : '查看热词' }}
        </n-button>
      </n-divider>
      <n-collapse-transition :show="showTreemap">
        <div ref="treemapRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
      </n-collapse-transition>
    </n-collapse-item>
  </n-collapse>
</template>

<style scoped>

</style>
