<template>
  <div>
    <header-bar showMenu showLogo>
      <search />
      <title />
      <action
        class="search-button"
        icon="search"
        :label="t('buttons.search')"
        @action="openSearch()"
      />

      <template #actions>
        <template v-if="!isMobile">
          <action
            v-if="headerButtons.share"
            icon="share"
            :label="t('buttons.share')"
            show="share"
          />
          <action
            v-if="headerButtons.rename"
            icon="mode_edit"
            :label="t('buttons.rename')"
            show="rename"
          />
          <action
            v-if="headerButtons.copy"
            id="copy-button"
            icon="content_copy"
            :label="t('buttons.copyFile')"
            show="copy"
          />
          <action
            v-if="headerButtons.move"
            id="move-button"
            icon="forward"
            :label="t('buttons.moveFile')"
            show="move"
          />
          <action
            v-if="headerButtons.delete"
            id="delete-button"
            icon="delete"
            :label="t('buttons.delete')"
            show="delete"
          />
        </template>

        <action
          v-if="headerButtons.shell"
          icon="code"
          :label="t('buttons.shell')"
          @action="layoutStore.toggleShell"
        />
        <div
          class="view-segment"
          role="group"
          :aria-label="t('buttons.switchView')"
        >
          <button
            v-for="mode in viewModes"
            :key="mode.value"
            class="view-segment__item"
            :class="{ active: currentViewMode === mode.value }"
            :title="mode.label"
            :aria-label="mode.label"
            @click="setView(mode.value)"
          >
            <i class="material-icons">{{ mode.icon }}</i>
          </button>
          <!-- 预览视图：独立开关，不修改后端 viewMode，仅本地控制右侧预览面板显隐 -->
          <button
            class="view-segment__item"
            :class="{ active: previewEnabled }"
            title="预览视图"
            aria-label="预览视图"
            @click="togglePreview"
          >
            <i class="material-icons">pageview</i>
          </button>
        </div>
        <action
          v-if="headerButtons.download"
          icon="file_download"
          :label="t('buttons.download')"
          @action="download"
          :counter="fileStore.selectedCount"
        />
        <action
          v-if="headerButtons.upload"
          icon="file_upload"
          id="upload-button"
          :label="t('buttons.upload')"
          @action="uploadFunc"
        />
        <action icon="info" :label="t('buttons.info')" show="info" />
        <action
          icon="check_circle"
          :label="t('buttons.selectMultiple')"
          @action="toggleMultipleSelection"
        />
      </template>
    </header-bar>

    <div
      v-if="isMobile"
      id="file-selection"
      :class="{
        'file-selection-margin-bottom': fileStore.multiple,
      }"
    >
      <span v-if="fileStore.selectedCount > 0">
        {{ t("prompts.filesSelected", fileStore.selectedCount) }}
      </span>
      <action
        v-if="headerButtons.share"
        icon="share"
        :label="t('buttons.share')"
        show="share"
      />
      <action
        v-if="headerButtons.rename"
        icon="mode_edit"
        :label="t('buttons.rename')"
        show="rename"
      />
      <action
        v-if="headerButtons.copy"
        icon="content_copy"
        :label="t('buttons.copyFile')"
        show="copy"
      />
      <action
        v-if="headerButtons.move"
        icon="forward"
        :label="t('buttons.moveFile')"
        show="move"
      />
      <action
        v-if="headerButtons.delete"
        icon="delete"
        :label="t('buttons.delete')"
        show="delete"
      />
    </div>

    <div v-if="layoutStore.loading">
      <h2 class="message delayed">
        <div class="spinner">
          <div class="bounce1"></div>
          <div class="bounce2"></div>
          <div class="bounce3"></div>
        </div>
        <span>{{ t("files.loading") }}</span>
      </h2>
    </div>
    <template v-else>
      <!-- ========== 预览视图：左侧文件列表 + 右侧预览面板 ========== -->
      <div v-if="currentNumDirs + currentNumFiles == 0">
        <h2 class="message">
          <i class="material-icons">sentiment_dissatisfied</i>
          <span>{{ t("files.lonely") }}</span>
        </h2>
        <input
          style="display: none"
          type="file"
          id="upload-input"
          @change="uploadInput($event)"
          multiple
        />
        <input
          style="display: none"
          type="file"
          id="upload-folder-input"
          @change="uploadInput($event)"
          webkitdirectory
          multiple
        />
      </div>
      <!-- 非空目录：有文件时才提供预览分栏 -->
      <div
        v-else
        class="files-layout"
        :class="{ 'with-preview': previewEnabled && !isMobile }"
        :style="previewLayoutStyle"
      >
        <div
          id="listing"
          ref="listing"
          style="max-height: 80vh;overflow-y: auto;position: relative;"
          class="file-icons listing-area"
          data-clear-on-click="true"
          :class="authStore.user?.viewMode ?? ''"
          @click="handleEmptyAreaClick"
          @scroll="onListingScroll"
        >
          <!-- 列表空态：搜索无结果 / 筛选后无任何渲染项（注意：这里必须用"实际渲染数组 dirs+files 的长度"，
                 不能用 currentNumDirs+currentNumFiles，因为后者是 currentItems 原始数量，没经过文件类型筛选——
                 例如当前目录全是 PDF，用户选了"压缩包"筛选，currentNumFiles>0 但筛选后 files=空，原判断会卡死在假空态） -->
          <div
            v-if="dirs.length + files.length === 0"
            class="empty-placeholder"
          >
            <i class="material-icons empty-icon" aria-hidden="true">
              {{ fileStore.searchMode ? "search_off" : "folder_open" }}
            </i>
            <p class="empty-text">
              {{
                fileStore.searchMode
                  ? t("search.noMatches")
                  : t("files.lonely")
              }}
            </p>
          </div>

          <template v-else>
            <div>
              <div class="item header">
                <div>
                  <p
                    :class="{ active: nameSorted }"
                    class="name"
                    role="button"
                    tabindex="0"
                    @click.prevent="sort('name', $event)"
                    @keyup.enter.prevent="sort('name', $event)"
                    @keyup.space.prevent="sort('name', $event)"
                    :title="t('files.sortByName')"
                    :aria-label="t('files.sortByName')"
                  >
                    <span>{{ t("files.name") }}</span>
                    <i class="material-icons">{{ nameIcon }}</i>
                  </p>

                  <p
                    :class="{ active: sizeSorted }"
                    class="size"
                    role="button"
                    tabindex="0"
                    @click.prevent="sort('size', $event)"
                    @keyup.enter.prevent="sort('size', $event)"
                    @keyup.space.prevent="sort('size', $event)"
                    :title="t('files.sortBySize')"
                    :aria-label="t('files.sortBySize')"
                  >
                    <span>{{ t("files.size") }}</span>
                    <i class="material-icons">{{ sizeIcon }}</i>
                  </p>
                  <p
                    :class="{ active: modifiedSorted }"
                    class="modified"
                    role="button"
                    tabindex="0"
                    @click.prevent="sort('modified', $event)"
                    @keyup.enter.prevent="sort('modified', $event)"
                    @keyup.space.prevent="sort('modified', $event)"
                    :title="t('files.sortByLastModified')"
                    :aria-label="t('files.sortByLastModified')"
                  >
                    <span>{{ t("files.lastModified") }}</span>
                    <i class="material-icons">{{ modifiedIcon }}</i>
                  </p>
                </div>
              </div>
            </div>

            <h2 data-clear-on-click="true" v-if="currentNumDirs">
              {{ t("files.folders") }}
            </h2>
            <div
              v-if="currentNumDirs"
              data-clear-on-click="true"
              class="list-coentet"
              @contextmenu="showContextMenu"
            >
              <item
                v-for="item in dirs"
                :key="base64(item.name)"
                v-bind:index="item.index"
                v-bind:name="item.name"
                v-bind:isDir="item.isDir"
                v-bind:url="item.url"
                v-bind:modified="item.modified"
                v-bind:type="item.type"
                v-bind:size="item.size"
                v-bind:path="item.path"
              >
              </item>
            </div>

            <h2 data-clear-on-click="true" v-if="currentNumFiles">
              {{ t("files.files") }}
            </h2>

            <!-- 列表视图：文件区使用【父级滚动驱动的虚拟滚动】——
                 滚动容器仍然是外层 #listing（单一滚动条，包含 header/文件夹/文件共享），
                 文件区只渲染可视窗口内的 DOM，解决 4000+ 文件滚动卡顿。 -->
            <VirtualList
              v-if="isListView && currentNumFiles"
              ref="virtualListRef"
              class="listing-files-virtual"
              mode="parent"
              :items="files"
              :item-height="listRowEstimate"
              :buffer="8"
              :get-key="getFileKey"
              :outer-scroll-top="listingScrollTop"
              :outer-height="listingHeight"
              :scroll-container-el="listing"
              @contextmenu="showContextMenu"
            >
              <template #default="{ item }">
                <item
                  v-bind:index="item.index"
                  v-bind:name="item.name"
                  v-bind:isDir="item.isDir"
                  v-bind:url="item.url"
                  v-bind:modified="item.modified"
                  v-bind:type="item.type"
                  v-bind:size="item.size"
                  v-bind:path="item.path"
                  v-bind:product-code="lookupProductCode(item)"
                  v-bind:blur-up="item.blurUp"
                  v-bind:thumbs-eager="true"
                >
                </item>
              </template>
            </VirtualList>

            <!-- 网格视图 / 画廊视图：保持原渲染方式不变 -->
            <div
              v-if="!isListView && currentNumFiles"
              data-clear-on-click="true"
              @contextmenu="showContextMenu"
            >
              <item
                v-for="item in files"
                :key="base64(item.name)"
                v-bind:index="item.index"
                v-bind:name="item.name"
                v-bind:isDir="item.isDir"
                v-bind:url="item.url"
                v-bind:modified="item.modified"
                v-bind:type="item.type"
                v-bind:size="item.size"
                v-bind:path="item.path"
                v-bind:product-code="lookupProductCode(item)"
                v-bind:blur-up="item.blurUp"
              >
              </item>
            </div>
          </template>
          <context-menu
            :show="isContextMenuVisible"
            :pos="contextMenuPos"
            @hide="hideContextMenu"
          >
            <!-- 编辑产品编号：仅单选 PDF 时可用，放在分享按钮前面 -->
            <action
              v-if="productCodeTarget"
              icon="sell"
              :label="t('buttons.productCode')"
              show="productCode"
            />
            <action
              v-if="headerButtons.share"
              icon="share"
              :label="t('buttons.share')"
              show="share"
            />
            <action
              v-if="headerButtons.rename"
              icon="mode_edit"
              :label="t('buttons.rename')"
              show="rename"
            />
            <action
              v-if="headerButtons.copy"
              id="copy-button"
              icon="content_copy"
              :label="t('buttons.copyFile')"
              show="copy"
            />
            <action
              v-if="headerButtons.move"
              id="move-button"
              icon="forward"
              :label="t('buttons.moveFile')"
              show="move"
            />
            <action
              v-if="headerButtons.delete"
              id="delete-button"
              icon="delete"
              :label="t('buttons.delete')"
              show="delete"
            />
            <action
              v-if="headerButtons.download"
              icon="file_download"
              :label="t('buttons.download')"
              @action="download"
              :counter="fileStore.selectedCount"
            />
            <!-- 复制当前选中/右键项的前端可访问链接，多选/未选中时隐藏 -->
            <action
              v-if="selectedForContextMenuLink.length === 1"
              icon="link"
              :label="t('buttons.copyPageLink', '复制页面链接')"
              @action="copyCurrentPageLink"
            />
            <action icon="info" :label="t('buttons.info')" show="info" />
          </context-menu>

          <input
            style="display: none"
            type="file"
            id="upload-input"
            @change="uploadInput($event)"
            multiple
          />
          <input
            style="display: none"
            type="file"
            id="upload-folder-input"
            @change="uploadInput($event)"
            webkitdirectory
            multiple
          />

          <div :class="{ active: fileStore.multiple }" id="multiple-selection">
            <p>{{ t("files.multipleSelectionEnabled") }}</p>
            <div
              @click="() => (fileStore.multiple = false)"
              tabindex="0"
              role="button"
              :title="t('buttons.clear')"
              :aria-label="t('buttons.clear')"
              class="action"
            >
              <i class="material-icons">clear</i>
            </div>
          </div>
        </div>
        <!-- ===== 拖拽分隔条：左右拖拽调整预览面板宽度 ===== -->
        <div
          v-if="previewEnabled && !isMobile"
          class="preview-resizer"
          :class="{ dragging: isResizing }"
          role="separator"
          aria-orientation="vertical"
          tabindex="0"
          @mousedown="onResizerMouseDown"
          @dblclick="resetPreviewWidth"
          :title="'拖拽调整预览区宽度（双击恢复默认）'"
        >
          <span class="preview-resizer__handle"></span>
        </div>
        <!-- ===== 右侧预览面板（预览视图开启时显示） ===== -->
        <aside
          v-if="previewEnabled && !isMobile"
          class="preview-pane"
          :style="previewPaneStyle"
        >
          <template v-if="previewedItem">
            <!-- 预览头：文件图标 + 名称 -->
            <div class="preview-pane__header">
              <div class="preview-pane__icon">
                <i class="material-icons">{{ previewedItemIcon }}</i>
              </div>
              <div class="preview-pane__meta">
                <p class="preview-pane__name" :title="previewedItem.name">
                  {{ previewedItem.name }}
                </p>
                <p class="preview-pane__sub">
                  {{ formatSize(previewedItem.size) }}
                  <span v-if="previewedItem.modified">
                    · {{ formatDate(previewedItem.modified) }}</span
                  >
                </p>
              </div>
            </div>

            <!-- 预览主体：按文件类型渲染 -->
            <div class="preview-pane__body" ref="previewBodyRef">
              <!-- 加载中 -->
              <div v-if="previewLoading" class="preview-pane__placeholder">
                <div class="preview-spinner" aria-hidden="true"></div>
                <p class="preview-pane__hint">{{ previewLoadingText }}</p>
              </div>

              <!-- 错误 -->
              <template v-else-if="previewError">
                <div class="preview-pane__placeholder">
                  <i class="material-icons preview-pane__big-icon error">
                    error_outline
                  </i>
                  <p class="preview-pane__hint error">
                    预览加载失败：{{ previewError }}
                  </p>
                  <a
                    class="button button--flat preview-pane__download-btn"
                    target="_blank"
                    :href="getDownloadLink(previewedItem)"
                  >
                    <i class="material-icons">file_download</i>
                    下载文件
                  </a>
                </div>
              </template>

              <!-- 1) 图片 -->
              <template v-else-if="isImagePreview">
                <!-- 注意：不能用 previewedItem.url（那是前端路由路径 /files/...，
                     返回的是 SPA index.html 而非图片字节）；走 /api/raw?inline=true，
                     鉴权由登录时种下的 auth cookie 完成（后端 auth.go 支持 cookie 回退）。
                     预览窗格是用户点击文件立即需要看到的，所以用 eager=true 跳过懒加载，
                     但仍然享受 LazyImage 的 iOS 菊花 + 错误重试处理。 -->
                <LazyImage
                  class="preview-pane__image"
                  :src="getDownloadLink(previewedItem)"
                  :alt="previewedItem.name"
                  :blurUp="previewedItem.blurUp"
                  eager
                  @error="onPreviewImageError"
                />
              </template>

              <!-- 2) PDF：使用 pdfjs 渲染到 canvas 容器（对齐 Example macOS 风格） -->
              <template v-else-if="isPdf">
                <div
                  class="preview-pdf-container"
                  v-if="previewPdf.totalPages > 0"
                >
                  <div class="preview-pdf-toolbar">
                    <!-- 翻页组 -->
                    <div class="preview-pdf-group">
                      <button
                        class="preview-pdf-btn"
                        :disabled="previewPdf.currentPage <= 1"
                        @click="pdfPrevPage"
                        title="上一页"
                      >
                        <i class="material-icons">chevron_left</i>
                      </button>
                      <span class="preview-pdf-page">
                        {{ previewPdf.currentPage }}/{{ previewPdf.totalPages }}
                      </span>
                      <button
                        class="preview-pdf-btn"
                        :disabled="
                          previewPdf.currentPage >= previewPdf.totalPages
                        "
                        @click="pdfNextPage"
                        title="下一页"
                      >
                        <i class="material-icons">chevron_right</i>
                      </button>
                    </div>
                    <div class="preview-pdf-divider"></div>
                    <!-- 适合模式组 -->
                    <div class="preview-pdf-group">
                      <button
                        class="preview-pdf-btn"
                        :class="{ active: previewPdf.fitMode === 'width' }"
                        @click="pdfSetFit('width')"
                        title="适合宽度"
                      >
                        <i class="material-icons">swap_horiz</i>
                      </button>
                      <button
                        class="preview-pdf-btn"
                        :class="{ active: previewPdf.fitMode === 'page' }"
                        @click="pdfSetFit('page')"
                        title="适合整页"
                      >
                        <i class="material-icons">fullscreen</i>
                      </button>
                    </div>
                    <div class="preview-pdf-divider"></div>
                    <!-- 缩放组 -->
                    <div class="preview-pdf-group">
                      <button
                        class="preview-pdf-btn"
                        @click="pdfZoomOut"
                        title="缩小"
                      >
                        <i class="material-icons">zoom_out</i>
                      </button>
                      <span class="preview-pdf-page">
                        {{ Math.round(previewPdf.scale * 100) }}%
                      </span>
                      <button
                        class="preview-pdf-btn"
                        @click="pdfZoomIn"
                        title="放大"
                      >
                        <i class="material-icons">zoom_in</i>
                      </button>
                    </div>
                    <div class="preview-pdf-divider"></div>
                    <!-- 旋转组（FileListing 内联详情卡片版） -->
                    <div class="preview-pdf-group">
                      <button
                        class="preview-pdf-btn"
                        @click="pdfRotateLeft"
                        title="逆时针旋转 90°（快捷键 [）"
                      >
                        <i class="material-icons">rotate_left</i>
                      </button>
                      <button
                        class="preview-pdf-btn"
                        @click="pdfRotateRight"
                        title="顺时针旋转 90°（快捷键 ]）"
                      >
                        <i class="material-icons">rotate_right</i>
                      </button>
                    </div>
                    <div class="preview-pdf-spacer"></div>
                    <button
                      class="preview-pdf-btn preview-pdf-btn--text"
                      @click="pdfPrint"
                      title="打印 PDF（⌘P）"
                    >
                      <i class="material-icons">print</i>
                    </button>
                    <button
                      class="preview-pdf-btn preview-pdf-btn--text"
                      @click="pdfResetScale"
                      title="重置缩放"
                    >
                      <i class="material-icons">refresh</i>
                    </button>
                  </div>
                  <div
                    class="preview-pdf-scroll"
                    ref="pdfScrollRef"
                  >
                    <canvas
                      ref="pdfCanvasRef"
                      class="preview-pdf-canvas"
                    ></canvas>
                  </div>
                </div>
                <!-- PDF fallback：canvas 加载失败或未加载时的占位 -->
                <div v-else class="preview-pane__placeholder">
                  <i class="material-icons preview-pane__big-icon">
                    picture_as_pdf
                  </i>
                  <p class="preview-pane__hint">{{ previewLoadingText || "正在加载 PDF..." }}</p>
                </div>
              </template>

              <!-- 3) Markdown -->
              <template v-else-if="isMarkdownPreview">
                <div
                  class="preview-markdown markdown-body"
                  v-html="previewRenderedHtml"
                ></div>
              </template>

              <!-- 4) Word (docx)：docx-preview 保真渲染（对齐 Example wordPview） -->
              <template v-else-if="isWordPreview">
                <div class="preview-word-container">
                  <!-- docx-preview 样式注入容器（隐藏） -->
                  <div ref="wordStyleRef" class="preview-word-style" aria-hidden="true"></div>
                  <!-- 精简工具栏：页数 + 缩放 + 适合页宽 -->
                  <div class="preview-word-toolbar">
                    <span class="preview-word-pages" v-if="previewWord.pageCount > 0">
                      {{ previewWord.pageCount }} 页
                    </span>
                    <div class="preview-pdf-spacer"></div>
                    <div class="preview-pdf-group">
                      <button class="preview-pdf-btn" @click="wordZoomOut" title="缩小">
                        <i class="material-icons">zoom_out</i>
                      </button>
                      <span class="preview-pdf-page">
                        {{ previewWord.fitWidth ? "页宽" : Math.round(previewWord.scale * 100) + "%" }}
                      </span>
                      <button class="preview-pdf-btn" @click="wordZoomIn" title="放大">
                        <i class="material-icons">zoom_in</i>
                      </button>
                    </div>
                    <div class="preview-pdf-divider"></div>
                    <button
                      class="preview-pdf-btn"
                      :class="{ active: previewWord.fitWidth }"
                      @click="previewWord.fitWidth = !previewWord.fitWidth"
                      title="适合页宽"
                    >
                      <i class="material-icons">swap_horiz</i>
                    </button>
                    <button
                      class="preview-pdf-btn"
                      :class="{ active: !previewWord.fitWidth && previewWord.scale === 1 }"
                      @click="previewWord.fitWidth = false; previewWord.scale = 1"
                      title="实际大小"
                    >
                      <i class="material-icons">crop_free</i>
                    </button>
                  </div>
                  <!-- 滚动渲染区 -->
                  <div class="preview-word-scroll">
                    <div v-if="previewWord.loading" class="preview-word-loading">
                      <div class="preview-spinner" aria-hidden="true"></div>
                    </div>
                    <div
                      class="preview-word-scalewrap"
                      :class="{ 'is-fitwidth': previewWord.fitWidth }"
                      :style="previewWord.fitWidth ? undefined : { transform: `scale(${previewWord.scale})` }"
                    >
                      <div ref="wordHostRef" class="preview-word-host"></div>
                    </div>
                  </div>
                </div>
              </template>

              <!-- 5) 文本类（txt/log/csv/code/json 等） -->
              <template v-else-if="isTextPreview && previewText !== undefined">
                <pre
                  class="preview-text-code"
                  :class="{ 'is-csv': isCsvPreview }"
                ><code>{{ previewText }}</code></pre>
              </template>

              <!-- 6) CAD：无预览，保持原占位 -->
              <template v-else-if="isDwg">
                <div class="preview-pane__placeholder">
                  <i class="material-icons preview-pane__big-icon">{{
                    previewedItemIcon
                  }}</i>
                  <p class="preview-pane__hint">
                    CAD 图纸（请下载后用 AutoCAD / 中望 CAD 打开）
                  </p>
                </div>
              </template>

              <!-- 7) 文件夹 -->
              <template v-else-if="previewedItem.isDir">
                <div class="preview-pane__placeholder">
                  <i class="material-icons preview-pane__big-icon">folder</i>
                  <p class="preview-pane__hint">文件夹</p>
                </div>
              </template>

              <!-- 8) 其他：大图标占位 + 下载链接 -->
              <template v-else>
                <div class="preview-pane__placeholder">
                  <i class="material-icons preview-pane__big-icon">{{
                    previewedItemIcon
                  }}</i>
                  <p class="preview-pane__hint">
                    {{ (previewedItem.extension || "文件").toUpperCase() }}
                    文件暂不支持在线预览
                  </p>
                  <a
                    class="button button--flat preview-pane__download-btn"
                    target="_blank"
                    :href="getDownloadLink(previewedItem)"
                  >
                    <i class="material-icons">file_download</i>
                    下载文件
                  </a>
                </div>
              </template>
            </div>
          </template>
          <template v-else>
            <!-- 未选中文件时的空状态 -->
            <div class="preview-pane__empty">
              <i class="material-icons">panorama</i>
              <p>选择一个文件查看预览</p>
            </div>
          </template>
        </aside>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { useAuthStore } from "@/stores/auth";
