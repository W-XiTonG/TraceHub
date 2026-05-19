<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

const model = defineModel<string>({ default: '' })

withDefaults(
  defineProps<{
    autocomplete?: string
    required?: boolean
    minlength?: number
    id?: string
  }>(),
  {
    autocomplete: 'current-password',
    required: false,
  },
)

const { t } = useI18n()
const show = ref(false)
const inputType = computed(() => (show.value ? 'text' : 'password'))
const toggleLabel = computed(() => (show.value ? t('common.hidePassword') : t('common.showPassword')))
</script>

<template>
  <div class="th-password-input">
    <input
      :id="id"
      v-model="model"
      :type="inputType"
      :required="required"
      :minlength="minlength"
      :autocomplete="autocomplete"
      class="th-input th-password-input__field"
    />
    <button
      type="button"
      class="th-password-input__toggle"
      :aria-label="toggleLabel"
      :title="toggleLabel"
      :aria-pressed="show"
      tabindex="-1"
      @click="show = !show"
    >
      <svg
        v-if="show"
        class="th-password-input__icon"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.75"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7Z" />
        <circle cx="12" cy="12" r="3" />
      </svg>
      <svg
        v-else
        class="th-password-input__icon"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.75"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-10-8-10-8a18.45 18.45 0 0 1 5.06-5.94" />
        <path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 10 8 10 8a18.5 18.5 0 0 1-2.16 3.19" />
        <path d="M1 1l22 22" />
        <path d="M14.12 14.12a3 3 0 1 1-4.24-4.24" />
      </svg>
    </button>
  </div>
</template>
