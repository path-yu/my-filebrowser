<template>
  <div
    class="lazy-image"
    ref="wrapRef"
    :class="{
      'lazy-image--fill':    fill,
      'lazy-image--loading': state === 'loading',
      'lazy-image--loaded':  state === 'loaded',
      'lazy-image--error':   state === 'error',
    }"
  >
    <!-- Spinner (iOS-style UIActivityIndicator): 12 radial bars with staggered fade+spin. -->
    <div v-if="state === 'loading'" class="lazy-image__spinner" aria-hidden="true">
      <span class="ios-spinner">
        <i v-for="i in 12" :key="i" class="ios-spinner__bar" :style="iosSpinnerBarStyle(i)"></i>
      </span>
    </div>

    <!-- Error placeholder (clickable to retry via reload) -->
    <button
      v-if="state === 'error'"
      type="button"
      class="lazy-image__error"
      :title="errorTitle"
      @click="reload"
    >
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" class="lazy-image__error-icon">
        <rect x="3"  y="5"  width="18" height="14" rx="2" />
        <circle cx="9"  cy="10" r="1.6" fill="currentColor" />
        <path d="M21 16l-5-5L7 20" />
      </svg>
      <span class="lazy-image__error-text">{{ errorText }}</span>
    </button>

    <!--
      Real image.
      - We keep it mounted always with v-show so ref/expose access works even while loading.
      - Before it's time to load we set `:src="blankDataUrl"` (transparent 1x1 PNG) so the
        browser never issues a real request prematurely.
      - All other attributes fall through via useAttrs (class, style, id, data-*, aria-*,
        @click, alt, crossorigin, referrerpolicy, sizes, srcset, draggable…).
    -->
    <img
      v-show="state !== 'error'"
      ref="imgRef"
      :src="resolvedSrc"
      :class="['lazy-image__img', { 'lazy-image__img-hidden': state !== 'loaded' }]"
      v-bind="imgBindings"
      @load="onImgLoad"
      @error="onImgError"
    />
  </div>
</template>

<script setup lang="ts">
// Do NOT auto-inherit attrs onto the wrapper <div>.
// We forward every caller-provided attribute (class, style, alt, id, draggable, …)
// onto the inner <img> exclusively, so this component behaves indistinguishably
// from a plain <img> tag for all existing CSS rules and layout.
defineOptions({ inheritAttrs: false });

import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  useAttrs,
  watch,
} from "vue";

type State = "loading" | "loaded" | "error";

interface Props {
  /** Image source URL. Required. */
  src: string;

  /**
   * If true, skip IntersectionObserver and start loading immediately on mount.
   * Use this for above-the-fold images (Login logo, HeaderBar logo).
   */
  eager?: boolean;

  /**
   * If true, the wrapper fills 100% width/height of its parent and the inner
   * <img> uses object-fit. Use this when the component is placed inside a
   * sized container (ExtendedImage, PDF thumbnails, grid cells).
   * When false (default), the wrapper behaves like a plain <img>: it shrinks
   * to the image's intrinsic size; any explicit style/class passed through
   * (e.g. `style="height: 12em"`) still applies to the real <img> element.
   */
  fill?: boolean;

  /** IntersectionObserver.rootMargin (default: 300px so images just off-screen start loading). */
  rootMargin?: string;

  /** IntersectionObserver threshold. */
  threshold?: number | number[];

  /** Shown on error overlay. Falls back to a localized message if omitted. */
  errorTitle?: string;

  /** Short text inside the error placeholder. Falls back to a default hint. */
  errorText?: string;
}

const props = withDefaults(defineProps<Props>(), {
  eager: false,
  fill: false,
  rootMargin: "300px 0px",
  threshold: 0.01,
  errorTitle: "",
  errorText: "",
});

// Transparent 1x1 PNG (data URL) used as the placeholder *before* the real URL is assigned.
// Keeping it as `src` ensures the <img> element stays valid and has deterministic layout.
const BLANK =
  "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=";

const attrs = useAttrs();

// Strip our known-props from attributes that should flow to <img>.
// Also exclude any raw `@load`/`@error` event handlers (listeners through attrs)
// because we wrap those and emit on success/failure/retry.
const imgBindings = computed<Record<string, unknown>>(() => {
  const out: Record<string, unknown> = {};
  const skipProps = new Set<string>([
    "src", "eager", "fill", "rootMargin", "threshold", "errorTitle", "errorText",
  ]);
  // With inheritAttrs=false, Vue routes every non-prop attribute (class/style/alt/id/…)
  // into attrs, so we can safely bind them all onto <img> — exactly as if the caller
  // had written a plain <img> tag.
  for (const k of Object.keys(attrs)) {
    if (skipProps.has(k)) continue;
    if (/^(onLoad|onError)$/i.test(k)) continue;
    out[k] = (attrs as any)[k];
  }
  return out;
});