import { useClipboardStore } from "@/stores/clipboard";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";

import { users, files as api, productcode as productcodeApi } from "@/api";
import { createURL, removePrefix } from "@/api/utils";
import { enableExec } from "@/utils/constants";
import * as upload from "@/utils/upload";
import css from "@/utils/css";
import { filesize } from "@/utils";
import { throttle } from "lodash-es";
import { Base64 } from "js-base64";
import * as dayjs from "dayjs";

// 重型预览库（pdfjs/marked/dompurify/docx-preview）的惰性加载器：
// 只在首次预览对应类型文件时才加载，避免拖慢首屏渲染。
import {
  ensurePdfLib,
  ensureMarkdownLibs,
  ensureDocxLib,
} from "@/utils/previewLoaders";

import HeaderBar from "@/components/header/HeaderBar.vue";
import Action from "@/components/header/Action.vue";
import Search from "@/components/Search.vue";
import Item from "@/components/files/ListingItem.vue";
import VirtualList from "@/components/files/VirtualList.vue";
import LazyImage from "@/components/files/LazyImage.vue";
import ContextMenu from "@/components/ContextMenu.vue";
import { matchesTypeFilter } from "@/composables/fileTypeFilter";
import {
  matchesTimeFilter,
  resolveTimeFilter,
  timeFilter,
} from "@/composables/timeFilter";
import {
  computed,
  inject,
  nextTick,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
  shallowReactive,
  watch,
} from "vue";
import { useRoute, onBeforeRouteUpdate } from "vue-router";
import { useI18n } from "vue-i18n";
import { storeToRefs } from "pinia";
import { copy } from "@/utils/clipboard";

