import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { api } from '@/api/client'

interface ConvertResponse {
  success: boolean
  output: string
  summary: string
  warnings: string[]
  errors: string[]
}

export const useConverterStore = defineStore('converter', () => {
  const inputYaml = ref('')
  const outputYaml = ref('')
  const target = ref<'APISIX' | 'GatewayAPI'>('APISIX')
  const sslRedirect = ref(true)
  const loading = ref(false)
  const result = ref<ConvertResponse | null>(null)
  const error = ref('')

  const warnings = computed(() => result.value?.warnings ?? [])
  const errors = computed(() => result.value?.errors ?? [])
  const summary = computed(() => result.value?.summary ?? '')

  async function convert() {
    if (!inputYaml.value.trim()) return
    loading.value = true
    error.value = ''
    outputYaml.value = ''
    result.value = null

    try {
      const res = await api.post<ConvertResponse>('/api/convert', {
        yaml: inputYaml.value,
        target: target.value,
        sslRedirect: sslRedirect.value,
      })
      result.value = res

      if (!res.success && res.errors?.length) {
        error.value = res.errors.join('\n')
        return
      }

      outputYaml.value = res.output || ''
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e)
    } finally {
      loading.value = false
    }
  }

  function clear() {
    inputYaml.value = ''
    outputYaml.value = ''
    result.value = null
    error.value = ''
  }

  return {
    inputYaml,
    outputYaml,
    target,
    sslRedirect,
    loading,
    result,
    error,
    warnings,
    errors,
    summary,
    convert,
    clear,
  }
})
