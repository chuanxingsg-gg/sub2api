<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl space-y-6">
      <div class="flex flex-wrap items-end justify-between gap-4">
        <div><p class="text-sm font-medium text-primary-600">新流量时代 · 客户服务</p><h1 class="mt-2 text-3xl font-bold text-gray-900 dark:text-white">运营工作台</h1><p class="mt-2 text-sm text-gray-500">管理客户、记录充值、开通与续期共享月套餐。</p></div>
        <button class="btn btn-primary" @click="open('create')">＋ 创建客户</button>
      </div>
      <div v-if="success" role="status" class="rounded-xl bg-green-50 p-4 text-sm text-green-800 dark:bg-green-950 dark:text-green-200">{{ success }}</div>
      <div v-if="error" role="alert" class="rounded-xl bg-red-50 p-4 text-sm text-red-700 dark:bg-red-950 dark:text-red-200">{{ error }}</div>
      <OperationsPlans @updated="updateCatalog" />
      <section class="overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
        <form class="flex flex-wrap items-center gap-3 border-b border-gray-100 p-5 dark:border-dark-700" @submit.prevent="searchCustomers">
          <label class="sr-only" for="customer-search">搜索客户</label><input id="customer-search" v-model="search" class="input max-w-sm" placeholder="搜索客户邮箱或名称" />
          <button class="btn btn-secondary" :disabled="loading">{{ loading ? '加载中…' : '搜索' }}</button><span class="ml-auto text-sm text-gray-500">共 {{ total }} 位客户</span>
        </form>
        <div class="overflow-x-auto">
          <table class="w-full text-left text-sm">
            <thead class="bg-gray-50 text-gray-500 dark:bg-dark-800"><tr><th class="px-5 py-3">客户</th><th class="px-5 py-3">余额</th><th class="px-5 py-3">状态／并发</th><th class="px-5 py-3">操作</th></tr></thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="customer in customers" :key="customer.id" :class="selected?.id === customer.id ? 'bg-primary-50 dark:bg-dark-800' : ''">
                <td class="px-5 py-4"><div class="font-medium text-gray-900 dark:text-white">{{ customer.username || '未设置名称' }}</div><div class="mt-1 text-gray-500">{{ customer.email }}</div><div v-if="customer.notes" class="mt-1 max-w-xs truncate text-xs text-gray-400" :title="customer.notes">{{ customer.notes }}</div></td>
                <td class="px-5 py-4 font-semibold">¥{{ money(customer.balance) }}</td>
                <td class="px-5 py-4"><span :class="customer.status === 'active' ? 'text-green-600' : 'text-gray-400'">{{ customer.status === 'active' ? '启用' : '停用' }}</span><div class="mt-1 text-gray-500">{{ customer.concurrency }} 路并发</div></td>
                <td class="px-5 py-4"><div class="flex flex-wrap gap-2"><button class="btn btn-secondary" @click="open('edit', customer)">编辑</button><button class="btn btn-secondary" @click="open('recharge', customer)">充值</button><button class="btn btn-secondary" @click="showSubscriptions(customer)">套餐</button></div></td>
              </tr>
              <tr v-if="!customers.length"><td colspan="4" class="p-10 text-center text-gray-400">{{ loading ? '正在加载客户…' : '没有找到客户' }}</td></tr>
            </tbody>
          </table>
        </div>
        <div class="flex items-center justify-end gap-3 border-t border-gray-100 p-4 dark:border-dark-700"><button class="btn btn-secondary" :disabled="page <= 1 || loading" @click="changePage(-1)">上一页</button><span class="text-sm text-gray-500">第 {{ page }} 页</span><button class="btn btn-secondary" :disabled="page * 20 >= total || loading" @click="changePage(1)">下一页</button></div>
      </section>
      <section v-if="selected" class="rounded-2xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-900">
        <div class="mb-5 flex flex-wrap items-center justify-between gap-3"><div><h2 class="text-lg font-semibold">客户套餐</h2><p class="mt-1 text-sm text-gray-500">{{ selected.email }}</p></div><button class="btn btn-primary" :disabled="subscriptionLoading || !groups.length" @click="open('assign', selected)">开通套餐</button></div>
        <p v-if="subscriptionLoading" class="py-5 text-gray-400">正在读取套餐…</p>
        <div v-else-if="!subscriptions.length" class="py-5 text-gray-400">该客户尚无套餐，可选择基础版、标准版或高级版。</div>
        <div v-else class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          <article v-for="subscription in subscriptions" :key="subscription.id" class="rounded-xl border border-gray-100 p-4 dark:border-dark-700">
            <div class="flex items-center justify-between"><h3 class="font-semibold">{{ planName(subscription.group_name) }}</h3><span class="text-xs text-gray-500">{{ subscription.status === 'active' ? '有效' : subscription.status === 'expired' ? '已到期' : subscription.status }}</span></div>
            <p class="mt-3 text-sm text-gray-500">到期：{{ date(subscription.expires_at) }}</p><p class="mt-2 text-xs text-gray-400">日／周／周期已用：{{ money(subscription.daily_usage) }} ／ {{ money(subscription.weekly_usage) }} ／ {{ money(subscription.monthly_usage) }}</p>
            <button v-if="groups.some(group => group.id === subscription.group_id)" class="btn btn-secondary mt-4" @click="open('extend', selected, subscription)">续期</button>
          </article>
        </div>
        <p class="mt-5 text-xs text-gray-400">套餐与钱包分别计费。续期只延长有效期，不重置已用额度。高级版客户的并发需在“编辑”中设置为 2。</p>
      </section>
    </div>
    <BaseDialog :show="action !== null" :title="titles[action || 'create']" @close="close">
      <form id="operations-form" class="space-y-4" @submit.prevent="submit">
        <p v-if="target" class="rounded-lg bg-gray-50 p-3 text-sm dark:bg-dark-800">客户：<strong>{{ target.email }}</strong></p>
        <template v-if="action === 'create'">
          <label class="block"><span class="input-label">客户邮箱</span><input v-model="form.email" type="email" class="input" required maxlength="254" autocomplete="off" /></label>
          <label class="block"><span class="input-label">初始密码</span><input v-model="form.password" type="password" class="input" required minlength="15" maxlength="72" autocomplete="new-password" /></label>
        </template>
        <template v-if="action === 'create' || action === 'edit'">
          <label class="block"><span class="input-label">客户名称</span><input v-model="form.username" class="input" maxlength="100" /></label>
          <label v-if="action === 'edit'" class="block"><span class="input-label">状态</span><select v-model="form.status" class="input"><option value="active">启用</option><option value="disabled">停用</option></select></label>
          <label v-if="action === 'edit'" class="block"><span class="input-label">客户并发</span><select v-model.number="form.concurrency" class="input"><option :value="1">1 路（按量／基础版／标准版）</option><option :value="2">2 路（高级版）</option></select></label>
        </template>
        <template v-if="action === 'recharge'">
          <p class="text-sm text-gray-500">当前余额 ¥{{ money(target?.balance || 0) }}；实收 ¥1 增加 1 余额单位。</p>
          <label class="block"><span class="input-label">实收充值金额（元）</span><input v-model.number="form.amount" type="number" class="input" required min="0.01" max="1000000" step="0.01" /></label>
          <p class="text-sm">确认后余额增加 <strong>¥{{ money(Number(form.amount) || 0) }}</strong>。</p>
        </template>
        <template v-if="action === 'assign'">
          <label class="block"><span class="input-label">共享月套餐</span><select v-model.number="form.plan_id" class="input" required><option v-for="group in catalogPlans.filter(p=>p.for_sale && p.ready)" :key="group.id" :value="group.id">{{ group.name }} · ¥{{ group.price }}／{{ group.validity_days }}天</option></select></label>
          <p class="text-sm text-gray-500">确认收款后开通。套餐费用不应同时充入钱包。</p>
        </template>
        <label v-if="action === 'extend'" class="block"><span class="input-label">{{ action === 'extend' ? '增加天数' : '有效天数' }}</span><input v-model.number="form.days" type="number" class="input" required min="1" max="365" step="1" /></label>
        <label v-if="action !== 'extend'" class="block"><span class="input-label">{{ action === 'recharge' || action === 'assign' ? '收款备注（必填）' : '客户备注' }}</span><textarea v-model="form.notes" class="input" rows="3" :required="action === 'recharge' || action === 'assign'" maxlength="600" placeholder="填写收款方式、时间或客户说明" /></label>
        <p v-if="action === 'create'" class="text-sm text-gray-500">创建为普通客户，初始余额为 0，并发为 1，可用分组限定为新业务。</p>
        <p v-if="dialogError" role="alert" class="text-sm text-red-600">{{ dialogError }}</p>
      </form>
      <template #footer><div class="flex justify-end gap-3"><button class="btn btn-secondary" :disabled="saving" @click="close">取消</button><button class="btn btn-primary" form="operations-form" type="submit" :disabled="saving">{{ saving ? '处理中…' : action === 'recharge' ? '确认充值' : action === 'assign' ? '确认开通' : action === 'extend' ? '确认续期' : '保存' }}</button></div></template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import OperationsPlans, { type CatalogPlan, type CatalogGroup } from '@/components/OperationsPlans.vue'
