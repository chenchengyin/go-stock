<script setup>
import {h, reactive, ref, computed, watch} from "vue";
import {NButton, NText, NFlex, NTag} from "naive-ui";
import {
  FollowFund,
  GetFollowedFund,
  GetFundRanking,
  OpenURL
} from "../../wailsjs/go/main/App";
import {Environment} from "../../wailsjs/runtime";
import {useMessage} from "naive-ui";

const message = useMessage()

const marketType = ref('kf')
const rankingFundType = ref('all')
const rankingSortField = ref('jnzf')
const rankingLoading = ref(false)
const rankingData = ref([])
const followList = ref([])
const searchKeyword = ref('')

const paginationReactive = reactive({
  page: 1,
  pageCount: 1,
  pageSize: 50,
  itemCount: 0,
  prefix({itemCount}) {
    return `${itemCount} 只基金`
  }
})

const filteredData = computed(() => {
  const kw = searchKeyword.value.trim().toLowerCase()
  if (!kw) return rankingData.value
  return rankingData.value.filter(item =>
    item.code?.toLowerCase().includes(kw) || item.name?.toLowerCase().includes(kw) || item.pinyin?.toLowerCase().includes(kw)
  )
})

const marketTypeOptions = [
  {label: '场外基金', value: 'kf'},
  {label: '场内基金', value: 'fb'},
]

const offExchangeFundTypeOptions = [
  {label: '全部', value: 'all'},
  {label: '股票型', value: 'gp'},
  {label: '混合型', value: 'hh'},
  {label: '债券型', value: 'zq'},
  {label: '指数型', value: 'zs'},
  {label: 'QDII', value: 'qdii'},
  {label: 'FOF', value: 'fof'},
]

const onExchangeFundTypeOptions = [
  {label: '全部', value: 'ct'},
  {label: 'ETF', value: 'etf'},
  {label: 'LOF', value: 'lof'},
]

const fundTypeOptions = computed(() => {
  return marketType.value === 'fb' ? onExchangeFundTypeOptions : offExchangeFundTypeOptions
})

const sortFieldOptions = [
  {label: '今年来涨幅', value: 'jnzf'},
  {label: '日涨幅', value: 'rzdf'},
  {label: '近1周涨幅', value: '1yzf'},
  {label: '近1月涨幅', value: '1mzf'},
  {label: '近3月涨幅', value: '3mzf'},
  {label: '近6月涨幅', value: '6mzf'},
  {label: '近1年涨幅', value: '1nzf'},
  {label: '近2年涨幅', value: '2nzf'},
  {label: '近3年涨幅', value: '3nzf'},
  {label: '成立来涨幅', value: 'clzf'},
  {label: '规模', value: 'gm'},
]

watch(marketType, () => {
  rankingFundType.value = marketType.value === 'fb' ? 'ct' : 'all'
  paginationReactive.page = 1
  searchKeyword.value = ''
  fetchFundRanking()
})

watch(rankingFundType, () => {
  paginationReactive.page = 1
  fetchFundRanking()
})

watch(rankingSortField, () => {
  paginationReactive.page = 1
  fetchFundRanking()
})

function renderGrowth(val) {
  if (val == null) return '-'
  const color = val > 0 ? '#ef5350' : val < 0 ? '#26a69a' : undefined
  return h(NText, {style: {color}}, () => (val > 0 ? '+' : '') + val.toFixed(2) + '%')
}

