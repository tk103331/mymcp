<template>
  <n-card>
    <n-h1>设置</n-h1>
    <n-space vertical size="large">
      <n-form>
        <n-form-item label="主题">
          <n-select v-model:value="settings.theme" :options="themeOptions" />
        </n-form-item>
        <n-form-item label="语言">
          <n-select v-model:value="settings.language" :options="languageOptions" />
        </n-form-item>
      </n-form>
      <n-button type="primary" @click="saveSettings">
        保存设置
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