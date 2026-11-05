<script setup lang="ts">
import { computed, ref, watch } from 'vue'

type Page = 'payments' | 'reports' | 'reminders'
interface Item {
  id: number
  name: string
  currency: string
  amount_minor: number
  status: string
}
interface Payment {
  id: number
  item_id: number
  item_name: string
  amount_minor: number
  currency: string
  paid_date: string
  notes: string
}
interface Total { currency: string; amount_minor: number; count: number }
interface Trend { month: string; currency: string; amount_minor: number; count: number }
interface Report { month: string; totals: Total[]; trend: Trend[] }
interface Rule { id: number; item_id: number; item_name: string; days_before: number; enabled: boolean }
interface Notice {
  id: number
  item_name: string
  due_date: string
  days_before: number
  read_at: string | null
  created_at: string
}

const props = defineProps<{ page: Page }>()
const items = ref<Item[]>([])
const payments = ref<Payment[]>([])
const rules = ref<Rule[]>([])
const notices = ref<Notice[]>([])
const unreadCount = ref(0)
const report = ref<Report>({ month: '', totals: [], trend: [] })
const busy = ref(false)
const saving = ref(false)
const error = ref('')
const showPaymentForm = ref(false)
const editingPaymentId = ref<number | null>(null)
const paidMonth = ref('')
const reportMonth = ref(new Date().toLocaleDateString('sv-SE').slice(0, 7))
const showUnreadOnly = ref(false)

function today(): string {
  const d = new Date()
  return [d.getFullYear(), String(d.getMonth() + 1).padStart(2, '0'), String(d.getDate()).padStart(2, '0')].join('-')
}
const form = ref({ item_id: 0, amount: '', currency: 'CNY', paid_date: today(), notes: '' })
const ruleForm = ref({ item_id: 0, days_before: 7, enabled: true })