const wrapRef = ref<HTMLElement | null>(null);
const imgRef  = ref<HTMLImageElement | null>(null);

// Build the per-bar inline style for the iOS-style activity indicator.
// 12 bars arranged radially, each offset by 30°; a 1s staggered opacity-fade
// cycle (equally distributed over 12 bars) produces the classic "moving light
// trail" effect identical to UIActivityIndicatorView on iOS.
const FADE_CYCLE_MS = 1000;
const iosSpinnerBarStyle = (index: number): Record<string, string> => {
  const zeroBased = index - 1; // 0..11
  return {
    transform: `rotate(${zeroBased * 30}deg)`,
    animationDelay: `-${(FADE_CYCLE_MS * zeroBased) / 12}ms`,
  };
};

const state   = ref<State>("loading");
const entered = ref<boolean>(!!props.eager);
let observer: IntersectionObserver | null = null;

const effectiveErrorTitle = computed(
  () => props.errorTitle || "Image failed to load. Click to retry."
);
const effectiveErrorText = computed(
  () => props.errorText || "Load failed · Retry"
);

// Emit a well-known events without breaking v-bind="$attrs" re-bindings.
// Using defineEmits here also makes @load/@error used on <LazyImage> compile correctly.
const emit = defineEmits<{
  (e: "load",  ev: Event): void;
  (e: "error", ev: Event): void;
}>();

const resolvedSrc = computed<string>(() => {
  // Not yet entered viewport → never load real URL.
  if (!entered.value) return BLANK;
  // src is empty → blank + error state.
  if (!props.src) return BLANK;
  return props.src;
});

// Reset state whenever `src` actually changes (but not if we haven't entered viewport yet).
watch(
  () => props.src,
  async () => {
    if (!entered.value) return;
    state.value = "loading";
    await nextTick();
    // If <img> src has not been updated yet because browser deduplicated it, force re-load.
    if (imgRef.value) {
      try {
        // Force browser to re-evaluate by briefly resetting src.
        if (imgRef.value.src && new URL(imgRef.value.src).href !== props.src) {
          imgRef.value.src = props.src;
        } else {
          const current = imgRef.value.src;
          imgRef.value.src = BLANK;
          await nextTick();
          imgRef.value.src = current;
        }
      } catch {
        imgRef.value.src = props.src;
      }
    }
  }
);

const onImgLoad = (ev: Event) => {
  state.value = "loaded";
  emit("load", ev);
  // Fire any raw attrs listener (e.g. ExtendedImage's @load).
  const onLoadAttr = (attrs as any)?.onLoad;
  if (typeof onLoadAttr === "function") onLoadAttr(ev);
};

const onImgError = (ev: Event) => {
  state.value = "error";
  emit("error", ev);
  const onErrorAttr = (attrs as any)?.onError;
  if (typeof onErrorAttr === "function") onErrorAttr(ev);
};

const reload = () => {
  if (!imgRef.value || !props.src) return;
  state.value = "loading";
  // Bust browser cache for the same URL: append a timestamp query (only when not data URI).
  let nextSrc = props.src;
  if (nextSrc.startsWith("data:") || nextSrc.startsWith("blob:")) {
    // No cache bust for inline data.
  } else {
    const sep = nextSrc.includes("?") ? "&" : "?";
    nextSrc = `${nextSrc}${sep}__retry=${Date.now()}`;
  }
  imgRef.value.src = BLANK;
  nextTick(() => {
    if (imgRef.value) imgRef.value.src = nextSrc;
  });
};

// ---- IntersectionObserver to gate real load until near viewport ----
const onIntersect = (entries: IntersectionObserverEntry[]) => {
  for (const entry of entries) {
    if (entry.isIntersecting) {
      entered.value = true;
      // Trigger once per mount (subsequent src changes handled by watch above).
      observer?.disconnect();
      observer = null;
      break;
    }
  }
};

onMounted(() => {
  if (props.eager) {
    entered.value = true;
    return;
  }
  if (typeof IntersectionObserver === "undefined") {
    // Very old browsers (no modern IO): degrade to eager.
    entered.value = true;
    return;
  }
  const el = wrapRef.value || imgRef.value;
  if (!el) return;
  try {
    observer = new IntersectionObserver(onIntersect, {
      root: null,
      rootMargin: props.rootMargin,
      threshold: props.threshold,
    });
    observer.observe(el);
  } catch {
    entered.value = true;
  }
});

onBeforeUnmount(() => {
  if (observer) {
    observer.disconnect();
    observer = null;
  }
});

// Expose inner <img> element so consumers (ExtendedImage) that need raw DOM
// access (manual .src assignment, UTIF decode, style manipulation) still work.
defineExpose<{
  imgEl: HTMLImageElement | null;
  reload: () => void;
}>({
  get imgEl() { return imgRef.value ?? null; },
  reload,
});
</script>

