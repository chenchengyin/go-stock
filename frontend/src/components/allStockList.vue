<script setup>
import {h, onBeforeMount, onMounted, ref, reactive} from 'vue'
import {
  GetAllStockInfoList,
  GetAllStocks,
  GetConfig
} from "../../wailsjs/go/main/App";
import {NButton, NInput, NTag, NText, useMessage, useNotification, NDataTable, NSpace, NPagination} from "naive-ui";
import sparkLine from "./stockSparkLine.vue"
import klineChart from "./KLineChart.vue"
import KLineChart from "./KLineChart.vue";

const notify = useNotification()
const message = useMessage()

const editorDataRef = reactive({
  darkTheme: false
})

onBeforeMount(() => {
  GetConfig().then(result => {
    if (result.darkTheme) {
      editorDataRef.darkTheme = true
    }
  })
})

onMounted(() => {
  console.log('stock-list mounted')
  loadStocks(1, paginationReactive.pageSize)
})

const dataRef = ref([])
const loadingRef = ref(false)

const columnsRef = ref([
  // {
  //   title: '数据时间',
  //   key: 'MAX_TRADE_DATE',
  //   width: 120,
  // },
  {
    title: '股票代码',
    key: 'SECUCODE',
    width: 120,
    render(row) {
      return h(NText, { type: "info" }, { default: () => row.SECUCODE })
    }
  },
  {
    title: '股票名称',
    key: 'SECURITY_NAME_ABBR',
    width: 120,
    render(row) {
      return h(NText, { type: "success" }, { default: () => row.SECURITY_NAME_ABBR })
    }
  },
  {
    title: '最新价',
    key: 'NEW_PRICE',
    width: 100,
    render(row) {
      return h(NText, { type: "info" }, { default: () => row.NEW_PRICE.toFixed(2) })
    }
  },
  {
    title: '涨跌幅(%)',
    key: 'CHANGE_RATE',
    width: 120,
    render(row) {
      const rate = row.CHANGE_RATE
      const type = rate >= 0 ? 'error' : 'success'
      const sign = rate >= 0 ? '+' : ''
      return h(NText, { type: type }, { default: () => `${sign}${rate.toFixed(2)}%` })
    }
  },
  {
    title: '分时图',
    key: 'sparkline',
    width: 120,
    render(row) {
      return h(sparkLine, {
        idSuffix: row.SECUCODE,
        stockName: row.SECURITY_NAME_ABBR,
        stockCode: row.SECUCODE,
        lastPrice: row.NEW_PRICE,
        openPrice: row.PRE_CLOSE_PRICE,
        tooltip: true
      })
    }
  },
  {
    title: '最高价',
    key: 'HIGH_PRICE',
    width: 100,
    render(row) {
      return h(NText, { type: "info" }, { default: () => row.HIGH_PRICE.toFixed(2) })
    }
  },
  {
    title: '最低价',
    key: 'LOW_PRICE',
    width: 100,
    render(row) {
      return h(NText, { type: "info" }, { default: () => row.LOW_PRICE.toFixed(2) })
    }
  },
  // {
  //   title: '前收价',
  //   key: 'PRE_CLOSE_PRICE',
  //   width: 100,
  //   render(row) {
  //     return h(NText, { type: "info" }, { default: () => row.PRE_CLOSE_PRICE.toFixed(2) })
  //   }
  // },
  {
    title: '成交量',
    key: 'VOLUME',
    width: 120,
    render(row) {
      const volume = row.VOLUME
      let displayVolume = volume
      if (volume >= 100000000) {
        displayVolume = (volume / 100000000).toFixed(2) + '亿'
      } else if (volume >= 10000) {
        displayVolume = (volume / 10000).toFixed(2) + '万'
      }
      return h(NText, { type: "info" }, { default: () => displayVolume })
    }
  },
  {
    title: '成交额',
    key: 'DEAL_AMOUNT',
    width: 120,
    render(row) {
      const amount = row.DEAL_AMOUNT
      let displayAmount = amount
      if (amount >= 100000000) {
        displayAmount = (amount / 100000000).toFixed(2) + '亿'
      } else if (amount >= 10000) {
        displayAmount = (amount / 10000).toFixed(2) + '万'
      }
      return h(NText, { type: "info" }, { default: () => displayAmount })
    }
  },
  {
    title: '换手率(%)',
    key: 'TURNOVERRATE',
    width: 100,
    render(row) {
      return h(NText, { type: "info" }, { default: () => row.TURNOVERRATE.toFixed(2) + '%' })
    }
  },
  {
    title: '量比',
    key: 'VOLUME_RATIO',
    width: 80,
    render(row) {
      return h(NText, { type: "info" }, { default: () => row.VOLUME_RATIO.toFixed(2) })
    }
  },
  {
    title: '所属行业',
    key: 'INDUSTRY',
    width: 120,
    render(row) {
      return h(NTag, { type: "primary", size: "small" }, { default: () => row.INDUSTRY })
    }
  },
  // {
  //   title: '所属概念',
  //   key: 'CONCEPT',
  //   width: 200,
  //   render(row) {
  //     if(typeof row.CONCEPT === 'string'){
  //       return h(NTag, { type: "info", size: "small" ,style: "margin-right: 4px;" }, { default: () => row.CONCEPT })
  //     }else{
  //       if (!row.CONCEPT || row.CONCEPT.length === 0) {
  //         return h(NText, { type: "secondary" }, { default: () => '无' })
  //       }
  //       return row.CONCEPT.map(concept =>
  //           h(NTag, { type: "info", size: "small", style: "margin-right: 4px;" }, { default: () => concept })
  //       )
  //     }
  //   }
  // },
  // {
  //   title: '交易所',
  //   key: 'MARKET',
  //   width: 100,
  //   render(row) {
  //     return h(NTag, { type: "warning", size: "small" }, { default: () => row.MARKET })
  //   }
  // },
  {
    title: '操作',
    render(row, index) {
      return [h(
          NButton,
          {
            secondary: true,
            size: 'small',
            type: 'warning', // 橙色按钮
            onClick: () => showKline(row)
          },
          { default: () => '日K' }
      ),]
    }
  },
])

