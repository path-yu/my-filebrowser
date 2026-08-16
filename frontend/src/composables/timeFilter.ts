/**
 * 文件列表时间筛选
 * - 提供预设：全部时间 / 1小时前 / 12小时前 / 今天 / 三天内 / 一周内 / 一月内
 * - 状态跨标签页持久化（localStorage）
 * - resolveTimeFilter 返回 { modifiedAfter, modifiedBefore } 供 API 作为 query 参数（Unix 毫秒）
 */
import { ref, watch } from "vue";
import type { SelectOption } from "@/components/MacOSSelect.vue";

export type TimeFilterId =
  | "all"
  | "hour1"
  | "hour12"
  | "today"
  | "days3"
  | "week1"
  | "month1";

export interface TimeRange {
  modifiedAfter?: number; // unix ms (inclusive)
  modifiedBefore?: number; // unix ms (inclusive)
}

export interface TimeFilterOption extends SelectOption<TimeFilterId> {
  value: TimeFilterId;
}

export const TIME_FILTER_OPTIONS: TimeFilterOption[] = [
  { value: "all", label: "全部时间" },
  { value: "hour1", label: "1小时内" },
  { value: "hour12", label: "12小时内" },
  { value: "today", label: "今天" },
  { value: "days3", label: "3天内" },
  { value: "week1", label: "1周内" },
  { value: "month1", label: "1月内" },
];

const STORAGE_KEY = "timeFilter";

export const timeFilter = ref<TimeFilterId>(
  (() => {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (raw && TIME_FILTER_OPTIONS.some((o) => o.value === raw)) {
        return raw as TimeFilterId;
      }
    } catch {
      /* ignore */
    }
    return "all";
  })()
);

watch(timeFilter, (v) => {
  try {
    localStorage.setItem(STORAGE_KEY, v);
  } catch {
    /* ignore */
  }
});

const startOfToday = (now: Date) => {
  const d = new Date(now);
  d.setHours(0, 0, 0, 0);
  return d;
};

/**
 * 把预设 id 翻译成时间范围边界（毫秒时间戳）。
 * - "all" 返回空对象 → 不附加筛选参数
 * - "today" => modifiedAfter = 今日 00:00:00
 * - "hour1" => modifiedAfter = 1 小时前
 * - 等等...
 */
export function resolveTimeFilter(
  id: TimeFilterId,
  now: Date = new Date()
): TimeRange {
  switch (id) {
    case "hour1": {
      const after = new Date(now.getTime() - 60 * 60 * 1000);
      return { modifiedAfter: after.getTime(), modifiedBefore: now.getTime() };
    }
    case "hour12": {
      const after = new Date(now.getTime() - 12 * 60 * 60 * 1000);
      return { modifiedAfter: after.getTime(), modifiedBefore: now.getTime() };
    }
    case "today": {
      const after = startOfToday(now);
      return { modifiedAfter: after.getTime(), modifiedBefore: now.getTime() };
    }
    case "days3": {
      const after = new Date(now.getTime() - 3 * 24 * 60 * 60 * 1000);
      return { modifiedAfter: after.getTime(), modifiedBefore: now.getTime() };
    }
    case "week1": {
      const after = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000);
      return { modifiedAfter: after.getTime(), modifiedBefore: now.getTime() };
    }
    case "month1": {
      const after = new Date(now);
      after.setMonth(after.getMonth() - 1);
      return { modifiedAfter: after.getTime(), modifiedBefore: now.getTime() };
    }
    case "all":
    default:
      return {};
  }
}

/**
 * 纯前端兜底过滤（与 matchesTypeFilter 并列使用）：
 * 如果后端路径上没有透传筛选参数，至少保证 UI 上条目被正确过滤。
 * modified: 符合 RFC3339 / Date 对象的字符串
 */
export function matchesTimeFilter(
  modified: string | Date | number,
  range: TimeRange
): boolean {
  if (!range.modifiedAfter && !range.modifiedBefore) return true;
  let ms: number;
  if (typeof modified === "number") {
    ms = modified;
  } else if (modified instanceof Date) {
    ms = modified.getTime();
  } else {
    const d = new Date(modified);
    if (Number.isNaN(d.getTime())) return true; // 非法字符串：不过滤
    ms = d.getTime();
  }
  if (typeof range.modifiedAfter === "number" && ms < range.modifiedAfter) {
    return false;
  }
  if (typeof range.modifiedBefore === "number" && ms > range.modifiedBefore) {
    return false;
  }
  return true;
}
