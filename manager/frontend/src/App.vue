<template>
  <n-config-provider :hljs="hljs">
    <n-message-provider>
      <n-notification-provider>
        <n-dialog-provider>
          <n-loading-bar-provider>
            <main-layout>
              <router-view></router-view>
            </main-layout>
          </n-loading-bar-provider>
        </n-dialog-provider>
      </n-notification-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<script setup>
import hljs from 'highlight.js/lib/core'
import json from 'highlight.js/lib/languages/json'
import MainLayout from './views/layouts/MainLayout.vue'
// 组件逻辑

hljs.registerLanguage('json', json)
</script>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}
</style> 