const rankingColumns = computed(() => {
  const cols = [
    {title: '代码', key: 'code', width: 80, fixed: 'left'},
    {title: '名称', key: 'name', width: 150, ellipsis: {tooltip: true}},
  ]
  if (marketType.value === 'fb') {
    cols.push({title: '类型', key: 'fundTypeDetail', width: 90, render: (row) => row.fundTypeDetail ? h(NTag, {size: 'tiny', bordered: false, type: 'info'}, () => row.fundTypeDetail) : '-'})
  }
  cols.push(
    {title: '日期', key: 'netValueDate', width: 82},
    {title: '净值', key: 'netUnitValue', width: 72, align: 'right', render: (row) => row.netUnitValue?.toFixed(4) ?? '-'},
    {title: '累计', key: 'netAccumulated', width: 72, align: 'right', render: (row) => row.netAccumulated?.toFixed(4) ?? '-'},
    {title: '日涨幅', key: 'dailyGrowth', width: 80, align: 'right', render: (row) => renderGrowth(row.dailyGrowth)},
    {title: '周涨幅', key: 'weekGrowth', width: 80, align: 'right', render: (row) => renderGrowth(row.weekGrowth)},
    {title: '月涨幅', key: 'monthGrowth', width: 80, align: 'right', render: (row) => renderGrowth(row.monthGrowth)},
    {title: '季涨幅', key: 'threeMonthGrowth', width: 80, align: 'right', render: (row) => renderGrowth(row.threeMonthGrowth)},
    {title: '半年', key: 'sixMonthGrowth', width: 80, align: 'right', render: (row) => renderGrowth(row.sixMonthGrowth)},
    {title: '年涨幅', key: 'yearGrowth', width: 80, align: 'right', render: (row) => renderGrowth(row.yearGrowth)},
    {title: '3年', key: 'threeYearGrowth', width: 80, align: 'right', render: (row) => renderGrowth(row.threeYearGrowth)},
    {title: '今年来', key: 'ytdGrowth', width: 82, align: 'right', render: (row) => renderGrowth(row.ytdGrowth)},
    {title: '成立来', key: 'sinceInception', width: 82, align: 'right', render: (row) => renderGrowth(row.sinceInception)},
    {title: '规模', key: 'scale', width: 70, align: 'right', render: (row) => row.scale?.toFixed(2) ?? '-'},
    {title: '成立日', key: 'establishDate', width: 82},
    {
      title: '操作', key: 'actions', width: 120, fixed: 'right',
      render: (row) => {
        const isFollowed = followList.value.some(f => f.code === row.code)
        return h(NFlex, {size: 4, wrap: false}, () => [
          h(NButton, {
            size: 'tiny',
            type: isFollowed ? 'default' : 'primary',
            disabled: isFollowed,
            onClick: () => rankingFollowFund(row.code)
          }, () => isFollowed ? '已关注' : '关注'),
          h(NButton, {size: 'tiny', type: 'warning', onClick: () => search(row.code)}, () => '详情'),
        ])
      }
    },
  )
  return cols
})

function fetchFundRanking() {
  rankingLoading.value = true
  GetFundRanking(marketType.value, rankingFundType.value, rankingSortField.value, 'desc', paginationReactive.page, paginationReactive.pageSize).then(result => {
    if (result) {
      rankingData.value = result.items || []
      paginationReactive.pageCount = result.totalPages || 0
      paginationReactive.itemCount = result.totalCount || 0
    }
  }).catch(() => {
    rankingData.value = []
  }).finally(() => {
    rankingLoading.value = false
  })
}

function handlePageChange(currentPage) {
  if (!rankingLoading.value) {
    paginationReactive.page = currentPage
    fetchFundRanking()
  }
}

function resetRankingFilter() {
  rankingFundType.value = marketType.value === 'fb' ? 'ct' : 'all'
  rankingSortField.value = 'jnzf'
  searchKeyword.value = ''
  paginationReactive.page = 1
  fetchFundRanking()
}

function rankingFollowFund(code) {
  FollowFund(code).then(result => {
    if (result) {
      message.success('关注成功')
      loadFollowList()
    }
  })
}

function loadFollowList() {
  GetFollowedFund().then(result => {
    followList.value = result
  })
}

function search(code) {
  setTimeout(() => {
    Environment().then(env => {
      switch (env.platform) {
        case 'windows':
          window.open("https://fund.eastmoney.com/" + code + ".html", "_blank", "noreferrer,width=1000,top=100,left=100,status=no,toolbar=no,location=no,scrollbars=no")
          break
        default:
          OpenURL("https://fund.eastmoney.com/" + code + ".html")
      }
    })
  }, 300)
}

loadFollowList()
fetchFundRanking()
</script>

<template>
  <n-input-group>
    <n-select v-model:value="marketType" :options="marketTypeOptions" style="width: 110px;" size="small"/>
    <n-input v-model:value="searchKeyword" placeholder="基金名称/代码" clearable size="small" style="width: 160px;"/>
    <n-select v-model:value="rankingFundType" :options="fundTypeOptions" style="width: 110px;" size="small"/>
    <n-select v-model:value="rankingSortField" :options="sortFieldOptions" style="width: 130px;" size="small"/>
    <n-button type="primary" size="small" @click="fetchFundRanking" :loading="rankingLoading">查询</n-button>
    <n-button size="small" @click="resetRankingFilter">重置</n-button>
  </n-input-group>
  <n-data-table
    remote
    size="small"
    :columns="rankingColumns"
    :data="filteredData"
    :loading="rankingLoading"
    :pagination="paginationReactive"
    :row-key="(row) => row.code"
    @update:page="handlePageChange"
    flex-height
    style="height: calc(100vh - 170px);"
    :bordered="false"
    striped
    :scroll-x="1500"
  />
</template>

<style scoped>
:deep(.n-data-table-td),
:deep(.n-data-table-th) {
  white-space: nowrap;
}
</style>
