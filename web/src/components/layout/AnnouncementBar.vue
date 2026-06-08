<script setup lang="ts">
import { useAppStore } from '@/stores/app'

const app = useAppStore()
</script>

<template>
  <div v-if="app.announcements.length > 0" class="announcement-bar">
    <div
      v-for="ann in app.announcements"
      :key="ann.id"
      class="announcement-item"
      :class="`level-${ann.level}`"
    >
      <span class="ann-icon">
        <template v-if="ann.level === 'warning'">&#9888;</template>
        <template v-else-if="ann.level === 'error'">&#10006;</template>
        <template v-else>&#8505;</template>
      </span>
      <strong v-if="ann.title">{{ ann.title }}:</strong>
      <span>{{ ann.content }}</span>
    </div>
  </div>
</template>

<style scoped>
.announcement-bar {
  position: fixed;
  top: 56px;
  left: 0;
  right: 0;
  z-index: 90;
  padding: 8px 20px;
  background: linear-gradient(135deg, var(--announcement-grad-start), var(--announcement-grad-end));
  backdrop-filter: var(--glass-blur-light);
  -webkit-backdrop-filter: var(--glass-blur-light);
  border-bottom: 0.5px solid var(--border);
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  justify-content: center;
  font-size: 0.82rem;
}

.announcement-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.ann-icon {
  font-size: 0.9rem;
}

.level-warning { color: var(--yellow); }
.level-error { color: var(--red); }
.level-info { color: var(--accent); }
</style>
