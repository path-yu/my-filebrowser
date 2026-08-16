<template>
  <div class="mac-audio-wrapper" :style="wrapperStyle">
    <!-- 1. 独立于播放器容器之外的 Canvas 真实频域波形画布 -->
    <div class="spectrum-outer-container">
      <canvas ref="canvasRef" class="spectrum-canvas"></canvas>
    </div>

    <!-- 原生 Audio 标签 -->
    <audio
      ref="audioRef"
      :src="src"
      crossorigin="anonymous"
      @timeupdate="onTimeUpdate"
      @loadedmetadata="onLoadedMetadata"
      @ended="onEnded"
    ></audio>

    <!-- 2. 底部 macOS 极简控制条容器 (动态绑定尺寸 class) -->
    <div :class="['audio-controls', `size-${size}`]">
      <!-- 后退 10 秒 -->
      <button class="icon-btn" @click="skip(-10)" title="快退 10 秒">
        <svg viewBox="0 0 24 24" fill="currentColor">
          <path d="M12.5 8c-2.65 0-5.05.99-6.9 2.6L2 7v9h9l-3.62-3.62c1.39-1.16 3.16-1.88 5.12-1.88 3.54 0 6.55 2.31 7.6 5.5l2.37-.78C21.08 11.03 17.15 8 12.5 8z"/>
        </svg>
      </button>

      <!-- 播放/暂停 -->
      <button class="play-btn" @click="togglePlay" :title="isPlaying ? '暂停 (Space)' : '播放 (Space)'">
        <svg v-if="!isPlaying" viewBox="0 0 24 24" fill="currentColor">
          <path d="M8 5v14l11-7z" />
        </svg>
        <svg v-else viewBox="0 0 24 24" fill="currentColor">
          <path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z" />
        </svg>
      </button>

      <!-- 前进 10 秒 -->
      <button class="icon-btn" @click="skip(10)" title="快进 10 秒">
        <svg viewBox="0 0 24 24" fill="currentColor">
          <path d="M11.5 8c2.65 0 5.05.99 6.9 2.6L22 7v9h-9l3.62-3.62c-1.39-1.16-3.16-1.88-5.12-1.88-3.54 0-6.55 2.31-7.6 5.5l-2.37-.78C2.92 11.03 6.85 8 11.5 8z"/>
        </svg>
      </button>

      <!-- 时间 -->
      <span class="time-text">{{ formatTime(currentTime) }} / {{ formatTime(duration) }}</span>

      <!-- 进度条 -->
      <div class="slider-wrapper">
        <input
          type="range"
          class="mac-range progress-range"
          min="0"
          :max="duration || 100"
          step="0.1"
          :value="currentTime"
          @input="onSeekInput"
          :style="{ '--progress-percent': `${progressPercent}%` }"
        />
      </div>

      <!-- 倍速 -->
      <button class="rate-btn" @click="changePlaybackRate" title="切换倍  速">
        {{ currentRate }}x
      </button>

      <!-- 音量 -->
      <div class="volume-container">
        <button class="icon-btn" @click="toggleMute">
          <svg v-if="isMuted || volume === 0" viewBox="0 0 24 24" fill="currentColor">
            <path d="M16.5 12c0-1.77-1.02-3.29-2.5-4.03v2.21l2.45 2.45c.03-.2.05-.41.05-.63zm2.5 0c0 .94-.2 1.82-.54 2.64l1.51 1.51C20.63 14.91 21 13.5 21 12c0-4.28-2.99-7.86-7-8.77v2.06c2.89.86 5 3.54 5 6.71zM4.27 3L3 4.27 7.73 9H3v6h4l5 5v-6.73l4.25 4.25c-.67.52-1.42.93-2.25 1.18v2.06c1.38-.31 2.63-.95 3.69-1.81L19.73 21 21 19.73l-9-9L4.27 3zM12 4L9.91 6.09 12 8.18V4z"/>
          </svg>
          <svg v-else viewBox="0 0 24 24" fill="currentColor">
            <path d="M3 9v6h4l5 5V4L7 9H3zm13.5 3c0-1.77-1.02-3.29-2.5-4.03v8.05c1.48-.73 2.5-2.25 2.5-4.02zM14 3.23v2.06c2.89.86 5 3.54 5 6.71s-2.11 5.85-5 6.71v2.06c4.01-.91 7-4.49 7-8.77s-2.99-7.86-7-8.77z"/>
          </svg>
        </button>
        <input
          type="range"
          class="mac-range volume-range"
          min="0"
          max="1"
          step="0.05"
          :value="isMuted ? 0 : volume"
          @input="onVolumeChange"
          :style="{ '--progress-percent': `${(isMuted ? 0 : volume) * 100}%` }"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from "vue";

