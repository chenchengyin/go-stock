<script setup>
import {computed, h, onBeforeMount, onBeforeUnmount, onMounted,onUnmounted, ref,reactive} from 'vue'
import {
  GetAiRecommendStocksList,
  GetConfig,
  GetSponsorInfo,
  DeleteAiRecommendStocks,
  ShareAnalysis
} from "../../wailsjs/go/main/App";
import {NAvatar, NButton, NEllipsis, NTag, NText, useMessage, useNotification} from "naive-ui";
import KLineChart from "./KLineChart.vue";
import sparkLine from "./stockSparkLine.vue"
import {format} from "date-fns";

const notify = useNotification()
const vipLevel=ref("");
const vipStartTime=ref("");
const vipEndTime=ref("");
const expired=ref(false)
const isValidVip=ref(false) // 是否是会员

onBeforeMount(()=> {
  GetConfig().then(result => {
    if (result.darkTheme) {
      editorDataRef.darkTheme = true
    }
  })

  GetSponsorInfo().then((res) => {
   // console.log(res)
    vipLevel.value = res.vipLevel;
    vipStartTime.value = res.vipStartTime;
    vipEndTime.value = res.vipEndTime;
    //判断时间是否到期
    if (res.vipLevel) {
      if (res.vipEndTime < format(new Date(), 'yyyy-MM-dd HH:mm:ss')) {
        //notify.warning({content: 'VIP已到期'})
        expired.value = true;
      }
    }else{
      //notify.success({content: '未开通VIP'})
    }
    isValidVip.value = !(vipLevel.value === "" || Number(vipLevel.value) <= 0);
  })
})
onMounted(() => {
  query({
    page: 1,
    pageSize: paginationReactive.pageSize,
    order: "desc",
    keyword: paginationReactive.keyword,
    startDate: paginationReactive.range[0],
    endDate: paginationReactive.range[1]
  }).then((data) => {
    console.log( data)
    dataRef.value = data.data
    paginationReactive.page = 1
    paginationReactive.pageCount = data.pageCount
    paginationReactive.itemCount = data.total
    loadingRef.value = false
  })
})
const message = useMessage()
const mdPreviewRef = ref(null)
const mdEditorRef = ref(null)
const editorDataRef = reactive({
  show: false,
  loading: false,
  darkTheme: false,
  chatId: "",
  modelName: "",
  CreatedAt: "",
  stockName: "",
  stockCode: "",
  question: "",
  content: "",
})
const dataRef = ref([])
const loadingRef = ref(true)

