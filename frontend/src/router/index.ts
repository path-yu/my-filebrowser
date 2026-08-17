import type { RouteLocation } from "vue-router";
import { createRouter, createWebHistory } from "vue-router";
import Login from "@/views/Login.vue";
import Layout from "@/views/Layout.vue";
import Files from "@/views/Files.vue";
import Share from "@/views/Share.vue";
import Errors from "@/views/Errors.vue";
import { useAuthStore } from "@/stores/auth";
import { useFileStore } from "@/stores/file";
import { baseURL, name } from "@/utils/constants";
import i18n from "@/i18n";
import { recaptcha, loginPage } from "@/utils/constants";
import { login, validateLogin } from "@/utils/auth";

// 设置类页面不在首屏关键路径上，且自带较重依赖（ace 主题列表、qrcode 等），
// 全部按需加载，缩小首屏 bundle。
const Settings = () => import("@/views/Settings.vue");
const ProfileSettings = () => import("@/views/settings/Profile.vue");
const Shares = () => import("@/views/settings/Shares.vue");
const GlobalSettings = () => import("@/views/settings/Global.vue");
const Users = () => import("@/views/settings/Users.vue");
const User = () => import("@/views/settings/User.vue");

const titles = {
  Login: "sidebar.login",
  Share: "buttons.share",
  Files: "files.files",
  Settings: "sidebar.settings",
  ProfileSettings: "settings.profileSettings",
  Shares: "settings.shareManagement",
  GlobalSettings: "settings.globalSettings",
  Users: "settings.users",
  User: "settings.user",
  Forbidden: "errors.forbidden",
  NotFound: "errors.notFound",
  InternalServerError: "errors.internal",
};

const routes = [
  {
    path: "/login",
    name: "Login",
    component: Login,
  },
  {
    path: "/share",
    component: Layout,
    children: [
      {
        path: ":path*",
        name: "Share",
        component: Share,
      },
    ],
  },
  {
    path: "/files",
    component: Layout,
    meta: {
      requiresAuth: true,
    },
    children: [
      {
        path: ":path*",
        name: "Files",
        component: Files,
      },
    ],
  },
  {
    path: "/settings",
    component: Layout,
    meta: {
      requiresAuth: true,
    },
    children: [
      {
        path: "",
        name: "Settings",
        component: Settings,
        redirect: {
          path: "/settings/profile",
        },
        children: [
          {
            path: "profile",
            name: "ProfileSettings",
            component: ProfileSettings,
          },
          {
            path: "shares",
            name: "Shares",
            component: Shares,
          },
          {
            path: "global",
            name: "GlobalSettings",
            component: GlobalSettings,
            meta: {
              requiresAdmin: true,
            },
          },
          {
            path: "users",
            name: "Users",
            component: Users,
            meta: {
              requiresAdmin: true,
            },
          },
          {
            path: "users/:id",
            name: "User",
            component: User,
            meta: {
              requiresAdmin: true,
            },
          },
        ],
      },
    ],
  },
  {
    path: "/403",
    name: "Forbidden",
    component: Errors,
    props: {
      errorCode: 403,
      showHeader: true,
    },
  },
  {
    path: "/404",
    name: "NotFound",
    component: Errors,
    props: {
      errorCode: 404,
      showHeader: true,
    },
  },
  {
    path: "/500",
    name: "InternalServerError",
    component: Errors,
    props: {
      errorCode: 500,
      showHeader: true,
    },
  },
  {
    path: "/:catchAll(.*)*",
    redirect: (to: RouteLocation) => {
      const catchAll = to.params.catchAll;
      if (!catchAll) return "/files/";
      return `/files/${Array.isArray(catchAll) ? catchAll.join("/") : catchAll}`;
    },
  },
];

async function initAuth() {
  if (loginPage) {
    await validateLogin();
  } else {
    await login("", "", "");
  }

  if (recaptcha) {
    await new Promise<void>((resolve) => {
      const check = () => {
        if (typeof window.grecaptcha === "undefined") {
          setTimeout(check, 100);
        } else {
          resolve();
        }
      };

      check();
    });
  }
}

const router = createRouter({
  history: createWebHistory(baseURL),
  routes,
});

router.beforeResolve(async (to, from) => {
  const title = i18n.global.t(titles[to.name as keyof typeof titles]);
  document.title = title + " - " + name;

  const authStore = useAuthStore();

  // this will only be null on first route
  if (from.name == null) {
    try {
      await initAuth();
    } catch (error) {
      console.error(error);
    }
  }

  if (to.path.endsWith("/login") && authStore.isLoggedIn) {
    return { path: "/files/" };
  }

  if (to.matched.some((record) => record.meta.requiresAuth)) {
    if (!authStore.isLoggedIn) {
      return {
        path: "/login",
        query: { redirect: to.fullPath },
      };
    }

    if (to.matched.some((record) => record.meta.requiresAdmin)) {
      if (authStore.user === null || !authStore.user.perm.admin) {
        return { path: "/403" };
      }
    }
  }

  return true;
});

/** 判断 to.fullPath 指向的是不是「文件预览页」而不是目录列表页。
 *  启发式规则：
 *   - 必须在 /files 路由下
 *   - 路径不能以 "/" 结尾（目录结尾是 "/"）
 *   - 路径最后一段文件名的尾缀匹配「常见文件扩展名」白名单（1-6 位字母数字，常见 pdf/docx/png/mp4 等）
 *     → 目录名虽然偶尔也会带点（例如 v1.0），但几乎不会以 ".pdf" ".docx" 这类扩展名结尾。*/
