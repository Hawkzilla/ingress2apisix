<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'

const router = useRouter()
const showMenu = ref(false)
const isLoggedIn = ref(false) // Will be wired to auth store later

function logout() {
  localStorage.removeItem('ingress2apisix-token')
  isLoggedIn.value = false
  showMenu.value = false
  router.push('/login')
}
</script>

<template>
  <div class="user-menu" @mouseleave="showMenu = false">
    <button class="avatar-btn" @click="showMenu = !showMenu">
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
        <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" /><circle cx="12" cy="7" r="4" />
      </svg>
    </button>
    <transition name="fade">
      <div v-if="showMenu" class="dropdown glass-card">
        <template v-if="isLoggedIn">
          <router-link to="/admin" class="dropdown-item" @click="showMenu = false">Admin</router-link>
          <button class="dropdown-item" @click="logout">Logout</button>
        </template>
        <template v-else>
          <router-link to="/login" class="dropdown-item" @click="showMenu = false">Login</router-link>
          <router-link to="/admin" class="dropdown-item" @click="showMenu = false">Admin</router-link>
        </template>
      </div>
    </transition>
  </div>
</template>

<style scoped>
.user-menu {
  position: relative;
}

.avatar-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border-radius: 50%;
  background: var(--bg-hover);
  border: 1px solid var(--border);
  color: var(--text-muted);
  cursor: pointer;
  transition: all 0.15s;
}

.avatar-btn:hover {
  color: var(--text);
  border-color: var(--accent);
}

.dropdown {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  min-width: 140px;
  padding: 6px;
  border-radius: 10px;
  z-index: 200;
}

.dropdown-item {
  display: block;
  width: 100%;
  padding: 8px 12px;
  border: none;
  background: none;
  color: var(--text);
  font-size: 0.85rem;
  font-family: var(--font-sans);
  text-align: left;
  border-radius: 6px;
  cursor: pointer;
  text-decoration: none;
  transition: background 0.15s;
}

.dropdown-item:hover {
  background: var(--bg-hover);
}
</style>