const showLimit = ref<number>(50);
const columnWidth = ref<number>(280);
const dragCounter = ref<number>(0);
const width = ref<number>(window.innerWidth);
const itemWeight = ref<number>(0);
const isContextMenuVisible = ref<boolean>(false);
const contextMenuPos = ref<{ x: number; y: number }>({ x: 0, y: 0 });

const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess");

const clipboardStore = useClipboardStore();
const authStore = useAuthStore();
const fileStore = useFileStore();
const layoutStore = useLayoutStore();

/* ========== 列表视图文件区虚拟滚动：外层滚动驱动（滚动容器仍为 #listing） ========== */
const virtualListRef = ref<InstanceType<typeof VirtualList> | null>(null);

/** 是否列表视图：仅 list 模式的文件区启用虚拟滚动，网格/画廊完全保持原渲染 */
const isListView = computed(
  () => (authStore.user?.viewMode ?? "list") === "list"
);

/** 列表视图单行高度估算（px）——实际测量后缓存真实高度，此处只是未加载行的兜底值 */
const listRowEstimate = 48;

/** 虚拟列表 key：与原 v-for 的 :key="base64(item.name)" 保持一致 */
const getFileKey = (item: Resource) => base64(item.name);

/** #listing 当前 scrollTop（@scroll 监听 → 喂给 VirtualList mode=parent） */
const listingScrollTop = ref(0);

/** #listing 当前可视高度（clientHeight），用于计算可视窗口 */
const listingHeight = ref(600);

const { req } = storeToRefs(fileStore);

const route = useRoute();
const { t } = useI18n();

/**
 * 当前显示的 items 来源：
 * - 搜索模式：搜索结果（store.visibleItems）
 * - 普通目录：原 req.items
 * 同时提供按 index 取值和计数，避免各函数到处写 if/else。
 */
const currentItems = computed(
  (): ResourceItem[] =>
    (fileStore.searchMode ? fileStore.visibleItems : fileStore.req?.items) ?? []
);
const currentNumDirs = computed(
  (): number => currentItems.value.filter((i) => i.isDir).length
);
const currentNumFiles = computed(
  (): number => currentItems.value.filter((i) => !i.isDir).length
);
const getItemByIndex = (i: number): ResourceItem | undefined => {
  return currentItems.value[i];
};

/** 右键菜单"复制页面链接"的目标：必须且只能有一个文件/文件夹被选中；
 *  多选或空选区时隐藏该菜单项（和 download / share 等行为一致，
 *  download 在单文件时直接下载、其他情况弹选择框）。 */
const selectedForContextMenuLink = computed((): ResourceItem[] => {
  if (fileStore.selectedCount === 1) {
    const t = getItemByIndex(fileStore.selected[0]);
    return t ? [t] : [];
  }
  return [];
});

/* ======================== 产品编号（数据库 + PDF 元数据双写） ======================== */

/** 右键菜单"编辑产品编号"的目标：仅单选且为 PDF 时可用 */
const productCodeTarget = computed((): ResourceItem | null => {
  if (fileStore.selectedCount !== 1) return null;
  const item = getItemByIndex(fileStore.selected[0]);
  if (!item || item.isDir || item.type !== "pdf") return null;
  return item;
});

/** path → 产品编号 映射（Pinia，编辑弹窗保存后可直接更新，列表立即可见） */
const { productCodes } = storeToRefs(fileStore);
let productCodeLoadSeq = 0;
let productCodeLoadTimer: number | null = null;

/** 产品编号双 key 兜底查找：
 *  先用 item.path 查（后端 FileInfo 直接返回），如果没有命中或 path 为空，
 *  再用 removePrefix(item.url) 查（前端自己拼的 url 去掉 /files 前缀），
 *  避免某一侧路径格式不完整导致 subtitle 空白。 */
const lookupProductCode = (item: ResourceItem): string => {
  const direct = (item.path && productCodes.value[item.path]) || "";
  if (direct) return direct;
  const altKey = removePrefix(item.url);
  return (altKey && productCodes.value[altKey]) || "";
};

/** 批量拉取当前列表（含搜索结果）中 PDF 的产品编号 */
const loadProductCodes = () => {
  if (productCodeLoadTimer !== null) {
    window.clearTimeout(productCodeLoadTimer);
  }
  // 搜索结果流式追加时会频繁触发，做 300ms 防抖合并请求
  productCodeLoadTimer = window.setTimeout(async () => {
    productCodeLoadTimer = null;
    const pdfs = currentItems.value.filter(
      (i) => !i.isDir && i.type === "pdf"
    );
    if (pdfs.length === 0 || !authStore.jwt) {
      fileStore.setProductCodes({});
      return;
    }
    const seq = ++productCodeLoadSeq;
    try {
      const map = await productcodeApi.batch(
        pdfs.map((i) => i.path ?? removePrefix(i.url))
      );
      if (seq === productCodeLoadSeq) fileStore.setProductCodes(map);
    } catch {
      /* 产品编号属于增强信息，加载失败静默降级（不展示 subtitle） */
    }
  }, 300);
};

// currentItems 可能被流式搜索原地修改，watch 派生出的字符串签名保证触发
watch(
  () => currentItems.value.map((i) => i.path).join("\n"),
  () => loadProductCodes(),
  { immediate: true }
);


/** 生成"当前页面 URL + 选中项定位参数"：
 *  目录：导航到 `/files/{dirPath}`；
 *  文件：路由仍是 `/files/{parentDir}`，但带上 sel=fileName 参数以便再次打开时高亮它
 *  （这和应用内部双击打开某文件的导航契约一致）。
 *  直接粘贴到浏览器地址栏即可打开同一目录并高亮同一个文件/文件夹。 */
const copyCurrentPageLink = async () => {
  const item = selectedForContextMenuLink.value[0];
  if (!item) return;
  hideContextMenu();

  // 去掉尾部 /，再拼子项：
  //  - 目录：/files/path/to/dir
  //  - 文件：/files/path/to/parent?sel=fileName（父目录路径 + 查询参数定位）
  const base = route.path.replace(/\/+$/, "");
  const rawUrl = item.url;               // e.g. "/质量证明书.pdf" or "/sub/foo.txt"
  let target: string;
  if (item.isDir) {
    target = base + rawUrl;
  } else {
    // sel 参数是要 URL 编码的文件名，和前端"双击文件"路由保持一致
    const fileName = rawUrl.slice(rawUrl.lastIndexOf("/") + 1);
    target = `${base}?sel=${encodeURIComponent(fileName)}`;
  }
  const full =
    window.location.origin +
    target +
    (route.hash && !target.includes(route.hash) ? route.hash : "");

  try {
    await copy({ text: full });
    $showSuccess?.(t("success.linkCopied", "链接已复制！"));
  } catch (e) {
    // 与 Shares.vue 的二次兜底一致：再请求权限重试
    try {
      await copy({ text: full }, { permission: true });
      $showSuccess?.(t("success.linkCopied", "链接已复制！"));
    } catch (e2) {
      $showError(e2 as Error);
    }
  }
};

onBeforeRouteUpdate(() => {
  hideContextMenu();
});

/** 路由切换到新目录 / 返回上一级时，自动退出搜索模式并清空搜索结果。
 *  否则 searchMode 残留为 true，会导致 currentItems 仍使用旧 searchResults，
 *  进入子目录后看到的仍是之前的搜索结果（假条目）而不是新目录的真实内容。 */
watch(
  () => route.path,
  (newPath, oldPath) => {
    if (newPath !== oldPath && fileStore.searchMode) {
      fileStore.clearSearch();
    }
  }
);

const listing = ref<HTMLElement | null>(null);

const nameSorted = computed(() =>
  fileStore.req ? fileStore.req.sorting.by === "name" : false
);

const sizeSorted = computed(() =>
  fileStore.req ? fileStore.req.sorting.by === "size" : false
);

const modifiedSorted = computed(() =>
  fileStore.req ? fileStore.req.sorting.by === "modified" : false
);

const ascOrdered = computed(() =>
  fileStore.req ? fileStore.req.sorting.asc : false
);

const items = computed(() => {
  const dirs: any[] = [];
  const files: any[] = [];
  const range = resolveTimeFilter(timeFilter.value);

  currentItems.value.forEach((item) => {
    if (item.isDir) {
      dirs.push(item);
    } else if (
      matchesTypeFilter(
        item,
        authStore.user?.viewMode ?? "list",
        fileStore.searchMode
      ) &&
      matchesTimeFilter(item.modified, range)
    ) {
      files.push(item);
    }
  });

  return { dirs, files };
});

const dirs = computed(() => {
  // 列表视图：目录区通常很少（虚拟滚动主要解决 files 4000+ 条问题），直接全量渲染
  if (isListView.value) return items.value.dirs;
  // 网格 / 画廊视图：保持原 showLimit 渐进加载
  return items.value.dirs.slice(0, showLimit.value);
});

const files = computed((): Resource[] => {
  // 列表视图：虚拟滚动自己按可视窗口 slice，这里返回全量
  if (isListView.value) return items.value.files;

  // 网格 / 画廊视图：保持原 showLimit 渐进加载
  let _showLimit = showLimit.value - items.value.dirs.length;

  if (_showLimit < 0) _showLimit = 0;

  return items.value.files.slice(0, _showLimit);
});

const nameIcon = computed(() => {
  if (nameSorted.value && !ascOrdered.value) {
    return "arrow_upward";
  }

  return "arrow_downward";
});

const sizeIcon = computed(() => {
  if (sizeSorted.value && ascOrdered.value) {
    return "arrow_downward";
  }

  return "arrow_upward";
});

const modifiedIcon = computed(() => {
  if (modifiedSorted.value && ascOrdered.value) {
    return "arrow_downward";
  }

  return "arrow_upward";
});

const viewModes = [
  { value: "list", icon: "view_module", label: "列表视图" },
  { value: "mosaic", icon: "grid_view", label: "网格视图" },
  { value: "mosaic gallery", icon: "view_list", label: "画廊视图" },
];

/* ======================== 预览视图（本地开关，不修改后端 viewMode） ======================== */
const PREVIEW_KEY = "fb_preview_enabled";
const previewEnabled = ref<boolean>(
  (() => {
    try {
      return localStorage.getItem(PREVIEW_KEY) === "1";
    } catch {
      return false;
    }
  })()
);

const togglePreview = () => {
  previewEnabled.value = !previewEnabled.value;
  try {
    localStorage.setItem(PREVIEW_KEY, previewEnabled.value ? "1" : "0");
  } catch {
    /* ignore quota / disabled storage */
  }
  // 布局变化后重算滚动容器高度
  nextTick(() => fillWindow());
};

/** 当前预览的文件：选中列表中第一个（或唯一个）文件。文件夹/图片/PDF 都可以显示信息或缩略图 */
const previewedItem = computed<ResourceItem | undefined>(() => {
  if (!fileStore.selected || fileStore.selected.length === 0) return undefined;
  const idx = fileStore.selected[0];
  return getItemByIndex(idx);
});

/** 统一扩展名格式：ResourceItem.extension 带点号 (".pdf")，本模块内部使用无点号小写 ("pdf") */
const normExt = (ext: string | undefined | null): string => {
  if (!ext) return "";
  let s = ext.toLowerCase();
  if (s.charAt(0) === ".") s = s.slice(1);
  return s;
};

// 浏览器可直接 <img> 标签预览的图片扩展名
const IMAGE_EXTS = new Set<string>([
  "png",
  "jpg",
  "jpeg",
  "gif",
  "webp",
  "bmp",
  "svg",
  "ico",
  "avif",
]);
const isImagePreview = computed<boolean>(() => {
  if (!previewedItem.value || previewedItem.value.isDir) return false;
  return IMAGE_EXTS.has(normExt(previewedItem.value.extension));
});
const isPdf = computed<boolean>(
  () =>
    !!previewedItem.value &&
    !previewedItem.value.isDir &&
    normExt(previewedItem.value.extension) === "pdf"
);
const isDwg = computed<boolean>(
  () =>
    !!previewedItem.value &&
    !previewedItem.value.isDir &&
    ["dwg", "dxf", "dwt"].includes(normExt(previewedItem.value.extension))
);

