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
    <!-- Blur-Up placeholder layer.
         - Paints a tiny pre-generated inline JPEG over the wrapper while the
           real image downloads. A heavy blur + a slight zoom ensures no pixel
           artefacts show at the edges and the colour palette is representative.
         - Shown only when (a) the back-end supplied a valid data URL AND
           (b) the HD image is still loading or errored. Once loaded, v-if
           destroys this node so GPU memory is released.
         - Takes z-index precedence over the skeleton so the two never overlap. -->
    <div
      v-if="useBlurUp && state !== 'loaded'"
      class="lazy-image__blur-up"
      :style="blurUpStyle"
      aria-hidden="true"
    ></div>

    <!-- Loading skeleton: same-size gray block with a shimmer sweep.
         - Rendered only while state==='loading', so it cannot hide the real
           image after load (avoids the classic "skeleton stays on top" bug).
         - SKIPPED entirely when a Blur-Up placeholder is available; that gives
           us a coloured preview without layout shift and feels instant. -->
    <div
      v-if="state === 'loading' && !useBlurUp"
      class="lazy-image__skeleton"
      aria-hidden="true"
    >
      <span class="lazy-image__skeleton-shimmer"></span>
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
import { ensureAuthCookie, readAuthCookie } from "@/utils/auth";

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

  /**
   * Blur-Up / BlurHash-style progressive placeholder. Accepts a data URL
   * (data:image/jpeg;base64,...) as produced by the back-end. When set, this
   * layer replaces the skeleton screen — giving the user an instant coloured
   * preview that then cross-fades to the real image. An invalid value is
   * silently ignored (skeleton shows as normal).
   */
  blurUp?: string;
}

