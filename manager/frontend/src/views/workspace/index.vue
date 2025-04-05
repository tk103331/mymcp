<template>
  <n-card>
    <n-h1>{{ t('workspace.title') }}</n-h1>
    <n-space vertical size="large">
      <n-grid :cols="3" :x-gap="16" :y-gap="16">
        <n-grid-item v-for="workspace in workspaces" :key="workspace.id">
          <n-card hoverable>
            <n-space vertical>
              <n-h3>{{ workspace.name }}</n-h3>
              <n-text depth="3">{{ workspace.description }}</n-text>
              <n-button type="primary" @click="openWorkspace(workspace.id)">
                {{ t('workspace.open') }}
              </n-button>
            </n-space>
          </n-card>
        </n-grid-item>
      </n-grid>
      <n-button @click="createWorkspace">
        {{ t('workspace.create') }}
      </n-button>
    </n-space>
  </n-card>
</template>

<script setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const workspaces = ref([
  {
    id: 1,
    name: t('workspace.title') + ' 1',
    description: t('workspace.description')
  },
  {
    id: 2,
    name: t('workspace.title') + ' 2',
    description: t('workspace.description')
  }
])

function openWorkspace(id) {
  // TODO: Implement workspace opening logic
  console.log('Opening workspace:', id)
}

function createWorkspace() {
  // TODO: Implement workspace creation logic
  console.log('Creating new workspace')
}
</script>