<style>
/* -----------------------------------------------------------------
 * Default sizing: behave like a plain <img>.
 *   - inline-block so it flows with text / uses intrinsic dimensions.
 *   - max-width/max-height keep it inside its parent.
 *   - object-fit on the inner <img> only applies when fill=true (see below).
 * ----------------------------------------------------------------- */
.lazy-image {
  position: relative;
  display: inline-block;
  line-height: 0;
  overflow: hidden;
  max-width: 100%;
  max-height: 100%;
  vertical-align: middle;
  background: var(--lazy-image-bg, transparent);
}
/* fill=true: stretch the wrapper to its parent; inner image uses object-fit. */
.lazy-image--fill {
  display: block;
  width: 100%;
  height: 100%;
  max-width: none;
  max-height: none;
}

/*
 * Inner <img>.
 *
 * IMPORTANT: do NOT force `width: auto` / `height: auto` here.
 * Callers pass height/width via class/style (e.g. `header img { height: 2.5em }`)
 * and those rules (tag + class specificity: 0-1-0 / 0-0-2) need to win over this
 * default block. We only set *conservative* defaults so that the component falls
 * back to intrinsic image sizing when nothing constrains it.
 */
.lazy-image__img {
  display: block;
  max-width: 100%;
  max-height: 100%;
  transition: opacity 0.2s ease;
}
.lazy-image--fill .lazy-image__img {
  width: 100%;
  height: 100%;
  object-fit: var(--lazy-image-object-fit, contain);
}
.lazy-image__img-hidden {
  opacity: 0;
}

/* Loading spinner (centered, size-matched so it doesn't shift content) */
.lazy-image__spinner {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;
  z-index: 1;
}

/*
 * iOS-style UIActivityIndicator.
 *   - 24x24 logical size (em-based so it scales with surrounding font-size).
 *   - 12 radial bars; only the bar closest to the current "light phase" is
 *     fully opaque — exactly like iOS UIActivityIndicatorViewStyleMedium.
 */
.ios-spinner {
  position: relative;
  display: inline-block;
  width: 1.75em;
  height: 1.75em;
  min-width: 26px;
  min-height: 26px;
  /*
   * Whole-indicator spin is optional but gives that classic iOS "flow".
   * The staggered per-bar fade does the heavy lifting visually.
   */
  animation: ios-spinner-spin 10s linear infinite;
  color: var(--lazy-image-spinner, rgba(127, 127, 127, 0.85));
}
.ios-spinner__bar {
  position: absolute;
  left: 50%;
  top: 0;
  width: 10%;            /* ~2.4px on a 24px indicator */
  height: 28%;           /* ~6.7px  on a 24px indicator */
  margin-left: -5%;      /* horizontal centering (half of width) */
  transform-origin: 50% 178.5714%;   /* 50% / 0.28 ≈ 178.57%: rotate around indicator center */
  border-radius: 999px;
  background: currentColor;
  opacity: 0.1;          /* default off-state: the animation picks one bar at a time */
  animation: ios-spinner-fade 1s linear infinite;
}
/* Bar opacity phases: the fade animation reaches 1.0 at exactly one point during
 * the 1s cycle; combined with the 12-step animation-delay we end up with a
 * smooth rotating "bright line" that walks around the circle, identical to iOS.
 */
@keyframes ios-spinner-fade {
  0%, 100% { opacity: 0.10; }
  50%      { opacity: 1.00; }
}
/* Very slow whole-spin is optional polish; if you want 1:1 iOS you can set
 * this duration to 0 and rely purely on per-bar fades. Kept as a subtle touch.
 */
@keyframes ios-spinner-spin {
  0%   { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

/* Error card */
.lazy-image__error {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.35em;
  padding: 0.5em;
  background: var(--lazy-image-error-bg, rgba(180, 180, 180, 0.08));
  color: var(--lazy-image-error-fg, rgba(127, 127, 127, 0.85));
  border: 1px dashed var(--lazy-image-error-border, rgba(127, 127, 127, 0.3));
  cursor: pointer;
  z-index: 2;
  border-radius: 4px;
  font: inherit;
  transition: background 0.15s ease, color 0.15s ease, border-color 0.15s ease;
}
.lazy-image__error:hover,
.lazy-image__error:focus-visible {
  background: var(--lazy-image-error-bg--hover, rgba(20, 120, 220, 0.08));
  color: var(--lazy-image-error-fg--hover, rgba(30, 110, 230, 0.95));
  border-color: var(--lazy-image-error-border--hover, rgba(30, 110, 230, 0.55));
  outline: none;
}
.lazy-image__error-icon {
  width: 2em;
  height: 2em;
  min-width: 28px;
  min-height: 28px;
}
.lazy-image__error-text {
  font-size: 0.82em;
  line-height: 1.25;
  text-align: center;
  max-width: 100%;
}
</style>
