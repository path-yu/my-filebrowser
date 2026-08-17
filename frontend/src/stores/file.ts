import { defineStore } from "pinia";

export const useFileStore = defineStore("file", {
  state: (): {
    req: Resource | null;
    oldReq: Resource | null;
    reload: boolean;
    selected: number[];
    multiple: boolean;
    isFiles: boolean;
    preselect: string | null;
    searchMode: boolean;
    searchQuery: string;
    searchResults: ResourceItem[];
    productCodes: Record<string, string>;
  } => ({
    req: null,
    oldReq: null,
    reload: false,
    selected: [],
    multiple: false,
    isFiles: false,
    preselect: null,
    searchMode: false,
    searchQuery: "",
    searchResults: [],
    productCodes: {},
  }),
  getters: {
    selectedCount: (state) => state.selected.length,
    isListing: (state) => {
      return state.isFiles && state?.req?.isDir;
    },
    visibleItems: (state): ResourceItem[] => {
      if (state.searchMode) {
        return state.searchResults;
      }
      return state.req?.items ?? [];
    },
    visibleNumDirs(): number {
      return this.visibleItems.filter((i: ResourceItem) => i.isDir).length;
    },
    visibleNumFiles(): number {
      return this.visibleItems.filter((i: ResourceItem) => !i.isDir).length;
    },
  },
  actions: {
    toggleMultiple() {
      this.multiple = !this.multiple;
    },
    updateRequest(value: Resource | null) {
      const selectedItems = this.selected.map((i) => this.req?.items[i]);
      this.oldReq = this.req;
      this.req = value;

      this.selected = [];

      if (!this.req?.items) return;
      this.selected = this.req.items
        .filter((item) =>
          selectedItems.some((rItem) => rItem?.url === item.url)
        )
        .map((item) => item.index);
    },
    removeSelected(value: any) {
      const i = this.selected.indexOf(value);
      if (i === -1) return;
      this.selected.splice(i, 1);
    },
    /** 进入搜索模式，清空结果；之后用 appendSearchResult 追加 */
    setSearchResults(query: string, results: ResourceItem[]) {
      this.searchQuery = query;
      this.searchMode = true;
      this.selected = [];
      this.searchResults = results.map((r, idx) => this.normalize(r, idx));
    },
    appendSearchResult(item: any) {
      const idx = this.searchResults.length;
      this.searchResults.push(this.normalize(item, idx));
    },
    clearSearch() {
      this.searchMode = false;
      this.searchQuery = "";
      this.searchResults = [];
      this.selected = [];
    },
    /** 给搜索返回的 item 补 index 与默认字段，避免渲染时缺字段崩 */
    normalize(item: any, idx: number): ResourceItem {
      // 搜索 API 只用了 "dir" 字段（后端 http/search.go），这里兼容映射
      const isDir: boolean =
        typeof item.isDir === "boolean" ? item.isDir : !!item.dir;
      const rawPath: string = item.path ?? "";
      // 规范化路径：始终以 "/" 开头，避免后续拼接 "api/raw" + "filename" → "api/rawfilename"
      const normalizedPath =
        rawPath === "" ? "/" : rawPath.startsWith("/") ? rawPath : "/" + rawPath;
      const rawName: string =
        item.name ??
        (rawPath
          ? (() => {
              const p = rawPath.replace(/\/+$/, "");
              const slash = p.lastIndexOf("/");
              return slash >= 0 ? p.slice(slash + 1) : p;
            })()
          : "");
      const ext = (() => {
        if (typeof item.extension === "string" && item.extension !== "")
          return item.extension;
        const dot = rawName.lastIndexOf(".");
        if (dot > 0 && dot < rawName.length - 1) return rawName.slice(dot + 1);
        return "";
      })();
      return {
        index: idx,
        path: normalizedPath,
        name: rawName,
        size:
          typeof item.size === "number"
            ? item.size
            : typeof (item as any).Size === "number"
              ? (item as any).Size
              : 0,
        extension: ext,
        modified: item.modified ?? item.Modified ?? "",
        mode: typeof item.mode === "number" ? item.mode : 0,
        isDir,
        isSymlink: !!item.isSymlink,
        type:
          item.type ??
          (isDir ? ("dir" as ResourceType) : ("blob" as ResourceType)),
        url: item.url ?? "",
        subtitles: item.subtitles,
      };
    },
    clearFile() {
      this.$reset();
    },
    /** 批量设置当前列表的产品编号映射
     *  与 updateProductCode 保持一致：每条记录同时写入「纯 /xxx」和「/files/xxx」
     *  两种 key，避免 item.path / item.url 两种格式之间不匹配导致 subtitle 不显示。 */
    setProductCodes(map: Record<string, string>) {
      const out: Record<string, string> = {};
      for (const [rawKey, code] of Object.entries(map)) {
        if (!code) continue;
        const keys = new Set<string>();
        keys.add(rawKey);
        if (rawKey.startsWith("/files/")) {
          keys.add(rawKey.slice(6) === "" ? "/" : rawKey.slice(6));
        } else {
          keys.add("/files" + (rawKey.startsWith("/") ? "" : "/") + rawKey);
        }
        for (const k of keys) out[k] = code;
      }
      this.productCodes = out;
    },
    /**
     * 编辑产品编号后的即时同步：
     *  - 在 item.path / url 上更新（列表行、搜索结果、预览页都能立即看到）
     *  - 空 code 表示清除，保证 subtitle 立即消失
     * 用户路径可能从 req.path 继承为 /files/xxx 或纯粹 /xxx，两种 key 同时写入，
     * 避免 ListingItem 传 path 与 productCodes 键不一致。
     */
    updateProductCode(rawPath: string, code: string) {
      const keys = new Set<string>();
      keys.add(rawPath);
      if (rawPath.startsWith("/files/")) {
        keys.add(rawPath.slice(6) === "" ? "/" : rawPath.slice(6));
      } else {
        keys.add("/files" + (rawPath.startsWith("/") ? "" : "/") + rawPath);
      }
      if (code) {
        for (const k of keys) this.productCodes[k] = code;
      } else {
        for (const k of keys) delete this.productCodes[k];
      }
    },
  },
});
