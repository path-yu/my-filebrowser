<template>
  <!-- iCloud 风格半圆存储容量仪表（SVG 半圆弧 + 渐变填充 + 中心百分比） -->
  <div class="storage-gauge" :class="level">
    <svg viewBox="0 0 120 68" class="gauge-svg" aria-hidden="true">
      <defs>
        <linearGradient id="gaugeGrad" x1="0%" y1="0%" x2="100%" y2="0%">
          <stop offset="0%" stop-color="#0a84ff" />
          <stop offset="55%" stop-color="#32ade6" />
          <stop offset="100%" stop-color="#30d158" />
        </linearGradient>
        <linearGradient id="gaugeGradWarn" x1="0%" y1="0%" x2="100%" y2="0%">
          <stop offset="0%" stop-color="#ff9f0a" />
          <stop offset="100%" stop-color="#ff453a" />
        </linearGradient>
      </defs>
      <!-- 背景轨道 -->
      <path
        class="gauge-track"
        d="M 12 60 A 48 48 0 0 1 108 60"
        fill="none"
        stroke-width="9"
        stroke-linecap="round"
      />
      <!-- 进度弧：半圆周长 ≈ π*48 ≈ 150.8 -->
      <path
        class="gauge-value"
        d="M 12 60 A 48 48 0 0 1 108 60"
        fill="none"
        :stroke="high ? 'url(#gaugeGradWarn)' : 'url(#gaugeGrad)'"
        stroke-width="9"
        stroke-linecap="round"
        :style="valueStyle"
      />
    </svg>
    <div class="gauge-center">
      <span class="gauge-pct">{{ clampedPct }}<small>%</small></span>
    </div>
  </div>
</template>

<script>
const SEMI_CIRCUMFERENCE = Math.PI * 48; // ≈ 150.8

export default {
  name: "storage-gauge",
  props: {
    val: { type: Number, default: 0 },
  },
  computed: {
    clampedPct() {
      return Math.max(0, Math.min(100, Math.round(this.val)));
    },
    high() {
      // iCloud 风格：容量紧张（>80%）时切换为橙红警示渐变
      return this.clampedPct >= 80;
    },
    level() {
      return this.clampedPct >= 95 ? "critical" : this.high ? "warn" : "ok";
    },
    valueStyle() {
      return {
        strokeDasharray: `${SEMI_CIRCUMFERENCE}`,
        strokeDashoffset: `${SEMI_CIRCUMFERENCE * (1 - this.clampedPct / 100)}`,
      };
    },
  },
};
</script>

<style scoped>
.storage-gauge {
  position: relative;
  width: 108px;
  margin: 0 auto;
}
.gauge-svg {
  display: block;
  width: 100%;
  height: auto;
}
.gauge-track {
  stroke: var(--gauge-track, rgba(120, 120, 128, 0.16));
  transition: stroke 0.3s ease;
}
.gauge-value {
  transition: stroke-dashoffset 0.7s cubic-bezier(0.4, 0, 0.2, 1), stroke 0.3s ease;
}
.gauge-center {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  justify-content: center;
  pointer-events: none;
}
.gauge-pct {
  font-size: 17px;
  font-weight: 700;
  letter-spacing: -0.02em;
  color: var(--textSecondary);
  font-variant-numeric: tabular-nums;
}
.gauge-pct small {
  font-size: 11px;
  font-weight: 600;
  margin-left: 1px;
  opacity: 0.7;
}
.storage-gauge.warn .gauge-pct {
  color: #ff9f0a;
}
.storage-gauge.critical .gauge-pct {
  color: #ff453a;
}
</style>