const props = withDefaults(
  defineProps<{
    src: string;
    width?: string;
    bottom?: string;
    position?: "fixed" | "absolute" | "relative";
    spectrumHeight?: number;
    size?: "sm" | "md" | "lg"; // 新增 size 属性
  }>(),
  {
    width: "700px",
    bottom: "50px",
    position: "fixed",
    spectrumHeight: 120,
    size: "md", // 默认中等尺寸
  }
);

const audioRef = ref<HTMLAudioElement | null>(null);
const canvasRef = ref<HTMLCanvasElement | null>(null);

const isPlaying = ref(false);
const currentTime = ref(0);
const duration = ref(0);
const volume = ref(1);
const isMuted = ref(false);
const currentRate = ref(1.0);
const rates = [0.75, 1.0, 1.25, 1.5, 2.0];

let audioCtx: AudioContext | null = null;
let analyser: AnalyserNode | null = null;
let sourceNode: MediaElementAudioSourceNode | null = null;
let animFrameId: number | null = null;

const wrapperStyle = computed(() => {
  if (props.position === "relative") {
    return { width: props.width, margin: "0 auto" };
  }
  return {
    position: props.position,
    bottom: props.bottom,
    left: "50%",
    transform: "translateX(-50%)",
    width: props.width,
    zIndex: 1000,
  };
});

const progressPercent = computed(() => {
  if (!duration.value) return 0;
  return (currentTime.value / duration.value) * 100;
});

const initAudioContext = () => {
  if (audioCtx || !audioRef.value) return;

  const AudioContextClass = window.AudioContext || (window as any).webkitAudioContext;
  audioCtx = new AudioContextClass();
  analyser = audioCtx.createAnalyser();
  analyser.fftSize = 128;

  sourceNode = audioCtx.createMediaElementSource(audioRef.value);
  sourceNode.connect(analyser);
  analyser.connect(audioCtx.destination);
};

const drawSpectrum = () => {
  if (!canvasRef.value || !analyser) return;

  const canvas = canvasRef.value;
  const ctx = canvas.getContext("2d");
  if (!ctx) return;

  const bufferLength = analyser.frequencyBinCount;
  const dataArray = new Uint8Array(bufferLength);

  const render = () => {
    animFrameId = requestAnimationFrame(render);
    analyser!.getByteFrequencyData(dataArray);

    const width = canvas.width;
    const height = canvas.height;

    ctx.clearRect(0, 0, width, height);

    const barCount = 48;
    const gap = 4;
    const barWidth = (width - gap * (barCount - 1)) / barCount;

    const computedStyle = getComputedStyle(canvas);
    const accentColor = computedStyle.getPropertyValue("--accent-color").trim() || "#0071e3";

    for (let i = 0; i < barCount; i++) {
      const dataIndex = Math.floor((i / barCount) * (bufferLength * 0.75));
      const value = dataArray[dataIndex] || 0;

      const percent = value / 255;
      const barHeight = Math.max(3, percent * height * 0.9);

      const x = i * (barWidth + gap);
      const y = height - barHeight;

      ctx.fillStyle = accentColor;
      ctx.beginPath();
      if (ctx.roundRect) {
        ctx.roundRect(x, y, barWidth, barHeight, [2, 2, 0, 0]);
      } else {
        ctx.rect(x, y, barWidth, barHeight);
      }
      ctx.fill();
    }
  };

  render();
};

const togglePlay = async () => {
  if (!audioRef.value) return;

  initAudioContext();

  if (audioCtx && audioCtx.state === "suspended") {
    await audioCtx.resume();
  }

  if (isPlaying.value) {
    audioRef.value.pause();
    if (animFrameId) cancelAnimationFrame(animFrameId);
  } else {
    try {
      await audioRef.value.play();
      drawSpectrum();
    } catch (e) {
      console.error("播放失败:", e);
    }
  }
  isPlaying.value = !isPlaying.value;
};

