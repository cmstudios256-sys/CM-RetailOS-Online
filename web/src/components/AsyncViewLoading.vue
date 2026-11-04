<template>
  <div class="view-section async-loading-skeleton" role="status" aria-label="Loading view">
    <div class="skeleton-header-row">
      <div class="skeleton-box skeleton-shimmer title-box"></div>
      <div class="skeleton-box skeleton-shimmer filter-box"></div>
    </div>
    <div class="skeleton-cards-grid">
      <div class="skeleton-card skeleton-box skeleton-shimmer" v-for="i in 3" :key="i"></div>
    </div>
    <div class="skeleton-panel skeleton-box skeleton-shimmer"></div>
  </div>
</template>

<style scoped>
.async-loading-skeleton {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
  width: 100%;
  min-height: 480px;
  animation: fadeIn 0.2s ease-out;
}

.skeleton-header-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1rem;
}

.title-box {
  width: 220px;
  height: 32px;
  border-radius: 8px;
}

.filter-box {
  width: 160px;
  height: 32px;
  border-radius: 999px;
}

.skeleton-cards-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 1rem;
}

.skeleton-card {
  height: 96px;
  border-radius: 14px;
}

.skeleton-panel {
  flex: 1 1 360px;
  min-height: 320px;
  border-radius: 16px;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}
</style>