// StockClosePrice          string     `json:"StockClosePrice" md:"推荐时股票收盘价格"`
// StockPrePrice            string     `json:"stockPrePricePrice" md:"前一交易日股票价格"`
// RecommendReason          string     `json:"recommendReason" md:"推荐理由/驱动因素/逻辑"`
// RecommendBuyPrice        string     `json:"recommendBuyPrice" md:"ai建议买入价"`
// RecommendStopProfitPrice string     `json:"recommendStopProfitPrice" md:"ai建议止盈价"`
// RecommendStopLossPrice   string     `json:"recommendStopLossPrice" md:"ai建议止损价"`
// RiskRemarks              string     `json:"riskRemarks" md:"风险提示"`
// Remarks                  string     `json:"remarks" md:"备注"`
const columnsRef = ref([
  {
    title: '推荐模型',
    key: 'modelName',
    render(row, index) {
      return h(NText, { type: "info" }, { default: () => row.modelName })
    }
  },
  {
    title: '推荐时间',
    key: 'dataTime',
    render(row, index) {
      //2026-01-14T22:13:27.2693252+08:00 格式化为常用时间格式
      return row.CreatedAt.substring(0, 19).replace('T', ' ')
    }
  },
  {
    title: '板块概念',
    key: 'bkName'
  },
  {
    title: '股票名称',
    key: 'stockName',
    render(row, index) {
      return h(NText, { type: "info" }, { default: () => row.stockName })
    }
  },
  {
    title: '股票代码',
    key: 'stockCode'
  },
  {
    title: '最新分时',
    key: 'stockCode',
    render(row, index) {
      return h(sparkLine, { idSuffix:row.ID, stockName: row.stockName, stockCode: row.stockCode, lastPrice: row.stockCurrentPrice, openPrice: row.stockPrePrice, tooltip: true }, )
    }
  },
  {
    title: '最新',
    key: 'stockCurrentPrice',
    minWidth: 120,
    render(row, index) {

      let diff = ((Number(row.stockCurrentPrice) - Number(row.stockPrePrice))/ Number(row.stockPrePrice)*100).toFixed(2)

      if(Number(row.stockCurrentPrice)< Number(row.stockPrePrice)) {
        return [h(NText, { type: "success", bordered: false }, { default: () => row.stockCurrentPrice+` |  ${diff}%` })]
      } else {
        return [h(NText, { type: "error" , bordered: false}, { default: () => row.stockCurrentPrice+` |  ${diff}%` })]
      }
    }
  },
  {
    title: '推荐时',
    key: 'stockPrice',
    render(row, index) {

      if(vipLevel.value===""|| Number(vipLevel.value) <=0){
        return h(NText, { type: "info" }, { default: () => row.stockPrice })
      }

      let diff = ((Number(row.stockCurrentPrice) - Number(row.stockPrice))/ Number(row.stockPrice)*100).toFixed(2)
      let flagStr="暂平"
      let flag="info"
      if(Number(row.stockCurrentPrice)>Number(row.stockPrice)) {
        flagStr="暂赢 "+diff+"%"
        flag="error"
      }else if(Number(row.stockCurrentPrice)===Number(row.stockPrice)){
        flagStr="暂平"
        flag="info"
      }else{
        flagStr="暂亏 "+ diff+"%"
        flag="success"
      }

      return [h(NText, { type: "info" }, { default: () => row.stockPrice }),h(NTag, { type: flag,size: "tiny", bordered: false }, { default: () => flagStr })]
    }
  },
  {
    title: '昨收',
    key: 'stockPrePrice',
    render(row, index) {
      return h(NText, { type: "info" }, { default: () => row.stockPrePrice })
    }
  },
  {
    title: 'ai建议买入价',
    key: 'recommendBuyPrice',
    render(row, index) {
      if(vipLevel.value===""|| Number(vipLevel.value) <=0){
        return h(NText, { type: "info" }, { default: () => row.recommendBuyPrice })
      }


      if(row.recommendBuyPrice.includes("-")){
        let prices= row.recommendBuyPrice.split("-")
        if(Number(row.stockCurrentPrice)>=Number(prices[0])&&Number(row.stockCurrentPrice)<=Number(prices[1])){
          return [h(NText, { type: "success" }, { default: () => row.recommendBuyPrice }),h(NTag, { type: "error", size: "tiny", bordered: false }, { default: () => "Buy" })]
        }
      }
      if(row.recommendBuyPriceMin&&row.recommendBuyPriceMax&&Number(row.stockCurrentPrice)<Number(row.recommendBuyPriceMax)&&Number(row.stockCurrentPrice)>Number(row.recommendBuyPriceMin)){
        return [h(NText, { type: "success" }, { default: () => row.recommendBuyPrice }),h(NTag, { type: "error", size: "tiny", bordered: false }, { default: () => "Buy" })]
      }
      return h(NText, { type: "info" }, { default: () => row.recommendBuyPrice })

    }
  },
  {
    title: 'ai建议止盈价',
    key: 'recommendStopProfitPrice',
    render(row, index) {
      if(vipLevel.value===""|| Number(vipLevel.value) <=0){
        return h(NText, { type: "info" }, { default: () => row.recommendStopProfitPrice })
      }
      if(row.recommendStopProfitPrice.includes("-")){
        let prices= row.recommendStopProfitPrice.split("-")
        if(Number(row.stockCurrentPrice)>=Number(prices[0])&&Number(row.stockCurrentPrice)<=Number(prices[1])){
          return [h(NText, { type: "success" }, { default: () => row.recommendStopProfitPrice }),h(NTag, { type: "error", size: "tiny", bordered: false }, { default: () => "Sell" })]
        }
      }
      if(row.recommendStopProfitPriceMin&&Number(row.stockCurrentPrice)>row.recommendStopProfitPriceMin){
        return [h(NText, { type: "success" }, { default: () => row.recommendStopProfitPrice }),h(NTag, { type: "error", size: "tiny", bordered: false }, { default: () => "Sell" })]
      }

      return h(NText, { type: "info" }, { default: () => row.recommendStopProfitPrice })
    }
  },
  {
    title: 'ai建议止损价',
    key: 'recommendStopLossPrice',
    render(row, index) {
      if(vipLevel.value===""|| Number(vipLevel.value) <=0){
        return h(NText, { type: "info" }, { default: () => row.recommendStopLossPrice })
      }
      if(row.recommendStopLossPrice.includes("-")){
        let prices= row.recommendStopLossPrice.split("-")
        if(Number(row.stockCurrentPrice)<=Number(prices[0])){
          return [h(NText, { type: "success" }, { default: () => row.recommendStopLossPrice }),h(NTag, { type: "error", size: "tiny", bordered: false }, { default: () => "Sell" })]
        }
      }else{
        let prices=row.recommendStopLossPrice
        if(Number(row.stockCurrentPrice)<=Number(prices)){
          return [h(NText, { type: "success" }, { default: () => row.recommendStopLossPrice }),h(NTag, { type: "error", size: "tiny", bordered: false }, { default: () => "Sell" })]
        }
      }
      return h(NText, { type: "info" }, { default: () => row.recommendStopLossPrice })

    }
  },
  {
    title: '推荐理由',
    key: 'recommendReason',
    ellipsis: {
      tooltip: isValidVip
    }
  },
  {
    title: '风险提示',
    key: 'riskRemarks',
    ellipsis: {
      tooltip: isValidVip
    }
  },
  {
    title: '备注',
    key: 'remarks',
    ellipsis: {
      tooltip: isValidVip
    }
  },
  {
    title: '操作',
    render(row, index) {
      return [h(
          NTag,
          {
            strong: true,
            tertiary: true,
            //size: 'small',
            type: 'warning', // 橙色按钮
            onClick: () => showDetail(row)
          },
          { default: () => '查看' }
      ),h(NTag, { strong: true,
        tertiary: true, type: 'error',  onClick: () => deleteAiRecommendStocks(row.ID) }, { default: () => '删除' })]
    }
  },
])
const paginationReactive = reactive({
  page: 1,
  pageCount: 1,
  pageSize: 12,
  itemCount: 0,
  keyword: "",
  range: [
    new Date(new Date().getTime() - 3 * 24 * 60 * 60 * 1000), // 前3天
    new Date() // 当天
  ],
  prefix({ itemCount }) {
    return `${itemCount} 条记录`
  }
})

