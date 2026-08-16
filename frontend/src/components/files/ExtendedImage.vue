<template>
  <div
    class="image-ex-container"
    ref="container"
    @touchstart="touchStart"
    @touchmove="touchMove"
    @dblclick="zoomAuto"
    @mousedown="mousedownStart"
    @mousemove="mouseMove"
    @mouseup="mouseUp"
    @wheel="wheelMove"
  >
    <!--
      LazyImage (eager) wraps the real <img> and gives us:
       * loading spinner while bytes arrive
       * inline error placeholder (with retry) on broken URLs
       * raw <img> exposed as .imgEl (still supports UTIF decode,
         transform translate/scale, setCenter left/top assignment).
      We pass the original image-ex-* positioning classes through to
      the inner <img> via the usual class binding (LazyImage forwards
      anything in attrs onto <img>, including `class`), so the
      container-level drag / pinch / wheel zoom logic continues to work
      with zero semantic change.
    -->
    <LazyImage
      ref="lazyRef"
      :src="src"
      :blurUp="blurUp"
      eager
      fill
      :class="['image-ex-img', imageLoaded ? 'image-ex-img-ready' : 'image-ex-img-center']"
      @load="onLoad"
    />
  </div>
</template>
<script setup lang="ts">
import { throttle } from "lodash-es";
import UTIF from "utif";
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import LazyImage from "@/components/files/LazyImage.vue";

interface IProps {
  src: string;
  moveDisabledTime?: number;
  classList?: any[];
  zoomStep?: number;
  /** Progressive blur-up placeholder (data:image/jpeg;base64). */
  blurUp?: string;
}

const props = withDefaults(defineProps<IProps>(), {
  moveDisabledTime: () => 200,
  classList: () => [],
  zoomStep: () => 0.25,
});

const scale = ref<number>(1);
const lastX = ref<number | null>(null);
const lastY = ref<number | null>(null);
const inDrag = ref<boolean>(false);
const touches = ref<number>(0);
const lastTouchDistance = ref<number | null>(0);
const moveDisabled = ref<boolean>(false);
const disabledTimer = ref<number | null>(null);
const imageLoaded = ref<boolean>(false);
const position = ref<{
  center: { x: number; y: number };
  relative: { x: number; y: number };
}>({
  center: { x: 0, y: 0 },
  relative: { x: 0, y: 0 },
});
const maxScale = ref<number>(4);
const minScale = ref<number>(0.25);

// Refs: lazyRef -> exposes { imgEl, reload() }; container stays the same.
const lazyRef = ref<InstanceType<typeof LazyImage> | null>(null);
const container = ref<HTMLDivElement | null>(null);

/** Get the raw HTMLImageElement held inside LazyImage (for DOM-level ops). */
const imgEl = (): HTMLImageElement | null => lazyRef.value?.imgEl ?? null;

onMounted(() => {
  // UTIF path: if we have a TIFF-family URL, assign to <img>.src is handled
  // by decodeUTIF's XHR path directly (same as original code — it sets
  // imgex = UTIF._imgs[i] then triggers UTIF._imgLoaded).
  if (!decodeUTIF()) {
    // Non-TIFF path: since LazyImage eager=true already assigns props.src,
    // we only need to explicitly re-assign if eager somehow hasn't loaded yet.
    const el = imgEl();
    if (el && el.getAttribute("src") !== props.src) {
      el.src = props.src;
    }
  }

  props.classList.forEach((className) =>
    container.value !== null ? container.value.classList.add(className) : ""
  );

  if (container.value === null) {
    return;
  }

  // set width and height if they are zero
  if (getComputedStyle(container.value).width === "0px") {
    container.value.style.width = "100%";
  }
  if (getComputedStyle(container.value).height === "0px") {
    container.value.style.height = "100%";
  }

  window.addEventListener("resize", onResize);
});

onBeforeUnmount(() => {
  window.removeEventListener("resize", onResize);
  document.removeEventListener("mouseup", onMouseUp);
});

watch(
  () => props.src,
  () => {
    if (!decodeUTIF()) {
      const el = imgEl();
      // Reset positioning classes so spinner shows correctly on new src.
      imageLoaded.value = false;
      if (el) {
        el.classList.remove("image-ex-img-ready");
        el.classList.add("image-ex-img-center");
        el.src = props.src;
      }
    }

    scale.value = 1;
    setZoom();
    setCenter();
  }
);

// Modified from UTIF.replaceIMG
// NOTE: UTIF._imgs expects raw HTMLImageElements; we pass imgEl() which is
// exactly the same DOM node that the original code referenced.
const decodeUTIF = () => {
  const sufs = ["tif", "tiff", "dng", "cr2", "nef"];
  if (document?.location?.pathname === undefined) {
    return false;
  }
  const suff =
    document.location.pathname.split(".")?.pop()?.toLowerCase() ?? "";

  if (sufs.indexOf(suff) == -1) return false;
  const xhr = new XMLHttpRequest();
  UTIF._xhrs.push(xhr);
  const rawImg = imgEl();
  if (!rawImg) return false;
  UTIF._imgs.push(rawImg);
  xhr.open("GET", props.src);
  xhr.responseType = "arraybuffer";
  xhr.onload = UTIF._imgLoaded;
  xhr.send();
  return true;
};

