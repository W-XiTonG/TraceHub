<script setup lang="ts">
import { inject } from 'vue'
import { useI18n } from 'vue-i18n'
import AdminSiteImageField from '../components/admin/AdminSiteImageField.vue'
import { adminSiteSettingsKey } from '../composables/useAdminSiteSettings'

const { t } = useI18n()
const s = inject(adminSiteSettingsKey)!
</script>

<template>
  <div class="space-y-8">
    <section class="th-glass rounded-[var(--th-radius-lg)] p-5 md:p-6">
      <h2 class="text-sm font-semibold text-slate-100">{{ t('adminSystem.secBrand') }}</h2>
      <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('adminSystem.secBrandHint') }}</p>
      <div class="mt-5 space-y-8">
        <div>
          <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{
            t('adminSystem.lblBrandName')
          }}</label>
          <input v-model="s.form.brandName" type="text" class="th-input max-w-xl" autocomplete="off" />
        </div>

        <AdminSiteImageField
          :label="t('adminSystem.lblBrandIcon')"
          :hint="t('adminSystem.hintBrandIcon')"
          :url="s.form.brandIconUrl"
          :preview-src="s.brandIconPreviewSrc"
          :preview-key="`${s.brandIconPreviewSrc}|${s.brandPreviewNonce}`"
          :uploading="s.uploadingImageField === 'icon'"
          :saving="s.saving"
          :upload-label="t('adminSystem.brandIconUpload')"
          :uploading-label="t('adminSystem.brandIconUploading')"
          :preview-label="t('adminSystem.brandIconPreview')"
          :library-label="t('adminSystem.brandIconLibrary')"
          :library-hint="t('adminSystem.brandIconLibraryHint')"
          :library-empty="t('adminSystem.brandIconLibraryEmpty')"
          :delete-title="t('adminSystem.brandIconDelete')"
          :icons-for-ui="s.brandIconsForUi"
          :icons-uploaded-count="s.brandIcons.length"
          :deleting-filename="s.deletingBrandFilename"
          :is-tile-selected="(ico) => s.isImageTileSelected('icon', ico)"
          :icon-title="(ico) => s.brandIconTitle(ico, 'icon')"
          @update:url="s.form.brandIconUrl = $event"
          @upload="s.uploadBrandIcon"
          @select="s.selectLibraryIcon('icon', $event)"
          @delete="void s.confirmDeleteBrandIcon($event)"
        />

        <AdminSiteImageField
          :label="t('adminSystem.lblBrandFavicon')"
          :hint="t('adminSystem.hintBrandFavicon')"
          :url="s.form.brandFaviconUrl"
          :preview-src="s.brandFaviconPreviewSrc"
          :preview-key="`${s.brandFaviconPreviewSrc}|${s.brandPreviewNonce}`"
          :uploading="s.uploadingImageField === 'favicon'"
          :saving="s.saving"
          :upload-label="t('adminSystem.brandFaviconUpload')"
          :uploading-label="t('adminSystem.brandFaviconUploading')"
          :preview-label="t('adminSystem.brandIconPreview')"
          :library-label="t('adminSystem.brandIconLibrary')"
          :library-hint="t('adminSystem.brandFaviconLibraryHint')"
          :library-empty="t('adminSystem.brandIconLibraryEmpty')"
          :delete-title="t('adminSystem.brandIconDelete')"
          :icons-for-ui="s.brandFaviconIconsForUi"
          :icons-uploaded-count="s.brandIcons.length"
          :deleting-filename="s.deletingBrandFilename"
          :is-tile-selected="(ico) => s.isImageTileSelected('favicon', ico)"
          :icon-title="(ico) => s.brandIconTitle(ico, 'favicon')"
          @update:url="s.form.brandFaviconUrl = $event"
          @upload="s.uploadBrandFavicon"
          @select="s.selectLibraryIcon('favicon', $event)"
          @delete="void s.confirmDeleteBrandIcon($event)"
        />

        <AdminSiteImageField
          :label="t('adminSystem.lblBrandOg')"
          :hint="t('adminSystem.hintBrandOg')"
          :url="s.form.brandOgImageUrl"
          :preview-src="s.brandOgPreviewSrc"
          :preview-key="`${s.brandOgPreviewSrc}|${s.brandPreviewNonce}`"
          :uploading="s.uploadingImageField === 'og'"
          :saving="s.saving"
          :upload-label="t('adminSystem.brandOgUpload')"
          :uploading-label="t('adminSystem.brandOgUploading')"
          :preview-label="t('adminSystem.brandOgPreview')"
          :library-label="t('adminSystem.brandIconLibrary')"
          :library-hint="t('adminSystem.brandOgLibraryHint')"
          :library-empty="t('adminSystem.brandIconLibraryEmpty')"
          :delete-title="t('adminSystem.brandIconDelete')"
          :icons-for-ui="s.brandOgIconsForUi"
          :icons-uploaded-count="s.brandIcons.length"
          :deleting-filename="s.deletingBrandFilename"
          :is-tile-selected="(ico) => s.isImageTileSelected('og', ico)"
          :icon-title="(ico) => s.brandIconTitle(ico, 'og')"
          @update:url="s.form.brandOgImageUrl = $event"
          @upload="s.uploadBrandOgImage"
          @select="s.selectLibraryIcon('og', $event)"
          @delete="void s.confirmDeleteBrandIcon($event)"
        />

        <AdminSiteImageField
          :label="t('adminSystem.lblBrandContactCard')"
          :hint="t('adminSystem.hintBrandContactCard')"
          :url="s.form.brandContactCardUrl"
          :preview-src="s.brandContactCardPreviewSrc"
          :preview-key="`${s.brandContactCardPreviewSrc}|${s.brandPreviewNonce}`"
          :uploading="s.uploadingImageField === 'contactCard'"
          :saving="s.saving"
          :upload-label="t('adminSystem.brandContactCardUpload')"
          :uploading-label="t('adminSystem.brandContactCardUploading')"
          :preview-label="t('adminSystem.brandIconPreview')"
          :library-label="t('adminSystem.brandIconLibrary')"
          :library-hint="t('adminSystem.brandContactCardLibraryHint')"
          :library-empty="t('adminSystem.brandIconLibraryEmpty')"
          :delete-title="t('adminSystem.brandIconDelete')"
          :icons-for-ui="s.brandContactCardIconsForUi"
          :icons-uploaded-count="s.brandIcons.length"
          :deleting-filename="s.deletingBrandFilename"
          :is-tile-selected="(ico) => s.isImageTileSelected('contactCard', ico)"
          :icon-title="(ico) => s.brandIconTitle(ico, 'contactCard')"
          @update:url="s.form.brandContactCardUrl = $event"
          @upload="s.uploadContactCard"
          @select="s.selectLibraryIcon('contactCard', $event)"
          @delete="void s.confirmDeleteBrandIcon($event)"
        />
      </div>
    </section>
  </div>
</template>
