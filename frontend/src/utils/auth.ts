import { useAuthStore } from "@/stores/auth";
import router from "@/router";
import type { JwtPayload } from "jwt-decode";
import { jwtDecode } from "jwt-decode";
import { authMethod, baseURL, noAuth, logoutPage } from "./constants";
import { StatusError } from "@/api/utils";
import { setSafeTimeout } from "@/api/utils";

/**
 * Extract the JWT payload.exp (milliseconds) from a JWT string.
 * Returns null if the token is falsy or malformed.
 */
function readExpMs(token: string): number | null {
  try {
    const payload = jwtDecode<{ exp?: number }>(token);
    return payload.exp ? payload.exp * 1000 : null;
  } catch {
    return null;
  }
}

/**
 * Read the raw value of the `auth` cookie (if any). Returns "" if missing.
 */
export function readAuthCookie(): string {
  if (typeof document === "undefined") return "";
  const match = document.cookie.match(/(?:^|;\s*)auth=([^;]*)/);
  return match ? decodeURIComponent(match[1]) : "";
}

/**
 * Ensures a SameSite=Lax + Expires `auth` cookie is written whenever the
 * localStorage `jwt` exists. Runs at every API call and every LazyImage
 * preload so we never serve `<img src>` GET requests without a cookie.
 *
 * Returns the JWT that was written / already present.
 */
export function ensureAuthCookie(): string {
  if (typeof document === "undefined" || typeof localStorage === "undefined") return "";
  const token = localStorage.getItem("jwt") ?? "";
  if (!token) return "";
  const cookieToken = readAuthCookie();
  if (cookieToken === token) return token; // already in sync, short-circuit

  const expMs = readExpMs(token);
  const expiresPart = expMs ? `Expires=${new Date(expMs).toUTCString()}; ` : "";
  const secureFlag =
    typeof window !== "undefined" && window.location.protocol === "https:"
      ? "Secure; "
      : "";
  document.cookie = `auth=${token}; Path=/; SameSite=Lax; ${secureFlag}${expiresPart}`;
  return token;
}

export function parseToken(token: string) {
  // falsy or malformed jwt will throw InvalidTokenError
  const data = jwtDecode<JwtPayload & { user: IUser }>(token);

  // SameSite=Lax（不是 Strict）：
  //   - 图片 <img src>、<a href> 跳转、用户刷新、从收藏夹打开均发送 cookie
  //   - Strict 在用户点击外部链/某些 DevTools 新标签场景会丢弃 cookie，
  //     导致 /api/preview 缩略图/原始大图 GET 401。
  // Expires 使用 JWT 的 exp，让 cookie 与 token 同生命周期，
  // 避免"会话 cookie"语义让浏览器关闭时丢失 auth（配合 30 天长token 持久登录）。
  const expiresAt = new Date(data.exp! * 1000);
  const secureFlag =
    typeof window !== "undefined" && window.location.protocol === "https:"
      ? "Secure; "
      : "";
  document.cookie = `auth=${token}; Path=/; SameSite=Lax; ${secureFlag}Expires=${expiresAt.toUTCString()};`;

  localStorage.setItem("jwt", token);

  const authStore = useAuthStore();
  authStore.jwt = token;
  authStore.setUser(data.user);

  // proxy auth with custom logout subject to unknown external timeout
  if (logoutPage !== "/login" && authMethod === "proxy") {
    console.warn("idle timeout disabled with proxy auth and custom logout");
    return;
  }

  if (authStore.logoutTimer) {
    clearTimeout(authStore.logoutTimer);
  }

  const timeout = expiresAt.getTime() - Date.now();
  authStore.setLogoutTimer(
    setSafeTimeout(() => {
      logout("inactivity");
    }, timeout)
  );
}

export async function validateLogin() {
  try {
    if (localStorage.getItem("jwt")) {
      await renew(<string>localStorage.getItem("jwt"));
    }
  } catch (error) {
    console.warn("Invalid JWT token in storage");
    throw error;
  }
}

export async function login(
  username: string,
  password: string,
  recaptcha: string
) {
  const data = { username, password, recaptcha };

  const res = await fetch(`${baseURL}/api/login`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(data),
  });

  const body = await res.text();

  if (res.status === 200) {
    parseToken(body);
  } else {
    throw new StatusError(
      body || `${res.status} ${res.statusText}`,
      res.status
    );
  }
}

export async function renew(jwt: string) {
  const res = await fetch(`${baseURL}/api/renew`, {
    method: "POST",
    headers: {
      "X-Auth": jwt,
    },
  });

  const body = await res.text();

  if (res.status === 200) {
    parseToken(body);
  } else {
    throw new StatusError(
      body || `${res.status} ${res.statusText}`,
      res.status
    );
  }
}

export async function signup(username: string, password: string) {
  const data = { username, password };

  const res = await fetch(`${baseURL}/api/signup`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(data),
  });

  if (res.status !== 200) {
    const body = await res.text();
    throw new StatusError(
      body || `${res.status} ${res.statusText}`,
      res.status
    );
  }
}

export function logout(reason?: string) {
  document.cookie = "auth=; Max-Age=0; Path=/; SameSite=Lax;";

  const authStore = useAuthStore();
  authStore.clearUser();

  localStorage.setItem("jwt", "");
  if (noAuth) {
    window.location.reload();
  } else if (logoutPage !== "/login") {
    document.location.href = `${logoutPage}`;
  } else {
    if (typeof reason === "string" && reason.trim() !== "") {
      router.push({
        path: "/login",
        query: { "logout-reason": reason },
      });
    } else {
      router.push({
        path: "/login",
      });
    }
  }
}
