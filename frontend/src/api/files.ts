import { useAuthStore } from "@/stores/auth";
import { useLayoutStore } from "@/stores/layout";
import { baseURL } from "@/utils/constants";
import { upload as postTus, useTus } from "./tus";
import { createURL, fetchURL, removePrefix, StatusError } from "./utils";
import { isEncodableResponse, makeRawResource } from "@/utils/encodings";
import {
  isMockEnabled,
  mockResourceResponse,
} from "./mockTimeFilter";

export interface ResourceQuery {
  modified_after?: number;
  modified_before?: number;
  [k: string]: string | number | boolean | undefined;
}

export async function fetch(
  url: string,
  signal?: AbortSignal,
  query?: ResourceQuery
) {
  const encoding = isEncodableResponse(url);
  url = removePrefix(url);

  // --------------------------------------------------------------------
  // Mock 开关：?mockTime=1 或 localStorage.mockTime==="1"
  // 用于本地验证时间筛选逻辑是否生效，无需启动真实 Go 后端
  // 行为：生成 7 个跨不同时间桶的 PDF，并且走与后端相同的
  //       modified_after / modified_before 过滤逻辑；
  //       子目录「存档」始终保留（符合后端 inModifiedRange 语义）。
  // --------------------------------------------------------------------
  if (isMockEnabled()) {
    // 模拟网络延迟 80ms，便于观察 loading 状态
    await new Promise((r) => setTimeout(r, 80));
    if (signal?.aborted) {
      throw new StatusError("000 No connection", 0, true);
    }

    const data = mockResourceResponse(url, new Date(), {
      modified_after: query?.modified_after,
      modified_before: query?.modified_before,
    });
    data.url = `/files${url}`;
    if (data.isDir) {
      if (!data.url.endsWith("/")) data.url += "/";
      data.items = data.items.map((item: any, index: any) => {
        item.index = index;
        item.url = `${data.url}${encodeURIComponent(item.name)}`;
        if (item.isDir) {
          item.url += "/";
        }
        return item;
      });
    }
    return data;
  }

  let endpoint = `/api/resources${url}`;
  if (query) {
    const qp: string[] = [];
    for (const [k, v] of Object.entries(query)) {
      if (v === undefined || v === null || v === "") continue;
      qp.push(
        `${encodeURIComponent(k)}=${encodeURIComponent(String(v))}`
      );
    }
    if (qp.length) endpoint += "?" + qp.join("&");
  }
  const res = await fetchURL(endpoint, {
    signal,
    headers: {
      "X-Encoding": encoding ? "true" : "false",
    },
  });

  let data: Resource;
  try {
    if (res.headers.get("Content-Type") == "application/octet-stream") {
      data = await makeRawResource(res, url);
    } else {
      data = (await res.json()) as Resource;
    }
  } catch (e) {
    // Check if the error is an intentional cancellation
    if (e instanceof Error && e.name === "AbortError") {
      throw new StatusError("000 No connection", 0, true);
    }
    throw e;
  }
  data.url = `/files${url}`;

  if (data.isDir) {
    if (!data.url.endsWith("/")) data.url += "/";
    // Perhaps change the any
    data.items = data.items.map((item: any, index: any) => {
      item.index = index;
      item.url = `${data.url}${encodeURIComponent(item.name)}`;

      if (item.isDir) {
        item.url += "/";
      }

      return item;
    });
  }

  return data;
}

export async function fetchAll(url: string): Promise<RecursiveEntry[]> {
  url = removePrefix(url);
  const res = await fetchURL(`/api/resources/recursive${url}`, {});
  return (await res.json()) as RecursiveEntry[];
}

async function resourceAction(url: string, method: ApiMethod, content?: any) {
  url = removePrefix(url);

  const opts: ApiOpts = {
    method,
  };

  if (content) {
    opts.body = content;
  }

  const res = await fetchURL(`/api/resources${url}`, opts);

  return res;
}

export async function remove(url: string) {
  return resourceAction(url, "DELETE");
}

export async function put(url: string, content = "") {
  return resourceAction(url, "PUT", content);
}

export function download(format: any, ...files: string[]) {
  let url = `${baseURL}/api/raw`;

  if (files.length === 1) {
    url += removePrefix(files[0]) + "?";
  } else {
    let arg = "";

    for (const file of files) {
      arg += removePrefix(file) + ",";
    }

    arg = arg.substring(0, arg.length - 1);
    arg = encodeURIComponent(arg);
    url += `/?files=${arg}&`;
  }

  if (format) {
    url += `algo=${format}&`;
  }

  window.open(url);
}

