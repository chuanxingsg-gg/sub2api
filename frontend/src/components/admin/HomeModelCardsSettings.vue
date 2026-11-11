<template>
  <section class="card space-y-5 p-6" aria-labelledby="home-model-cards-title">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div><h2 id="home-model-cards-title" class="text-lg font-semibold text-gray-900 dark:text-white">首页模型卡片</h2><p class="mt-1 text-sm text-gray-500">管理首页浮动卡片的名称、Logo 和展示位置。保存后对所有访客生效。</p></div>
      <button type="button" class="btn btn-primary" :disabled="loading || saving || !dirty" @click="save">{{ saving ? '保存中…' : '保存卡片设置' }}</button>
    </div>
    <p v-if="loading" class="text-sm">正在读取卡片设置…</p>
    <template v-else-if="loaded">
      <fieldset class="flex flex-wrap gap-5 rounded-xl bg-gray-50 p-4 dark:bg-dark-800"><legend class="px-1 font-medium">展示方式</legend>
        <label class="flex cursor-pointer items-center gap-2"><input v-model="config.mode" type="radio" value="random" />随机错落</label>
        <label class="flex cursor-pointer items-center gap-2"><input v-model="config.mode" type="radio" value="manual" />手动排列</label>
        <p class="w-full text-sm text-gray-500">{{ config.mode === 'random' ? '每次刷新重新随机分布，避免重叠；“更多模型”始终在右下方范围浮动。' : '在下方首页预览中拖放卡片。卡片靠近边缘或其他卡片时会停止移动，避免重叠。' }}</p>
      </fieldset>
      <div class="space-y-3">
        <div v-for="card in config.cards" :key="card.id" class="grid items-start gap-3 rounded-xl border border-gray-200 p-3 dark:border-dark-600 sm:grid-cols-[40px_1fr_1fr_auto]">
          <img v-if="logoSource(card.logo)" :src="logoSource(card.logo)" alt="" class="mt-2 h-8 w-8 rounded bg-indigo-800 p-1 object-contain" />
          <span v-else class="mt-2 h-8 w-8 rounded bg-gray-100 dark:bg-dark-700"></span>
          <label class="text-sm">模型名称<input v-model="card.name" maxlength="24" class="input mt-1 w-full" :aria-label="card.id + ' 模型名称'" /><small v-if="card.id === 'more'" class="text-gray-500">随机模式的右下方固定卡片</small></label>
          <div class="space-y-2 text-sm"><label class="block">Logo
            <select class="input mt-1 w-full" :value="customIds.has(card.id) && !card.logo ? 'custom' : logoChoice(card.logo)" :aria-label="card.name + ' Logo'" @change="chooseLogo(card, ($event.target as HTMLSelectElement).value)">
              <option value="">不显示 Logo</option><option v-for="name in builtinNames" :key="name" :value="'builtin:' + name">{{ name }} 内置 Logo</option><option value="custom">自定义图片</option>
            </select></label>
            <template v-if="logoChoice(card.logo) === 'custom' || customIds.has(card.id)"><input class="input w-full" :value="card.logo.startsWith('data:') ? '' : card.logo" placeholder="https:// 图片地址" :aria-label="card.name + ' Logo 图片地址'" @input="card.logo = ($event.target as HTMLInputElement).value" /><label class="block cursor-pointer text-primary-600">上传图片<input type="file" class="ml-2 max-w-full text-xs" accept="image/png,image/jpeg,image/webp,image/gif" :aria-label="card.name + ' 上传 Logo'" @change="uploadLogo($event, card)" /></label><small class="text-gray-500">PNG/JPG/WebP/GIF，每张不超过 150KB。</small></template>
          </div>
          <button type="button" class="btn btn-secondary mt-5" :disabled="card.id === 'more' || config.cards.length <= 2" :aria-label="'删除 ' + card.name" @click="remove(card.id)">删除</button>
        </div>
      </div>
      <button type="button" class="btn btn-secondary" :disabled="config.cards.length >= 10" @click="add">添加模型卡片</button><span class="ml-3 text-sm text-gray-500">{{ config.cards.length }}/10 张</span>
      <div class="flex flex-wrap items-center justify-between gap-2"><h3 class="font-medium">{{ config.mode === 'manual' ? '首页拖放排列' : '首页效果预览' }}</h3><div class="flex gap-2"><button type="button" class="btn btn-secondary" @click="previewKey++">刷新预览</button><button type="button" class="btn btn-secondary" :disabled="!dirty" @click="discard">撤销未保存修改</button></div></div>
      <p class="text-sm text-gray-500">预览中的改动暂未发布。点击“保存卡片设置”后生效；小屏幕会自动避让，确保卡片完整显示。</p>
      <div ref="previewContainer" class="overflow-hidden rounded-xl border border-gray-200 bg-indigo-950" :style="{ height: previewHeight + 'px' }">
        <iframe :key="previewKey" ref="frame" :src="previewURL" title="首页模型卡片拖放预览" class="origin-top-left border-0" :style="{width:'1280px',height:'900px',transform:`scale(${previewScale})`}" @load="sendPreview" />
      </div>
    </template>
    <p v-if="error" role="alert" class="rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-950 dark:text-red-200">{{ error }}<button v-if="!loaded" type="button" class="ml-3 underline" @click="load">重新读取</button></p>
    <p v-if="status" role="status" class="text-sm text-green-600">{{ status }}</p>
    <p v-if="dirty && loaded" class="text-sm text-amber-600">有尚未保存的修改。</p>
  </section>
