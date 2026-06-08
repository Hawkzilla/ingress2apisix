<script setup lang="ts">
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { api } from '@/api/client'

const router = useRouter()
const route = useRoute()
const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function doLogin() {
  if (!username.value || !password.value) return
  loading.value = true
  error.value = ''
  try {
    const res = await api.post<{ token: string }>('/api/auth/login', {
      username: username.value,
      password: password.value,
    })
    localStorage.setItem('ingress2apisix-token', res.token)
    const redirect = (route.query.redirect as string) || '/'
    router.push(redirect)
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Login failed'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <div class="login-card glass-card">
      <div class="login-header">
        <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" style="color: var(--accent)">
          <path d="M12 2L2 7l10 5 10-5-10-5z" /><path d="M2 17l10 5 10-5" /><path d="M2 12l10 5 10-5" />
        </svg>
        <h1>ingress2apisix</h1>
        <p>Sign in to access admin features</p>
      </div>

      <form @submit.prevent="doLogin" class="login-form">
        <div class="form-field">
          <label>Username</label>
          <input
            v-model="username"
            class="glass-input"
            type="text"
            placeholder="admin"
            autocomplete="username"
          />
        </div>

        <div class="form-field">
          <label>Password</label>
          <input
            v-model="password"
            class="glass-input"
            type="password"
            placeholder="Password"
            autocomplete="current-password"
          />
        </div>

        <div v-if="error" class="login-error">{{ error }}</div>

        <button
          class="glass-btn glass-btn--primary"
          style="width: 100%; justify-content: center;"
          :disabled="loading"
          type="submit"
        >
          {{ loading ? 'Signing in...' : 'Sign In' }}
        </button>
      </form>
    </div>
  </div>
</template>

<style scoped>
.login-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: calc(100vh - 48px);
  padding: 20px;
}

.login-card {
  width: 100%;
  max-width: 380px;
  padding: 32px;
}

.login-header {
  text-align: center;
  margin-bottom: 24px;
}

.login-header h1 {
  font-size: 1.3rem;
  margin-top: 12px;
}

.login-header p {
  font-size: 0.85rem;
  color: var(--text-muted);
  margin-top: 4px;
}

.login-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.form-field label {
  font-size: 0.8rem;
  font-weight: 500;
  color: var(--text-muted);
}

.login-error {
  padding: 8px 12px;
  background: rgba(255,69,58,0.1);
  border: 1px solid rgba(255,69,58,0.2);
  border-radius: 6px;
  color: var(--red);
  font-size: 0.82rem;
}
</style>
