import { mount, flushPromises } from '@vue/test-utils'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import Panel from './HomeModelCardsSettings.vue'
const api=vi.hoisted(()=>({get:vi.fn(),put:vi.fn()}))
vi.mock('@/api/client',()=>({apiClient:api}))
const defaults=()=>({mode:'random',cards:[{id:'gpt',name:'GPT',logo:'builtin:GPT',x:.2,y:.2},{id:'more',name:'更多模型',logo:'',x:.8,y:.85}]})
beforeEach(()=>{api.get.mockReset();api.put.mockReset();api.get.mockResolvedValue({data:defaults()});vi.stubGlobal('ResizeObserver',class{observe(){}disconnect(){}})})
describe('Home model cards',()=>{
 it('loads, edits and saves only the homepage configuration',async()=>{const w=mount(Panel);await flushPromises();expect(w.findAll('input[maxlength="24"]')).toHaveLength(2);await w.get('input[aria-label="gpt 模型名称"]').setValue('新名称');api.put.mockImplementation(async(_p,v)=>({data:v}));await w.get('button.btn-primary').trigger('click');await flushPromises();expect(api.put).toHaveBeenCalledWith('/admin/settings/home-model-cards',expect.objectContaining({cards:expect.arrayContaining([expect.objectContaining({name:'新名称'})])}));expect(w.text()).toContain('卡片设置已保存');w.unmount()})
 it('protects more and validates blank names',async()=>{const w=mount(Panel);await flushPromises();expect(w.get('button[aria-label="删除 更多模型"]').attributes('disabled')).toBeDefined();await w.get('input[aria-label="gpt 模型名称"]').setValue('');await w.get('button.btn-primary').trigger('click');expect(api.put).not.toHaveBeenCalled();expect(w.text()).toContain('请填写所有模型名称');w.unmount()})
 it('retains failed edits for retry and supports discard',async()=>{const w=mount(Panel);await flushPromises();await w.get('input[aria-label="gpt 模型名称"]').setValue('未保存');api.put.mockRejectedValue({response:{data:{message:'保存失败测试'}}});await w.get('button.btn-primary').trigger('click');await flushPromises();expect(w.text()).toContain('保存失败测试');expect((w.get('input[aria-label="gpt 模型名称"]').element as HTMLInputElement).value).toBe('未保存');await w.findAll('button').find(b=>b.text()==='撤销未保存修改')!.trigger('click');expect((w.get('input[aria-label="gpt 模型名称"]').element as HTMLInputElement).value).toBe('GPT');w.unmount()})
})