import { apiClient } from '@/api/client'

type Customer = { id: number; email: string; username: string; notes: string; balance: number; concurrency: number; status: string }
type Plan = CatalogGroup
const catalogPlans = ref<CatalogPlan[]>([])
function updateCatalog(plans:CatalogPlan[],businessGroups:CatalogGroup[]){catalogPlans.value=plans;groups.value=businessGroups.filter(g=>g.kind==='subscription')}
type Subscription = { id: number; group_id: number; group_name: string; status: string; expires_at: string; daily_usage: number; weekly_usage: number; monthly_usage: number }
type Action = 'create' | 'edit' | 'recharge' | 'assign' | 'extend'
const customers = ref<Customer[]>([]), groups = ref<Plan[]>([]), subscriptions = ref<Subscription[]>([])
const selected = ref<Customer | null>(null), target = ref<Customer | null>(null), subscriptionTarget = ref<Subscription | null>(null)
const search = ref(''), page = ref(1), total = ref(0), loading = ref(false), saving = ref(false), subscriptionLoading = ref(false)
const error = ref(''), success = ref(''), dialogError = ref(''), action = ref<Action | null>(null)
const titles = { create: '创建客户', edit: '编辑客户', recharge: '客户充值', assign: '开通共享月套餐', extend: '套餐续期' }
const form = reactive({ email: '', password: '', username: '', notes: '', status: 'active', concurrency: 1, amount: 0, days: 30, group_id: 0, plan_id: 0 })
let requestKey = ''
let subscriptionRequest = 0
const money = (value: number) => Number(value).toFixed(2)
const date = (value: string) => new Date(value).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai' })
const planName = (name:string) => name.replace('-Monthly-', ' ').replace('Basic','基础版').replace('Standard','标准版').replace('Advanced','高级版')
const message = (e: unknown) => (e as { message?: string })?.message || '操作失败，请重试。'

