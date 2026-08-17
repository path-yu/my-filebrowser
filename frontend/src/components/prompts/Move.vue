<template>
  <div class="card floating">
    <div class="card-title">
      <h2>{{ $t("prompts.move") }}</h2>
    </div>

    <div class="card-content">
      <p>{{ $t("prompts.moveMessage") }}</p>
      <file-list
        ref="fileList"
        @update:selected="(val) => (dest = val)"
        :exclude="excludedFolders"
        tabindex="1"
      />
    </div>

    <div
      class="card-action"
      style="display: flex; align-items: center; justify-content: space-between"
    >
      <template v-if="user.perm.create">
        <button
          class="button button--flat"
          @click="$refs.fileList.createDir()"
          :aria-label="$t('sidebar.newFolder')"
          :title="$t('sidebar.newFolder')"
          style="justify-self: left"
        >
          <span>{{ $t("sidebar.newFolder") }}</span>
        </button>
      </template>
      <div>
        <button
          class="button button--flat button--grey"
          @click="closeHovers"
          :aria-label="$t('buttons.cancel')"
          :title="$t('buttons.cancel')"
          tabindex="3"
        >
          {{ $t("buttons.cancel") }}
        </button>
        <button
          id="focus-prompt"
          class="button button--flat"
          @click="move"
          :disabled="$route.path === dest"
          :aria-label="$t('buttons.move')"
          :title="$t('buttons.move')"
          tabindex="2"
        >
          {{ $t("buttons.move") }}
        </button>
      </div>
    </div>
  </div>
</template>

<script>
import { mapActions, mapState, mapWritableState } from "pinia";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";
import { useAuthStore } from "@/stores/auth";
import FileList from "./FileList.vue";
import { files as api } from "@/api";
import buttons from "@/utils/buttons";
import * as upload from "@/utils/upload";
import { removePrefix } from "@/api/utils";

export default {
  name: "move",
  components: { FileList },
  data: function () {
    return {
      current: window.location.pathname,
      dest: null,
    };
  },
  inject: ["$showError"],
  computed: {
    ...mapState(useFileStore, ["req", "selected", "visibleItemAt"]),
    ...mapState(useAuthStore, ["user"]),
    ...mapWritableState(useFileStore, ["reload", "preselect"]),
    excludedFolders() {
      return this.selected
        .map((idx) => this.visibleItemAt(idx))
        .filter((it) => it && it.isDir)
        .map((it) => it && it.url);
    },
  },
  methods: {
    ...mapActions(useLayoutStore, ["showHover", "closeHovers"]),
    move: async function (event) {
      event.preventDefault();
      const items = [];

      for (const idx of this.selected) {
        const it = this.visibleItemAt(idx);
        if (!it) continue;
        items.push({
          from: it.url,
          to: this.dest + encodeURIComponent(it.name),
          name: it.name,
          size: it.size,
          isDir: it.isDir,
          modified: it.modified,
          overwrite: false,
          rename: false,
        });
      }

      const action = async (overwrite, rename) => {
        buttons.loading("move");

        await api
          .move(items, overwrite, rename)
          .then(() => {
            buttons.success("move");
            this.preselect = removePrefix(items[0].to);
            if (this.user.redirectAfterCopyMove)
              this.$router.push({ path: this.dest });
            else this.reload = true;
          })
          .catch((e) => {
            buttons.done("move");
            this.$showError(e);
          });
      };

      const conflict = await upload.checkConflict(items, this.dest, true);

      if (conflict.length > 0) {
        this.showHover({
          prompt: "resolve-conflict",
          props: {
            conflict: conflict,
            files: items,
          },
          confirm: (event, result) => {
            event.preventDefault();
            this.closeHovers();
            for (let i = result.length - 1; i >= 0; i--) {
              const item = result[i];
              if (item.checked.length == 2) {
                items[item.index].rename = true;
              } else if (
                item.checked.length == 1 &&
                item.checked[0] == "origin"
              ) {
                items[item.index].overwrite = true;
              } else {
                items.splice(item.index, 1);
              }
            }
            if (items.length > 0) {
              action();
            }
          },
        });

        return;
      }

      action(false, false);
    },
  },
};
</script>
