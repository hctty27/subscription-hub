<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import LedgerWorkspace from './components/LedgerWorkspace.vue'
import ItemDetails from './components/ItemDetails.vue'

type Kind = 'subscription' | 'resource'
type Status = 'active' | 'paused' | 'canceled'
type Period = 'none' | 'day' | 'month' | 'year'
type Page = 'overview' | Kind | 'categories' | 'payments' | 'reports' | 'reminders'

interface Category {
  id: number
  name: string
  color: string
}

interface ExpenseTotal {
  currency: string
  amount_minor: number
  count: number
}

interface Item {
  id: number
  category_id: number | null
  name: string
  kind: Kind
  status: Status
  amount_minor: number
  currency: string
  period_unit: Period
  period_count: number
  first_due_date: string
  next_due_date: string
  days_until: number
  renewal_completed: boolean
  notes: string
}

type ItemDraft = Omit<Item, 'id' | 'next_due_date' | 'days_until' | 'amount_minor' | 'renewal_completed'>

const page = ref<Page>('overview')
const initialDetailPaymentID = ref<number | null>(null)
const showAdvanced = ref(false)
const ledgerRefreshKey = ref(0)
const categories = ref<Category[]>([])
const items = ref<Item[]>([])
const selectedItem = ref<Item | null>(null)
const monthExpenses = ref<ExpenseTotal[]>([])
const expenseError = ref('')
const expenseLoading = ref(false)
const busy = ref(true)
const authenticated = ref(false)
const authChecking = ref(true)
const loginPassword = ref('')
const loginBusy = ref(false)
const loginError = ref('')
const backupInfo = ref('')
const error = ref('')
const search = ref('')
const statusFilter = ref('')
const showEditor = ref(false)
const editingID = ref<number | null>(null)
const saving = ref(false)
const amountText = ref('0.00')
const editor = ref<ItemDraft>(blankItem('subscription'))
const categoryName = ref('')
const categoryColor = ref('#5470e8')
const editingCategoryID = ref<number | null>(null)

function navigate(target: Page) {
  page.value = target
}

function refreshPage() {
  if (page.value === 'payments' || page.value === 'reports' || page.value === 'reminders') {
    ledgerRefreshKey.value += 1
    return
  }
  void reload()
}

function todayString(): string {
  const date = new Date()
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return year + '-' + month + '-' + day
}

function blankItem(kind: Kind): ItemDraft {
  return {
    category_id: null,
    name: '',
    kind,
    status: 'active',
    currency: 'CNY',
    period_unit: kind === 'subscription' ? 'month' : 'year',
    period_count: 1,
    first_due_date: todayString(),
    notes: '',
  }
}

async function api<T>(path: string, method = 'GET', payload?: unknown): Promise<T> {
  const response = await fetch('/api/v1' + path, {
    method,
    headers: {
      ...(payload === undefined ? {} : { 'Content-Type': 'application/json' }),
      ...(method === 'GET' ? {} : { 'X-CSRF-Token': '1' }),
    },
    body: payload === undefined ? undefined : JSON.stringify(payload),
  })
  if (response.status === 204) return undefined as T
  if (response.status === 401) authenticated.value = false
  const data = await response.json()
  if (!response.ok) throw new Error(data?.error?.message || '请求失败，请稍后重试')
  return data as T
}

async function loadMonthExpenses() {
  expenseLoading.value = true
  expenseError.value = ''
  const month = todayString().slice(0, 7)
  try {
    const data = await api<{ totals: ExpenseTotal[] }>('/reports/expenses?month=' + month)
    monthExpenses.value = data.totals
  } catch (reason) {
    expenseError.value = messageOf(reason)
  } finally {
    expenseLoading.value = false
  }
}

