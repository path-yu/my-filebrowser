import { useAuthStore } from "@/stores/auth";
import { renew, logout, ensureAuthCookie } from "@/utils/auth";
import { baseURL } from "@/utils/constants";
import { encodePath } from "@/utils/url";

export class StatusError extends Error {
  public readonly bodyText?: string;
  public readonly body?: Record<string, any>;
  public readonly hint?: string;
  constructor(
    message: any,
    public status?: number,
    public is_canceled?: boolean,
    bodyText?: string
  ) {
    super(message);
    this.name = "StatusError";
    this.bodyText = bodyText;
    if (bodyText) {
      try {
        const parsed = JSON.parse(bodyText);
        if (parsed && typeof parsed === "object") {
          this.body = parsed as Record<string, any>;
          const errMsg = parsed.error || parsed.message;
          if (typeof errMsg === "string") {
            // 替换 message 为后端真正写的中文错误（更清晰）
            this.message = errMsg;
          }
          if (typeof parsed.hint === "string") {
            this.hint = parsed.hint;
          }
        }
      } catch {
        /* body 不是 JSON：按原样保留 bodyText 给前端判断 */
      }
    }
  }
}

export async function fetchURL(
  url: string,
  opts: ApiOpts,
  auth = true
): Promise<Response> {
  const authStore = useAuthStore();

  // Make sure the `auth` cookie stays in sync with localStorage JWT on
  // EVERY API call. This guarantees <img src="/api/preview/..."> (which
  // can only carry its auth via cookie) never runs into a 401 just because
  // the cookie got dropped / wasn't rewritten after a SameSite flag change.
  if (auth) {
    ensureAuthCookie();
  }

  opts = opts || {};
  opts.headers = opts.headers || {};

  const { headers, ...rest } = opts;
  let res;
  try {
    res = await fetch(`${baseURL}${url}`, {
      headers: {
        "X-Auth": authStore.jwt,
        ...headers,
      },
      ...rest,
    });
  } catch (e) {
    // Check if the error is an intentional cancellation
    if (e instanceof Error && e.name === "AbortError") {
      throw new StatusError("000 No connection", 0, true);
    }
    throw new StatusError("000 No connection", 0);
  }

  if (auth && res.headers.get("X-Renew-Token") === "true") {
    await renew(authStore.jwt);
  }

  if (res.status < 200 || res.status > 299) {
    const body = await res.text();
    const httpStatusMsg = body ? `${res.status} ${res.statusText}` : `${res.status} ${res.statusText}`;
    let msg = body || httpStatusMsg;
    // 优先用 body 里的后端中文错误文本（由 StatusError 构造函数再做 JSON 解析提取 .error/.hint）
    const error = new StatusError(msg, res.status, undefined, body);

    if (auth && res.status == 401) {
      logout();
    }

    throw error;
  }

  return res;
}

export async function fetchJSON<T>(url: string, opts?: any): Promise<T> {
  const res = await fetchURL(url, opts);

  if (res.status === 200) {
    return res.json() as Promise<T>;
  }

  throw new StatusError(`${res.status} ${res.statusText}`, res.status);
}

export function removePrefix(url: string): string {
  url = url.split("/").splice(2).join("/");

  if (url === "") url = "/";
  if (url[0] !== "/") url = "/" + url;
  return url;
}

export function createURL(endpoint: string, searchParams: Record<string, any> = {}): string {
  // ---- Why we STOPPED using `new URL(prefix + encodePath(endpoint), origin)`? ----
  // Browser's URL constructor silently DECODES %5B → [ and %5D → ] in the *path*
  // portion because RFC 3986 lists "[]" as gen-delims reserved for IPv6-literal
  // hostnames. The resulting URL.toString() would then contain BARE brackets in
  // the path segment, and Vite's http-proxy-middleware / Node http.request /
  // Go net/http would any of them:
  //   - treat `[foo]` as malformed → 403/400
  //   - mis-parse the path (e.g. strip trailing segments) → wrong file → 404
  //   - fall back to the SPA NotFoundHandler → serve index.html for an <img>
  // So we build the URL by hand as a plain string to keep percent-encoded
  // delimiters exactly as `encodePath` produced them.
  // ------------------------------------------------------------------------------
  const encodedPath = encodePath(endpoint)
    // Belt-and-braces: never allow a raw `[` or `]` to leak into the URL path,
    // even if a future change to `encodePath` accidentally skips them.
    .replace(/\[/g, "%5B")
    .replace(/\]/g, "%5D");

  let prefix = baseURL;
  if (prefix.endsWith("/")) {
    prefix = prefix.slice(0, -1);
  }

  let pathPart = encodedPath;
  if (!pathPart.startsWith("/")) {
    pathPart = "/" + pathPart;
  }

  const originBase = typeof origin !== "undefined" ? origin : "";
  let out = `${originBase}${prefix}${pathPart}`;

  const sp = new URLSearchParams();
  for (const k of Object.keys(searchParams)) {
    const v = searchParams[k];
    if (v === undefined || v === null || v === "") continue;
    sp.append(k, String(v));
  }
  const qs = sp.toString();
  if (qs) {
    out += "?" + qs;
  }
  return out;
}

export function setSafeTimeout(callback: () => void, delay: number): number {
  const MAX_DELAY = 86_400_000;
  let remaining = delay;

  function scheduleNext(): number {
    if (remaining <= MAX_DELAY) {
      return window.setTimeout(callback, remaining);
    } else {
      return window.setTimeout(() => {
        remaining -= MAX_DELAY;
        scheduleNext();
      }, MAX_DELAY);
    }
  }

  return scheduleNext();
}
