<template>
  <div class="flow-auth">
    <div class="flow-auth-sky" aria-hidden="true"><div class="flow-auth-clouds"></div><img src="/flow-energy.svg" alt=""/></div>
    <header class="flow-auth-header">
      <router-link to="/home" class="flow-brand"><img src="/flow-mark.svg" alt=""/><span>{{ siteName }}<small>ALL MODELS. ONE FLOW.</small></span></router-link>
      <router-link to="/home" class="btn btn-secondary">返回官网 ↗</router-link>
    </header>
    <main class="flow-auth-body">
      <section class="flow-auth-vision" aria-label="品牌愿景">
        <div class="flow-eyebrow">AI · 全模型服务平台</div>
        <h1>掌控每一股流量<br/><span class="flow-gradient">奔赴下一个未来</span></h1>
        <p>连接全球模型生态，让每一次创造持续生长。</p>
        <div class="flow-ecosystem">GPT / Claude / Gemini / DeepSeek / Qwen / 更多模型</div>
      </section>
      <section class="flow-auth-panel">
        <div class="flow-eyebrow">NEW FLOW · ACCOUNT ACCESS</div>
        <slot />
        <div class="mt-6 text-center text-sm"><slot name="footer" /></div>
      </section>
    </main>
    <footer>© {{ currentYear }} {{ siteName }} · ALL MODELS. ONE FLOW.</footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'


const appStore = useAppStore()

const siteName = computed(() => appStore.siteName || '新流量时代')

const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