</template>
<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { apiClient } from '@/api/client'
import { homeModelLogos } from './homeModelLogos'
type Card = { id: string; name: string; logo: string; x: number; y: number }
type Config = { mode: 'random' | 'manual'; cards: Card[] }
const config = ref<Config>({mode:'random',cards:[]}), saved = ref(''), loading = ref(true), loaded = ref(false), saving = ref(false), error = ref(''), status = ref('')
const customIds = ref(new Set<string>()), frame = ref<HTMLIFrameElement>(), previewContainer = ref<HTMLElement>(), previewScale = ref(.7), previewKey = ref(0)
const builtinNames = Object.keys(homeModelLogos)
const dirty = computed(()=>JSON.stringify(config.value)!==saved.value)
const previewHeight = computed(()=>900*previewScale.value)
const previewURL = new URL('/hero-preview/preview.html?edit-model-cards=1', window.location.origin).href
let observer: ResizeObserver | undefined, timer: ReturnType<typeof setTimeout> | undefined
function logoChoice(logo: string){ return logo.startsWith('builtin:') ? logo : logo ? 'custom' : '' }
function logoSource(logo: string){return logo.startsWith('builtin:') ? homeModelLogos[logo.slice(8)] || '' : /^(https:\/\/|data:image\/(png|jpeg|webp|gif);base64,)/.test(logo) ? logo : ''}
function chooseLogo(card: Card, value: string){if(value==='custom'){customIds.value.add(card.id);card.logo=''}else{customIds.value.delete(card.id);card.logo=value}}
function add(){config.value.cards.push({id:'model-'+crypto.randomUUID().slice(0,12),name:'新模型',logo:'',x:.5,y:.5})}
function remove(id:string){config.value.cards=config.value.cards.filter(c=>c.id!==id)}
function uploadLogo(event: Event, card: Card){const input=event.target as HTMLInputElement,file=input.files?.[0];if(!file)return;if(!['image/png','image/jpeg','image/webp','image/gif'].includes(file.type)||file.size>150000){error.value='请选择 150KB 以内的 PNG/JPG/WebP/GIF 图片。';input.value='';return}const reader=new FileReader();reader.onload=()=>{if(config.value.cards.includes(card)){card.logo=String(reader.result);customIds.value.add(card.id);error.value=''}};reader.readAsDataURL(file)}
async function load(){loading.value=true;error.value='';try{const {data}=await apiClient.get<Config>('/admin/settings/home-model-cards');config.value=data;saved.value=JSON.stringify(data);loaded.value=true;await nextTick();observePreview()}catch(e){error.value=message(e,'读取失败，请重试')}finally{loading.value=false;await nextTick();observePreview()}}
function message(e:unknown,fallback:string){const x=e as {response?:{data?:{message?:string}},message?:string};return x.response?.data?.message||x.message||fallback}
async function save(){error.value='';status.value='';if(config.value.cards.some(c=>!c.name.trim())){error.value='请填写所有模型名称。';return}if(config.value.cards.some(c=>c.logo&&!logoSource(c.logo))){error.value='请检查 Logo，使用 HTTPS 图片地址或支持的图片格式。';return}saving.value=true;const snapshot=JSON.parse(JSON.stringify(config.value)) as Config;try{const {data}=await apiClient.put<Config>('/admin/settings/home-model-cards',snapshot);saved.value=JSON.stringify(data);status.value=dirty.value?'已保存提交时的设置，后续修改尚未保存。':'卡片设置已保存，首页刷新后生效。'}catch(e){error.value=message(e,'保存失败，请重试')}finally{saving.value=false}}
function discard(){config.value=JSON.parse(saved.value);customIds.value.clear();error.value='';status.value=''}
function sendPreview(){frame.value?.contentWindow?.postMessage({type:'home-model-cards:config',config:JSON.parse(JSON.stringify(config.value))},window.location.origin)}
function receive(event:MessageEvent){if(event.origin!==window.location.origin||event.source!==frame.value?.contentWindow)return;if(event.data?.type==='home-model-cards:ready')sendPreview();if(event.data?.type==='home-model-cards:positions'&&config.value.mode==='manual'&&Array.isArray(event.data.cards)){for(const p of event.data.cards){const c=config.value.cards.find(c=>c.id===p.id);if(c&&Number.isFinite(p.x)&&Number.isFinite(p.y)&&p.x>=0&&p.x<=1&&p.y>=0&&p.y<=1){c.x=p.x;c.y=p.y}}}}
function observePreview(){if(!previewContainer.value)return;observer?.disconnect();observer=new ResizeObserver(([e])=>{previewScale.value=e.contentRect.width/1280});observer.observe(previewContainer.value)}
watch(config,()=>{status.value='';clearTimeout(timer);timer=setTimeout(sendPreview,180)},{deep:true})
function unload(event:BeforeUnloadEvent){if(dirty.value&&loaded.value){event.preventDefault();event.returnValue=''}}
onMounted(()=>{window.addEventListener('message',receive);window.addEventListener('beforeunload',unload);void load()})
onBeforeUnmount(()=>{observer?.disconnect();clearTimeout(timer);window.removeEventListener('message',receive);window.removeEventListener('beforeunload',unload)})
</script>
