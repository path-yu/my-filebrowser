<template>
  <div v-show="active" @click="closeHovers" class="overlay"></div>
  <nav :class="{ active, collapsed: sidebarCollapsed }">
    <!-- 折叠/展开按钮：iOS 风格，始终可见；收起时位于顶部居中 -->
    <button
      type="button"
      class="sidebar-collapse-btn"
      :aria-label="sidebarCollapsed ? '展开菜单' : '收起菜单'"
      :title="sidebarCollapsed ? '展开菜单' : '收起菜单'"
      @click="toggleSidebar"
    >
      <i class="material-icons">
        {{ sidebarCollapsed ? "chevron_right" : "chevron_left" }}
      </i>
    </button>

    <template v-if="isLoggedIn">
      <button
        @click="toAccountSettings"
        class="action"
        :title="user.username"
      >
        <i class="material-icons">person</i>
        <span>{{ user.username }}</span>
      </button>
      <button
        class="action"
        :class="{ active: isFilesActive }"
        @click="toRoot"
        :aria-label="$t('sidebar.myFiles')"
        :title="$t('sidebar.myFiles')"
      >
        <i class="material-icons">folder</i>
        <span>{{ $t("sidebar.myFiles") }}</span>
      </button>

      <div v-if="user.perm.create">
        <button
          @click="showHover('newDir')"
          class="action"
          :aria-label="$t('sidebar.newFolder')"
          :title="$t('sidebar.newFolder')"
        >
          <i class="material-icons">create_new_folder</i>
          <span>{{ $t("sidebar.newFolder") }}</span>
        </button>

        <button
          @click="showHover('newFile')"
          class="action"
          :aria-label="$t('sidebar.newFile')"
          :title="$t('sidebar.newFile')"
        >
          <i class="material-icons">note_add</i>
          <span>{{ $t("sidebar.newFile") }}</span>
        </button>
      </div>

      <!-- 所有登录用户都应该能进入"设置"（至少查看个人资料/修改密码/切换主题）。
           全局管理（用户管理/系统设置）在子菜单内部按 perm.admin v-if 控制。
           分享管理在子菜单内部按 perm.share v-if 控制。 -->
      <button
        class="action"
        :class="{ active: isSettingsActive }"
        @click="toAccountSettings"
        :aria-label="$t('sidebar.settings')"
        :title="$t('sidebar.settings')"
      >
        <i class="material-icons">settings_applications</i>
        <span>{{ $t("sidebar.settings") }}</span>
      </button>
      <button
        v-if="canLogout"
        @click="logout"
        class="action"
        id="logout"
        :aria-label="$t('sidebar.logout')"
        :title="$t('sidebar.logout')"
      >
        <i class="material-icons">exit_to_app</i>
        <span>{{ $t("sidebar.logout") }}</span>
      </button>
    </template>
    <template v-else>
      <router-link
        v-if="!hideLoginButton"
        class="action"
        to="/login"
        :aria-label="$t('sidebar.login')"
        :title="$t('sidebar.login')"
      >
        <i class="material-icons">exit_to_app</i>
        <span>{{ $t("sidebar.login") }}</span>
      </router-link>

      <router-link
        v-if="signup"
        class="action"
        to="/login"
        :aria-label="$t('sidebar.signup')"
        :title="$t('sidebar.signup')"
      >
        <i class="material-icons">person_add</i>
        <span>{{ $t("sidebar.signup") }}</span>
      </router-link>
    </template>

    <!-- iCloud 风格半圆存储容量仪表 -->
    <!-- <div
      class="credits storage-section"
      v-if="isFiles && !disableUsedPercentage"
    >
      <storage-gauge :val="usage.usedPercentage"></storage-gauge>
      <p class="storage-text">
        {{ $t("sidebar.diskUsed", { used: usage.used, total: usage.total }) }}
      </p>
    </div> -->

    <p class="credits">
      <span>
        <!-- 定制：去除外部 GitHub 链接，仅显示系统名称和版本 -->
        <span>文件管理系统</span>
        <span> {{ " " }} {{ version }}</span>
      </span>
      <span>
        <a @click="help">{{ $t("sidebar.help") }}</a>
      </span>
    </p>
  </nav>
</template>

<script>
import { reactive } from "vue";
import { mapActions, mapState } from "pinia";
import { useAuthStore } from "@/stores/auth";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";

import * as auth from "@/utils/auth";
import {
  version,
  signup,
  hideLoginButton,
  disableExternal,
  disableUsedPercentage,
  noAuth,
  logoutPage,
  loginPage,
} from "@/utils/constants";
import { files as api } from "@/api";
import ProgressBar from "@/components/ProgressBar.vue";
import StorageGauge from "@/components/StorageGauge.vue";
import prettyBytes from "pretty-bytes";

const USAGE_DEFAULT = { used: "0 B", total: "0 B", usedPercentage: 0 };

export default {
  name: "sidebar",
  setup() {
    const usage = reactive(USAGE_DEFAULT);
    return { usage, usageAbortController: new AbortController() };
  },
  components: {
    ProgressBar,
    StorageGauge,
  },
  inject: ["$showError"],
  computed: {
    ...mapState(useAuthStore, ["user", "isLoggedIn"]),
    ...mapState(useFileStore, ["isFiles", "reload"]),
    ...mapState(useLayoutStore, ["currentPromptName", "sidebarCollapsed"]),
    active() {
      return this.currentPromptName === "sidebar";
    },
    // 菜单选中态：由当前路由驱动
    isFilesActive() {
      return this.$route.path.startsWith("/files");
    },
    isSettingsActive() {
      return this.$route.path.startsWith("/settings");
    },
    signup: () => signup,
    hideLoginButton: () => hideLoginButton,
    version: () => version,
    disableExternal: () => disableExternal,
    disableUsedPercentage: () => disableUsedPercentage,
    canLogout: () => !noAuth && (loginPage || logoutPage !== "/login"),
  },
  methods: {
    ...mapActions(useLayoutStore, [
      "closeHovers",
      "showHover",
      "toggleSidebar",
    ]),
    abortOngoingFetchUsage() {
      this.usageAbortController.abort();
    },
    async fetchUsage() {
      const path = this.$route.path.endsWith("/")
        ? this.$route.path
        : this.$route.path + "/";
      let usageStats = USAGE_DEFAULT;
      if (this.disableUsedPercentage) {
        return Object.assign(this.usage, usageStats);
      }
      try {
        this.abortOngoingFetchUsage();
        this.usageAbortController = new AbortController();
        const usage = await api.usage(path, this.usageAbortController.signal);
        usageStats = {
          used: prettyBytes(usage.used, { binary: true }),
          total: prettyBytes(usage.total, { binary: true }),
          usedPercentage: Math.round((usage.used / usage.total) * 100),
        };
      } finally {
        return Object.assign(this.usage, usageStats);
      }
    },
    toRoot() {
      this.$router.push({ path: "/files" });
      this.closeHovers();
    },
    toAccountSettings() {
      this.$router.push({ path: "/settings/profile" });
      this.closeHovers();
    },
    toGlobalSettings() {
      this.$router.push({ path: "/settings/global" });
      this.closeHovers();
    },
    help() {
      this.showHover("help");
    },
    logout: auth.logout,
  },
  watch: {
    $route: {
      handler(to) {
        if (to.path.includes("/files")) {
          this.fetchUsage();
        }
      },
      immediate: true,
    },
  },
  unmounted() {
    this.abortOngoingFetchUsage();
  },
};
</script>