/** 根据扩展名选择不同的 Material 大图标 */
const previewedItemIcon = computed<string>(() => {
  const it = previewedItem.value;
  if (!it) return "description";
  if (it.isDir) return "folder";
  const ext = normExt(it.extension);
  if (IMAGE_EXTS.has(ext)) return "image";
  if (ext === "pdf") return "picture_as_pdf";
  if (["doc", "docx"].includes(ext)) return "description";
  if (["xls", "xlsx"].includes(ext)) return "table_chart";
  if (["ppt", "pptx"].includes(ext)) return "slideshow";
  if (["zip", "rar", "7z", "tar", "gz"].includes(ext)) return "folder_zip";
  if (["mp4", "avi", "mov", "mkv"].includes(ext)) return "movie";
  if (["mp3", "wav", "flac"].includes(ext)) return "music_note";
  if (["txt", "md", "log"].includes(ext)) return "article";
  if (isDwg.value) return "draw";
  return "description";
});

const formatSize = (n: number | undefined | null): string => {
  if (typeof n !== "number" || isNaN(n)) return "—";
  return filesize(n);
};
const formatDate = (t: string | undefined | null): string => {
  if (!t) return "";
  try {
    if (authStore.user?.dateFormat) {
      return dayjs(t).format("L LT");
    }
    return dayjs(t).fromNow();
  } catch {
    return "";
  }
};
const onPreviewImageError = () => {
  // 图片加载失败：给出可见的错误提示 + 下载按钮，而不是只留一个裂图图标
  if (!previewError.value) {
    previewError.value = "图片加载失败";
  }
};

/* ================== 预览视图：拖拽调宽 & 多类型预览实现 ================== */

/* pdf.js worker 初始化逻辑已移至 @/utils/previewLoaders.ts，
 * 由 ensurePdfLib() 在首次预览 PDF 时统一执行（与 Preview.vue 共享）。 */

/* ------------ 1) 预览面板宽度 + 拖拽分隔条 ------------ */
const PREVIEW_WIDTH_KEY = "fb_preview_width";
const PREVIEW_WIDTH_DEFAULT = 420; // px，比之前的 380 稍宽以适配 pdf toolbar
const PREVIEW_WIDTH_MIN = 300;
const PREVIEW_WIDTH_MAX_RATIO = 0.6; // 预览区最大 = 屏幕宽度的 60%

const getInitialPreviewWidth = (): number => {
  if (typeof window === "undefined") return PREVIEW_WIDTH_DEFAULT;
  try {
    const v = parseInt(localStorage.getItem(PREVIEW_WIDTH_KEY) || "", 10);
    if (!isNaN(v) && v >= PREVIEW_WIDTH_MIN) return v;
  } catch {
    /* ignore */
  }
  return PREVIEW_WIDTH_DEFAULT;
};

const previewWidth = ref<number>(getInitialPreviewWidth());
const isResizing = ref<boolean>(false);
const _resizeState = {
  startX: 0,
  startWidth: 0,
  maxWidth: PREVIEW_WIDTH_DEFAULT * 2,
};

const previewLayoutStyle = computed(() => {
  // 不预览时不传自定义样式
  if (!previewEnabled.value || isMobile.value) return undefined;
  return undefined; // 布局已在 CSS 控制；preview-pane 自身用 :style 控制宽度
});

const previewPaneStyle = computed(() => {
  if (!previewEnabled.value || isMobile.value) return undefined;
  return {
    flex: `0 0 ${previewWidth.value}px`,
    width: `${previewWidth.value}px`,
    maxWidth: `${previewWidth.value}px`,
    minWidth: `${PREVIEW_WIDTH_MIN}px`,
  } as const;
});

const _onDocumentMouseMove = (e: MouseEvent) => {
  if (!isResizing.value) return;
  e.preventDefault();
  const dx = _resizeState.startX - e.clientX;
  let newWidth = _resizeState.startWidth + dx;
  newWidth = Math.max(PREVIEW_WIDTH_MIN, newWidth);
  newWidth = Math.min(_resizeState.maxWidth, newWidth);
  previewWidth.value = Math.round(newWidth);
};
const _onDocumentMouseUp = () => {
  if (!isResizing.value) return;
  isResizing.value = false;
  document.body.style.cursor = "";
  document.body.style.userSelect = "";
  try {
    localStorage.setItem(PREVIEW_WIDTH_KEY, String(previewWidth.value));
  } catch {
    /* ignore */
  }
};

const onResizerMouseDown = (e: MouseEvent) => {
  isResizing.value = true;
  _resizeState.startX = e.clientX;
  _resizeState.startWidth = previewWidth.value;
  const layoutMax = Math.max(
    PREVIEW_WIDTH_MIN,
    Math.floor(window.innerWidth * PREVIEW_WIDTH_MAX_RATIO)
  );
  _resizeState.maxWidth = layoutMax;
  document.body.style.cursor = "col-resize";
  document.body.style.userSelect = "none";
  e.preventDefault();
  e.stopPropagation();
};

const resetPreviewWidth = () => {
  previewWidth.value = PREVIEW_WIDTH_DEFAULT;
  try {
    localStorage.setItem(PREVIEW_WIDTH_KEY, String(PREVIEW_WIDTH_DEFAULT));
  } catch {
    /* ignore */
  }
};

/* ------------ 2) 预览内容状态 ------------ */
// Refs
const previewBodyRef = ref<HTMLElement | null>(null);
const pdfCanvasRef = ref<HTMLCanvasElement | null>(null);
const pdfScrollRef = ref<HTMLElement | null>(null);

// 通用加载 / 错误态
const previewLoading = ref<boolean>(false);
const previewLoadingText = ref<string>("加载中...");
const previewError = ref<string>("");

// 预览结果缓存
const previewText = ref<string | undefined>(undefined); // txt/csv/code
const previewRenderedHtml = ref<string>(""); // md

// Word (docx) 预览状态（docx-preview 保真渲染，对齐 Example）
const wordHostRef = ref<HTMLElement | null>(null);
const wordStyleRef = ref<HTMLElement | null>(null);
const previewWord = reactive({
  scale: 1,
  fitWidth: true,
  pageCount: 0,
  loading: false,
});

// PDF 状态（对齐 Example：fitMode / 渲染任务 / ResizeObserver）
// 注意：doc 必须放在 shallowReactive 里——pdfjs 的 PDFDocumentProxy 内部使用
// # 私有字段，深度响应式代理会让 getPage() 抛
// "Private element is not present on this object"。
type PdfFitMode = "auto" | "width" | "page";
interface PdfPreviewState {
  doc: any; // PDFDocumentProxy | null（不参与深度响应）
  totalPages: number;
  currentPage: number;
  scale: number;          // 用户缩放倍率（fitMode=auto时乘在自适应基础上）
  fitMode: PdfFitMode;
  rotation: 0 | 90 | 180 | 270;
}
const previewPdf = shallowReactive<PdfPreviewState>({
  doc: null,
  totalPages: 0,
  currentPage: 1,
  scale: 1.0,
  fitMode: "auto",
  rotation: 0,
});
// 取消正在加载的 PDF fetch / 渲染任务
let _pdfAbort: AbortController | null = null;
let _pdfDestroying = false;
// >0 时表示当前 seq 对应的 loadPdf 正在异步等待 fetch/解码，
// pdfDestroy() 不应把 _pdfAbort / previewPdf.doc 等状态干掉，否则
// loadPdf 苏醒后要么被 abort，要么拿到已被 destroy 的 doc，导致
// 空白占位无限停留。
let _pdfLoadActiveSeq: number | null = null;
let _pdfRenderTask: any = null;
let _pdfRo: ResizeObserver | null = null;
let _pdfRenderPending = false;
let _pdfRenderTimer: number | null = null;

const _pdfScheduleRender = () => {
  if (_pdfRenderTimer != null) {
    window.clearTimeout(_pdfRenderTimer);
  }
  _pdfRenderTimer = window.setTimeout(() => {
    _pdfRenderTimer = null;
    pdfRenderPage(previewPdf.currentPage);
  }, 100);
};

// 标记：防止切换过快时旧请求覆盖新内容
let _previewSeq = 0;
const _getSeq = () => ++_previewSeq;
const _seqOk = (mine: number) => mine === _previewSeq;

/* ------------ 3) 文件类型判断（除已有的 image/pdf/dwg 外新增） ------------ */
const MARKDOWN_EXTS = new Set(["md", "markdown", "mdx"]);
// doc：旧版格式，由后端 Word COM 转 .docx 后渲染
const WORD_EXTS = new Set(["docx", "doc"]);
// 文本类：小体积纯文本/代码文件，最大 2MB 限制
const TEXT_EXTS = new Set([
  "txt",
  "log",
  "csv",
  "json",
  "xml",
  "yml",
  "yaml",
  "toml",
  "ini",
  "conf",
  "js",
  "ts",
  "jsx",
  "tsx",
  "vue",
  "css",
  "scss",
  "less",
  "html",
  "htm",
  "py",
  "go",
  "rs",
  "java",
  "kt",
  "c",
  "h",
  "cpp",
  "hpp",
  "cs",
  "rb",
  "php",
  "sh",
  "bash",
  "zsh",
  "fish",
  "ps1",
  "bat",
  "sql",
  "swift",
  "lua",
  "dart",
  "dockerfile",
  "env",
  "gitignore",
  "properties",
]);
const TEXT_SIZE_LIMIT = 2 * 1024 * 1024; // 2MB
const WORD_SIZE_LIMIT = 10 * 1024 * 1024; // 10MB
const MD_SIZE_LIMIT = 2 * 1024 * 1024; // 2MB
const PDF_SIZE_LIMIT = 80 * 1024 * 1024; // 80MB 内才预览，否则提示下载

const extLower = computed(() => normExt(previewedItem.value?.extension));

const isMarkdownPreview = computed<boolean>(
  () =>
    !!previewedItem.value &&
    !previewedItem.value.isDir &&
    MARKDOWN_EXTS.has(extLower.value) &&
    (previewedItem.value.size || 0) <= MD_SIZE_LIMIT
);

const isWordPreview = computed<boolean>(
  () =>
    !!previewedItem.value &&
    !previewedItem.value.isDir &&
    WORD_EXTS.has(extLower.value) &&
    (previewedItem.value.size || 0) <= WORD_SIZE_LIMIT
);

const isCsvPreview = computed<boolean>(
  () =>
    !!previewedItem.value &&
    !previewedItem.value.isDir &&
    extLower.value === "csv" &&
    (previewedItem.value.size || 0) <= TEXT_SIZE_LIMIT
);

const isTextPreview = computed<boolean>(() => {
  if (!previewedItem.value || previewedItem.value.isDir) return false;
  if ((previewedItem.value.size || 0) > TEXT_SIZE_LIMIT) return false;
  return TEXT_EXTS.has(extLower.value);
});

const isPdfOverLimit = computed<boolean>(
  () =>
    !!previewedItem.value &&
    !previewedItem.value.isDir &&
    isPdf.value &&
    (previewedItem.value.size || 0) > PDF_SIZE_LIMIT
);

/* ------------ 4) 下载链接辅助函数 ------------ */
const getDownloadLink = (it: ResourceItem | undefined): string => {
  if (!it) return "";
  // 通过 FileStore 拼一个与现有接口一致的下载 URL
  try {
    return api.getDownloadURL(it as any, true) || "";
  } catch {
    return "";
  }
};

/* ------------ 5) 内容获取：fetch text / arrayBuffer ------------ */
const _authHeaders = (): Record<string, string> => {
  // 后端只接受 X-Auth header（与 fetchURL 约定一致，见 http/auth.go）
  // jwt 存于 localStorage "jwt"，同时浏览器 cookie auth=... 作为后备
  try {
    const jwt = localStorage.getItem("jwt");
    if (jwt) {
      return { "X-Auth": jwt };
    }
  } catch {
    /* ignore */
  }
  return {};
};

const fetchRawUrl = (it: ResourceItem): string => {
  // 与 Preview.vue 一致：api.getDownloadURL 带 raw=true
  try {
    return api.getDownloadURL(it as any, true) || "";
  } catch {
    return (it as any).url || "";
  }
};