// 键盘按键监听：空格键切换播放/暂停
const handleKeyDown = (e: KeyboardEvent) => {
  const activeEl = document.activeElement;
  const isInput =
    activeEl &&
    (activeEl.tagName === "INPUT" ||
      activeEl.tagName === "TEXTAREA" ||
      (activeEl as HTMLElement).isContentEditable);

  if (e.code === "Space" && !isInput) {
    e.preventDefault();
    togglePlay();
  }
};

const skip = (secs: number) => {
  if (!audioRef.value) return;
  audioRef.value.currentTime = Math.min(
    Math.max(0, audioRef.value.currentTime + secs),
    duration.value
  );
};

const changePlaybackRate = () => {
  if (!audioRef.value) return;
  const nextIdx = (rates.indexOf(currentRate.value) + 1) % rates.length;
  currentRate.value = rates[nextIdx];
  audioRef.value.playbackRate = currentRate.value;
};

const onTimeUpdate = () => {
  if (audioRef.value) currentTime.value = audioRef.value.currentTime;
};

const onLoadedMetadata = () => {
  if (audioRef.value) duration.value = audioRef.value.duration;
};

const onEnded = () => {
  isPlaying.value = false;
  currentTime.value = 0;
  if (animFrameId) cancelAnimationFrame(animFrameId);
};

const onSeekInput = (e: Event) => {
  const target = e.target as HTMLInputElement;
  const val = parseFloat(target.value);
  if (audioRef.value) {
    audioRef.value.currentTime = val;
    currentTime.value = val;
  }
};

const onVolumeChange = (e: Event) => {
  const target = e.target as HTMLInputElement;
  const val = parseFloat(target.value);
  volume.value = val;
  if (audioRef.value) audioRef.value.volume = val;
  isMuted.value = val === 0;
};

const toggleMute = () => {
  if (!audioRef.value) return;
  isMuted.value = !isMuted.value;
  audioRef.value.muted = isMuted.value;
};

const formatTime = (seconds: number) => {
  if (isNaN(seconds) || !seconds) return "0:00";
  const mins = Math.floor(seconds / 60);
  const secs = Math.floor(seconds % 60);
  return `${mins}:${secs < 10 ? "0" : ""}${secs}`;
};

const resizeCanvas = () => {
  if (!canvasRef.value) return;
  const canvas = canvasRef.value;
  const rect = canvas.getBoundingClientRect();
  const dpr = window.devicePixelRatio || 1;

  canvas.width = rect.width * dpr;
  canvas.height = props.spectrumHeight * dpr;
};

onMounted(() => {
  resizeCanvas();
  window.addEventListener("resize", resizeCanvas);
  window.addEventListener("keydown", handleKeyDown);
});

onUnmounted(() => {
  window.removeEventListener("resize", resizeCanvas);
  window.removeEventListener("keydown", handleKeyDown);
  if (animFrameId) cancelAnimationFrame(animFrameId);
  if (audioCtx) audioCtx.close();
});
</script>

<style scoped>
.mac-audio-wrapper {
  --player-bg: rgba(255, 255, 255, 0.75);
  --player-border: rgba(0, 0, 0, 0.08);
  --player-text: #1d1d1f;
  --player-subtext: #86868b;
  --track-bg: rgba(0, 0, 0, 0.1);
  --accent-color: #0071e3;
  --hover-bg: rgba(0, 0, 0, 0.05);

  max-width: 90vw;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

:global(html.dark .mac-audio-wrapper),
:global(.dark .mac-audio-wrapper) {
  --player-bg: rgba(35, 35, 37, 0.85);
  --player-border: rgba(255, 255, 255, 0.12);
  --player-text: #f5f5f7;
  --player-subtext: #a1a1a6;
  --track-bg: rgba(255, 255, 255, 0.18);
  --accent-color: #0a84ff;
  --hover-bg: rgba(255, 255, 255, 0.12);
}

.spectrum-outer-container {
  width: 100%;
  height: 100px;
  display: flex;
  align-items: flex-end;
  justify-content: center;
}

.spectrum-canvas {
  width: 100%;
  height: 100%;
  display: block;
  --accent-color: var(--accent-color);
}

/* --- 核心修改：尺寸变量配置 --- */
.audio-controls {
  /* 基础结构 */
  border-radius: 999px;
  background: var(--player-bg);
  backdrop-filter: blur(24px) saturate(180%);
  -webkit-backdrop-filter: blur(24px) saturate(180%);
  border: 1px solid var(--player-border);
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.12);
  display: flex;
  align-items: center;
  transition: background-color 0.25s, border-color 0.25s, all 0.2s;

  /* 动态应用变量 */
  height: var(--ctrl-height);
  padding: var(--ctrl-padding);
  gap: var(--ctrl-gap);
}

