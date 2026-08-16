<template>
  <video ref="videoPlayer" class="video-max video-js" controls preload="metadata">
    <source />
    <track
      kind="subtitles"
      v-for="(sub, index) in subtitles"
      :key="index"
      :src="sub"
      :label="subLabel(sub)"
      :default="index === 0"
    />
    <p class="vjs-no-js">
      Sorry, your browser doesn't support embedded videos, but don't worry, you
      can <a :href="source">download it</a>
      and watch it with your favorite video player!
    </p>
  </video>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, nextTick } from "vue";
import videojs from "video.js";
import type Player from "video.js/dist/types/player";
import "videojs-mobile-ui";
import "videojs-hotkeys";
import "video.js/dist/video-js.min.css";
import "videojs-mobile-ui/dist/videojs-mobile-ui.css";

const videoPlayer = ref<HTMLElement | null>(null);
const player = ref<Player | null>(null);

const props = withDefaults(
  defineProps<{
    source: string;
    subtitles?: string[];
    options?: any;
    posterTime?: number; // 增加可配置的截取时间点（秒）
  }>(),
  {
    options: {},
    posterTime: 0.5, // 默认截取第 0.5 秒，避免第 0 秒黑屏
  }
);

const source = ref(props.source);
const sourceType = ref("");

nextTick(() => {
  initVideoPlayer();
});

onBeforeUnmount(() => {
  if (player.value) {
    player.value.dispose();
    player.value = null;
  }
});

/**
 * 纯前端截取视频指定帧并生成 Base64 图片
 */
const generatePosterFromVideo = (
  videoUrl: string,
  timeOffset: number = 0.5
): Promise<string> => {
  return new Promise((resolve, reject) => {
    const video = document.createElement("video");
    video.src = videoUrl;
    video.crossOrigin = "anonymous"; // 避免 CORS 污染 Canvas
    video.currentTime = timeOffset;
    video.muted = true;
    video.preload = "metadata";

    // 当到达指定时间点且视频帧就绪时触发
    video.addEventListener("seeked", () => {
      try {
        const canvas = document.createElement("canvas");
        canvas.width = video.videoWidth || 1280;
        canvas.height = video.videoHeight || 720;

        const ctx = canvas.getContext("2d");
        if (ctx) {
          ctx.drawImage(video, 0, 0, canvas.width, canvas.height);
          const dataUrl = canvas.toDataURL("image/jpeg", 0.85);
          resolve(dataUrl);
        } else {
          reject("Canvas context 2d unavailable");
        }
      } catch (err) {
        reject(err);
      } finally {
        video.remove(); // 清理 DOM 内存
      }
    });

    video.addEventListener("error", (e) => {
      video.remove();
      reject(e);
    });
  });
};

const initVideoPlayer = async () => {
  try {
    const lang = document.documentElement.lang;
    const languagePack = await (
      languageImports[lang] || languageImports.en
    )?.();
    const code = languageImports[lang] ? lang : "en";
    videojs.addLanguage(code, languagePack.default);

    sourceType.value = getSourceType(source.value);

    const srcOpt = { sources: { src: props.source, type: sourceType.value } };
    const langOpt = { language: code };
    const playbackRatesOpt = { playbackRates: [0.5, 1, 1.5, 2, 2.5, 3] };
    const options = getOptions(
      props.options,
      langOpt,
      srcOpt,
      playbackRatesOpt
    );

    player.value = videojs(videoPlayer.value!, options, () => {});

    // @ts-expect-error no ts definition for mobileUi
    player.value!.mobileUi();

    // 尝试前端自动生成封面并设置给 Video.js
    if (props.source) {
      generatePosterFromVideo(props.source, props.posterTime)
        .then((posterUrl) => {
          if (player.value) {
            player.value.poster(posterUrl);
          }
        })
        .catch((err) => {
          console.warn("前端生成视频封面失败 (如遇跨域 CORS 限制可忽略):", err);
        });
    }
  } catch (error) {
    console.error("Error initializing video player:", error);
  }
};

const getOptions = (...srcOpt: any[]) => {
  const options = {
    controlBar: {
      skipButtons: {
        forward: 5,
        backward: 5,
      },
    },
    html5: {
      nativeTextTracks: false,
    },
    plugins: {
      hotkeys: {
        volumeStep: 0.1,
        seekStep: 10,
        enableModifiersForNumbers: false,
      },
    },
  };

  return videojs.obj.merge(options, ...srcOpt);
};

const getSourceType = (source: string) => {
  const fileExtension = source ? source.split("?")[0].split(".").pop() : "";
  if (fileExtension?.toLowerCase() === "mkv") {
    return "video/mp4";
  }
  return "";
};

const subLabel = (subUrl: string) => {
  let url: URL;
  try {
    url = new URL(subUrl);
  } catch {
    url = new URL(subUrl, window.location.origin);
  }

  const label = decodeURIComponent(
    url.pathname
      .split("/")
      .pop()!
      .replace(/\.[^/.]+$/, "")
  );

  return label;
};

interface LanguageImports {
  [key: string]: () => Promise<any>;
}

const languageImports: LanguageImports = {
  ar: () => import("video.js/dist/lang/ar.json"),
  bg: () => import("video.js/dist/lang/bg.json"),
  cs: () => import("video.js/dist/lang/cs.json"),
  de: () => import("video.js/dist/lang/de.json"),
  el: () => import("video.js/dist/lang/el.json"),
  en: () => import("video.js/dist/lang/en.json"),
  es: () => import("video.js/dist/lang/es.json"),
  fr: () => import("video.js/dist/lang/fr.json"),
  he: () => import("video.js/dist/lang/he.json"),
  hr: () => import("video.js/dist/lang/hr.json"),
  hu: () => import("video.js/dist/lang/hu.json"),
  it: () => import("video.js/dist/lang/it.json"),
  ja: () => import("video.js/dist/lang/ja.json"),
  ko: () => import("video.js/dist/lang/ko.json"),
  lv: () => import("video.js/dist/lang/lv.json"),
  nb: () => import("video.js/dist/lang/nb.json"),
  nl: () => import("video.js/dist/lang/nl.json"),
  "nl-be": () => import("video.js/dist/lang/nl.json"),
  pl: () => import("video.js/dist/lang/pl.json"),
  "pt-br": () => import("video.js/dist/lang/pt-BR.json"),
  "pt-pt": () => import("video.js/dist/lang/pt-PT.json"),
  ro: () => import("video.js/dist/lang/ro.json"),
  ru: () => import("video.js/dist/lang/ru.json"),
  sk: () => import("video.js/dist/lang/sk.json"),
  tr: () => import("video.js/dist/lang/tr.json"),
  uk: () => import("video.js/dist/lang/uk.json"),
  vi: () => import("video.js/dist/lang/vi.json"),
  "zh-cn": () => import("video.js/dist/lang/zh-CN.json"),
  "zh-tw": () => import("video.js/dist/lang/zh-TW.json"),
};
</script>

<style scoped>
/* 适配 Header 悬浮下的布局，不推荐写死 top: 64px */
.video-max {
  width: 100%;
  height: calc(100vh - 64px);
  margin-top: 64px;
}
</style>