const fetchFileAsText = async (it: ResourceItem): Promise<string> => {
  const url = fetchRawUrl(it);
  const resp = await fetch(url, {
    method: "GET",
    headers: _authHeaders(),
    credentials: "same-origin",
    cache: "no-store",
  });
  if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
  return await resp.text();
};

const fetchFileAsBuffer = async (
  it: ResourceItem,
  signal?: AbortSignal
): Promise<ArrayBuffer> => {
  const url = fetchRawUrl(it);
  const resp = await fetch(url, {
    method: "GET",
    headers: _authHeaders(),
    credentials: "same-origin",
    cache: "no-store",
    signal,
  });
  if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
  return await resp.arrayBuffer();
};

/* ------------ 6) PDF 渲染（对齐 Example：fitMode + 渲染取消 + DPR内联 + ResizeObserver） ------------ */
const pdfDestroy = () => {
  _pdfDestroying = true;
  try {
    if (_pdfRenderTimer != null) {
      window.clearTimeout(_pdfRenderTimer);
      _pdfRenderTimer = null;
    }
  } catch { /* ignore */ }
  try {
    if (_pdfRenderTask && typeof _pdfRenderTask.cancel === "function") {
      _pdfRenderTask.cancel();
    }
  } catch { /* ignore */ }
  _pdfRenderTask = null;
  try {
    // 只在没有正在进行中的 loadPdf 时真正 abort 掉当前 fetch/解码
    if (_pdfAbort && !_pdfLoadActiveSeq) {
      _pdfAbort.abort();
    }
  } catch {
    /* ignore */
  }
  if (!_pdfLoadActiveSeq) _pdfAbort = null;
  try {
    _pdfRo?.disconnect();
  } catch { /* ignore */ }
  _pdfRo = null;
  try {
    // 只在没有活动 loadPdf 时销毁 PDFDocument，否则正在加载的 doc 对象会被误销毁
    if (!_pdfLoadActiveSeq) previewPdf.doc?.destroy?.();
  } catch {
    /* ignore */
  }
  if (!_pdfLoadActiveSeq) {
    previewPdf.doc = null;
    previewPdf.totalPages = 0;
    previewPdf.currentPage = 1;
    previewPdf.scale = 1.0;
    previewPdf.fitMode = "auto";
    previewPdf.rotation = 0;
    // 清空 canvas
    if (pdfCanvasRef.value) {
      const ctx = pdfCanvasRef.value.getContext("2d");
      ctx?.clearRect(0, 0, pdfCanvasRef.value.width, pdfCanvasRef.value.height);
      pdfCanvasRef.value.width = 0;
      pdfCanvasRef.value.height = 0;
    }
  }
  _pdfDestroying = false;
};

const pdfRenderPage = async (pageNum: number) => {
  if (_pdfDestroying || !previewPdf.doc) return;
  if (_pdfRenderPending) {
    _pdfScheduleRender();
    return;
  }
  _pdfRenderPending = true;
  try {
    const canvas = pdfCanvasRef.value;
    const container = pdfScrollRef.value;
    if (!canvas || !container || _pdfDestroying || previewPdf.currentPage !== pageNum) {
      return;
    }
    // 取消旧渲染任务，避免乱序
    if (_pdfRenderTask && typeof _pdfRenderTask.cancel === "function") {
      try { _pdfRenderTask.cancel(); } catch { /* ignore */ }
      _pdfRenderTask = null;
    }

    const page = await previewPdf.doc.getPage(pageNum);
    if (_pdfDestroying || previewPdf.currentPage !== pageNum) return;

    // 旋转角度要提前带进 baseViewport，否则 90°/270° 下宽高互换后
    // 「适合页宽 / 适合整页」的 scale 会按错误方向算，导致画面溢出或过小。
    const baseViewport = page.getViewport({ scale: 1, rotation: previewPdf.rotation });
    const padding = 32;
    const availableW = Math.max(container.clientWidth - padding, 100);
    const availableH = Math.max(container.clientHeight - padding, 100);

    // 对齐 Example 的 effectiveScale 计算
    let effectiveScale: number;
    if (previewPdf.fitMode === "width") {
      effectiveScale = availableW / baseViewport.width;
    } else if (previewPdf.fitMode === "page") {
      effectiveScale = Math.min(availableW / baseViewport.width, availableH / baseViewport.height);
    } else {
      // auto：按适合宽度并限制最高 1.5 倍，再乘以用户 scale
      effectiveScale = Math.min(availableW / baseViewport.width, 1.5) * previewPdf.scale;
    }

    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    // DPR 内联进 viewport scale（对应 Example 写法），同时把 rotation 也带进去
    const viewport = page.getViewport({ scale: effectiveScale * dpr, rotation: previewPdf.rotation });
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    canvas.width = Math.floor(viewport.width);
    canvas.height = Math.floor(viewport.height);
    canvas.style.width = Math.floor(viewport.width / dpr) + "px";
    canvas.style.height = Math.floor(viewport.height / dpr) + "px";

    const task = page.render({ canvasContext: ctx, viewport, canvas });
    _pdfRenderTask = task;
    await task.promise;
    _pdfRenderTask = null;
  } catch (err: any) {
    const isCancel =
      err?.name === "RenderingCancelledException" ||
      (err && typeof err.message === "string" && /cancell/i.test(err.message));
    if (isCancel) return;
    throw err;
  } finally {
    _pdfRenderPending = false;
  }
};

const loadPdf = async (it: ResourceItem, seq: number) => {
  previewLoadingText.value = "正在加载 PDF 文档...";
  // 进入加载期：把 abort / doc 交给本轮 loadPdf 独占管理，
  // 防止 resetPreviewState() / pdfDestroy() 中途把关键状态清空。
  _pdfLoadActiveSeq = seq;
  try {
    // 清理旧的（仅 cancel 已排队的渲染定时器，不再 destroy 自己正在用的资源）
    pdfDestroy();
    if (!_seqOk(seq)) return;

    const abort = new AbortController();
    _pdfAbort = abort;
    try {
      // 首次预览 PDF 时才加载 pdfjs-dist（数百 KB），并与文件下载并行
      const [buffer, pdfjs] = await Promise.all([
        fetchFileAsBuffer(it, abort.signal),
        ensurePdfLib(),
      ]);
      if (!_seqOk(seq) || abort.signal.aborted) return;

      const loadingTask = pdfjs.getDocument({
        data: new Uint8Array(buffer),
        useWorkerFetch: false,
        isEvalSupported: false,
        disableFontFace: false,
      });
      const doc = await loadingTask.promise;
      if (!_seqOk(seq) || _pdfDestroying) return;

      previewPdf.doc = doc;
      previewPdf.totalPages = doc.numPages || 0;
      previewPdf.currentPage = 1;
      previewPdf.scale = 1.0;
      previewPdf.fitMode = "auto";
      previewPdf.rotation = 0;
      previewLoading.value = false;

      // 让 v-if="previewPdf.totalPages > 0" 里的 pdfScrollRef/pdfCanvasRef 先挂
      await nextTick();

      // DOM 已就绪才允许真正的 destroy 操作（上面 pdfDestroy 只是软清理）
      if (!_seqOk(seq)) return;

      // 绑定 ResizeObserver：容器尺寸变化→重新计算 fit
      try {
        _pdfRo?.disconnect();
        const scrollEl = pdfScrollRef.value;
        if (scrollEl && typeof ResizeObserver !== "undefined") {
          _pdfRo = new ResizeObserver(() => {
            if (_pdfDestroying) return;
            _pdfScheduleRender();
          });
          _pdfRo.observe(scrollEl);
        }
      } catch { /* ignore */ }

      // 等待一次布局，避免 container.clientWidth/clientHeight 为 0
      // （常见于预览视图刚打开、父容器还未完成布局）
      await _pdfWaitFrame();
      if (!_seqOk(seq)) return;

      await pdfRenderPage(1);
    } catch (err: any) {
      if (err?.name === "AbortError" || !_seqOk(seq)) return;
      throw err;
    }
  } finally {
    if (_pdfLoadActiveSeq === seq) _pdfLoadActiveSeq = null;
  }
};

// Promise-wrapped rAF：下一帧开始时 resolve，用于等浏览器算完布局
const _pdfWaitFrame = (): Promise<void> =>
  new Promise<void>((r) => {
    try {
      (window.requestAnimationFrame || ((cb) => window.setTimeout(cb, 16)))(() => r());
    } catch {
      r();
    }
  });

const pdfPrevPage = async () => {
  if (previewPdf.currentPage <= 1) return;
  previewPdf.currentPage--;
  await pdfRenderPage(previewPdf.currentPage);
};
const pdfNextPage = async () => {
  if (previewPdf.currentPage >= previewPdf.totalPages) return;
  previewPdf.currentPage++;
  await pdfRenderPage(previewPdf.currentPage);
};
const pdfZoomIn = async () => {
  previewPdf.scale = Math.min(4.0, +(previewPdf.scale + 0.25).toFixed(2));
  previewPdf.fitMode = "auto"; // 手动缩放切回 auto
  await pdfRenderPage(previewPdf.currentPage);
};
const pdfZoomOut = async () => {
  previewPdf.scale = Math.max(0.25, +(previewPdf.scale - 0.25).toFixed(2));
  previewPdf.fitMode = "auto";
  await pdfRenderPage(previewPdf.currentPage);
};
const pdfResetScale = async () => {
  previewPdf.scale = 1.0;
  previewPdf.fitMode = "auto";
  await pdfRenderPage(previewPdf.currentPage);
};
const pdfSetFit = async (mode: PdfFitMode) => {
  previewPdf.fitMode = mode;
  previewPdf.scale = 1.0;
  await pdfRenderPage(previewPdf.currentPage);
};

// 左右旋转：每次 ±90°，循环在 0/90/180/270 之间；一旋转就立刻重绘当前页，
// 同时手动把 fitMode 切回 auto 避免因宽高互换卡住之前的 width/page 固定缩放比。
const pdfRotateLeft = async () => {
  previewPdf.rotation = (((previewPdf.rotation - 90) % 360) + 360) % 360 as 0 | 90 | 180 | 270;
  previewPdf.scale = 1.0;
  await pdfRenderPage(previewPdf.currentPage);
};
const pdfRotateRight = async () => {
  previewPdf.rotation = ((previewPdf.rotation + 90) % 360) as 0 | 90 | 180 | 270;
  previewPdf.scale = 1.0;
  await pdfRenderPage(previewPdf.currentPage);
};