async function reload() {
  busy.value = true
  try {
    const [categoryData, itemData] = await Promise.all([
      api<{ items: Category[] }>('/categories'),
      api<{ items: Item[] }>('/items?limit=500'),
    ])
    categories.value = categoryData.items
    items.value = itemData.items
    if (selectedItem.value) {
      selectedItem.value = items.value.find(item => item.id === selectedItem.value?.id) ?? null
    }
    await loadMonthExpenses()
    error.value = ''
  } catch (reason) {
    error.value = messageOf(reason)
  } finally {
    busy.value = false
  }
}

function messageOf(reason: unknown): string {
  return reason instanceof Error ? reason.message : '操作失败'
}

const activeSubscriptions = computed(() => items.value.filter(item =>
  item.kind === 'subscription' && item.status === 'active').length)
const activeResources = computed(() => items.value.filter(item =>
  item.kind === 'resource' && item.status === 'active').length)
const upcoming = computed(() => items.value.filter(item =>
  item.status === 'active' && item.next_due_date && item.days_until <= 30))
const urgentDue = computed(() => upcoming.value.filter(item => item.days_until <= 7).length)
const upcomingSorted = computed(() => items.value
  .filter(item => item.status === 'active' && item.next_due_date)
  .sort((a, b) => a.days_until - b.days_until)
  .slice(0, 6))

const filtered = computed(() => items.value.filter(item => {
  if (page.value === 'subscription' && item.kind !== 'subscription') return false
  if (page.value === 'resource' && item.kind !== 'resource') return false
  if (statusFilter.value && item.status !== statusFilter.value) return false
  return item.name.toLocaleLowerCase().includes(search.value.trim().toLocaleLowerCase())
}))

const heading = computed(() => {
  if (page.value === 'subscription' || page.value === 'resource') return '全部项目'
  if (page.value === 'payments' || page.value === 'reports') return '账单'
  if (page.value === 'categories' || page.value === 'reminders') return '设置'
  return '概览'
})

function categoryFor(id: number | null): Category | undefined {
  return categories.value.find(category => category.id === id)
}

const statusLabel: Record<Status, string> = { active: '正常', paused: '已暂停', canceled: '已取消' }
const unitLabel: Record<Period, string> = { none: '一次性', day: '天', month: '月', year: '年' }

function periodLabel(item: Item): string {
  if (item.period_unit === 'none') return '一次性'
  return '每' + (item.period_count === 1 ? '' : String(item.period_count)) + unitLabel[item.period_unit]
}

function digits(currency: string): number {
  return currency === 'JPY' ? 0 : 2
}

function formatAmount(minor: number, currency: string): string {
  const scale = Math.pow(10, digits(currency))
  return new Intl.NumberFormat('zh-CN', {
    style: 'currency',
    currency,
  }).format(minor / scale)
}

function amountToMinor(value: string, currency: string): number {
  const precision = digits(currency)
  const pattern = precision === 0 ? /^\d+$/ : /^\d+(\.\d{1,2})?$/
  if (!pattern.test(value.trim())) throw new Error('金额格式不正确')
  const parts = value.trim().split('.')
  const unit = Number(parts[0])
  const fractional = Number((parts[1] || '').padEnd(precision, '0'))
  const amount = unit * Math.pow(10, precision) + fractional
  if (!Number.isSafeInteger(amount) || amount > 1000000000000) throw new Error('金额超出范围')
  return amount
}

function openCreate(kind: Kind) {
  editingID.value = null
  editor.value = blankItem(kind)
  amountText.value = '0' + (digits(editor.value.currency) === 0 ? '' : '.00')
  showEditor.value = true
  showAdvanced.value = false
  error.value = ''
}

function openEdit(item: Item) {
  editingID.value = item.id
  editor.value = {
    category_id: item.category_id,
    name: item.name,
    kind: item.kind,
    status: item.status,
    currency: item.currency,
    period_unit: item.period_unit,
    period_count: item.period_count,
    first_due_date: item.first_due_date,
    notes: item.notes,
  }
  amountText.value = (item.amount_minor / Math.pow(10, digits(item.currency)))
    .toFixed(digits(item.currency))
  showEditor.value = true
  showAdvanced.value = true
  error.value = ''
}