const onLoad = () => {
  imageLoaded.value = true;

  const el = imgEl();
  if (el === null) {
    return;
  }

  el.classList.remove("image-ex-img-center");
  setCenter();
  el.classList.add("image-ex-img-ready");

  document.addEventListener("mouseup", onMouseUp);

  let realSize = el.naturalWidth;
  let displaySize = el.offsetWidth;

  // Image is in portrait orientation
  if (el.naturalHeight > el.naturalWidth) {
    realSize = el.naturalHeight;
    displaySize = el.offsetHeight;
  }

  // Scale needed to display the image on full size
  const fullScale = realSize / displaySize;

  // Full size plus additional zoom
  maxScale.value = fullScale + 4;
};

const onMouseUp = () => {
  inDrag.value = false;
};

const onResize = throttle(function () {
  if (imageLoaded.value) {
    setCenter();
    doMove(position.value.relative.x, position.value.relative.y);
  }
}, 100);

const setCenter = () => {
  if (container.value === null) return;
  const el = imgEl();
  if (el === null) return;

  position.value.center.x = Math.floor(
    (container.value.clientWidth - el.clientWidth) / 2
  );
  position.value.center.y = Math.floor(
    (container.value.clientHeight - el.clientHeight) / 2
  );

  el.style.left = position.value.center.x + "px";
  el.style.top = position.value.center.y + "px";
};

const mousedownStart = (event: MouseEvent) => {
  if (event.button !== 0) return;
  lastX.value = null;
  lastY.value = null;
  inDrag.value = true;
  event.preventDefault();
};
const mouseMove = (event: MouseEvent) => {
  if (!inDrag.value) return;
  doMove(event.movementX, event.movementY);
  event.preventDefault();
};
const mouseUp = (event: Event) => {
  if (inDrag.value) {
    event.preventDefault();
  }
  inDrag.value = false;
};
const touchStart = (event: TouchEvent) => {
  lastX.value = null;
  lastY.value = null;
  lastTouchDistance.value = null;
  if (event.targetTouches.length < 2) {
    setTimeout(() => {
      touches.value = 0;
    }, 300);
    touches.value++;
    if (touches.value > 1) {
      zoomAuto(event);
    }
  }
  event.preventDefault();
};

const zoomAuto = (event: Event) => {
  switch (scale.value) {
    case 1:
      scale.value = 2;
      break;
    case 2:
      scale.value = 4;
      break;
    default:
    case 4:
      scale.value = 1;
      setCenter();
      break;
  }
  setZoom();
  event.preventDefault();
};

const touchMove = (event: TouchEvent) => {
  event.preventDefault();
  if (lastX.value === null) {
    lastX.value = event.targetTouches[0].pageX;
    lastY.value = event.targetTouches[0].pageY;
    return;
  }
  const el = imgEl();
  if (el === null) {
    return;
  }
  const step = el.width / 5;
  if (event.targetTouches.length === 2) {
    moveDisabled.value = true;
    if (disabledTimer.value) clearTimeout(disabledTimer.value);
    disabledTimer.value = window.setTimeout(
      () => (moveDisabled.value = false),
      props.moveDisabledTime
    );

    const p1 = event.targetTouches[0];
    const p2 = event.targetTouches[1];
    const touchDistance = Math.sqrt(
      Math.pow(p2.pageX - p1.pageX, 2) + Math.pow(p2.pageY - p1.pageY, 2)
    );
    if (!lastTouchDistance.value) {
      lastTouchDistance.value = touchDistance;
      return;
    }
    scale.value += (touchDistance - lastTouchDistance.value) / step;
    lastTouchDistance.value = touchDistance;
    setZoom();
  } else if (event.targetTouches.length === 1) {
    if (moveDisabled.value) return;
    const x = event.targetTouches[0].pageX - (lastX.value ?? 0);
    const y = event.targetTouches[0].pageY - (lastY.value ?? 0);
    if (Math.abs(x) >= step && Math.abs(y) >= step) return;
    lastX.value = event.targetTouches[0].pageX;
    lastY.value = event.targetTouches[0].pageY;
    doMove(x, y);
  }
};

const doMove = (x: number, y: number) => {
  const el = imgEl();
  if (el === null) {
    return;
  }
  const style = el.style;

  const posX = pxStringToNumber(style.left) + x;
  const posY = pxStringToNumber(style.top) + y;

  style.left = posX + "px";
  style.top = posY + "px";

  position.value.relative.x = Math.abs(position.value.center.x - posX);
  position.value.relative.y = Math.abs(position.value.center.y - posY);

  if (posX < position.value.center.x) {
    position.value.relative.x = position.value.relative.x * -1;
  }

  if (posY < position.value.center.y) {
    position.value.relative.y = position.value.relative.y * -1;
  }
};
const wheelMove = (event: WheelEvent) => {
  scale.value += -Math.sign(event.deltaY) * props.zoomStep;
  setZoom();
};
const setZoom = () => {
  scale.value = scale.value < minScale.value ? minScale.value : scale.value;
  scale.value = scale.value > maxScale.value ? maxScale.value : scale.value;
  const el = imgEl();
  if (el !== null) el.style.transform = `scale(${scale.value})`;
};
const pxStringToNumber = (style: string) => {
  return +style.replace("px", "");
};
</script>
<style>
.image-ex-container {
  margin: auto;
  overflow: hidden;
  position: relative;
}

/*
 * image-ex-img is forwarded to <LazyImage>'s inner <img> via `:class`
 * (LazyImage useAttrs passes it through). The classes handle absolute
 * positioning. LazyImage's wrapper sets 100% width/height so the
 * absolutely-positioned child stays inside the preview viewport.
 */
.image-ex-img {
  position: absolute;
}

.image-ex-img-center {
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  position: absolute;
  transition: none;
}

.image-ex-img-ready {
  left: 0;
  top: 0;
  transition: transform 0.1s ease;
}
</style>
