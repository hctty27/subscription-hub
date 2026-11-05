<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'

type Status = 'active' | 'paused' | 'canceled'
type Period = 'none' | 'day' | 'month' | 'year'
type Kind = 'subscription' | 'resource'

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
interface Category { id: number; name: string; color: string }
interface Renewal {
  id: number
  payment_id: number
  due_date: string
  next_due_date: string
}
interface Payment {
  id: number
  item_id: number
  amount_minor: number
  currency: string
  paid_date: string
  notes: string
}

const props = defineProps<{ item: Item; categories: Category[]; startWithPayment?: boolean }>()
const emit = defineEmits<{
  (event: 'close'): void
  (event: 'edit', item: Item): void
  (event: 'payment-created'): void
}>()

const dialog = ref<HTMLElement | null>(null)
const payments = ref<Payment[]>([])
const renewals = ref<Renewal[]>([])
const settleCurrentCycle = ref(false)
const loading = ref(false)
const saving = ref(false)
const historyError = ref('')
const formError = ref('')
const showPaymentForm = ref(false)
const amountText = ref('')
const paymentDate = ref('')
const paymentNotes = ref('')

function todayString(): string {
  const date = new Date()
  return [date.getFullYear(), String(date.getMonth() + 1).padStart(2, '0'), String(date.getDate()).padStart(2, '0')].join('-')
}

function digits(currency: string): number { return currency === 'JPY' ? 0 : 2 }
function formatAmount(minor: number, currency: string): string {
  return new Intl.NumberFormat('zh-CN', { style: 'currency', currency }).format(minor / 10 ** digits(currency))
}
function toMinor(text: string, currency: string): number {
  const precision = digits(currency)
  const pattern = precision === 0 ? /^\d+$/ : /^\d+(\.\d{1,2})?$/
  if (!pattern.test(text.trim())) throw new Error('请输入正确的金额')
  const [whole, decimals = ''] = text.trim().split('.')
  const value = Number(whole) * 10 ** precision + Number(decimals.padEnd(precision, '0'))
  if (!Number.isSafeInteger(value) || value < 1 || value > 1000000000000) throw new Error('付款金额超出范围')
  return value
}
function messageOf(reason: unknown): string {
  return reason instanceof Error ? reason.message : '操作失败'
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
  if (response.status === 401) throw new Error('登录状态已过期，请刷新页面重新登录')
  const data = await response.json()
  if (!response.ok) throw new Error(data?.error?.message || '请求失败')
  return data as T
}
async function loadPayments(): Promise<void> {
  loading.value = true
  historyError.value = ''
  try {
    const [paymentData, renewalData] = await Promise.all([
      api<{ items: Payment[] }>('/payments?item_id=' + props.item.id + '&limit=100'),
      api<{ items: Renewal[] }>('/items/' + props.item.id + '/renewals'),
    ])
    payments.value = paymentData.items
    renewals.value = renewalData.items
  } catch (reason) {
    historyError.value = messageOf(reason)
  } finally {
    loading.value = false
  }
}
watch(() => props.item.id, loadPayments, { immediate: true })

const category = computed(() => props.categories.find(entry => entry.id === props.item.category_id))
const statusLabel = computed(() => ({
  active: '正常',
  paused: '已暂停',
  canceled: '已取消',
}[props.item.status]))
const renewalByPayment = computed(() => new Map(renewals.value.map(entry => [entry.payment_id, entry.due_date])))
const canSettle = computed(() => props.item.status === 'active' && !!props.item.next_due_date &&
  !props.item.renewal_completed)
const recentPayments = computed(() => [...payments.value]
  .sort((a, b) => b.paid_date.localeCompare(a.paid_date) || b.id - a.id)
  .slice(0, 8))
const periodLabel = computed(() => {
  const unit = { none: '一次性', day: '天', month: '月', year: '年' }[props.item.period_unit]
  if (props.item.period_unit === 'none') return unit
  return '每' + (props.item.period_count === 1 ? '' : props.item.period_count) + unit
})

function beginPayment(): void {
  formError.value = ''
  amountText.value = (props.item.amount_minor / 10 ** digits(props.item.currency)).toFixed(digits(props.item.currency))
  paymentDate.value = todayString()
  paymentNotes.value = ''
  settleCurrentCycle.value = canSettle.value
  showPaymentForm.value = true
}
watch(() => props.startWithPayment, enabled => {
  if (enabled) beginPayment()
}, { immediate: true })

async function savePayment(): Promise<void> {
  if (saving.value) return
  saving.value = true
  formError.value = ''
  try {
    const payment = {
      amount_minor: toMinor(amountText.value, props.item.currency),
      currency: props.item.currency,
      paid_date: paymentDate.value,
      notes: paymentNotes.value,
    }
    if (settleCurrentCycle.value && canSettle.value) {
      await api('/items/' + props.item.id + '/renew', 'POST', {
        due_date: props.item.next_due_date, ...payment,
      })
    } else {
      await api('/payments', 'POST', { item_id: props.item.id, ...payment })
    }
    showPaymentForm.value = false
    emit('payment-created')
    await loadPayments()
  } catch (reason) {
    formError.value = messageOf(reason)
  } finally {
    saving.value = false
  }
}
onMounted(() => dialog.value?.focus())

