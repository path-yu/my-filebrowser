<template>
  <div id="login" :class="{ recaptcha: recaptcha }">
    <form @submit="submit">
      <LazyImage eager :src="logoURL" alt="文件管理系统" class="login-logo" />
      <h1 class="">{{ name }}</h1>
      <p v-if="reason != null" class="logout-message">
        {{ t(`login.logout_reasons.${reason}`) }}
      </p>
      <div v-if="error !== ''" class="wrong">{{ error }}</div>

      <input
        autofocus
        class="input input--block"
        type="text"
        autocapitalize="off"
        v-model="username"
        :placeholder="t('login.username')"
      />
      <input
        class="input input--block"
        type="password"
        v-model="password"
        :placeholder="t('login.password')"
      />
      <input
        class="input input--block"
        v-if="createMode"
        type="password"
        v-model="passwordConfirm"
        :placeholder="t('login.passwordConfirm')"
      />

      <div v-if="recaptcha" id="recaptcha"></div>
      <input
        class="button button--block"
        type="submit"
        :value="createMode ? t('login.signup') : t('login.submit')"
      />

      <p @click="toggleMode" v-if="signup">
        {{ createMode ? t("login.loginInstead") : t("login.createAnAccount") }}
      </p>
    </form>
  </div>
</template>

<script setup lang="ts">
import { StatusError } from "@/api/utils";
import * as auth from "@/utils/auth";
import LazyImage from "@/components/files/LazyImage.vue";
import {
  name,
  logoURL,
  recaptcha,
  recaptchaKey,
  signup,
} from "@/utils/constants";
import { inject, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";

// Define refs
const createMode = ref<boolean>(false);
const error = ref<string>("");
const username = ref<string>("");
const password = ref<string>("");
const passwordConfirm = ref<string>("");

const route = useRoute();
const router = useRouter();
const { t } = useI18n({});
console.log(t);
// Define functions
const toggleMode = () => (createMode.value = !createMode.value);

const $showError = inject<IToastError>("$showError")!;

const reason = route.query["logout-reason"] ?? null;

const submit = async (event: Event) => {
  event.preventDefault();
  event.stopPropagation();

  const redirect = (route.query.redirect || "/files/") as string;

  let captcha = "";
  if (recaptcha) {
    captcha = window.grecaptcha.getResponse();

    if (captcha === "") {
      error.value = t("login.wrongCredentials");
      return;
    }
  }

  if (createMode.value) {
    if (password.value !== passwordConfirm.value) {
      error.value = t("login.passwordsDontMatch");
      return;
    }
  }

  try {
    if (createMode.value) {
      await auth.signup(username.value, password.value);
    }

    await auth.login(username.value, password.value, captcha);
    router.push({ path: redirect });
  } catch (e: any) {
    // console.error(e);
    if (e instanceof StatusError) {
      if (e.status === 409) {
        error.value = t("login.usernameTaken");
      } else if (e.status === 403) {
        error.value = t("login.wrongCredentials");
      } else if (e.status === 400) {
        const match = e.message.match(/minimum length is (\d+)/);
        if (match) {
          error.value = t("login.passwordTooShort", { min: match[1] });
        } else {
          error.value = e.message;
        }
      } else {
        $showError(e);
      }
    }
  }
};

// Run hooks
onMounted(() => {
  // 定制：URL 参数自动登录（必须在 recaptcha 判断之前执行）
  // 支持 ?u=admin&p=123456 或 ?username=admin&password=123456
  // 可选 ?redirect=/files/ 指定跳转目标
  autoLoginFromURL();

  if (!recaptcha) return;

  window.grecaptcha.ready(function () {
    window.grecaptcha.render("recaptcha", {
      sitekey: recaptchaKey,
    });
  });
});

const autoLoginFromURL = async () => {
  const q = route.query;
  let u = q.u || q.username;
  let p = q.p || q.password;

  // 兼容从根路径被重定向过来的场景：
  // /?u=admin&p=123456 → /login?redirect=/files/?u=admin%26p=123456
  if (!u && q.redirect) {
    try {
      const url = new URL(String(q.redirect), window.location.origin);
      u = url.searchParams.get("u") || url.searchParams.get("username");
      p = url.searchParams.get("p") || url.searchParams.get("password");
    } catch {
      // 忽略解析失败
    }
  }

  if (!u || !p) return;

  username.value = String(u);
  password.value = String(p);

  const redirect = (q.redirect || "/files/") as string;

  try {
    await auth.login(String(u), String(p), "");
    // 跳转目标去掉 URL 中的敏感参数（避免地址栏残留明文密码）
    const cleanRedirect = String(redirect).split("?")[0] || "/files/";
    router.push({ path: cleanRedirect });
  } catch (e: any) {
    if (e instanceof StatusError) {
      error.value = t("login.wrongCredentials");
    }
  }
};
</script>
<style scoped>
#login h1{
 display: inline;
 padding-left: 20px;
}
</style>
