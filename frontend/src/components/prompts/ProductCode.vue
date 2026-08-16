<template>
  <div class="card floating">
    <div class="card-title">
      <h2>{{ $t("prompts.productCode") }}</h2>
    </div>

    <div class="card-content">
      <p>
        {{ $t("prompts.productCodeMessage") }} <code>{{ fileName }}</code>
      </p>
      <input
        id="focus-prompt"
        class="input input--block"
        type="text"
        :placeholder="$t('prompts.productCodePlaceholder')"
        maxlength="128"
        @keyup.enter="submit"
        v-model.trim="code"
      />
      <p v-if="!isPdf" class="product-code-warning">
        {{ $t("prompts.productCodeNotPdf") }}
      </p>
      <p v-else-if="loaded" class="product-code-hint">
        {{ $t("prompts.productCodeHint") }}
      </p>
    </div>

    <div class="card-action">
      <button
        class="button button--flat button--grey"
        @click="closeHovers"
        :aria-label="$t('buttons.cancel')"
        :title="$t('buttons.cancel')"
      >
        {{ $t("buttons.cancel") }}
      </button>
      <button
        v-if="originalCode"
        class="button button--flat button--red"
        @click="clear"
        :aria-label="$t('buttons.clear')"
        :title="$t('buttons.clear')"
      >
        {{ $t("buttons.clear") }}
      </button>
      <button
        @click="submit"
        class="button button--flat"
        type="submit"
        :aria-label="$t('buttons.save')"
        :title="$t('buttons.save')"
        :disabled="!isPdf || saving || code === originalCode"
      >
        {{ $t("buttons.save") }}
      </button>
    </div>
  </div>
</template>

<script>
import { mapActions, mapState, mapWritableState } from "pinia";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";
import { removePrefix } from "@/api/utils";
import { productcode as api } from "@/api";

export default {
  name: "productCode",
  data: function () {
    return {
      code: "",
      originalCode: "",
      loaded: false,
      saving: false,
    };
  },
  inject: ["$showError", "$showSuccess"],
  computed: {
    ...mapState(useFileStore, [
      "req",
      "selected",
      "selectedCount",
      "isListing",
    ]),
    ...mapWritableState(useFileStore, ["reload"]),
    target() {
      if (!this.isListing) {
        return this.req ?? null;
      }
      if (this.selectedCount !== 1) {
        return null;
      }
      return this.req?.items[this.selected[0]] ?? null;
    },
    fileName() {
      return this.target?.name ?? "";
    },
    isPdf() {
      return this.target?.type === "pdf";
    },
    targetPath() {
      if (!this.target) return "";
      return this.target.path || removePrefix(this.target.url);
    },
  },
  async created() {
    if (!this.isPdf) return;
    try {
      const entry = await api.get(this.targetPath);
      this.code = entry.code ?? "";
      this.originalCode = this.code;
    } catch (e) {
      this.$showError(e);
    } finally {
      this.loaded = true;
    }
  },
  methods: {
    ...mapActions(useLayoutStore, ["closeHovers"]),
    async save(code) {
      this.saving = true;
      try {
        const result = await api.put(this.targetPath, code);
        // 立即同步到 Pinia：列表/搜索结果中的 subtitle 会即时更新，不需要刷页面
        const fileStore = useFileStore();
        fileStore.updateProductCode(this.targetPath, code);
        if (result.pdfUpdated) {
          this.$showSuccess?.(this.$t("success.productCodeSaved"));
        } else {
          // 数据库已保存，但 PDF 元数据写入失败（如文件只读/损坏）
          this.$showError(
            new Error(
              this.$t("errors.productCodeMetaFailed", {
                message: result.pdfError || "",
              })
            )
          );
        }
      } catch (e) {
        this.$showError(e);
      } finally {
        this.saving = false;
      }
      this.closeHovers();
    },
    submit() {
      if (!this.isPdf || this.saving || this.code === this.originalCode) {
        return;
      }
      this.save(this.code);
    },
    clear() {
      if (!this.isPdf || this.saving || !this.originalCode) {
        return;
      }
      this.save("");
    },
  },
};
</script>