function close(): void {
  if (saving.value) return
  if (showPaymentForm.value) {
    showPaymentForm.value = false
    formError.value = ''
  } else {
    emit('close')
  }
}
</script>

<template>
  <div class="detail-backdrop" @click.self="close" @keydown.esc="close">
    <section ref="dialog" class="detail-dialog" tabindex="-1" role="dialog" aria-modal="true" :aria-label="item.name + ' 项目详情'">
      <div class="detail-header">
        <div>
          <span class="detail-eyebrow">{{ item.kind === 'subscription' ? '订阅详情' : '资源详情' }}</span>
          <h2>{{ item.name }}</h2>
          <span class="state-pill" :class="item.status">{{ statusLabel }}</span>
        </div>
        <button type="button" class="detail-close" aria-label="关闭详情" @click="close">×</button>
      </div>

      <template v-if="!showPaymentForm">
        <div class="detail-price">{{ formatAmount(item.amount_minor, item.currency) }} <small>/ {{ periodLabel }}</small></div>
        <div class="detail-facts">
          <div><span>当前待处理账期</span><strong>{{ item.renewal_completed ? '已结算' : item.next_due_date || '无续费计划' }}</strong></div>
          <div><span>起始续费日期</span><strong>{{ item.first_due_date }}</strong></div>
          <div><span>分类</span><strong>{{ category?.name || '未分类' }}</strong></div>
          <div><span>账期状态</span><strong>{{ item.renewal_completed ? '已完成' : item.status !== 'active' ? '已暂停处理' : item.days_until < 0 ? '逾期 ' + Math.abs(item.days_until) + ' 天' : item.days_until === 0 ? '今天到期' : '待续期 · ' + item.days_until + ' 天后' }}</strong></div>
        </div>
        <div v-if="item.notes" class="detail-notes"><strong>备注</strong><p>{{ item.notes }}</p></div>
        <div class="detail-actions">
          <button type="button" class="button primary" @click="beginPayment">记录付款</button>
          <button type="button" class="button secondary" @click="emit('edit', item)">编辑项目</button>
        </div>
        <div class="detail-history-head">
          <h3>付款记录</h3>
          <span>只统计真实付款</span>
        </div>
        <p v-if="loading" class="detail-message">正在加载付款记录…</p>
        <p v-else-if="historyError" class="detail-message detail-error">{{ historyError }} <button type="button" @click="loadPayments">重试</button></p>
        <p v-else-if="payments.length === 0" class="detail-message">暂无付款记录</p>
        <div v-else class="detail-history">
          <div v-for="payment in recentPayments" :key="payment.id" class="detail-history-row">
            <div><strong>{{ payment.paid_date }}</strong>
              <small v-if="renewalByPayment.has(payment.id)" class="renewal-payment-label">已结算账期 {{ renewalByPayment.get(payment.id) }}</small>
              <small v-if="payment.notes">{{ payment.notes }}</small>
            </div>
            <strong>{{ formatAmount(payment.amount_minor, payment.currency) }}</strong>
          </div>
          <p v-if="payments.length > 8" class="detail-message">只显示最近 8 笔，可到「账单」查看全部记录。</p>
        </div>
      </template>

      <form v-else class="detail-payment-form" @submit.prevent="savePayment">
        <h3>记录付款</h3>
        <p>为「{{ item.name }}」登记实际支出，金额可修改。</p>
        <label>实付金额
          <input v-model="amountText" class="field" inputmode="decimal" required />
        </label>
        <label>币种
          <input class="field" :value="item.currency" disabled />
        </label>
        <label>付款日期
          <input v-model="paymentDate" class="field" type="date" :max="todayString()" required />
        </label>
        <label>备注
          <textarea v-model="paymentNotes" class="field" rows="3" maxlength="500" placeholder="可选"></textarea>
        </label>
        <label v-if="canSettle" class="renewal-settle-option">
          <input v-model="settleCurrentCycle" type="checkbox" />
          <span>
            <strong>同时确认本期续期</strong>
            <small>结算到期日 {{ item.next_due_date }}，完成后推进到下一账期。</small>
          </span>
        </label>
        <p class="detail-message">{{ settleCurrentCycle && canSettle ? '付款与账期结算会同步保存，避免重复续期。' : '仅记录实际付款，不改变当前待处理账期。' }}</p>
        <p v-if="formError" class="detail-message detail-error" role="alert">{{ formError }}</p>
        <div class="detail-actions">
          <button class="button secondary" type="button" @click="close" :disabled="saving">返回详情</button>
          <button class="button primary" type="submit" :disabled="saving">{{ saving ? '保存中…' : settleCurrentCycle && canSettle ? '付款并确认续期' : '记录付款' }}</button>
        </div>
      </form>
    </section>
  </div>
</template>