/* 尺寸预设 - SM (小号，近似于最初的样子) */
.audio-controls.size-sm {
  --ctrl-height: 36px;
  --ctrl-padding: 6px 14px;
  --ctrl-gap: 8px;
  --btn-size: 26px;
  --icon-size: 13px;
  --text-size: 11px;
  --rate-padding: 3px 6px;
  --range-height: 4px;
  --range-width: 120px;
}

/* 尺寸预设 - MD (中号，默认，更易点击) */
.audio-controls.size-md {
  --ctrl-height: 44px;
  --ctrl-padding: 8px 18px;
  --ctrl-gap: 12px;
  --btn-size: 32px;
  --icon-size: 16px;
  --text-size: 12px;
  --rate-padding: 4px 8px;
  --range-height: 5px;
  --range-width: 140px;
}

/* 尺寸预设 - LG (大号，非常清晰宽敞) */
.audio-controls.size-lg {
  --ctrl-height: 54px;
  --ctrl-padding: 10px 24px;
  --ctrl-gap: 16px;
  --btn-size: 40px;
  --icon-size: 20px;
  --text-size: 14px;
  --rate-padding: 6px 12px;
  --range-height: 6px;
  --range-width: 160px;
}

.play-btn, .icon-btn {
  background: transparent;
  border: none;
  color: var(--player-text);
  /* 使用变量 */
  width: var(--btn-size);
  height: var(--btn-size);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  flex-shrink: 0;
  transition: background-color 0.15s;
}

.play-btn svg, .icon-btn svg {
  /* 使用变量限制 SVG 尺寸 */
  width: var(--icon-size);
  height: var(--icon-size);
  transition: all 0.2s;
}

.play-btn:hover, .icon-btn:hover {
  background-color: var(--hover-bg);
}

.time-text {
  font-size: var(--text-size);
  color: var(--player-subtext);
  font-variant-numeric: tabular-nums;
  flex-shrink: 0;
  transition: font-size 0.2s;
}

.slider-wrapper {
  flex: 1;
  min-width: 120px;
  display: flex;
  align-items: center;
}

.rate-btn {
  background: var(--hover-bg);
  border: none;
  color: var(--player-text);
  font-weight: 600;
  cursor: pointer;
  flex-shrink: 0;
  border-radius: 6px;
  transition: all 0.15s;
  /* 使用变量 */
  font-size: var(--text-size);
  padding: var(--rate-padding);
}

.rate-btn:hover {
  background-color: var(--track-bg);
}

.volume-container {
  display: flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
  min-width: 120px;
}

.mac-range {
  -webkit-appearance: none;
  appearance: none;
  width: 100%;
  border-radius: 999px;
  outline: none;
  cursor: pointer;
  /* 使用变量支持进度条粗细变化 */
  height: var(--range-height);
  background: linear-gradient(
    to right,
    var(--accent-color) 0%,
    var(--accent-color) var(--progress-percent, 0%),
    var(--track-bg) var(--progress-percent, 0%),
    var(--track-bg) 100%
  );
  transition: height 0.15s ease;
}

.mac-range:hover {
  height: calc(var(--range-height) + 2px);
}

.mac-range::-webkit-slider-thumb {
  -webkit-appearance: none;
  appearance: none;
  width: calc(var(--range-height) * 2.5);
  height: calc(var(--range-height) * 2.5);
  border-radius: 50%;
  background: #ffffff;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.3);
  cursor: pointer;
}

/* 特殊控制一下音量条的长度 */
.volume-range {
  width: calc(var(--range-width) * 1);

}
</style>