function isPreviewPath(fullPath: string): boolean {
  // 去掉 query + hash 部分，只看 path
  const qIdx = fullPath.indexOf("?");
  const hIdx = fullPath.indexOf("#");
  const sliceEnd =
    qIdx >= 0
      ? hIdx >= 0
        ? Math.min(qIdx, hIdx)
        : qIdx
      : hIdx >= 0
        ? hIdx
        : fullPath.length;
  const path = fullPath.slice(0, sliceEnd);
  if (!path.startsWith("/files")) return false;
  if (path.endsWith("/")) return false;
  const lastSlash = path.lastIndexOf("/");
  const fileName = lastSlash >= 0 ? path.slice(lastSlash + 1) : path;
  if (!fileName) return false;
  // 白名单常见扩展名：文档、图片、音视频、压缩包、代码、电子书、纯文本
  const COMMON_EXT_RE =
    /\.(pdf|docx?|xlsx?|pptx?|vsd|vsdx|txt|md|csv|rtf|odt|ods|odp|epub|mobi|azw|azw3|png|jpe?g|gif|bmp|webp|svg|tif|tiff|ico|raw|cr2|nef|arw|dng|mp3|wav|flac|aac|ogg|m4a|ape|opus|wma|mp4|avi|mov|mkv|wmv|flv|webm|m4v|3gp|ts|mpeg|mpg|zip|rar|7z|tar|gz|tgz|bz2|xz|iso|json|ya?ml|xml|html?|css|jsx?|tsx?|vue|py|go|rs|java|c|cc|cpp|h|hpp|cs|rb|php|swift|kt|scala|sh|bat|ps1|sql|asm|r|mat|m|f|f90|lua|pl|dart|ex|exs|clj|hs|lhs|elm|ejs|vue)$/i;
  return COMMON_EXT_RE.test(fileName);
}

const PREVIEW_BACK_KEY = "fb:previewBack";

/** 导航到预览页「之前」：强制把返回目标写好。
 *  解决一个非常高频的真实场景：用户先点某行（选中，触发 router.replace 写 ?sel=）
 *  → 紧接着在同一文件上双击（触发 router.push 跳预览）。Vue Router 4 的规则是：
 *  「前一个 replace 导航还未完成就来了新的 push → 取消 replace」，结果 URL 里的 sel
 *  永远写不进去，afterEach 也就记录不到带 sel 的 fullPath。
 *  所以这里在进入预览页的最后一刻，直接从 Pinia store 读 fileStore.selected（它是
 *  选中状态的唯一真源），强制编码成 sel 写进 sessionStorage，不管 URL 有没有。 */
router.beforeEach((to, from) => {
  if (!isPreviewPath(to.fullPath) || isPreviewPath(from.fullPath)) {
    return; // 只关心「从非预览 → 预览」的边界
  }
  try {
    const query: Record<string, any> = { ...from.query };
    const fileStore = useFileStore();
    // 必须区分搜索模式：搜索模式下可见列表是 searchResults，不是 req.items；
    // 否则用错数组按索引映射 → 文件名对不上 → sel 编码错误。
    const items = fileStore.searchMode
      ? fileStore.searchResults
      : fileStore.req?.items;
    const selected = fileStore.selected;
    if (Array.isArray(items) && Array.isArray(selected) && selected.length > 0) {
      const names = selected
        .map((idx) => items[idx])
        .filter((it: any) => !!it && typeof it.name === "string" && it.name !== "")
        .map((it: any) => it.name as string);
      if (names.length > 0) {
        /** 这里的拼接/编码与 Files.vue encodeSel 保持一致：先逐个 encodeURIComponent 再 join(","),
         *  然后再整体 decodeURIComponent 一次再交给 router.resolve，
         *  因为 router.resolve 生成 fullPath 时会再 encode value 一次，避免双重 encode 出 %25。 */
        const encoded = names.map((n) => encodeURIComponent(n)).join(",");
        if (encoded) {
          try {
            query.sel = decodeURIComponent(encoded);
          } catch {
            query.sel = encoded;
          }
        }
      }
    } else if (Array.isArray(selected) && selected.length === 0) {
      delete query.sel;
    }
    const resolved = router.resolve({
      path: from.path,
      query: Object.fromEntries(
        Object.entries(query).filter(([, v]) => v !== undefined && v !== null && v !== "")
      ),
      hash: from.hash,
    });
    sessionStorage.setItem(PREVIEW_BACK_KEY, resolved.fullPath);
  } catch {
    /* store 初始化未完成 / sessionStorage 不可用 → 忽略，afterEach 另有兜底 */
  }
});

/** 每次路由落地后，如果这一页不是「文件预览页」，就把它的 fullPath 写进 sessionStorage。
 *  这样用户从「/files/图纸/?q=CQG50」点进 PDF 预览 → PDF 页关闭（×）时，
 *  可以精确跳回到原来的列表页，完整保留 ?q / scope / sel 等所有 query 参数。
 *  用 sessionStorage：刷新预览页不丢，关 tab 自动清理（符合会话语义）。
 *  注意：这是「非预览页跳转」的兜底记录，上面那个 beforeEach 已经在进入预览前
 *        用 fileStore.selected 强制写过一次（更精确），两者配合不冲突。*/
router.afterEach((to) => {
  try {
    if (!isPreviewPath(to.fullPath)) {
      sessionStorage.setItem(PREVIEW_BACK_KEY, to.fullPath);
    }
  } catch {
    /* sessionStorage 不可用时忽略（隐私模式等），Preview.close 另有兜底 */
  }
});

export { router, router as default };