// PDF 打印（FileListing 内嵌详情卡片版）：
// 策略与 Preview.pdfPrint 完全一致：
//   A) 页数 <= 100：pdfjs canvas 渲染所有页（带当前 rotation）进临时打印窗口 +
//      @media print 分页 + 自动 win.print()。
//   B) 页数 > 100：fetch + blob(强制 application/pdf) + <embed> 在新标签原生预览，
//      顶部引导横幅，打印时横幅自动隐藏。
// 根因：之前 iframe.contentWindow.print() 在多数浏览器对 PDF 插件无效，导致 catch
// → 走 window.open(blobUrl) → 在无 PDF 预览插件的环境里退化成下载文件。
const FILELISTING_MAX_CANVAS_PRINT = 100;
const pdfPrint = async () => {
  const item = previewedItem.value;
  const rawUrl = item ? fetchRawUrl(item) : "";
  if (!rawUrl) return;

  let blobUrl: string | null = null;

  try {
    // 1. 获取 PDF 二进制 Blob
    const res = await fetch(rawUrl, {
      method: "GET",
      credentials: "same-origin",
      cache: "no-store",
      headers: _authHeaders(),
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);

    const blob = await res.blob();

    // 检查返回的是否真的是 PDF，防止接口权限报错返回 JSON/HTML 导致页面空白
    if (blob.type && !blob.type.includes("pdf") && !blob.type.includes("octet-stream")) {
      const text = await blob.text();
      console.error("后端返回的不是 PDF 文件流:", text);
      alert("无法获取有效的 PDF 文件，请检查接口权限或网络！");
      return;
    }

    // 强行指定 MIME 类型为 application/pdf
    const pdfBlob = blob.type === "application/pdf"
      ? blob
      : new Blob([blob], { type: "application/pdf" });

    blobUrl = URL.createObjectURL(pdfBlob);

    // 2. 创建隐藏 iframe
    const iframe = document.createElement("iframe");
    iframe.style.position = "fixed";
    iframe.style.right = "100%";
    iframe.style.bottom = "100%";
    iframe.style.width = "0px";
    iframe.style.height = "0px";
    iframe.style.border = "none";
    iframe.src = blobUrl;

    // 3. 加载与打印触发逻辑
    iframe.onload = () => {
      // 解绑 onload 防止二次触发
      iframe.onload = null;

      // 给浏览器留出 1 秒渲染 PDF 矢量图的时间
      setTimeout(() => {
        try {
          const iframeWin = iframe.contentWindow;
          const iframeDoc = iframe.contentDocument || iframeWin?.document;

          if (iframeWin && iframeDoc) {
            // 注入打印控制样式：解决 Chrome 默认强制 100% 缩放导致图纸右侧/底部被截断的问题
            const style = iframeDoc.createElement("style");
            style.textContent = `
              @page {
                size: auto;   /* 自动按 PDF 原始尺寸/横纵向排版 */
                margin: 0;    /* 清除边距，防止超出 1 页 */
              }
              html, body {
                margin: 0 !important;
                padding: 0 !important;
                width: 100% !important;
                height: 100% !important;
              }
            `;
            iframeDoc.head?.appendChild(style);

            // 唤起原生打印
            iframeWin.focus();
            try {
              iframeWin.print();
            } catch {
              iframeDoc.execCommand("print", false);
            }
          }
        } catch (e) {
          console.error("唤起原生打印失败，降级在新窗口打开:", e);
          if (blobUrl) window.open(blobUrl, "_blank");
        }

        // 延迟 1 分钟后清除节点和内存，留足打印设置窗口的维持时间
        setTimeout(() => {
          if (document.body.contains(iframe)) {
            document.body.removeChild(iframe);
          }
          if (blobUrl) URL.revokeObjectURL(blobUrl);
        }, 60000);
      }, 1000);
    };

    document.body.appendChild(iframe);
  } catch (err) {
    console.error("[pdfPrint] 打印失败:", err);
    if (blobUrl) URL.revokeObjectURL(blobUrl);
    const fallback = item ? getDownloadLink(item) : "";
    if (fallback) window.open(fallback, "_blank");
  }
};
/* ------------ 7) Markdown / Word / Text 渲染 ------------ */
const loadMarkdown = async (it: ResourceItem, seq: number) => {
  previewLoadingText.value = "正在解析 Markdown...";
  // marked + dompurify 按需加载，与文件下载并行
  const [text, [markedMod, dompurifyMod]] = await Promise.all([
    fetchFileAsText(it),
    ensureMarkdownLibs(),
  ]);
  if (!_seqOk(seq)) return;
  const rawHtml = await markedMod.marked.parse(text, {
    gfm: true,
    breaks: true,
  });
  if (!_seqOk(seq)) return;
  previewRenderedHtml.value = dompurifyMod.sanitize(rawHtml, {
    ADD_ATTR: ["target"],
  });
  previewLoading.value = false;
};

// 旧版 .doc：调后端 Word COM 转换端点获取 docx 字节
const fetchConvertedDocBuffer = async (it: ResourceItem): Promise<ArrayBuffer> => {
  const safePath = it.path?.startsWith("/") ? it.path : "/" + (it.path || "");
  const url = createURL("api/convert/doc" + safePath);
  const resp = await fetch(url, {
    method: "GET",
    headers: _authHeaders(),
    credentials: "same-origin",
    cache: "no-store",
  });
  if (!resp.ok) {
    if (resp.status === 503)
      throw new Error("转换 .doc 失败：服务器需安装 Microsoft Office（Word）");
    throw new Error(`HTTP ${resp.status}`);
  }
  return await resp.arrayBuffer();
};

const loadWord = async (it: ResourceItem, seq: number) => {
  const isLegacyDoc = extLower.value === "doc";
  previewLoadingText.value = isLegacyDoc ? "正在转换并加载 .doc 文档..." : "正在加载 Word 文档...";
  // 清理旧内容 & 重置缩放
  try { wordHostRef.value && (wordHostRef.value.innerHTML = ""); } catch { /* ignore */ }
  try { wordStyleRef.value && (wordStyleRef.value.innerHTML = ""); } catch { /* ignore */ }
  previewWord.pageCount = 0;

  // 模板 v-if 链中 loading 占位优先于 Word 容器：
  // 必须先关闭 previewLoading，wordHostRef / wordStyleRef 才会挂载
  previewLoading.value = false;
  previewWord.loading = true;
  await nextTick();
  const host = wordHostRef.value;
  const styleContainer = wordStyleRef.value;
  if (!_seqOk(seq) || !host || !styleContainer) {
    previewWord.loading = false;
    return;
  }

  const [buffer, docxLib] = await Promise.all([
    isLegacyDoc ? fetchConvertedDocBuffer(it) : fetchFileAsBuffer(it),
    ensureDocxLib(),
  ]);
  if (!_seqOk(seq)) {
    previewWord.loading = false;
    return;
  }

  try {
    const { renderAsync } = docxLib;
    await renderAsync(buffer, host, styleContainer, {
      className: "docx-side-preview",
      inWrapper: true,
      ignoreWidth: false,
      ignoreHeight: false,
      ignoreFonts: false,
      breakPages: true,
      ignoreLastRenderedPageBreak: true,
      experimental: true,
      trimXmlDeclaration: true,
      useBase64URL: true,
      renderChanges: false,
      renderHeaders: true,
      renderFooters: true,
      renderFootnotes: true,
      renderEndnotes: true,
    });
    if (!_seqOk(seq)) return;

    const pages = host.querySelectorAll(
      ".docx-side-preview-wrapper > section.docx, .docx-wrapper > section, section.docx"
    );
    previewWord.pageCount = pages.length || host.querySelectorAll("section").length || 1;
  } catch (e: any) {
    const msg = String(e?.message || e || "");
    if (
      msg.includes("Can't find end of central directory") ||
      msg.includes("Corrupted zip") ||
      msg.toLowerCase().includes("zip")
    ) {
      throw new Error("无法解析该文件，请确认是有效的 Word 文档");
    }
    throw e;
  } finally {
    previewWord.loading = false;
  }
};

const wordZoomIn = () => {
  previewWord.fitWidth = false;
  previewWord.scale = Math.min(2.5, Math.round((previewWord.scale + 0.1) * 10) / 10);
};
const wordZoomOut = () => {
  previewWord.fitWidth = false;
  previewWord.scale = Math.max(0.5, Math.round((previewWord.scale - 0.1) * 10) / 10);
};
const wordResetZoom = () => {
  previewWord.fitWidth = true;
  previewWord.scale = 1;
};
const wordDestroy = () => {
  try { wordHostRef.value && (wordHostRef.value.innerHTML = ""); } catch { /* ignore */ }
  try { wordStyleRef.value && (wordStyleRef.value.innerHTML = ""); } catch { /* ignore */ }
  previewWord.pageCount = 0;
  previewWord.scale = 1;
  previewWord.fitWidth = true;
  previewWord.loading = false;
};

const loadText = async (it: ResourceItem, seq: number) => {
  previewLoadingText.value = "正在加载文本...";
  const text = await fetchFileAsText(it);
  if (!_seqOk(seq)) return;
  previewText.value = text;
  previewLoading.value = false;
};

/* ------------ 8) 统一入口：切换文件时加载对应预览 ------------ */
const resetPreviewState = () => {
  previewLoading.value = false;
  previewError.value = "";
  previewLoadingText.value = "加载中...";
  previewText.value = undefined;
  previewRenderedHtml.value = "";
  pdfDestroy();
  wordDestroy();
};

const loadPreviewContent = async (it: ResourceItem | undefined) => {
  const seq = _getSeq();
  resetPreviewState();
  if (!it || it.isDir) return;

  // 图片：浏览器自己处理 <img src>，无需 fetch
  if (isImagePreview.value) return;

  // PDF 过大 → 显示错误（提示下载）
  if (isPdfOverLimit.value) {
    previewError.value = "文件过大，请下载后查看";
    return;
  }

  previewLoading.value = true;

  try {
    if (isPdf.value) {
      await loadPdf(it, seq);
      return;
    }
    if (isMarkdownPreview.value) {
      await loadMarkdown(it, seq);
      return;
    }
    if (isWordPreview.value) {
      await loadWord(it, seq);
      return;
    }
    if (isTextPreview.value) {
      await loadText(it, seq);
      return;
    }
    // 其他：无需异步加载（由模板显示 placeholder）
    previewLoading.value = false;
  } catch (err: any) {
    if (!_seqOk(seq)) return;
    previewLoading.value = false;
    previewError.value = (err && (err.message || String(err))) || "未知错误";
  }
};

/* 当选中项变化时，自动加载预览 */
watch(
  previewedItem,
  (next) => {
    loadPreviewContent(next);
  },
  { immediate: false }
);

/* 预览面板从关闭切换为打开时补一次渲染：
 * 面板是 v-if 挂载，关闭期间选中文件触发的加载虽然会下载内容，
 * 但渲染依赖的 DOM（pdfCanvasRef/pdfScrollRef/wordHostRef）当时还不存在：
 *  - PDF：doc 已就绪，pdfRenderPage 因 canvas==null 被跳过
 *  - Word：loadWord 因 host==null 直接放弃（pageCount 保持 0）
 * 打开面板后 previewedItem 并未变化，上面的 watch 不会再触发，
 * 必须在这里补渲染；文本/Markdown/图片内容存在响应式状态里，
 * 面板挂载即自动显示，无需处理。 */
watch(previewEnabled, async (enabled) => {
  if (!enabled) return;
  const it = previewedItem.value;
  if (!it || it.isDir) return;

  if (isPdf.value && previewPdf.doc) {
    // doc 已下载过：仅补绑 ResizeObserver + 重渲当前页，不重新下载
    await nextTick();
    if (!previewEnabled.value) return; // 等待期间又被关掉
    try {
      _pdfRo?.disconnect();
      const scrollEl = pdfScrollRef.value;
      if (scrollEl && typeof ResizeObserver !== "undefined") {
        _pdfRo = new ResizeObserver(() => {
          if (_pdfDestroying) return;
          _pdfScheduleRender();
        });
        _pdfRo.observe(scrollEl);
      }
    } catch { /* ignore */ }
    await _pdfWaitFrame(); // 等面板布局完成，拿到真实容器尺寸
    if (previewEnabled.value) await pdfRenderPage(previewPdf.currentPage);
    return;
  }

  // Word 在面板隐藏期间放弃过加载（pageCount 为 0 且不在加载中）：重新加载
  if (
    isWordPreview.value &&
    previewWord.pageCount === 0 &&
    !previewLoading.value &&
    !previewWord.loading
  ) {
    loadPreviewContent(it);
  }
});

/* 卸载前清理 */
onBeforeUnmount(() => {
  pdfDestroy();
  wordDestroy();
  document.removeEventListener("mousemove", _onDocumentMouseMove);
  document.removeEventListener("mouseup", _onDocumentMouseUp);
});

/* 预览拖拽：注册 document 级监听器（全局一次） */
onMounted(() => {
  document.addEventListener("mousemove", _onDocumentMouseMove);
  document.addEventListener("mouseup", _onDocumentMouseUp);
  // 刷新恢复场景：?sel= 的选中在 FileListing 挂载前就已恢复，
  // previewedItem 挂载后不再"变化"，上面的 watch（immediate:false）不会触发，
  // 需要在这里补一次初始加载，否则预览面板一直停在"加载中"
  if (previewedItem.value) {
    loadPreviewContent(previewedItem.value);
  }
});

const currentViewMode = computed(() => authStore.user?.viewMode ?? "list");

const setView = async (mode: string) => {
  layoutStore.closeHovers();

  if (mode === currentViewMode.value) return;

  const data = {
    id: authStore.user?.id,
    viewMode: mode as ViewModeType,
  };

  users.update(data, ["viewMode"]).catch($showError);

  authStore.updateUser(data);

  setItemWeight();
  fillWindow();
};

const headerButtons = computed(() => {
  return {
    upload: authStore.user?.perm.create,
    download: authStore.user?.perm.download,
    shell: authStore.user?.perm.execute && enableExec,
    delete: fileStore.selectedCount > 0 && authStore.user?.perm.delete,
    rename: fileStore.selectedCount === 1 && authStore.user?.perm.rename,
    share:
      fileStore.selectedCount === 1 &&
      authStore.user?.perm.share &&
      authStore.user?.perm.download,
    move: fileStore.selectedCount > 0 && authStore.user?.perm.rename,
    copy: fileStore.selectedCount > 0 && authStore.user?.perm.create,
  };
});

const isMobile = computed(() => {
  // 仅手机竖屏视为移动设备（禁用预览分栏），平板/桌面都启用左右分栏
  return width.value <= 480;
});

watch(req, () => {
  // Reset the show value
  showLimit.value = 50;

  nextTick(() => {
    // 列表视图：初始测量 listing 高度 + 滚回顶部
    const el = listing.value as HTMLElement | undefined;
    if (el) {
      listingHeight.value = el.clientHeight;
      listingScrollTop.value = el.scrollTop;
      if (isListView.value) el.scrollTo({ top: 0 });
    }

    // Ensures that the listing is displayed
    // How much every listing item affects the window height
    setItemWeight();

    // Scroll to the item opened previously
    if (!revealPreviousItem()) {
      // Fill and fit the window with listing items
      fillWindow(true);
    }
  });
});

/**
 * 兜底：当 files 数据首次到达（currentNumFiles 从 0 → >0）时，
 * VirtualList 通过 v-if=true 才刚完成挂载，双 rAF 后显式再测量一次。
 * 没有这个 watch，onMounted 很可能在数据加载前就运行（那时 VirtualList 根本没挂），
 * 之后就再也不会触发 selfOffsetTop 测量 → measuredOnce 一直为 false 或者 selfOffsetTop=0
 * → startIndex 错乱，出现"滚动之前的元素消失/上方空白/从 7.pdf 开始"的 Bug。
 */
watch(
  () => [isListView.value, currentNumFiles.value] as const,
  ([isList, numFiles], [wasList, wasNum]) => {
    if (!isList) return;
    // 从空变为有，或者切换视图模式
    if ((wasNum === 0 && numFiles > 0) || (isList && !wasList)) {
      requestAnimationFrame(() => {
        requestAnimationFrame(() => {
          const el = listing.value as HTMLElement | undefined;
          if (el) {
            listingHeight.value = el.clientHeight;
            listingScrollTop.value = el.scrollTop;
          }
          // 通过强制切换 buffer prop（ref 不换）无法触发，直接靠 VirtualList 内部 watch:[items.length] 自行测量
          // 这里主动触发一次 virtual list 的 runMeasures（通过组件内部 onMounted + items watch 已有处理）
        });
      });
    }
  },
  { flush: "post" }
);

onMounted(() => {
  // Check the columns size for the first time.
  columnsResize();

  // 初始化一次 listing 尺寸（虚拟滚动依赖的初始值，@scroll 首次触发前兜底）
  nextTick(() => {
    const el = listing.value as HTMLElement | undefined;
    if (el) {
      listingHeight.value = el.clientHeight;
      listingScrollTop.value = el.scrollTop;
    }
  });

  // How much every listing item affects the window height
  setItemWeight();

  // Scroll to the item opened previously
  if (!revealPreviousItem()) {
    // Fill and fit the window with listing items
    fillWindow(true);
  }

  // Add the needed event listeners to the window and document.
  window.addEventListener("keydown", keyEvent);
  window.addEventListener("scroll", scrollEvent);
  window.addEventListener("resize", windowsResize);

  if (!authStore.user?.perm.create) return;
  document.addEventListener("dragover", preventDefault);
  document.addEventListener("dragenter", dragEnter);
  document.addEventListener("dragleave", dragLeave);
  document.addEventListener("drop", drop);
});

onBeforeUnmount(() => {
  // Remove event listeners before destroying this page.
  window.removeEventListener("keydown", keyEvent);
  window.removeEventListener("scroll", scrollEvent);
  window.removeEventListener("resize", windowsResize);

  if (authStore.user && !authStore.user?.perm.create) return;
  document.removeEventListener("dragover", preventDefault);
  document.removeEventListener("dragenter", dragEnter);
  document.removeEventListener("dragleave", dragLeave);
  document.removeEventListener("drop", drop);
});

const base64 = (name: string) => Base64.encodeURI(name);

const keyEvent = (event: KeyboardEvent) => {
  // No prompts are shown
  if (layoutStore.currentPrompt !== null) {
    return;
  }

  if (event.key === "Escape") {
    // Reset files selection.
    fileStore.selected = [];
  }

  if (event.key === "Delete") {
    if (!authStore.user?.perm.delete || fileStore.selectedCount == 0) return;

    // Show delete prompt.
    layoutStore.showHover("delete");
  }

  if (event.key === "F2") {
    if (!authStore.user?.perm.rename || fileStore.selectedCount !== 1) return;

    // Show rename prompt.
    layoutStore.showHover("rename");
  }

  // FileListing 内嵌 PDF 预览快捷键：只有当前详情卡片是 PDF 才生效，
  // 避免在输入框里误触发（Chrome/Edge PDF 阅读器的 [ / ] 惯例，不需修饰键）。
  const targetTag = (event.target as HTMLElement)?.tagName;
  const isInput = targetTag === "INPUT" || targetTag === "TEXTAREA" || targetTag === "SELECT";
  if (isPdf && !isInput && previewPdf.totalPages > 0) {
    const mod = event.ctrlKey || event.metaKey;
    // ⌘P 打印：拦截浏览器默认的页面打印，用我们封装的 PDF 专用打印（鉴权安全）
    if (mod && (event.key === "p" || event.key === "P")) {
      event.preventDefault();
      void pdfPrint();
      return;
    }
    if (event.key === "[" || event.key === "{") {
      event.preventDefault();
      void pdfRotateLeft();
      return;
    }
    if (event.key === "]" || event.key === "}") {
      event.preventDefault();
      void pdfRotateRight();
      return;
    }
  }

  // Ctrl is pressed
  if (!event.ctrlKey && !event.metaKey) {
    return;
  }

  switch (event.key) {
    case "f":
    case "F":
      if (event.shiftKey) {
        event.preventDefault();
        layoutStore.showHover("search");
      }
      break;
    case "c":
    case "x":
      copyCut(event);
      break;
    case "v":
      paste(event);
      break;
    case "a":
      event.preventDefault();
      for (const file of items.value.files) {
        if (fileStore.selected.indexOf(file.index) === -1) {
          fileStore.selected.push(file.index);
        }
      }
      for (const dir of items.value.dirs) {
        if (fileStore.selected.indexOf(dir.index) === -1) {
          fileStore.selected.push(dir.index);
        }
      }
      break;
    case "s":
      event.preventDefault();
      document.getElementById("download-button")?.click();
      break;
  }
};

const preventDefault = (event: Event) => {
  // Wrapper around prevent default.
  event.preventDefault();
};

const copyCut = (event: Event | KeyboardEvent): void => {
  if ((event.target as HTMLElement).tagName?.toLowerCase() === "input") return;

  if (currentItems.value.length === 0) return;

  const items = [];

  for (const i of fileStore.selected) {
    const target = getItemByIndex(i);
    if (!target) continue;
    items.push({
      from: target.url,
      name: target.name,
      size: target.size,
      isDir: target.isDir,
      modified: target.modified,
    });
  }

  if (items.length === 0) {
    return;
  }

  clipboardStore.$patch({
    key: (event as KeyboardEvent).key,
    items,
    path: route.path,
  });
};

const paste = async (event: Event) => {
  if ((event.target as HTMLElement).tagName?.toLowerCase() === "input") return;

  // TODO router location should it be
  const items: any[] = [];

  for (const item of clipboardStore.items) {
    const from = item.from.endsWith("/") ? item.from.slice(0, -1) : item.from;
    const to = route.path + encodeURIComponent(item.name);
    items.push({
      from,
      to,
      name: item.name,
      size: item.size,
      isDir: item.isDir,
      modified: item.modified,
      overwrite: false,
      rename: clipboardStore.path == route.path,
    });
  }

  if (items.length === 0) {
    return;
  }

  const preselect = removePrefix(route.path) + items[0].name;

  let action = (overwrite?: boolean, rename?: boolean) => {
    api
      .copy(items, overwrite, rename)
      .then(() => {
        fileStore.preselect = preselect;
        fileStore.reload = true;
      })
      .catch($showError);
  };

  if (clipboardStore.key === "x") {
    action = (overwrite, rename) => {
      api
        .move(items, overwrite, rename)
        .then(() => {
          clipboardStore.resetClipboard();
          fileStore.preselect = preselect;
          fileStore.reload = true;
        })
        .catch($showError);
    };
  }

  const path = route.path.endsWith("/") ? route.path : route.path + "/";
  const conflict = await upload.checkConflict(items, path, true);

  if (conflict.length > 0) {
    layoutStore.showHover({
      prompt: "resolve-conflict",
      props: {
        conflict: conflict,
      },
      confirm: (event: Event, result: Array<ConflictingResource>) => {
        event.preventDefault();
        layoutStore.closeHovers();
        for (let i = result.length - 1; i >= 0; i--) {
          const item = result[i];
          if (item.checked.length == 2) {
            items[item.index].rename = true;
          } else if (item.checked.length == 1 && item.checked[0] == "origin") {
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
};

const columnsResize = () => {
  // Update the columns size based on the window width.
  const items_ = css(["#listing.mosaic .item", ".mosaic#listing .item"]);
  if (items_ === null) return;

  let columns = Math.floor(
    (document.querySelector("main")?.offsetWidth ?? 0) / columnWidth.value
  );
  if (columns === 0) columns = 1;
  items_.style.width = `calc(${100 / columns}% - 1em)`;
};

/**
 * #listing 滚动事件：
 * - list 视图：更新响应式 listingScrollTop / listingHeight，交给虚拟滚动组件取可视区间
 *             （showLimit 渐进加载完全禁用，因为数据已全量传入虚拟列表）
 * - 网格 / 画廊视图：触发原有 scrollEvent 渐进加载逻辑（保持 showLimit 分批）
 * - 节流 16ms（≈ 1 帧），避免滚动时触发过多响应式更新
 */
let _listingRaf: number | null = null;
const onListingScroll = (event: Event) => {
  if (_listingRaf != null) return;
  _listingRaf = requestAnimationFrame(() => {
    _listingRaf = null;
    const el = listing.value as HTMLElement | undefined;
    if (!el) return;
    listingScrollTop.value = el.scrollTop;
    listingHeight.value = el.clientHeight;
    if (!isListView.value) scrollEvent();
  });
};

const scrollEvent = throttle(() => {
  // 列表视图：文件区虚拟滚动自己取可视区间，不走 window 级渐进加载
  if (isListView.value) return;

  const totalItems = currentNumDirs.value + currentNumFiles.value;

  // All items are displayed
  if (showLimit.value >= totalItems) return;

  // 网格 / 画廊视图：滚动容器已从 window 改为 #listing（max-height:80vh overflow-y:auto）
  // 所以触发点也改成基于 #listing 的 scrollTop/clientHeight/scrollHeight
  const el = listing.value as HTMLElement | undefined;
  const currentPos = el
    ? el.scrollTop + el.clientHeight
    : window.innerHeight + window.scrollY;
  const totalH = el
    ? el.scrollHeight
    : document.body.offsetHeight;
  const viewH = el ? el.clientHeight : window.innerHeight;

  // Trigger at the 75% of the viewport height
  const triggerPos = totalH - viewH * 0.25;

  if (currentPos > triggerPos) {
    const showQuantity = Math.ceil((viewH * 2) / itemWeight.value);
    showLimit.value += showQuantity;
  }
}, 100);

const dragEnter = () => {
  dragCounter.value++;

  // When the user starts dragging an item, put every
  // file on the listing with 50% opacity.
  const items = document.getElementsByClassName("item");

  Array.from(items).forEach((file: Element) => {
    (file as HTMLElement).style.opacity = "0.5";
  });
};

const dragLeave = () => {
  dragCounter.value--;

  if (dragCounter.value == 0) {
    resetOpacity();
  }
};

const drop = async (event: DragEvent) => {
  event.preventDefault();
  dragCounter.value = 0;
  resetOpacity();

  const dt = event.dataTransfer;
  let el: HTMLElement | null = event.target as HTMLElement;

  if (fileStore.req === null || dt === null || dt.files.length <= 0) return;

  for (let i = 0; i < 5; i++) {
    if (el !== null && !el.classList.contains("item")) {
      el = el.parentElement;
    }
  }

  const files: UploadList = (await upload.scanFiles(dt)) as UploadList;
  let path = route.path.endsWith("/") ? route.path : route.path + "/";

  if (
    el !== null &&
    el.classList.contains("item") &&
    el.dataset.dir === "true"
  ) {
    // Get url from the ListingItem DOM element (data-url attribute)
    path = el.dataset.url ?? path;

    try {
      (await api.fetch(path)).items;
    } catch (error: any) {
      $showError(error);
      return;
    }
  }

  const conflict = await upload.checkConflict(files, path);

  const preselect = removePrefix(path) + (files[0].fullPath || files[0].name);

  if (conflict.length > 0) {
    layoutStore.showHover({
      prompt: "resolve-conflict",
      props: {
        conflict: conflict,
        isUploadAction: true,
      },
      confirm: (event: Event, result: Array<ConflictingResource>) => {
        event.preventDefault();
        layoutStore.closeHovers();
        for (let i = result.length - 1; i >= 0; i--) {
          const item = result[i];
          if (item.checked.length == 2) {
            continue;
          } else if (item.checked.length == 1 && item.checked[0] == "origin") {
            files[item.index].overwrite = true;
          } else {
            files.splice(item.index, 1);
          }
        }
        if (files.length > 0) {
          upload.handleFiles(files, path, true);
          fileStore.preselect = preselect;
        }
      },
    });

    return;
  }

  upload.handleFiles(files, path);
  fileStore.preselect = preselect;
};

const uploadInput = async (event: Event) => {
  const files = (event.currentTarget as HTMLInputElement)?.files;
  if (files === null) return;

  const folder_upload = !!files[0].webkitRelativePath;

  const uploadFiles: UploadList = [];
  for (let i = 0; i < files.length; i++) {
    const file = files[i];
    const fullPath = folder_upload ? file.webkitRelativePath : undefined;
    uploadFiles.push({
      file,
      name: file.name,
      size: file.size,
      isDir: false,
      fullPath,
    });
  }

  const path = route.path.endsWith("/") ? route.path : route.path + "/";
  const conflict = await upload.checkConflict(uploadFiles, path);

  if (conflict.length > 0) {
    layoutStore.showHover({
      prompt: "resolve-conflict",
      props: {
        conflict: conflict,
        isUploadAction: true,
      },
      confirm: (event: Event, result: Array<ConflictingResource>) => {
        event.preventDefault();
        layoutStore.closeHovers();
        for (let i = result.length - 1; i >= 0; i--) {
          const item = result[i];
          if (item.checked.length == 2) {
            continue;
          } else if (item.checked.length == 1 && item.checked[0] == "origin") {
            uploadFiles[item.index].overwrite = true;
          } else {
            uploadFiles.splice(item.index, 1);
          }
        }
        if (uploadFiles.length > 0) {
          upload.handleFiles(uploadFiles, path, true);
        }
      },
    });

    return;
  }

  upload.handleFiles(uploadFiles, path);
};

const resetOpacity = () => {
  const items = document.getElementsByClassName("item");

  Array.from(items).forEach((file: Element) => {
    (file as HTMLElement).style.opacity = "1";
  });
};

const sort = (by: string, e?: Event) => {
  // 经验 342110：<p role=button> 点击统一 preventDefault + stopPropagation，
  // 避免键盘 Enter/Space 激活按钮时触发滚动/冒泡到父级重绘/焦点闪烁。
  e?.preventDefault?.();
  e?.stopPropagation?.();

  const req = fileStore.req;
  // 无列表数据（例如：单文件预览页 / 分享未加载完成 / 搜索模式空）→ 直接忽略
  if (!req?.items || !req.sorting) return;

  // 方向切换：同一列反复点击 ↔ 升/降序 toggle；切新列默认先降序（与原 icon 逻辑一致）
  const asc = req.sorting.by === by ? !req.sorting.asc : false;
  const newSorting = { by, asc } as typeof req.sorting;

  // 1. 先在内存里排已有的 items → 完全不需要走后端，点击瞬间出结果
  //    Array.sort 是不稳定（不同引擎实现差异），这里拷贝一份避免就地修改。
  const srcItems = req.items as ResourceItem[];
  const sorted: ResourceItem[] = [...srcItems];

  const cmp = (a: ResourceItem, b: ResourceItem): number => {
    // 目录永远排在文件前（与原后端返回顺序一致的视觉规则）
    if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;

    let r = 0;
    switch (by) {
      case "name":
        r = a.name.localeCompare(b.name, undefined, {
          numeric: true,
          sensitivity: "base",
        });
        break;
      case "size":
        r = a.size - b.size;
        break;
      case "modified":
        r =
          new Date(a.modified).getTime() -
          new Date(b.modified).getTime();
        break;
      default:
        r = 0;
    }
    return asc ? r : -r;
  };
  sorted.sort(cmp);

  // 重新分配 index（item.index 被 fileStore.selected 用作选择索引，以及 v-bind:index）
  sorted.forEach((it, i) => {
    // 直接写回 mutable 字段；ResourceItem 的 index 在定义中是 writeable
    (it as unknown as { index: number }).index = i;
  });

  // 2. 立即写回 store → 箭头方向（nameSorted / sizeSorted / ascOrdered / nameIcon）立刻响应式变化
  //    并且使用 updateRequest：它会按 item.url 映射保留已勾选的文件（排序后勾选不丢）
  const nextReq: Resource = {
    ...req,
    sorting: newSorting,
    items: sorted,
    numDirs: sorted.filter((i) => i.isDir).length,
    numFiles: sorted.filter((i) => !i.isDir).length,
  };
  fileStore.updateRequest(nextReq);

  // 3. 偏好持久化异步执行，不阻塞 UI
  //    ❌ 原代码这里 await users.update → fileStore.reload=true 两次 HTTP 往返 + DOM 销毁重建 = 明显闪烁
  //    现在本地排序已出结果，保存偏好只作为下次打开恢复默认排序使用，失败不影响当前排序
  if (authStore.user?.id) {
    users
      .update(
        { id: authStore.user.id, sorting: newSorting as any },
        ["sorting"]
      )
      .catch((err) => {
        // eslint-disable-next-line no-console
        console.warn("[listing.sort] persist user preference failed:", err);
      });
  }

  // 4. 彻底移除 fileStore.reload = true
  //    reload 会触发整个 Resource 重新 fetch → req 先空 → 列表 DOM 先被清掉显示 spinner →
  //    HTTP 回来再重新渲染 → 整片"先空白再恢复"，就是用户感知到的 "页面闪烁"。
  //    现在只做内存排序，req.items 实时更新，v-for :key=base64(name) 稳定，只会移动 DOM。
};

const openSearch = () => {
  layoutStore.showHover("search");
};

const toggleMultipleSelection = () => {
  fileStore.toggleMultiple();
  layoutStore.closeHovers();
};

const windowsResize = throttle(() => {
  columnsResize();
  width.value = window.innerWidth;

  // Listing element is not displayed
  if (listing.value == null) return;

  // How much every listing item affects the window height
  setItemWeight();

  // Fill but not fit the window
  fillWindow();
}, 100);

const download = () => {
  if (currentItems.value.length === 0) return;

  if (fileStore.selectedCount === 1) {
    const target = getItemByIndex(fileStore.selected[0]);
    if (target && !target.isDir) {
      api.download(null, target.url);
      return;
    }
  }

  layoutStore.showHover({
    prompt: "download",
    confirm: (format: any) => {
      layoutStore.closeHovers();

      const files = [];

      if (fileStore.selectedCount > 0) {
        for (const i of fileStore.selected) {
          const target = getItemByIndex(i);
          if (target) files.push(target.url);
        }
      } else {
        files.push(route.path);
      }

      api.download(format, ...files);
    },
  });
};

const uploadFunc = () => {
  if (
    typeof window.DataTransferItem !== "undefined" &&
    typeof DataTransferItem.prototype.webkitGetAsEntry !== "undefined"
  ) {
    layoutStore.showHover("upload");
  } else {
    document.getElementById("upload-input")?.click();
  }
};

const setItemWeight = () => {
  // Listing element is not displayed
  if (listing.value === null) return;

  let itemQuantity = currentNumDirs.value + currentNumFiles.value;
  if (itemQuantity === 0) return;
  if (itemQuantity > showLimit.value) itemQuantity = showLimit.value;

  // How much every listing item affects the window height
  itemWeight.value = listing.value.offsetHeight / itemQuantity;
};

const fillWindow = (fit = false) => {
  // 列表视图：files 全量传入虚拟滚动，showLimit 不限制，无需扩张
  if (isListView.value) return;

  const totalItems = currentNumDirs.value + currentNumFiles.value;
  if (totalItems === 0) return;

  // More items are displayed than the total
  if (showLimit.value >= totalItems && !fit) return;

  // 网格 / 画廊：滚动容器是 #listing（clientHeight）而不是 window
  const el = listing.value as HTMLElement | undefined;
  const viewH = el ? el.clientHeight : window.innerHeight;

  // Quantity of items needed to fill 2x of the window height
  const showQuantity = Math.ceil((viewH + viewH * 2) / itemWeight.value);

  // 加载瞬间 height/itemWeight 可能为 0，导致 showLimit 被置 0
  if (!Number.isFinite(showQuantity) || showQuantity <= 0) return;

  // Less items to display than current
  if (showLimit.value > showQuantity && !fit) return;

  showLimit.value = showQuantity > totalItems ? totalItems : showQuantity;
};

const revealPreviousItem = () => {
  if (!fileStore.req || !fileStore.oldReq) return;

  const index = fileStore.selected[0];
  if (index === undefined) return;

  const el = listing.value as HTMLElement | undefined;
  const viewH = el ? el.clientHeight : window.innerHeight;

  if (isListView.value) {
    // 列表视图：根据 index 是目录还是文件分别定位
    const fileIdx = items.value.files.findIndex((f) => f.index === index);

    if (fileIdx >= 0) {
      // 选中的是文件：走虚拟滚动精确计算所需父级 scrollTop
      nextTick(() => {
        const top = virtualListRef.value?.scrollToIndex(fileIdx);
        if (el && typeof top === "number") el.scrollTo({ top, behavior: "auto" });
      });
      return true;
    }

    // 选中的是目录：目录区全量渲染 → 按 DOM 定位
    nextTick(() => {
      const els = document.querySelectorAll<HTMLElement>("#listing .item");
      const target = Array.from(els).find(
        (e) => Number((e as any).dataset?.index) === index
      ) ?? els[index];
      if (target) {
        target.scrollIntoView({ block: "center" });
      }
    });
    return true;
  }

  // 网格 / 画廊：保持原 showLimit + scrollIntoView
  showLimit.value = index + Math.ceil((viewH * 2) / itemWeight.value);

  nextTick(() => {
    const items = document.querySelectorAll("#listing .item");
    items[index]?.scrollIntoView({ block: "center" });
  });

  return true;
};

const showContextMenu = (event: MouseEvent) => {
  event.preventDefault();
  isContextMenuVisible.value = true;
  // ContextMenu 现已统一使用 position: fixed（相对视口），并内部自动：
  // 1. 测量自身真实宽高（nextTick + rAF 二次测量兜底）
  // 2. 超出右/下边界时自动 flip 到鼠标点左/上方
  // 3. clamp 到视口四边内（留 4px 安全间距）
  // 因此两种视图模式直接透传 clientX / clientY 即可
  contextMenuPos.value = {
    x: event.clientX,
    y: event.clientY,
  };
};

const hideContextMenu = () => {
  isContextMenuVisible.value = false;
};

const handleEmptyAreaClick = (e: MouseEvent) => {
  const target = e.target;
  if (!(target instanceof HTMLElement)) return;

  if (target.dataset.clearOnClick === "true") {
    fileStore.selected = [];
  }
};
</script>
<style scoped>
#listing {
  min-height: calc(100vh - 8rem);
}

.file-selection-margin-bottom {
  margin-bottom: 3.5rem;
}

/* ========== 列表视图文件区虚拟滚动：
 * listing.css 里 #listing > div 会设置 display:flex; flex-wrap:wrap，
 * 会让 VirtualList 根容器变成 flex 容器 → 内部 spacer 无法正常撑高。
 *
 * 注意：.listing-files-virtual 是加在子组件 <VirtualList> 根 div 上的，
 * 必须用 :deep() 才能穿透 scoped 的属性选择器，否则该规则不生效。 */
#listing > :deep(.listing-files-virtual) {
  display: block;
}

/* ========== 列表空态占位（与项目整体 message 风格一致） ========== */
.empty-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  min-height: 60vh;
  padding: 2rem 1rem;
  color: var(--text-color-secondary, #6e6e73);
}

.empty-icon {
  font-size: 4.5rem !important;
  line-height: 1;
  margin-bottom: 1.25rem;
  opacity: 0.7;
  color: inherit;
}

.empty-text {
  margin: 0;
  font-size: 1.15rem;
  font-weight: 400;
  color: inherit;
}

html.dark .empty-placeholder {
  color: #8e8e93;
}
</style>
