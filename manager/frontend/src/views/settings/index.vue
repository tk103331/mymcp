<template>
  <n-card>
    <n-h1>{{ t('settings.title') }}</n-h1>
    <n-space vertical size="large">
      <n-form>
        <n-form-item :label="t('settings.theme')">
          <n-select v-model:value="settings.theme" :options="themeOptions" />
        </n-form-item>
        <n-form-item :label="t('settings.language')">
          <n-select v-model:value="settings.language" :options="languageOptions" />
        </n-form-item>
        <n-form-item :label="t('settings.baseUrl')">
          <n-input v-model:value="settings.baseUrl" placeholder="http://localhost:8080" />
        </n-form-item>
      </n-form>
      <n-button type="primary" @click="saveSettings">
        {{ t('settings.save') }}
      </n-button>
    </n-space>
  </n-card>
</template>

<script setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

const { t, locale } = useI18n()

const settings = ref({
  theme: 'light',
  language: locale.value,
  baseUrl: '',
  notifications: true
})

const themeOptions = [
  { label: t('settings.themes.light'), value: 'light' },
  { label: t('settings.themes.dark'), value: 'dark' }
]

const languageOptions = [
  { label: '简体中文', value: 'zh' },
  { label: 'English', value: 'en' }
]

function saveSettings() {
  locale.value = settings.value.language
  // ... 其他设置保存逻辑
}
</script>