const modalDataRef = reactive({
  visible: false,
  title: "",
  content: "",
  riskRemarks: "",
  stockCode: "",
  stockName: "",
  remarks: "",
})

const theme = computed(() => {
  return editorDataRef.darkTheme ? 'dark' : 'light'
})


function query({
                 page,
                 pageSize = 10,
                 order = 'desc',
                 keyword = "",
                 startDate = "",
                 endDate = ""
               }) {
  return new Promise((resolve) => {

    GetAiRecommendStocksList({
      "page": page,
      "pageSize": pageSize,
      "modelName":keyword,
      "stockName":keyword,
      "stockCode":keyword,
      "bkName":keyword,
      "startDate": startDate,
      "endDate": endDate
    }).then((res) => {
      const pagedData =res.list
      const total = res.total
      const pageCount =res.totalPages
      resolve({
        pageCount,
        data: pagedData,
        total
      })
    })
  })
}

function handlePageChange(currentPage) {
  if (!loadingRef.value) {
    loadingRef.value = true
    query({
      page: currentPage,
      pageSize: paginationReactive.pageSize,
      order: "desc",
      keyword: paginationReactive.keyword,
      startDate: formatDate(paginationReactive.range[0]), // Format date to string
      endDate: formatDate(paginationReactive.range[1]) // Format date to string
    }).then((data) => {
      dataRef.value = data.data
      paginationReactive.page = currentPage
      paginationReactive.pageCount = data.pageCount
      paginationReactive.itemCount = data.total
      loadingRef.value = false
    })
  }
}
function handleSearch() {
  if (!loadingRef.value) {
    loadingRef.value = true
    query({
      page: 1,
      pageSize: paginationReactive.pageSize,
      order: "desc",
      keyword: paginationReactive.keyword,
      startDate: formatDate(paginationReactive.range[0]),
      endDate: formatDate(paginationReactive.range[1])
    }).then((data) => {
      dataRef.value = data.data
      paginationReactive.page = 1
      paginationReactive.pageCount = data.pageCount
      paginationReactive.itemCount = data.total
      loadingRef.value = false
    })
  }
}
function formatDate(dateString) {
  const date = new Date(dateString)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  // const hours = String(date.getHours()).padStart(2, '0')
  // const minutes = String(date.getMinutes()).padStart(2, '0')
  // const seconds = String(date.getSeconds()).padStart(2, '0')
  //return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
  return `${year}-${month}-${day}`
}
function getStockCode(stockCode) {
  if(stockCode.indexOf( ".")>0){
    stockCode=stockCode.split(".")[1]+stockCode.split(".")[0]
  }
  //转化为小写
  stockCode=stockCode.toLowerCase()
  return stockCode

}
function showDetail(row) {
  if(vipLevel.value===""|| Number(vipLevel.value) <=0){
    notify.warning({content: '未开通VIP或者已经过期'})
    return
  }
  modalDataRef.title = row.stockName
  modalDataRef.content = row.recommendReason
  modalDataRef.riskRemarks = row.riskRemarks
  modalDataRef.stockCode = getStockCode(row.stockCode)
  modalDataRef.stockName = row.stockName
  modalDataRef.visible = true
  modalDataRef.remarks = row.remarks
}
function rowProps(row) {
  return {
    style: 'cursor: pointer;',
    onClick: () => {
      showDetail(row)
    }
  }
}
function deleteAiRecommendStocks(id) {
  DeleteAiRecommendStocks(id).then((res) => {
    notify.info({content: res})
    handleSearch()
  })
}