function openDetails(item: Item) {
  initialDetailPaymentID.value = null
  selectedItem.value = item
}

function editFromDetails(item: Item) {
  selectedItem.value = null
  initialDetailPaymentID.value = null
  openEdit(item)
}

function recordPayment(item: Item) {
  selectedItem.value = item
  initialDetailPaymentID.value = item.id
}

async function saveItem() {
  if (saving.value) return
  saving.value = true
  error.value = ''
  try {
    const amount = amountToMinor(amountText.value, editor.value.currency)
    const payload = {
      ...editor.value,
      amount_minor: amount,
      period_count: editor.value.period_unit === 'none' ? 0 : Number(editor.value.period_count),
    }
    if (editingID.value === null) {
      await api('/items', 'POST', payload)
    } else {
      await api('/items/' + editingID.value, 'PUT', payload)
    }
    showEditor.value = false
    await reload()
  } catch (reason) {
    error.value = messageOf(reason)
  } finally {
    saving.value = false
  }
}

async function deleteItem(item: Item) {
  if (!window.confirm('确定删除「' + item.name + '」？此操作不可恢复。')) return
  try {
    await api('/items/' + item.id, 'DELETE')
    await reload()
  } catch (reason) {
    error.value = messageOf(reason)
  }
}

function resetCategory() {
  editingCategoryID.value = null
  categoryName.value = ''
  categoryColor.value = '#5470e8'
}

function editCategory(category: Category) {
  editingCategoryID.value = category.id
  categoryName.value = category.name
  categoryColor.value = category.color
}

async function saveCategory() {
  error.value = ''
  try {
    const payload = { name: categoryName.value, color: categoryColor.value }
    if (editingCategoryID.value === null) {
      await api('/categories', 'POST', payload)
    } else {
      await api('/categories/' + editingCategoryID.value, 'PUT', payload)
    }
    resetCategory()
    await reload()
  } catch (reason) {
    error.value = messageOf(reason)
  }
}

async function deleteCategory(category: Category) {
  if (!window.confirm('确定删除分类「' + category.name + '」？')) return
  try {
    await api('/categories/' + category.id, 'DELETE')
    resetCategory()
    await reload()
  } catch (reason) {
    error.value = messageOf(reason)
  }
}

async function checkLogin() {
  authChecking.value = true
  try {
    const response = await fetch('/api/v1/auth/me', { credentials: 'same-origin' })
    authenticated.value = response.ok
    if (response.ok) await reload()
    else if (response.status !== 401) loginError.value = '无法检查登录状态，请检查后端连接'
  } catch {
    authenticated.value = false
    loginError.value = '无法连接后端，请检查服务是否启动'
  } finally {
    authChecking.value = false
  }
}

async function login() {
  if (loginBusy.value) return
  loginBusy.value = true
  loginError.value = ''
  try {
    const response = await fetch('/api/v1/auth/login', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': '1' },
      body: JSON.stringify({ password: loginPassword.value }),
    })
    const body = await response.json()
    if (!response.ok) throw new Error(body?.error?.message || '登录失败')
    authenticated.value = true
    loginPassword.value = ''
    await reload()
  } catch (reason) {
    loginError.value = messageOf(reason)
  } finally {
    loginBusy.value = false
  }
}

async function logout() {
  try {
    await api('/auth/logout', 'POST')
    authenticated.value = false
    items.value = []
    categories.value = []
    page.value = 'overview'
    selectedItem.value = null
    initialDetailPaymentID.value = null
    monthExpenses.value = []
  } catch (reason) {
    error.value = messageOf(reason)
  }
}

async function createBackup() {
  try {
    const backup = await api<{ file: string }>('/backups', 'POST')
    backupInfo.value = '备份成功：' + backup.file
  } catch (reason) {
    backupInfo.value = '备份失败：' + messageOf(reason)
  }
}

onMounted(checkLogin)
</script>

