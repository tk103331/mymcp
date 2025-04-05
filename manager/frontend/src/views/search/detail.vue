<template>
  <n-card>
    <template #header>
      <n-space justify="space-between" align="center">
        <n-h1 style="margin: 0">{{ serverData.display_name }}</n-h1>
        <n-button type="primary" @click="installService(serverData.name)">
          {{ t('search.install') }}
        </n-button>
      </n-space>
    </template>

    <n-space vertical size="large">
      <!-- 基本信息 -->
      <n-descriptions bordered>
        <n-descriptions-item :label="t('search.detail.name')">
          {{ serverData.name }}
        </n-descriptions-item>
        <n-descriptions-item :label="t('search.detail.description')">
          {{ serverData.description }}
        </n-descriptions-item>
        <n-descriptions-item :label="t('search.detail.author')">
          {{ serverData.author?.name }}
        </n-descriptions-item>
        <n-descriptions-item :label="t('search.detail.license')">
          {{ serverData.license }}
        </n-descriptions-item>
        <n-descriptions-item :label="t('search.detail.repository')">
          <n-a :href="serverData.repository?.url" target="_blank">
            {{ serverData.repository?.url }}
          </n-a>
        </n-descriptions-item>
        <n-descriptions-item :label="t('search.detail.homepage')">
          <n-a :href="serverData.homepage" target="_blank">
            {{ serverData.homepage }}
          </n-a>
        </n-descriptions-item>
      </n-descriptions>

      <!-- 标签和分类 -->
      <n-space vertical>
        <n-h3>{{ t('search.detail.categories') }}</n-h3>
        <n-space>
          <n-tag v-for="category in serverData.categories" :key="category" type="info">
            {{ category }}
          </n-tag>
        </n-space>
        <n-space>
          <n-tag v-for="tag in serverData.tags" :key="tag" :type="getTagType(tag)">
            {{ tag }}
          </n-tag>
        </n-space>
      </n-space>

      <!-- 安装说明 -->
      <n-space vertical>
        <n-h3>{{ t('search.detail.installation') }}</n-h3>
        <n-card v-if="serverData.installations?.npm">
          <n-space vertical>
            <n-text>{{ t('search.detail.npm_installation') }}</n-text>
            <n-code :code="getNpmInstallCommand()" language="bash" />
          </n-space>
        </n-card>
      </n-space>

      <!-- 示例 -->
      <n-space vertical v-if="serverData.examples && serverData.examples.length > 0">
        <n-h3>{{ t('search.detail.examples') }}</n-h3>
        <n-collapse>
          <n-collapse-item
            v-for="(example, index) in serverData.examples"
            :key="index"
            :title="example.title"
          >
            <n-text>{{ example.description }}</n-text>
            <n-code :code="example.prompt" language="bash" />
          </n-collapse-item>
        </n-collapse>
      </n-space>

      <!-- 工具列表 -->
      <n-space vertical v-if="serverData.tools && serverData.tools.length > 0">
        <n-h3>{{ t('search.detail.tools') }}</n-h3>
        <n-collapse>
          <n-collapse-item
            v-for="tool in serverData.tools"
            :key="tool.name"
            :title="tool.name"
          >
            <n-space vertical>
              <n-text>{{ tool.description }}</n-text>
              <n-code
                v-if="tool.inputSchema"
                :code="JSON.stringify(tool.inputSchema, null, 2)"
                language="json"
              />
            </n-space>
          </n-collapse-item>
        </n-collapse>
      </n-space>
    </n-space>

    <!-- 安装弹窗 -->
    <install-dialog
      v-model:show="showInstallDialog"
      :server-data="serverData"
      @confirm="handleInstall"
    />
  </n-card>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import servers from '@/assets/data/servers.json'
import InstallDialog from '@/components/InstallDialog.vue'

const route = useRoute()
const { t } = useI18n()

const serverData = ref({})

onMounted(() => {
  const serverName = route.params.name
  serverData.value = servers[serverName] || {}
})

function getTagType(tag) {
  const types = ['default', 'info', 'success', 'warning', 'error']
  const index = Math.abs(tag.split('').reduce((acc, char) => acc + char.charCodeAt(0), 0)) % types.length
  return types[index]
}

function getNpmInstallCommand() {
  if (!serverData.value.installations?.npm) return ''
  const { command, args = [] } = serverData.value.installations.npm
  return `${command} ${args.join(' ')}`
}

const showInstallDialog = ref(false)

function installService(name) {
  showInstallDialog.value = true
}

function handleInstall(installConfig) {
  console.log('Installing service:', serverData.value.name, installConfig)
}
</script>