function digits(currency: string) { return currency === 'JPY' ? 0 : 2 }
function formatAmount(minor: number, currency: string) {
  return new Intl.NumberFormat('zh-CN', { style: 'currency', currency }).format(minor / Math.pow(10, digits(currency)))
}
function toMinor(amount: string, currency: string): number {
  const n = digits(currency)
  const re = n === 0 ? /^\d+$/ : /^\d+(\.\d{1,2})?$/
  if (!re.test(amount)) throw new Error('请输入正确的金额')
  const [whole, fraction = ''] = amount.split('.')
  const minor = Number(whole) * Math.pow(10, n) + Number(fraction.padEnd(n, '0'))
  if (!Number.isSafeInteger(minor) || minor < 1 || minor > 1000000000000) throw new Error('金额超出允许范围')
  return minor
}
function messageOf(reason: unknown) { return reason instanceof Error ? reason.message : '操作失败' }
async function api<T>(path: string, method = 'GET', body?: unknown): Promise<T> {
  const response = await fetch('/api/v1' + path, {
    method,
    headers: {
      ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
      ...(method === 'GET' ? {} : { 'X-CSRF-Token': '1' }),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (response.status === 204) return undefined as T
  if (response.status === 401) window.location.reload()
  const data = await response.json()
  if (!response.ok) throw new Error(data?.error?.message || '请求失败')
  return data as T
}

async function load() {
  busy.value = true
  error.value = ''
  try {
    const catalog = await api<{ items: Item[] }>('/items?limit=500')
    items.value = catalog.items
    if (props.page === 'payments') {
      const q = paidMonth.value ? '&month=' + paidMonth.value : ''
      payments.value = (await api<{ items: Payment[] }>('/payments?limit=500' + q)).items
    } else if (props.page === 'reports') {
      report.value = await api<Report>('/reports/expenses?month=' + reportMonth.value)
    } else {
      const [ruleData, noticeData] = await Promise.all([
        api<{ items: Rule[] }>('/reminder-rules'),
        api<{ items: Notice[]; unread_count: number }>('/notifications?limit=500' +
          (showUnreadOnly.value ? '&unread=true' : '')),
      ])
      rules.value = ruleData.items
      notices.value = noticeData.items
      unreadCount.value = noticeData.unread_count
    }
  } catch (reason) {
    error.value = messageOf(reason)
  } finally {
    busy.value = false
  }
}
watch(() => props.page, load, { immediate: true })

const activeItems = computed(() => items.value.filter(i => i.status === 'active'))
const reportCurrencies = computed(() => Array.from(new Set(report.value.trend.map(t => t.currency))).sort())
const reportMonths = computed(() => {
  const date = new Date(reportMonth.value + '-01T00:00:00')
  if (Number.isNaN(date.getTime())) return []
  return Array.from({ length: 6 }, (_, i) => {
    const d = new Date(date.getFullYear(), date.getMonth() - 5 + i, 1)
    return [d.getFullYear(), String(d.getMonth() + 1).padStart(2, '0')].join('-')
  })
})
function monthlyAmount(month: string, currency: string) {
  return report.value.trend.find(t => t.month === month && t.currency === currency)?.amount_minor ?? 0
}

function resetForm() {
  editingPaymentId.value = null
  form.value = { item_id: 0, amount: '', currency: 'CNY', paid_date: today(), notes: '' }
  showPaymentForm.value = true
}
function choosePaymentItem() {
  const selected = items.value.find(i => i.id === form.value.item_id)
  if (!selected) return
  form.value.currency = selected.currency
  form.value.amount = (selected.amount_minor / Math.pow(10, digits(selected.currency))).toFixed(digits(selected.currency))
}
function editPayment(payment: Payment) {
  editingPaymentId.value = payment.id
  form.value = {
    item_id: payment.item_id,
    amount: (payment.amount_minor / Math.pow(10, digits(payment.currency))).toFixed(digits(payment.currency)),
    currency: payment.currency,
    paid_date: payment.paid_date,
    notes: payment.notes,
  }
  showPaymentForm.value = true
}
async function savePayment() {
  if (saving.value) return
  saving.value = true
  error.value = ''
  try {
    const payload = {
      item_id: Number(form.value.item_id),
      amount_minor: toMinor(form.value.amount.trim(), form.value.currency),
      currency: form.value.currency,
      paid_date: form.value.paid_date,
      notes: form.value.notes,
    }
    if (editingPaymentId.value === null) await api('/payments', 'POST', payload)
    else await api('/payments/' + editingPaymentId.value, 'PUT', payload)
    showPaymentForm.value = false
    await load()
  } catch (reason) {
    error.value = messageOf(reason)
  } finally {
    saving.value = false
  }
}
async function deletePayment(payment: Payment) {
  if (!window.confirm('确定删除「' + payment.item_name + '」的这笔付款记录？')) return
  try {
    await api('/payments/' + payment.id, 'DELETE')
    await load()
  } catch (reason) { error.value = messageOf(reason) }
}
async function createRule() {
  if (saving.value) return
  saving.value = true
  error.value = ''
  try {
    await api('/reminder-rules', 'POST', {
      item_id: Number(ruleForm.value.item_id),
      days_before: Number(ruleForm.value.days_before),
      enabled: true,
    })
    ruleForm.value = { item_id: 0, days_before: 7, enabled: true }
    await scan()
  } catch (reason) {
    error.value = messageOf(reason)
  } finally { saving.value = false }
}
async function toggleRule(rule: Rule) {
  try {
    await api('/reminder-rules/' + rule.id, 'PUT', {
      days_before: rule.days_before,
      enabled: !rule.enabled,
    })
    await load()
  } catch (reason) { error.value = messageOf(reason) }
}
async function deleteRule(rule: Rule) {
  if (!window.confirm('删除「' + rule.item_name + '」提前 ' + rule.days_before + ' 天的提醒规则？')) return
  try {
    await api('/reminder-rules/' + rule.id, 'DELETE')
    await load()
  } catch (reason) { error.value = messageOf(reason) }
}
async function markRead(notice: Notice) {
  try {
    await api('/notifications/' + notice.id + '/read', 'POST')
    await load()
  } catch (reason) { error.value = messageOf(reason) }
}
async function scan() {
  try {
    await api('/notifications/scan', 'POST')
    await load()
  } catch (reason) { error.value = messageOf(reason) }
}
</script>

<template>
  <div class="ledger-workspace">
    <div v-if="error" class="ledger-error" role="alert">
      {{ error }} <button type="button" @click="error = ''" aria-label="关闭错误提示">×</button>
    </div>

    <template v-if="page === 'payments'">
      <section class="ledger-panel">
        <div class="ledger-panel-head">
          <div><h2>付款流水</h2><p>只记录已实际发生的付款，不自动把预计续费计入支出。</p></div>
          <button class="ledger-primary" type="button" @click="resetForm">+ 记录付款</button>
        </div>
        <div class="ledger-filters">
          <label>付款月份 <input v-model="paidMonth" type="month" @change="load" /></label>
          <button type="button" @click="paidMonth = ''; load()">全部月份</button>
          <span>最多显示 500 条</span>
        </div>
        <div class="ledger-table-scroll">
          <table class="ledger-table">
            <thead><tr><th>付款日期</th><th>项目</th><th>实付金额</th><th>备注</th><th>操作</th></tr></thead>
            <tbody>
              <tr v-for="payment in payments" :key="payment.id">
                <td>{{ payment.paid_date }}</td>
                <td class="ledger-strong">{{ payment.item_name }}</td>
                <td>{{ formatAmount(payment.amount_minor, payment.currency) }}</td>
                <td class="ledger-muted">{{ payment.notes || '—' }}</td>
                <td><div class="ledger-actions">
                  <button type="button" @click="editPayment(payment)">编辑</button>
                  <button class="ledger-danger" type="button" @click="deletePayment(payment)">删除</button>
                </div></td>
              </tr>
              <tr v-if="!busy && payments.length === 0"><td class="ledger-empty" colspan="5">还没有实际付款记录</td></tr>
            </tbody>
          </table>
        </div>
      </section>
    </template>

    <template v-else-if="page === 'reports'">
      <section class="ledger-panel">
        <div class="ledger-panel-head"><div><h2>实际费用统计</h2><p>按币种分别汇总，不做不准确的跨币种求和。</p></div>
          <label class="ledger-month">统计月份 <input v-model="reportMonth" type="month" @change="load" /></label>
        </div>
        <div v-if="!busy && report.totals.length === 0" class="ledger-empty">当前月份暂无实付账单</div>
        <div class="ledger-summary">
          <div v-for="total in report.totals" :key="total.currency" class="ledger-summary-card">
            <span>{{ total.currency }} · 当月支出</span>
            <strong>{{ formatAmount(total.amount_minor, total.currency) }}</strong>
            <small>{{ total.count }} 笔真实付款</small>
          </div>
        </div>
      </section>
      <section class="ledger-panel">
        <div class="ledger-panel-head"><div><h2>近六个月趋势</h2><p>金额均为实际支付记录，暂无汇率换算。</p></div></div>
        <div class="ledger-table-scroll">
          <table class="ledger-table">
            <thead><tr><th>月份</th><th v-for="currency in reportCurrencies" :key="currency">{{ currency }}</th></tr></thead>
            <tbody>
              <tr v-for="month in reportMonths" :key="month">
                <td class="ledger-strong">{{ month }}</td>
                <td v-for="currency in reportCurrencies" :key="currency">{{ formatAmount(monthlyAmount(month, currency), currency) }}</td>
              </tr>
              <tr v-if="reportCurrencies.length === 0"><td class="ledger-empty">暂无统计数据</td></tr>
            </tbody>
          </table>
        </div>
      </section>
    </template>

    <template v-else>
      <section class="ledger-panel">
        <div class="ledger-panel-head"><div><h2>到期提醒规则</h2><p>到期前 0–30 天生成站内提醒，每条规则每个到期日只生成一条。</p></div>
          <button type="button" class="ledger-secondary" @click="scan">立即检查</button>
        </div>
        <form class="ledger-rule-form" @submit.prevent="createRule">
          <label>选择项目
            <select v-model.number="ruleForm.item_id" required>
              <option :value="0" disabled>请选择订阅或资源</option>
              <option v-for="item in activeItems" :key="item.id" :value="item.id">{{ item.name }}</option>
            </select>
          </label>
          <label>提前天数
            <input v-model.number="ruleForm.days_before" type="number" min="0" max="30" required />
          </label>
          <button class="ledger-primary" type="submit" :disabled="saving">添加规则</button>
        </form>
        <div class="ledger-rule-list">
          <div v-for="rule in rules" :key="rule.id" class="ledger-rule-row">
            <div><strong>{{ rule.item_name }}</strong><span>到期前 {{ rule.days_before }} 天提醒</span></div>
            <span class="ledger-rule-state" :class="{ off: !rule.enabled }">{{ rule.enabled ? '已启用' : '已暂停' }}</span>
            <div class="ledger-actions">
              <button type="button" @click="toggleRule(rule)">{{ rule.enabled ? '暂停' : '启用' }}</button>
              <button type="button" class="ledger-danger" @click="deleteRule(rule)">删除</button>
            </div>
          </div>
          <p v-if="rules.length === 0 && !busy" class="ledger-empty">尚未添加提醒规则</p>
        </div>
      </section>
      <section class="ledger-panel">
        <div class="ledger-panel-head"><div><h2>站内提醒</h2><p>未读 {{ unreadCount }} 条 · 应用启动时及之后每小时自动检查。</p></div>
          <label class="ledger-checkbox"><input v-model="showUnreadOnly" type="checkbox" @change="load" />仅看未读</label>
        </div>
        <div class="ledger-rule-list">
          <div v-for="notice in notices" :key="notice.id" class="ledger-rule-row">
            <div><strong>{{ notice.item_name }}</strong>
              <span>到期日 {{ notice.due_date }} · 提前 {{ notice.days_before }} 天</span>
            </div>
            <span v-if="!notice.read_at" class="ledger-rule-state">未读</span>
            <span v-else class="ledger-muted">已读</span>
            <button v-if="!notice.read_at" type="button" class="ledger-secondary" @click="markRead(notice)">标记已读</button>
          </div>
          <p v-if="notices.length === 0 && !busy" class="ledger-empty">没有站内提醒</p>
        </div>
        <p class="ledger-note">当前为站内消息，需打开系统查看。邮件、Bark、Telegram 等主动推送将在后续版本支持。</p>
      </section>
    </template>

    <div v-if="showPaymentForm" class="ledger-overlay" @click.self="showPaymentForm = false">
      <form class="ledger-modal" @submit.prevent="savePayment">
        <div class="ledger-modal-head"><div><h2>{{ editingPaymentId === null ? '记录付款' : '编辑付款' }}</h2>
          <p>请填写实际支付的日期与金额。</p></div>
          <button type="button" aria-label="关闭" @click="showPaymentForm = false">×</button>
        </div>
        <div class="ledger-form-grid">
          <label class="wide">所属项目
            <select v-model.number="form.item_id" required @change="choosePaymentItem">
              <option :value="0" disabled>请选择项目</option>
              <option v-for="item in items" :key="item.id" :value="item.id">{{ item.name }}</option>
            </select>
          </label>
          <label>实付金额<input v-model="form.amount" required inputmode="decimal" placeholder="0.00" /></label>
          <label>币种
            <select v-model="form.currency">
              <option v-for="currency in ['CNY', 'USD', 'HKD', 'EUR', 'JPY']" :key="currency" :value="currency">{{ currency }}</option>
            </select>
          </label>
          <label class="wide">付款日期<input v-model="form.paid_date" type="date" :max="today()" required /></label>
          <label class="wide">备注<textarea v-model="form.notes" maxlength="500" rows="3"></textarea></label>
        </div>
        <div class="ledger-modal-footer">
          <button class="ledger-secondary" type="button" @click="showPaymentForm = false">取消</button>
          <button class="ledger-primary" type="submit" :disabled="saving">{{ saving ? '保存中…' : '保存' }}</button>
        </div>
      </form>
    </div>
  </div>
</template>