</script>

<template>
  <n-input-group>
    <n-date-picker  v-model:value="paginationReactive.range" type="daterange"   style="width: 50%"/>
    <n-input clearable placeholder="输入关键词搜索" v-model:value="paginationReactive.keyword"/>
    <n-button type="primary" ghost @click="handleSearch"  @input="handleSearch">
      搜索
    </n-button>
  </n-input-group>
        <n-data-table
            remote
            size="small"
            :columns="columnsRef"
            :data="dataRef"
            :loading="loadingRef"
            :pagination="paginationReactive"
            :row-key="(rowData)=>rowData.ID"
            @update:page="handlePageChange"
            flex-height
            style="height: calc(100vh - 210px);margin-top: 10px"
        />

  <n-modal v-model:show="modalDataRef.visible" :title="modalDataRef.title" preset="card" style="width: 850px;">
    <n-gradient-text :size="16" type="warning">{{modalDataRef.remarks}}</n-gradient-text>
    <n-card size="small">
      <KLineChart style="width: 800px" :code="getStockCode(modalDataRef.stockCode)" :chart-height="500" :stock-name="modalDataRef.stockName" :k-days="30" :dark-theme="editorDataRef.darkTheme"></KLineChart>
    </n-card>
    <n-card size="small">
    <n-text type="info">{{modalDataRef.content}}</n-text>
    <n-divider><n-gradient-text type="error">风险提示</n-gradient-text></n-divider>
    <n-text type="error">{{modalDataRef.riskRemarks}}</n-text>
    </n-card>
  </n-modal>
</template>

<style scoped>

</style>