export async function post(
  url: string,
  content: ApiContent = "",
  overwrite = false,
  onupload: any = () => {}
) {
  // Use the pre-existing API if:
  const useResourcesApi =
    // a folder is being created
    url.endsWith("/") ||
    // We're not using http(s)
    (content instanceof Blob &&
      !["http:", "https:"].includes(window.location.protocol)) ||
    // Tus is disabled / not applicable
    !(await useTus(content));
  return useResourcesApi
    ? postResources(url, content, overwrite, onupload)
    : postTus(url, content, overwrite, onupload);
}

async function postResources(
  url: string,
  content: ApiContent = "",
  overwrite = false,
  onupload: any
) {
  url = removePrefix(url);

  let bufferContent: ArrayBuffer;
  if (
    content instanceof Blob &&
    !["http:", "https:"].includes(window.location.protocol)
  ) {
    bufferContent = await new Response(content).arrayBuffer();
  }

  const authStore = useAuthStore();
  return new Promise((resolve, reject) => {
    const request = new XMLHttpRequest();
    request.open(
      "POST",
      `${baseURL}/api/resources${url}?override=${overwrite}`,
      true
    );
    request.setRequestHeader("X-Auth", authStore.jwt);

    if (typeof onupload === "function") {
      request.upload.onprogress = onupload;
    }

    request.onload = () => {
      if (request.status === 200) {
        resolve(request.responseText);
      } else if (request.status === 409) {
        reject(new Error(request.status.toString()));
      } else {
        reject(new Error(request.responseText));
      }
    };

    request.onerror = () => {
      reject(new Error("001 Connection aborted"));
    };

request.send((bufferContent || content) as XMLHttpRequestBodyInit);
  });
}

function moveCopy(
  items: any[],
  copy = false,
  overwrite = false,
  rename = false
) {
  const layoutStore = useLayoutStore();
  const promises = [];

  for (const item of items) {
    const from = item.from;
    const to = encodeURIComponent(removePrefix(item.to ?? ""));
    const finalOverwrite =
      item.overwrite == undefined ? overwrite : item.overwrite;
    const finalRename = item.rename == undefined ? rename : item.rename;
    const url = `${from}?action=${
      copy ? "copy" : "rename"
    }&destination=${to}&override=${finalOverwrite}&rename=${finalRename}`;
    promises.push(resourceAction(url, "PATCH"));
  }
  layoutStore.closeHovers();
  return Promise.all(promises);
}

export function move(items: any[], overwrite = false, rename = false) {
  return moveCopy(items, false, overwrite, rename);
}

export function copy(items: any[], overwrite = false, rename = false) {
  return moveCopy(items, true, overwrite, rename);
}

export async function checksum(url: string, algo: ChecksumAlg) {
  const data = await resourceAction(`${url}?checksum=${algo}`, "GET");
  return (await data.json()).checksums[algo];
}

/** 保证路径以 "/" 开头，避免与前缀（如 "api/raw"）直接拼接成错误的 "api/rawfilename" */
function ensureLeadingSlash(p: string): string {
  if (!p) return "/";
  return p.startsWith("/") ? p : "/" + p;
}

export function getDownloadURL(file: ResourceItem, inline: any) {
  const params = {
    ...(inline && { inline: "true" }),
  };

  return createURL("api/raw" + ensureLeadingSlash(file.path), params);
}

export function getPreviewURL(file: ResourceItem, size: string) {
  const params = {
    inline: "true",
    key: Date.parse(file.modified),
  };

  return createURL("api/preview/" + size + ensureLeadingSlash(file.path), params);
}

export function getSubtitlesURL(file: ResourceItem) {
  const params = {
    inline: "true",
  };

  return file.subtitles?.map((d) => createURL("api/subtitle" + ensureLeadingSlash(d), params));
}

export async function usage(url: string, signal: AbortSignal) {
  url = removePrefix(url);

  const res = await fetchURL(`/api/usage${url}`, { signal });

  try {
    return await res.json();
  } catch (e) {
    // Check if the error is an intentional cancellation
    if (e instanceof Error && e.name == "AbortError") {
      throw new StatusError("000 No connection", 0, true);
    }
    throw e;
  }
}
