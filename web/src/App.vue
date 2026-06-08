<script setup lang="ts">
import { onMounted } from 'vue'
import { useAppStore } from '@/stores/app'
import AppHeader from '@/components/layout/AppHeader.vue'
import AnnouncementBar from '@/components/layout/AnnouncementBar.vue'

const app = useAppStore()

onMounted(() => {
  app.loadVersion()
  app.loadAnnouncements()
})
</script>

<template>
  <div class="app-shell">
    <AppHeader />
    <AnnouncementBar v-if="app.announcements.length > 0" />
    <main class="app-main">
      <router-view v-slot="{ Component }">
        <transition name="fade" mode="out-in">
          <component :is="Component" />
        </transition>
      </router-view>
    </main>
  </div>
</template>

<style scoped>
.app-shell {
  display: flex;
  flex-direction: column;
  min-height: 100vh;
}

.app-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding-top: 56px; /* header height */
}
</style>