<template>
  <div v-if="authChecking" class="auth-shell"><div class="auth-card"><h1>Subscription Hub</h1><p>正在检查登录状态…</p></div></div>
  <div v-else-if="!authenticated" class="auth-shell">
    <form class="auth-card" @submit.prevent="login">
      <div class="auth-mark">S</div>
      <h1>Subscription Hub</h1>
      <p>请输入管理员密码，访问你的订阅与资源账本。</p>
      <label for="admin-password">管理员密码</label>
      <input id="admin-password" v-model="loginPassword" type="password"
        autocomplete="current-password" required autofocus minlength="12" maxlength="128" />
      <div v-if="loginError" role="alert" class="auth-error">{{ loginError }}</div>
      <button type="submit" :disabled="loginBusy">{{ loginBusy ? '登录中…' : '登录' }}</button>
      <small>单管理员本地部署 · 请勿使用公开访问的开发服务</small>
    </form>
  </div>
  <div v-else class="app-shell">
    <aside class="sidebar">
      <a class="brand" href="/" @click.prevent="navigate('overview')">
        <span class="brand-mark">S</span>
        <span>Subscription Hub</span>
      </a>
      <div class="nav-label">工作区</div>
      <nav class="nav-menu" aria-label="主导航">
        <button :class="['nav-link', { active: page === 'overview' }]" @click="navigate('overview')">概览</button>
        <button :class="['nav-link', { active: page === 'subscription' || page === 'resource' }]" @click="navigate('subscription')">全部项目</button>
        <button :class="['nav-link', { active: page === 'payments' || page === 'reports' }]" @click="navigate('payments')">账单</button>
        <button :class="['nav-link', { active: page === 'categories' || page === 'reminders' }]" @click="navigate('categories')">设置</button>
      </nav>
      <div class="sidebar-footer"><button class="logout-button" type="button" @click="logout">退出登录</button><span>开源项目 · MIT License</span></div>
    </aside>

    <main class="main">
      <header class="page-header">
        <div>
          <div class="eyebrow">Subscription Hub / {{ heading }}</div>
          <h1>{{ heading }}</h1>
          <p>{{ page === 'overview' ? '即将到期与续费操作，一眼掌握。' : page === 'subscription' || page === 'resource' ? '统一管理订阅与资源。' : page === 'payments' || page === 'reports' ? '查看付款记录与费用趋势。' : '配置分类、通知和数据备份。' }}</p>
        </div>
        <div class="header-actions">
          <button class="button secondary" @click="refreshPage">刷新</button>
          <button v-if="page === 'overview' || page === 'subscription' || page === 'resource'" class="button primary"
            @click="openCreate(page === 'resource' ? 'resource' : 'subscription')">
            + 新增{{ page === 'resource' ? '资源' : '订阅' }}
          </button>
        </div>
      </header>

      <div v-if="error" class="error-banner" role="alert">
        <span>{{ error }}</span>
        <button class="error-close" aria-label="关闭提示" @click="error = ''">×</button>
      </div>


      <nav v-if="page === 'subscription' || page === 'resource'" class="workspace-tabs" aria-label="项目类型">
        <button type="button" :class="{ active: page === 'subscription' }" @click="navigate('subscription')">订阅</button>
        <button type="button" :class="{ active: page === 'resource' }" @click="navigate('resource')">资源</button>
      </nav>
      <nav v-if="page === 'payments' || page === 'reports'" class="workspace-tabs" aria-label="账单功能">
        <button type="button" :class="{ active: page === 'payments' }" @click="navigate('payments')">付款记录</button>
        <button type="button" :class="{ active: page === 'reports' }" @click="navigate('reports')">费用统计</button>
      </nav>
      <nav v-if="page === 'categories' || page === 'reminders'" class="workspace-tabs" aria-label="设置功能">
        <button type="button" :class="{ active: page === 'categories' }" @click="navigate('categories')">分类与备份</button>
        <button type="button" :class="{ active: page === 'reminders' }" @click="navigate('reminders')">到期提醒</button>
      </nav>
      <LedgerWorkspace v-if="page === 'payments' || page === 'reports' || page === 'reminders'"
        :key="page + ':' + ledgerRefreshKey" :page="page" />
      <section v-if="page === 'overview'" class="metrics" aria-label="概览指标">
        <div class="metric">
          <span>活跃订阅</span>
          <strong>{{ activeSubscriptions }}</strong>
          <small>正在管理的周期性服务</small>
        </div>
        <div class="metric">
          <span>活跃资源</span>
          <strong>{{ activeResources }}</strong>
          <small>包括 VPS、域名等</small>
        </div>
        <div class="metric">
          <span>待处理账期（30天内及逾期）</span>
          <strong>{{ upcoming.length }}</strong>
          <small>需要留意的续费项目</small>
        </div>
        <div class="metric metric-urgent">
          <span>逾期或 7 天内到期</span>
          <strong>{{ urgentDue }}</strong>
          <small>包括未结算的逾期账单</small>
        </div>
      </section>

      <section v-if="page === 'overview'" class="panel expense-overview" aria-label="本月实际支出">
        <div class="panel-heading">
          <h2>本月实际支出</h2>
          <button type="button" class="expense-link" @click="navigate('reports')">查看统计</button>
        </div>
        <p class="expense-subtitle">根据已登记的付款汇总，不包含未来预计账单。</p>
        <p v-if="expenseLoading" class="expense-message">正在读取本月账单…</p>
        <p v-else-if="expenseError" class="expense-message expense-error">加载失败：{{ expenseError }} <button type="button" @click="loadMonthExpenses">重试</button></p>
        <p v-else-if="monthExpenses.length === 0" class="expense-message">本月暂无付款记录，可以从项目详情快速登记。</p>
        <div v-else class="expense-totals">
          <div v-for="total in monthExpenses" :key="total.currency" class="expense-total">
            <span>{{ total.currency }}</span>
            <strong>{{ formatAmount(total.amount_minor, total.currency) }}</strong>
            <small>{{ total.count }} 笔付款</small>
          </div>
        </div>
        <p v-if="monthExpenses.length > 1" class="expense-note">不同币种分开计算，不做未经换算的金额合计。</p>
      </section>

      <section v-if="page === 'overview'" class="content-grid">
        <article class="panel">
          <div class="panel-heading">
            <h2>待处理续费</h2>
            <span>按日期排序</span>
          </div>
          <p v-if="busy" class="empty-message">正在读取数据…</p>
          <div v-else-if="upcomingSorted.length === 0" class="empty-state">
            <div class="empty-illustration">↗</div>
            <h3>还没有续费计划</h3>
            <p>添加第一条订阅或资源，自动计算下一次到期日期。</p>
          </div>
          <div v-else class="due-list">
            <div v-for="item in upcomingSorted" :key="item.id" class="due-row">
              <div class="due-icon">{{ item.kind === 'subscription' ? 'S' : 'R' }}</div>
              <div class="due-info">
                <button type="button" class="item-name-button" @click="openDetails(item)">{{ item.name }}</button>
                <span>{{ item.next_due_date }} · {{ periodLabel(item) }}</span>
              </div>
              <div class="due-right">
                <strong>{{ formatAmount(item.amount_minor, item.currency) }}</strong>
                <span :class="{ urgent: item.days_until <= 7 }">
                  {{ item.days_until < 0 ? '逾期 ' + Math.abs(item.days_until) + ' 天' : item.days_until === 0 ? '今天到期' : item.days_until + ' 天后' }}
                </span>
                <button type="button" class="due-pay" @click="recordPayment(item)">记录付款</button>
              </div>
            </div>
          </div>
        </article>
        <article class="panel">
          <div class="panel-heading">
            <h2>快捷操作</h2>
            <span>常用功能</span>
          </div>
          <div class="quick-actions">
            <button type="button" @click="openCreate('subscription')"><strong>新增订阅</strong><small>记录下次续费日期</small></button>
            <button type="button" @click="openCreate('resource')"><strong>新增资源</strong><small>管理域名与服务器</small></button>
            <button type="button" @click="navigate('payments')"><strong>查看账单</strong><small>整理实际付款记录</small></button>
            <button type="button" @click="navigate('reminders')"><strong>到期提醒</strong><small>设置到期通知</small></button>
          </div>
          <p class="hint">付款流水仅记录实际支出；预计到期金额不自动计入账单。</p>
        </article>
      </section>

      <section v-if="page === 'subscription' || page === 'resource'" class="panel list-panel">
        <div class="list-toolbar">
          <input v-model="search" class="field search" placeholder="搜索名称…" aria-label="搜索项目" />
          <select v-model="statusFilter" class="field status-filter" aria-label="筛选状态">
            <option value="">全部状态</option>
            <option value="active">正常</option>
            <option value="paused">已暂停</option>
            <option value="canceled">已取消</option>
          </select>
          <span class="count-text">{{ filtered.length }} 条记录</span>
        </div>
        <div class="table-wrap">
          <table class="data-table">
            <thead>
              <tr>
                <th>名称</th><th>分类</th><th>价格</th><th>周期</th><th>下次到期</th><th>状态</th><th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in filtered" :key="item.id">
                <td class="cell-name"><button type="button" class="item-name-button" @click="openDetails(item)">{{ item.name }}</button></td>
                <td>
                  <span v-if="categoryFor(item.category_id)" class="category-chip">
                    <i :style="{ background: categoryFor(item.category_id)?.color }"></i>
                    {{ categoryFor(item.category_id)?.name }}
                  </span>
                  <span v-else class="muted">未分类</span>
                </td>
                <td>{{ formatAmount(item.amount_minor, item.currency) }}</td>
                <td>{{ periodLabel(item) }}</td>
                <td>
                  <span v-if="item.next_due_date">{{ item.next_due_date }}</span>
                  <span v-else class="muted">—</span>
                </td>
                <td><span class="state-pill" :class="item.status">{{ statusLabel[item.status] }}</span></td>
                <td>
                  <div class="row-actions">
                    <button v-if="item.status === 'active'" @click="recordPayment(item)">记录付款</button>
                    <button @click="openEdit(item)">编辑</button>
                    <button class="danger-text" @click="deleteItem(item)">删除</button>
                  </div>
                </td>
              </tr>
              <tr v-if="!busy && filtered.length === 0">
                <td colspan="7" class="table-empty">暂无记录，点击右上角添加。</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="mobile-item-list">
          <div v-for="item in filtered" :key="item.id" class="mobile-item-card">
            <div class="mobile-item-top">
              <div class="mobile-item-title">
                <button type="button" class="item-name-button" @click="openDetails(item)">{{ item.name }}</button>
                <span v-if="categoryFor(item.category_id)" class="category-chip">
                  <i :style="{ background: categoryFor(item.category_id)?.color }"></i>
                  {{ categoryFor(item.category_id)?.name }}
                </span>
                <span v-else class="muted">未分类</span>
              </div>
              <span class="state-pill" :class="item.status">{{ statusLabel[item.status] }}</span>
            </div>
            <div class="mobile-item-price">{{ formatAmount(item.amount_minor, item.currency) }}</div>
            <div class="mobile-item-meta">
              <span>{{ periodLabel(item) }}</span>
              <span>{{ item.renewal_completed ? '已完成结算' : item.next_due_date ? '待处理到期日 ' + item.next_due_date : '无到期日' }}</span>
            </div>
            <div class="mobile-item-actions">
              <button v-if="item.status === 'active'" type="button" @click="recordPayment(item)">记录付款</button>
              <button type="button" @click="openEdit(item)">编辑</button>
              <button type="button" class="danger-text" @click="deleteItem(item)">删除</button>
            </div>
          </div>
          <div v-if="!busy && filtered.length === 0" class="mobile-item-empty">暂无记录，点击“新增”开始记录。</div>
        </div>
      </section>

      <section v-if="page === 'categories'" class="category-layout">
        <form class="panel category-form" @submit.prevent="saveCategory">
          <div class="panel-heading"><h2>{{ editingCategoryID === null ? '新建分类' : '编辑分类' }}</h2></div>
          <label class="form-label" for="category-name">分类名称</label>
          <input id="category-name" v-model="categoryName" class="field" maxlength="40" required placeholder="例如：AI 工具" />
          <label class="form-label" for="category-color">标识颜色</label>
          <input id="category-color" v-model="categoryColor" class="color-picker" type="color" />
          <div class="form-actions">
            <button v-if="editingCategoryID !== null" class="button secondary" type="button" @click="resetCategory">取消</button>
            <button class="button primary" type="submit">{{ editingCategoryID === null ? '添加分类' : '保存修改' }}</button>
          </div>
        </form>
        <div class="panel category-list">
          <div class="panel-heading"><h2>已有分类</h2><span>{{ categories.length }} 个</span></div>
          <div v-for="category in categories" :key="category.id" class="category-row">
            <span class="category-name"><i :style="{ background: category.color }"></i>{{ category.name }}</span>
            <span class="row-actions">
              <button @click="editCategory(category)">编辑</button>
              <button class="danger-text" @click="deleteCategory(category)">删除</button>
            </span>
          </div>
          <p v-if="categories.length === 0" class="empty-message">尚未创建分类。</p>
          <p class="hint">被订阅或资源引用的分类不能删除，请先修改关联项目。</p>
        </div>
      </section>
      <section v-if="page === 'categories'" class="panel settings-backup">
        <div class="panel-heading"><h2>数据备份</h2><span>SQLite</span></div>
        <p class="hint">可手动生成数据库快照。请另外备份到独立存储设备。</p>
        <button class="button secondary" type="button" @click="createBackup">立即备份</button>
        <p v-if="backupInfo" class="backup-note">{{ backupInfo }}</p>
      </section>
      <section v-if="page === 'categories' || page === 'reminders'" class="panel settings-account">
        <div class="panel-heading"><h2>账号</h2></div>
        <button class="button secondary" type="button" @click="logout">退出登录</button>
      </section>
    </main>

    <nav class="mobile-bottom-nav" aria-label="移动端导航">
      <button type="button" :class="{ active: page === 'overview' }" @click="navigate('overview')"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="m3 10 9-7 9 7v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><path d="M9 21v-8h6v8"/></svg>概览</button>
      <button type="button" :class="{ active: page === 'subscription' || page === 'resource' }" @click="navigate('subscription')"><svg viewBox="0 0 24 24" aria-hidden="true"><rect x="3" y="4" width="18" height="16" rx="3"/><path d="M3 9h18M7 15h5"/></svg>项目</button>
      <button type="button" :class="{ active: page === 'payments' || page === 'reports' }" @click="navigate('payments')"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 3h16v18l-4-2-4 2-4-2-4 2z"/><path d="M8 8h8M8 12h8M8 16h5"/></svg>账单</button>
      <button type="button" :class="{ active: page === 'categories' || page === 'reminders' }" @click="navigate('categories')"><svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .34 1.87l.06.06-2.12 2.12-.06-.06A1.7 1.7 0 0 0 15.75 19l-.15.06V22H8.4v-2.94L8.25 19a1.7 1.7 0 0 0-1.87.34l-.06.06L4.2 17.28l.06-.06A1.7 1.7 0 0 0 4.6 15l-.06-.15H2v-5.7h2.54L4.6 9a1.7 1.7 0 0 0-.34-1.87L4.2 7.07l2.12-2.12.06.06A1.7 1.7 0 0 0 8.25 5l.15-.06V2h7.2v2.94l.15.06a1.7 1.7 0 0 0 1.87-.34l.06-.06 2.12 2.12-.06.06A1.7 1.7 0 0 0 19.4 9l.06.15H22v5.7h-2.54z"/></svg>设置</button>
    </nav>

    <ItemDetails v-if="selectedItem" :item="selectedItem" :categories="categories"
      :start-with-payment="initialDetailPaymentID === selectedItem.id"
      @close="selectedItem = null; initialDetailPaymentID = null" @edit="editFromDetails"
      @payment-created="reload" />

    <div v-if="showEditor" class="modal-backdrop" @click.self="showEditor = false">
      <form class="modal" role="dialog" aria-modal="true" :aria-label="editingID === null ? '新增记录' : '编辑记录'" @submit.prevent="saveItem">
        <div class="modal-header">
          <div>
            <h2>{{ editingID === null ? '新增' : '编辑' }}{{ editor.kind === 'subscription' ? '订阅' : '资源' }}</h2>
            <p>记录费用、状态及下一次到期规则。</p>
          </div>
          <button class="close-button" type="button" aria-label="关闭编辑器" @click="showEditor = false">×</button>
        </div>
        <div class="form-grid">
          <label class="form-group wide">
            <span>名称 *</span>
            <input v-model="editor.name" class="field" required maxlength="100" placeholder="例如：ChatGPT Plus" />
          </label>
          <label class="form-group">
            <span>金额 *</span>
            <input v-model="amountText" class="field" required inputmode="decimal" placeholder="20.00" />
          </label>
          <label class="form-group">
            <span>币种</span>
            <select v-model="editor.currency" class="field">
              <option value="CNY">CNY · 人民币</option>
              <option value="USD">USD · 美元</option>
              <option value="HKD">HKD · 港元</option>
              <option value="EUR">EUR · 欧元</option>
              <option value="JPY">JPY · 日元</option>
            </select>
          </label>
          <label class="form-group">
            <span>计费周期</span>
            <select v-model="editor.period_unit" class="field">
              <option v-if="editor.kind === 'resource'" value="none">一次性</option>
              <option value="day">按天</option>
              <option value="month">按月</option>
              <option value="year">按年</option>
            </select>
          </label>
          <label class="form-group">
            <span>{{ editor.period_unit === 'none' ? '到期日期' : '起始续费日期' }} *</span>
            <input v-model="editor.first_due_date" class="field" type="date" required />
          </label>
        </div>
        <p class="form-hint">系统将根据起始日期和计费周期推算后续到期日。</p>
        <button type="button" class="advanced-toggle" :aria-expanded="showAdvanced" @click="showAdvanced = !showAdvanced">
          {{ showAdvanced ? '收起更多选项 −' : '更多选项（分类、状态、备注等） +' }}
        </button>
        <div v-if="showAdvanced" class="form-grid advanced-fields">
          <label class="form-group">
            <span>项目类型</span>
            <select v-model="editor.kind" class="field" @change="editor.period_unit = editor.kind === 'subscription' ? 'month' : 'year'">
              <option value="subscription">订阅</option>
              <option value="resource">资源</option>
            </select>
          </label>
          <label class="form-group">
            <span>分类</span>
            <select v-model="editor.category_id" class="field">
              <option :value="null">未分类</option>
              <option v-for="category in categories" :key="category.id" :value="category.id">{{ category.name }}</option>
            </select>
          </label>
          <label class="form-group">
            <span>每几个周期</span>
            <input v-model.number="editor.period_count" class="field" type="number"
              min="1" max="365" :disabled="editor.period_unit === 'none'" />
          </label>
          <label class="form-group">
            <span>状态</span>
            <select v-model="editor.status" class="field">
              <option value="active">正常</option>
              <option value="paused">已暂停</option>
              <option value="canceled">已取消</option>
            </select>
          </label>
          <label class="form-group wide">
            <span>备注</span>
            <textarea v-model="editor.notes" class="field" rows="3" maxlength="500" placeholder="可选"></textarea>
          </label>
        </div>
        <div v-if="error" class="form-error" role="alert">{{ error }}</div>
        <div class="modal-footer">
          <button class="button secondary" type="button" @click="showEditor = false">取消</button>
          <button class="button primary" type="submit" :disabled="saving">{{ saving ? '保存中…' : '保存' }}</button>
        </div>
      </form>
    </div>
  </div>
</template>