const props = withDefaults(defineProps<Props>(), {
  eager: false,
  fill: false,
  rootMargin: "300px 0px",
  threshold: 0.01,
  errorTitle: "",
  errorText: "",
  blurUp: "",
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
    "blurUp",
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

const state   = ref<State>("loading");
const entered = ref<boolean>(!!props.eager);
let observer: IntersectionObserver | null = null;

/**
 * Fetch-authenticated override src:
 *   - When a plain Image() load fails (typically 401/403 because `auth`
 *     cookie got dropped / SameSite mis-match / Vite proxy path issue),
 *     we retry via fetch(...) with the explicit `X-Auth: <jwt>` header,
 *     convert the Blob to an object URL and feed it to the <img> instead.
 *   - This guarantees thumbnails / preview pictures render even if the
 *     cookie-based GET authenticator can't see the token.
 */
const overrideSrc = ref<string>("");
/** Tracks the last blob-URL we created, so we can revoke it on swap / unmount. */
let lastObjectUrl: { key: string | null; url: string | null } = { key: null, url: null };
const revokeObjectUrlIfAny = () => {
  if (lastObjectUrl.url) {
    try { URL.revokeObjectURL(lastObjectUrl.url); } catch { /* noop */ }
    lastObjectUrl = { key: null, url: null };
  }
};

const readJwt = (): string => {
  // 1) Make sure the plain-GET `auth` cookie that <img src>/Image() relies on
  //    is present, in-sync with localStorage and formatted as SameSite=Lax
  //    before we even attempt the network call.
  ensureAuthCookie();
  // 2) localStorage copy is always populated by parseToken() / renew().
  if (typeof localStorage !== "undefined") {
    const v = localStorage.getItem("jwt");
    if (v) return v;
  }
  // 3) Belt-and-braces: if for any reason localStorage was wiped but the
  //    cookie still holds the token (proxy-auth scenarios / external SSO),
  //    read the JWT back from the `auth` cookie so the X-Auth fallback can
  //    still authenticate the fetch() call.
  return readAuthCookie();
};

const effectiveErrorTitle = computed(
  () => props.errorTitle || "Image failed to load. Click to retry."
);
const effectiveErrorText = computed(
  () => props.errorText || "Load failed · Retry"
);

// Blur-Up placeholder support.
// Only accept real data URLs to avoid XSS via javascript: URLs.
const useBlurUp = computed<boolean>(
  () => !!props.blurUp && props.blurUp.startsWith("data:image/")
);
const blurUpStyle = computed<Record<string, string>>(() => ({
  backgroundImage: `url(${props.blurUp})`,
}));

// Emit a well-known events without breaking v-bind="$attrs" re-bindings.
// Using defineEmits here also makes @load/@error used on <LazyImage> compile correctly.
const emit = defineEmits<{
  (e: "load",  ev: Event): void;
  (e: "error", ev: Event): void;
}>();

const resolvedSrc = computed<string>(() => {
  // Fallback blob URL (fetch + X-Auth) always wins when it exists.
  if (overrideSrc.value) return overrideSrc.value;
  // Not yet entered viewport → never load real URL.
  if (!entered.value) return BLANK;
  // src is empty → blank + error state.
  if (!props.src) return BLANK;
  return props.src;
});

/**
 * Fallback loader: re-issue the request as `fetch(..., { headers: X-Auth })`
 * and convert the response bytes to a blob: URL. Used when the plain
 * Image() loader fails (cookie not available / 401 / 403 / proxy issue).
 *
 * Returns `true` if the fallback succeeded and state was flipped to loaded.
 */
const tryFetchFallback = async (origSrc: string, key: string): Promise<boolean> => {
  if (origSrc.startsWith("data:") || origSrc.startsWith("blob:")) {
    return false; // can't fallback on inline URIs
  }
  const jwt = readJwt();
  if (!jwt) return false; // nothing to authenticate with

  try {
    const res = await fetch(origSrc, {
      method: "GET",
      credentials: "include", // still send cookie if available
      headers: {
        "X-Auth": jwt,
      },
    });
    if (!res.ok) return false;
    const blob = await res.blob();
    if (blob.size === 0) return false;

    // revoke previous blob-url to avoid GPU memory leak on 4000-row list scroll
    revokeObjectUrlIfAny();
    const objectUrl = URL.createObjectURL(blob);
    lastObjectUrl = { key, url: objectUrl };

    overrideSrc.value = objectUrl;
    // Wait until Vue assigns the overrideSrc to <img src> so the actual
    // decode happens on the real DOM element; then emit `load` externally.
    await nextTick();
    const fireSynthetic = () => {
      const ev = new Event("load");
      onImgLoad(ev);
    };
    if (imgRef.value?.complete && imgRef.value.naturalWidth > 0) {
      fireSynthetic();
    } else {
      const el = imgRef.value;
      const onDone = () => {
        el?.removeEventListener("load", onDone);
        el?.removeEventListener("error", onFailed);
        fireSynthetic();
      };
      const onFailed = () => {
        el?.removeEventListener("load", onDone);
        el?.removeEventListener("error", onFailed);
        // fallback of the fallback: surface original error placeholder
        overrideSrc.value = "";
        revokeObjectUrlIfAny();
        onImgError(new Event("error"));
      };
      el?.addEventListener("load", onDone);
      el?.addEventListener("error", onFailed);
    }
    return true;
  } catch {
    return false;
  }
};

/**
 * Preload the image using a bare `new Image()` object.
 *
 * Why not rely on the template <img @load / @error> alone?
 *   - When the browser serves the resource from HTTP cache (disk/memory) the
 *     <img> onload event can fire *synchronously between* attribute assignment
 *     and Vue patching the listener, so we miss it entirely. The user then
 *     sees a forever-transparent image because state stays `loading` and the
 *     `--hidden` class keeps opacity:0.
 *   - A fresh Image() guarantees its onload/onerror will always run for the
 *     current src because we wire the callbacks BEFORE assigning .src, even
 *     if the bytes are already in cache.
 */
/**
 * The legacy `new Image()` pipeline. Used for non-API assets (logo SVG,
 * external CDN avatars, data URLs) and as the last-resort fallback if
 * the authenticated fetch pipeline below fails for any reason.
 */
const classicImagePreload = (src: string, key: string) => {
  // data URL (blur-up itself / pre-generated): never network-fetches,
  // triggers synchronous decode but Image() still fires onload reliably.
  const loader = new Image();
  // Pass through raw attrs that affect cross-origin loading (cors, referrer).
  // Without this some servers would reject with 403 and Image() would error
  // even though the template <img> loads fine.
  const cors = (attrs as any)?.crossorigin;
  const refp = (attrs as any)?.referrerpolicy;
  if (cors) loader.crossOrigin = cors;
  if (refp) loader.referrerPolicy = refp;

  loader.onload = (ev: Event) => {
    if (lastPreloadKey !== key) return; // stale
    // If we had a blob-url from an earlier fallback, drop it now that the
    // canonical URL has loaded cleanly — it's always preferable to let the
    // browser HTTP cache manage the canonical bytes directly.
    if (overrideSrc.value) {
      overrideSrc.value = "";
      revokeObjectUrlIfAny();
    }
    onImgLoad(ev);
  };
  loader.onerror = async (ev: Event | string) => {
    if (lastPreloadKey !== key) return; // stale
    // Try the fetch + X-Auth fallback before surfacing a broken-image icon.
    const ok = await tryFetchFallback(src, key);
    if (lastPreloadKey !== key) return; // src changed while awaiting fallback
    if (ok) return;
    const errEv = ev instanceof Event ? ev : new Event("error");
    onImgError(errEv);
  };
  loader.src = src;
};

let lastPreloadKey: string | null = null;
const preloadImage = (src: string): void => {
  // Ignore the blank placeholder — it isn't a real image.
  if (!src || src === BLANK || src.startsWith("data:image/svg+xml")) {
    if (!props.src) {
      // No source provided → surface an error placeholder so callers see something.
      const ev = new Event("error");
      onImgError(ev);
    } else {
      // Still waiting for IO to fire (resolvedSrc=BLANK case): keep loading.
      // If already in cache we get a fast synthetic "loaded" event to show
      // Blur-Up without a gap between skeleton → HD fade-in.
      const ev = new Event("load");
      onImgLoad(ev);
    }
    return;
  }

  // Guard against race conditions: if preload was requested for a different
  // src while a previous request is in-flight, discard the stale result.
  const key = src;
  lastPreloadKey = key;

  // Keep cookie in sync for any legacy callers / direct <a> downloads.
  ensureAuthCookie();

  // ================================================================
  // DEFAULT PIPELINE FOR /api/* IMAGES → fetch() + X-Auth header
  // ================================================================
  // Why abandon cookie-based <img src> GET as the primary path?
  //   Cookies for plain-GET resources keep failing in production for:
  //   1. SameSite=Lax edge cases (DevTools new tab, bookmark open)
  //   2. Safari ITP / Android WebViews cookie jars blocking
  //   3. Browser extension "privacy" tools nuking auth cookies
  //   4. Old cached SameSite=Strict cookie not yet overwritten on reload
  //   5. Vite dev proxy rewriting cookie paths on 5173↔8080 hop
  //   The X-Auth header is 100% deterministic: we read JWT from
  //   localStorage/cookie and inject it manually.  Backend withUser
  //   accepts that header FIRST (before checking cookie) and always
  //   returns 200 for valid tokens — verified above via curl tests.
  //   fetch() still benefits from the browser HTTP disk cache, so
  //   repeat list renders / scroll-back don't re-download bytes.
  // ----------------------------------------------------------------
  const apiPattern = /(^|\/)api\/(preview|raw|static|share)\//;
  const looksLikeApiUrl =
    apiPattern.test(src) ||
    (typeof location !== "undefined" && apiPattern.test(src.replace(location.origin, "")));

  if (looksLikeApiUrl) {
    const jwt = readJwt();
    if (jwt) {
      (async () => {
        const ok = await tryFetchFallback(src, key);
        if (lastPreloadKey !== key) return;
        if (ok) return;
        // Fetch + X-Auth unexpectedly failed too → last chance via
        // classic Image() cookie pipeline (still useful if fetch was
        // aborted by DevTools / offline while a cached Image() would work).
        classicImagePreload(src, key);
      })();
      return; // don't run the Image loader in parallel
    }
  }

  // Non-API assets or missing JWT → plain Image pipeline.
  classicImagePreload(src, key);
};

// When the image enters the viewport: start the real preload.
watch(entered, (val) => {
  if (!val || !props.src) return;
  state.value = "loading";
  nextTick(() => preloadImage(props.src));
});

// Reset state + re-preload whenever `src` changes (only after entering viewport).
// This also covers direct-src components like HeaderBar logos that are eager=true
// on first render but receive a second props.src update when routing resolves.
watch(
  () => props.src,
  (newSrc) => {
    if (!entered.value || !newSrc) return;
    state.value = "loading";
    // src 变了：清除上一次 fallback 的 blob-url，防止 GPU 内存泄漏 + 避免旧 overrideSrc 抢先渲染
    overrideSrc.value = "";
    revokeObjectUrlIfAny();
    nextTick(() => preloadImage(newSrc));
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
  if (!props.src) return;
  state.value = "loading";
  // Bust browser cache for the same URL: append a timestamp query (only when not data URI).
  let nextSrc = props.src;
  if (!nextSrc.startsWith("data:") && !nextSrc.startsWith("blob:")) {
    const sep = nextSrc.includes("?") ? "&" : "?";
    nextSrc = `${nextSrc}${sep}__retry=${Date.now()}`;
  }
  // Preload with cache-bust then swap the visible img src in via the usual watch.
  lastPreloadKey = nextSrc;
  preloadImage(nextSrc);
  // Also swap the DOM src so retina/HTIF pipelines still see the retry URL.
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
  } else if (typeof IntersectionObserver === "undefined") {
    // Very old browsers (no modern IO): degrade to eager.
    entered.value = true;
  } else {
    const el = wrapRef.value || imgRef.value;
    if (el) {
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
    }
  }

  // If component mounted with eager=true (or we immediately degraded),
  // kick off the preload synchronously. This also covers the case where
  // watch(entered) doesn't fire because mounted assigned the value.
  if (entered.value && props.src) {
    nextTick(() => preloadImage(props.src));
  }
});

onBeforeUnmount(() => {
  if (observer) {
    observer.disconnect();
    observer = null;
  }
  // VirtualList rows recycle fast; leak of blob URLs → GPU memory grows unbounded.
  // Always revoke on unmount.
  overrideSrc.value = "";
  revokeObjectUrlIfAny();
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
  border-radius: var(--lazy-image-radius, 8px);
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
 *
 * opacity transition 0.4s matches the Blur-Up cross-fade specification.
 */
.lazy-image__img {
  display: block;
  max-width: 100%;
  max-height: 100%;
  transition: opacity 0.4s ease-out;
  position: relative;
  z-index: 1; /* real image must paint ABOVE the blur-up layer when fading in */
}
.lazy-image--fill .lazy-image__img {
  width: 100%;
  height: 100%;
  object-fit: var(--lazy-image-object-fit, contain);
}
.lazy-image__img-hidden {
  opacity: 0;
}

/* ================================================================
 * Loading state: SKELETON (gray block + shimmer sweep).
 *
 * How we cover all scenarios without layout shift:
 *   1. `.lazy-image--loading` tints the wrapper background with the
 *      skeleton base color. The wrapper already inherits the target
 *      size from the inner <img> (via CSS class/style rules written
 *      by callers, e.g. `#listing .item img { width:4em;height:4em }`)
 *      or via fill=100%. So the background IS the visible skeleton
 *      rectangle by itself — no extra min-size forcing needed, no CLS.
 *   2. `.lazy-image__skeleton-shimmer` overlays a diagonal gradient
 *      that sweeps left→right (classic iOS/macOS skeleton shimmer).
 *   3. Everything is `inset: 0` so there's no bleed outside the image
 *      area. v-if="state==='loading'" guarantees we never obscure the
 *      real image after a successful load.
 * ================================================================ */
.lazy-image--loading {
  background-color: var(--lazy-image-skeleton, #E5E5EA);
}
/* Dark theme: iOS/macOS system gray level 2 equivalent */
:where(html.dark, [data-theme="dark"], :root[data-theme="dark"]) .lazy-image--loading {
  background-color: var(--lazy-image-skeleton, #2C2C2E);
}

.lazy-image__skeleton {
  position: absolute;
  inset: 0;
  overflow: hidden;
  pointer-events: none;
  z-index: 1;
  border-radius: inherit;
}

/* Shimmer sweep — a 50%-wide gradient band sliding from -100% to +100%. */
.lazy-image__skeleton-shimmer {
  position: absolute;
  inset: 0;
  transform: translateX(-100%);
  background: linear-gradient(
    90deg,
    transparent 0%,
    rgba(255, 255, 255, 0.55) 50%,
    transparent 100%
  );
  animation: lazy-image-skeleton-sweep 1.5s ease-in-out infinite;
  will-change: transform;
}
/* Dark theme: dim the sweep (full white would burn on near-black bg) */
:where(html.dark, [data-theme="dark"], :root[data-theme="dark"])
  .lazy-image__skeleton-shimmer {
  background: linear-gradient(
    90deg,
    transparent 0%,
    rgba(255, 255, 255, 0.08) 50%,
    transparent 100%
  );
}
@keyframes lazy-image-skeleton-sweep {
  0%   { transform: translateX(-100%); }
  100% { transform: translateX(100%); }
}

/* ================================================================
 * Blur-Up progressive placeholder.
 *
 * - Renders the inline 20×20 JPEG stretched to fill the wrapper.
 * - A heavy gaussian blur destroys pixel-level detail and gives us the
 *   familiar "blurred preview" colour wash; scale(1.1) + inset:-4px
 *   prevent the soft blur edges from showing a dark gap against the
 *   wrapper border (common "edge bleed" artefact with extreme blur).
 * - will-change hints the compositor to keep this as a cheap GPU layer
 *   so the heavy filter doesn't repaint during scroll.
 * ================================================================ */
.lazy-image__blur-up {
  position: absolute;
  inset: -4px;
  background-repeat: no-repeat;
  background-position: center;
  background-size: cover;
  filter: blur(16px);
  -webkit-backdrop-filter: blur(16px); /* Safari compositing parity */
  transform: scale(1.1);
  transform-origin: center center;
  pointer-events: none;
  will-change: transform, filter;
  z-index: 0;
  border-radius: inherit;
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
