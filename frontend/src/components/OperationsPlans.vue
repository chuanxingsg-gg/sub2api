<template>
 <section class="rounded-2xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-900">
  <div class="flex flex-wrap items-center justify-between gap-3"><div><h2 class="text-xl font-semibold">套餐管理</h2><p class="mt-2 text-sm text-gray-500">新增或编辑套餐商品，上架后自动显示在官网。额度与并发继承所选业务档位；上游及计费配置由系统管理员维护。</p></div><button class="btn btn-primary" @click="edit()">＋ 新增套餐</button></div>
  <p v-if="error" role="alert" class="mt-4 text-red-600">{{ error }}</p><p v-if="success" role="status" class="mt-4 text-green-600">{{ success }}</p>
  <div class="mt-5 overflow-x-auto"><table class="w-full text-left text-sm"><thead><tr><th class="p-3">套餐名称</th><th class="p-3">价格／有效期</th><th class="p-3">状态</th><th class="p-3">操作</th></tr></thead><tbody><tr v-for="plan in plans" :key="plan.id" class="border-t border-gray-100 dark:border-dark-700"><td class="p-3"><span class="inline-flex items-center gap-2"><PlatformIcon :platform="platformFor(plan.group_id)" class="h-5 w-5 shrink-0" />{{ plan.name }}</span></td><td class="p-3">¥{{ plan.price }}／{{ plan.validity_days }} 天</td><td class="p-3">{{ !plan.for_sale ? '未上架' : plan.ready ? '已上架' : '已展示 · 等待上游接入' }}</td><td class="p-3"><button class="btn btn-secondary" @click="edit(plan)">编辑</button></td></tr><tr v-if="!plans.length"><td colspan="4" class="p-5 text-gray-500">{{ loading ? '正在加载套餐…' : '暂无套餐，请新增。' }}</td></tr></tbody></table></div>
  <BaseDialog :show="visible" :title="editing ? '编辑套餐' : '新增套餐'" @close="close">
   <form id="catalog-form" class="space-y-4" @submit.prevent="save">
    <label class="block"><span class="input-label">业务档位</span><select v-model.number="form.group_id" class="input" required><option :value="0" disabled>请选择业务档位</option><option v-for="g in groups.filter(g => g.kind === 'subscription')" :value="g.id" :key="g.id">{{ g.name }}{{ g.ready ? '' : '（待接入）' }}</option></select></label>
    <p v-if="selectedGroup" class="text-sm text-gray-500">额度：日 {{ selectedGroup.daily_limit }}／周 {{ selectedGroup.weekly_limit }}／周期 {{ selectedGroup.monthly_limit }}；并发 {{ selectedGroup.concurrency }} 路。{{ selectedGroup.ready ? '' : '当前没有可用上游，官网将显示待接入，不能为客户开通。' }}</p>
    <label class="block"><span class="input-label">套餐名称</span><input v-model="form.name" class="input" required maxlength="30" placeholder="例如：GPT 基础版" /></label>
    <label class="block"><span class="input-label">售价（人民币元）</span><input v-model.number="form.price" type="number" class="input" required min="0.01" max="1000000" step="0.01" /></label>
    <label class="block"><span class="input-label">有效天数</span><input v-model.number="form.validity_days" type="number" class="input" required min="1" max="365" step="1" /></label>
    <label class="block"><span class="input-label">套餐说明</span><textarea v-model="form.description" class="input" rows="3" maxlength="600" /></label>
    <label class="block"><span class="input-label">显示顺序（数字小的在前）</span><input v-model.number="form.sort_order" type="number" class="input" required min="0" max="10000" step="1" /></label>
    <label class="flex items-center gap-2"><input v-model="form.for_sale" type="checkbox" />上架到官网</label>
    <p class="text-xs text-gray-500">编辑或下架商品不会更改客户已开通套餐的额度与到期时间。收款后在客户列表中开通套餐；不会自动在线扣款。</p>
    <p v-if="dialogError" role="alert" class="text-sm text-red-600">{{ dialogError }}</p>
   </form>
   <template #footer><div class="flex justify-end gap-3"><button class="btn btn-secondary" :disabled="saving" @click="close">取消</button><button class="btn btn-primary" type="submit" form="catalog-form" :disabled="saving">{{ saving ? '保存中…' : '保存套餐' }}</button></div></template>
  </BaseDialog>
 </section>
</template>
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { apiClient } from '@/api/client'
import BaseDialog from '@/components/common/BaseDialog.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
export type CatalogPlan = {id:number;group_id:number;name:string;description:string;price:number;validity_days:number;for_sale:boolean;sort_order:number;ready:boolean}
export type CatalogGroup = {id:number;name:string;family:string;kind:string;daily_limit:number;weekly_limit:number;monthly_limit:number;concurrency:number;ready:boolean}
const emit=defineEmits<{updated:[plans:CatalogPlan[],groups:CatalogGroup[]]}>()
const plans=ref<CatalogPlan[]>([]),groups=ref<CatalogGroup[]>([]),loading=ref(false),saving=ref(false),visible=ref(false),editing=ref(0),error=ref(''),dialogError=ref(''),success=ref('')
const form=reactive({group_id:0,name:'',description:'',price:0,validity_days:30,for_sale:false,sort_order:0})
const selectedGroup=computed(()=>groups.value.find(g=>g.id===form.group_id))
const platformFor=(id:number)=>({Claude:'anthropic',GPT:'openai',Grok:'grok',Gemini:'gemini'}[groups.value.find(g=>g.id===id)?.family||''] || 'anthropic')
let key=''
const message=(e:unknown)=>(e as {message?:string})?.message || '操作失败，请重试。'
async function load(){loading.value=true;error.value='';try{const {data}=await apiClient.get('/operations/plans');plans.value=data.plans;groups.value=data.groups;emit('updated',plans.value,groups.value)}catch(e){error.value=message(e)}finally{loading.value=false}}
function edit(plan?:CatalogPlan){editing.value=plan?.id||0;Object.assign(form,{group_id:plan?.group_id||0,name:plan?.name||'',description:plan?.description||'',price:plan?.price||0,validity_days:plan?.validity_days||30,for_sale:plan?.for_sale||false,sort_order:plan?.sort_order||0});key=crypto.randomUUID();dialogError.value='';visible.value=true}
function close(){if(!saving.value)visible.value=false}
async function save(){if(saving.value)return;saving.value=true;dialogError.value='';try{await apiClient.request({url:'/operations/plans'+(editing.value?'/'+editing.value:''),method:editing.value?'PUT':'POST',data:{...form,name:form.name.trim()},headers:{'Idempotency-Key':key}});visible.value=false;success.value='套餐已保存，官网将使用最新目录。';await load()}catch(e){dialogError.value=message(e)}finally{saving.value=false}}
onMounted(load)
</script>