async function load() {
  loading.value = true; error.value = ''
  try { const { data } = await apiClient.get('/operations/customers', { params: { page: page.value, page_size: 20, search: search.value } }); customers.value = data.items; total.value = data.total }
  catch (e) { error.value = message(e) } finally { loading.value = false }
}
function searchCustomers() { page.value = 1; void load() }
function changePage(delta: number) { page.value += delta; void load() }
async function showSubscriptions(customer: Customer) {
  selected.value = customer; subscriptions.value = []; subscriptionLoading.value = true
  const current = ++subscriptionRequest
  try { const { data } = await apiClient.get(`/operations/customers/${customer.id}/subscriptions`, { params: { page_size: 100 } }); if (current === subscriptionRequest) subscriptions.value = data.items }
  catch (e) { if (current === subscriptionRequest) error.value = message(e) } finally { if (current === subscriptionRequest) subscriptionLoading.value = false }
}
function open(next: Action, customer: Customer | null = null, subscription: Subscription | null = null) {
  action.value = next; target.value = customer; subscriptionTarget.value = subscription; dialogError.value = ''; success.value = ''
  requestKey = crypto.randomUUID()
  Object.assign(form, { email: '', password: '', username: customer?.username || '', notes: next === 'edit' ? customer?.notes || '' : '', status: customer?.status || 'active', concurrency: customer?.concurrency || 1, amount: 0, days: 30, group_id: groups.value[0]?.id || 0, plan_id: catalogPlans.value.find(p=>p.for_sale && p.ready)?.id || 0 })
}
function close() { if (saving.value) return; action.value = null; form.password = '' }
async function submit() {
  if (saving.value || !action.value) return
  saving.value = true; dialogError.value = ''
  const operation = action.value
  const url = `/operations/customers/${target.value?.id}`
  const config = { headers: { 'Idempotency-Key': requestKey } }
  try {
    if (operation === 'create') await apiClient.post('/operations/customers', { email: form.email.trim(), password: form.password, username: form.username, notes: form.notes })
    if (operation === 'edit') await apiClient.put(url, { username: form.username, notes: form.notes, status: form.status, concurrency: form.concurrency })
    if (operation === 'recharge') await apiClient.post(url + '/recharge', { amount: form.amount, notes: form.notes.trim() }, config)
    if (operation === 'assign') { const plan=catalogPlans.value.find(p=>p.id===form.plan_id); if(!plan)throw new Error('请选择套餐'); await apiClient.post(url + '/subscriptions', { plan_id:plan.id, group_id:plan.group_id, validity_days:plan.validity_days, notes: form.notes.trim() }, config) }
    if (operation === 'extend') await apiClient.post(url + `/subscriptions/${subscriptionTarget.value?.id}/extend`, { days: form.days }, config)
    action.value = null; form.password = ''; success.value = titles[operation] + '成功。'; await load()
    if (selected.value) { const refreshed = customers.value.find(c => c.id === selected.value?.id); await showSubscriptions(refreshed || selected.value) }
  } catch (e) { dialogError.value = message(e) } finally { saving.value = false }
}
onMounted(async () => {
  await load()
})
</script>