const paginationReactive = reactive({
  keyword:"",
  page: 1,
  pageCount: 1,
  pageSize: 15,
  itemCount: 0,
  prefix({ itemCount }) {
    return `${itemCount} 只股票`
  }
})
const optionsReactive= reactive([
  {
    label: '全部',
    value: ''
  },
 ])

function loadStocks(page, pageSize) {
  if (!loadingRef.value) {
    loadingRef.value = true
    GetAllStocks(page, pageSize, paginationReactive.keyword).then((res) => {
      console.log(res)
      if (res && res.result && res.result.data) {
        dataRef.value = res.result.data
        paginationReactive.page = page
        paginationReactive.pageCount = Math.ceil(res.result.count / pageSize)
        paginationReactive.itemCount = res.result.count
      } else {
        dataRef.value = []
        paginationReactive.page = 1
        paginationReactive.pageCount = 1
        paginationReactive.itemCount = 0
        message.error('获取股票数据失败')
      }
      loadingRef.value = false
    }).catch(err => {
      message.error('获取股票数据失败: ' + err.message)
      loadingRef.value = false
    })
  }
}

function handlePageChange(currentPage) {
  loadStocks(currentPage, paginationReactive.pageSize)
}

function handlePageSizeChange(pageSize) {
  paginationReactive.pageSize = pageSize
  loadStocks(1, pageSize)
}
function handleSearch() {
  loadStocks(1, paginationReactive.pageSize)
}
function handleUpdateVal(value) {
  console.log('handleUpdateVal', value)
  if (value === '') {
    optionsReactive.splice(1, optionsReactive.length - 1)
  } else {
    GetAllStockInfoList({
      searchKeyWord: value
    }).then((res) => {
      console.log('GetAllStockInfoList result:', res)
      if (res  && res.list) {
        optionsReactive.splice(1, optionsReactive.length - 1)
        optionsReactive.push(...res.list.map(item => {
          return {
            label: item.SECURITY_NAME_ABBR,
            value: item.SECURITY_NAME_ABBR,
            obj: item,
          }
        }))
      }
    }).catch(err => {
      message.error('获取股票数据失败: ' + err.message)
    })
  }
}
const modalDataRef = reactive({
  visible: false,
  title: "",
  content: "",
  riskRemarks: "",
  stockCode: "",
  stockName: "",
  remarks: "",
})
function showKline(row) {
  console.log('showKline', row)
  modalDataRef.title = row.SECURITY_NAME_ABBR
  modalDataRef.stockCode = getStockCode(row.SECUCODE)
  modalDataRef.stockName = row.SECURITY_NAME_ABBR
  modalDataRef.visible = true
}
function getStockCode(stockCode) {
  if(stockCode.indexOf( ".")>0){
    stockCode=stockCode.split(".")[1]+stockCode.split(".")[0]
  }
  //转化为小写
  stockCode=stockCode.toLowerCase()
  return stockCode

}
</script>

<template>
  <div>
    <n-input-group>
<!--    <n-input clearable placeholder="输入股票名称" v-model:value="paginationReactive.keyword"/>-->
      <n-auto-complete
          v-model:value="paginationReactive.keyword"
          :input-props="{
            autocomplete: 'disabled',
          }"
          :options="optionsReactive"
          placeholder="输入搜索关键词"
          clearable
          @input="handleUpdateVal"
          @select="(value) => {
            paginationReactive.keyword = value
            handleSearch()
          }"
      />
    <n-button type="primary" ghost @click="handleSearch"  @input="handleSearch">
      搜索
    </n-button>
    </n-input-group>
    <!-- 数据表格 -->
    <n-data-table
      remote
      size="small"
      :columns="columnsRef"
      :data="dataRef"
      :loading="loadingRef"
      :pagination="paginationReactive"
      :row-key="(rowData) => rowData.SECUCODE"
      flex-height
      style="height: calc(100vh - 210px);margin-top: 10px"
      @update:page="handlePageChange"
    />
    
    <!-- 分页控件 -->
<!--    <div style="margin-top: 16px; display: flex; justify-content: center;">-->
<!--      <n-pagination-->
<!--        v-model:page="paginationReactive.page"-->
<!--        v-model:page-size="paginationReactive.pageSize"-->
<!--        :page-count="paginationReactive.pageCount"-->
<!--        :item-count="paginationReactive.itemCount"-->
<!--        :page-sizes="[10, 20, 50, 100]"-->
<!--        show-size-picker-->
<!--        show-quick-jumper-->
<!--        @update:page="handlePageChange"-->
<!--        @update:page-size="handlePageSizeChange"-->
<!--      />-->
<!--    </div>-->
  </div>

  <n-modal v-model:show="modalDataRef.visible" :title="modalDataRef.title" preset="card" style="width: 850px;">
    <n-card size="small">
      <KLineChart style="width: 800px" :code="getStockCode(modalDataRef.stockCode)" :chart-height="500" :stock-name="modalDataRef.stockName" :k-days="30" :dark-theme="editorDataRef.darkTheme"></KLineChart>
    </n-card>
  </n-modal>
</template>

<style scoped>
</style>
