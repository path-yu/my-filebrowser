/**
 * 搜索关键词高亮工具
 * 采用"分片渲染"思路，而不是 v-html，避免 XSS 注入风险。
 * 使用：把文本 + 关键词 拆成 { text: string, match: boolean }[]，然后模板里
 *   v-for="seg in splitByKeyword(name, keyword)" :key="seg.i"
 *   <span v-if="seg.match" class="search-match">{{ seg.text }}</span>
 *   <span v-else>{{ seg.text }}</span>
 */

export interface HighlightSegment {
  text: string;
  match: boolean;
}

/** 把用户输入转义成可安全塞进正则字面量的字符 */
export const escapeRegExp = (s: string): string =>
  s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

/**
 * 按关键词（可能含多个，用空白 / 分隔符切分）把 text 分片。
 * 顺序 / 大小写不敏感；中文按字匹配，英文按子串匹配。
 * 关键词为空 / 文本为空 时直接返回 [{ text, match:false }]
 */
export function splitByKeyword(
  text: string | null | undefined,
  keyword: string | null | undefined
): HighlightSegment[] {
  const rawText = text ?? "";
  const rawKw = (keyword ?? "").trim();
  if (!rawText || !rawKw) {
    return [{ text: rawText, match: false }];
  }

  // 把关键词按空白切分为"多个 AND 关键词"，任一个命中都高亮
  const tokens = Array.from(
    new Set(
      rawKw
        .split(/[\s,，、;；/\\|]+/)
        .map((t) => t.trim())
        .filter(Boolean)
    )
  );
  if (tokens.length === 0) {
    return [{ text: rawText, match: false }];
  }

  // 正则：所有 token 用 | 拼接，大小写不敏感；u flag 支持 UTF-16 宽字符
  const pattern = new RegExp(
    tokens.map((t) => escapeRegExp(t)).join("|"),
    "giu"
  );

  const segments: HighlightSegment[] = [];
  let lastIndex = 0;
  let m: RegExpExecArray | null;
  while ((m = pattern.exec(rawText)) !== null) {
    const start = m.index;
    const end = start + m[0].length;
    // 防死循环（零宽匹配不会发生，但保险）
    if (end === lastIndex && start === lastIndex) {
      pattern.lastIndex++;
      continue;
    }
    if (start > lastIndex) {
      segments.push({
        text: rawText.slice(lastIndex, start),
        match: false,
      });
    }
    segments.push({
      text: rawText.slice(start, end),
      match: true,
    });
    lastIndex = end;
  }
  if (lastIndex < rawText.length) {
    segments.push({
      text: rawText.slice(lastIndex),
      match: false,
    });
  }
  if (segments.length === 0) {
    segments.push({ text: rawText, match: false });
  }
  return segments;
}
