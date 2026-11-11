import { ref, watch } from 'vue'

const STORAGE_KEY = 'admin.accounts.autoProNativeSeat'

function load(): boolean {
  try {
    return localStorage.getItem(STORAGE_KEY) !== '0' // 默认开启
  } catch {
    return true
  }
}

/**
 * 新增 Claude OAuth / Setup Token 账号时，是否自动创建 Claude-ProNative-Seat-N 并绑定。
 * 创建弹窗里有多处调用 accounts.create，统一由 API 层读取此开关。
 */
export const autoProNativeSeat = ref(load())

watch(autoProNativeSeat, (v) => {
  try {
    localStorage.setItem(STORAGE_KEY, v ? '1' : '0')
  } catch {
    /* ignore */
  }
})

export function shouldAutoProNativeSeat(platform: string, type: string): boolean {
  return autoProNativeSeat.value && platform === 'anthropic' && (type === 'oauth' || type === 'setup-token')
}
