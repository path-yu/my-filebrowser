/**
 * 离线验证时间筛选分桶的"预期对照表"是否与 resolveTimeFilter + 过滤逻辑一致。
 * 不依赖 Vue / Vite，直接用 Node + tsx 或 npx ts-node 都可。
 * 这里用 --experimental-strip-types 直接执行（Node 22+ 支持 TypeScript 源生），
 * 或者走下面 PowerShell 里的内联 transpile 也行。
 */

type TimeFilterId =
  | "all"
  | "hour1"
  | "hour12"
  | "today"
  | "days3"
  | "week1"
  | "month1";

interface TimeRange {
  modifiedAfter?: number;
  modifiedBefore?: number;
}

interface MockBucket {
  id: string;
  label: string;
  offsetMs: number;
}

const MOCK_BUCKETS: MockBucket[] = [
  { id: "10min", label: "10 分钟前", offsetMs: -10 * 60 * 1000 },
  { id: "30min", label: "30 分钟前", offsetMs: -30 * 60 * 1000 },
  { id: "2h", label: "2 小时前", offsetMs: -2 * 60 * 60 * 1000 },
  // today 桶：今天 02:00
  { id: "today", label: "今天 02:00", offsetMs: 0 },
  { id: "5d", label: "5 天前", offsetMs: -5 * 24 * 60 * 60 * 1000 },
  { id: "2w", label: "2 周前", offsetMs: -14 * 24 * 60 * 60 * 1000 },
  { id: "40d", label: "40 天前", offsetMs: -40 * 24 * 60 * 60 * 1000 },
];

const bucketModified = (b: MockBucket, now: Date): number => {
  if (b.id === "today") {
    const startOfToday = new Date(now);
    startOfToday.setHours(0, 0, 0, 0);
    const sixHoursAgo = new Date(now.getTime() - 6 * 60 * 60 * 1000);
    const lower = new Date(
      Math.max(startOfToday.getTime(), sixHoursAgo.getTime())
    );
    return lower.getTime() + 5 * 60 * 1000;
  }
  return now.getTime() + b.offsetMs;
};

const resolveTimeFilter = (id: TimeFilterId, now: Date): TimeRange => {
  switch (id) {
    case "hour1":
      return {
        modifiedAfter: now.getTime() - 60 * 60 * 1000,
        modifiedBefore: now.getTime(),
      };
    case "hour12":
      return {
        modifiedAfter: now.getTime() - 12 * 60 * 60 * 1000,
        modifiedBefore: now.getTime(),
      };
    case "today": {
      const d = new Date(now);
      d.setHours(0, 0, 0, 0);
      return { modifiedAfter: d.getTime(), modifiedBefore: now.getTime() };
    }
    case "days3":
      return {
        modifiedAfter: now.getTime() - 3 * 24 * 60 * 60 * 1000,
        modifiedBefore: now.getTime(),
      };
    case "week1":
      return {
        modifiedAfter: now.getTime() - 7 * 24 * 60 * 60 * 1000,
        modifiedBefore: now.getTime(),
      };
    case "month1": {
      const d = new Date(now);
      d.setMonth(d.getMonth() - 1);
      return { modifiedAfter: d.getTime(), modifiedBefore: now.getTime() };
    }
    case "all":
    default:
      return {};
  }
};

const EXPECTED: Record<TimeFilterId, string[]> = {
  all: ["10min", "30min", "2h", "today", "5d", "2w", "40d"],
  hour1: ["10min", "30min"],
  hour12: ["10min", "30min", "2h", "today"],
  today: ["10min", "30min", "2h", "today"],
  days3: ["10min", "30min", "2h", "today"],
  week1: ["10min", "30min", "2h", "today", "5d"],
  month1: ["10min", "30min", "2h", "today", "5d", "2w"],
};

const PRESET_LABEL: Record<TimeFilterId, string> = {
  all: "全部时间",
  hour1: "1小时内",
  hour12: "12小时内",
  today: "今天",
  days3: "3天内",
  week1: "1周内",
  month1: "1月内",
};

function runAssertions() {
  const now = new Date("2026-08-16T10:30:00"); // 固定当前为今天 10:30，避免"今天"桶在凌晨跑失效
  const presets = Object.keys(EXPECTED) as TimeFilterId[];
  let pass = 0;
  let fail = 0;
  for (const p of presets) {
    const range = resolveTimeFilter(p, now);
    const after =
      typeof range.modifiedAfter === "number" ? range.modifiedAfter : -Infinity;
    const before =
      typeof range.modifiedBefore === "number" ? range.modifiedBefore : Infinity;
    const visible = MOCK_BUCKETS.filter((b) => {
      const t = bucketModified(b, now);
      return t >= after && t <= before;
    }).map((b) => b.id);

    const want = EXPECTED[p];
    const same =
      visible.length === want.length &&
      visible.every((v, i) => v === want[i]);
    if (same) {
      console.log(
        `  ✓ ${PRESET_LABEL.padEnd ? "" : ""}${PRESET_LABEL[p].padEnd(6)} → ${visible.join("、") || "无"}`
      );
      pass++;
    } else {
      console.log(
        `  ✗ ${PRESET_LABEL[p]}：期望 ${want.join("、") || "无"}，实际 ${visible.join("、") || "无"}`
      );
      fail++;
    }
  }
  console.log(`\n结果：${pass} 个通过，${fail} 个失败`);
  if (fail > 0) {
    process.exit(1);
  }
}

runAssertions();
