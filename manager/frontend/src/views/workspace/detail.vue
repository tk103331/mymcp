<template>
  <n-card>
    <template #header>
      <n-space vertical>
        <span class="title">{{ workspace.name }}</span>
        <span class="description">{{ workspace.description }}</span>
      </n-space>
    </template>

    <n-space vertical size="large">
      <n-space justify="space-between" align="center">
        <n-button type="primary" @click="showAddDialog = true">
            {{ t('workspace.add_service') }}
          </n-button>
        <n-space>
          <n-button type="success" @click="handleStartAll" :loading="loading">
            {{ t('workspace.start_all') }}
          </n-button>
          <n-button type="warning" @click="handleStopAll" :loading="loading">
            {{ t('workspace.stop_all') }}
          </n-button>

        </n-space>
      </n-space>

      <!-- 服务实例列表 -->
      <n-data-table
        :columns="columns"
        :data="instances"
        :loading="loading"
        :pagination="{ pageSize: 10 }"
      />
    </n-space>
  </n-card>
</template>

<style scoped>
.title {
  font-size: 1.2em;
  font-weight: bold;
}
.description {
  color: #666;
}
</style>

<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { h } from 'vue'
import {NCard, NSpace, NH1, NDescriptions, NDescriptionsItem, NDataTable, NButton, NTooltip} from 'naive-ui'
import StepDialog from '@/components/StepDialog.vue'
import { GetWorkspace, SaveServerConfig, DeleteServerConfig } from '../../../wailsjs/go/bind/Data'
import { GetWorkspaceServerInstances, StartWorkspace, StopServerInstance, StartServerInstance } from '../../../wailsjs/go/bind/Manager'

const { t } = useI18n()
const route = useRoute()

const workspace = ref({})
const instances = ref([])
const loading = ref(true)
const showAddDialog = ref(false)

const columns = [
  {
    title: t('instances.name'),
    key: 'config.name',
    render(row) {
      return h(NSpace,{}, {
        default: () => [
            h(NTooltip, {}, {
              trigger: () => [
                  row.config.name
              ]
            })
        ]
      });
    }
  },
  {
    title: t('instances.type'),
    key: 'type'
  },
  {
    title: t('instances.status'),
    key: 'status'
  },
  {
    title: t('instances.actions'),
    key: 'actions',
    width: 220,
    render(row) {
      return h(NSpace, {}, {
        default: () => [
          h(NButton, {
            type: 'success',
            size: 'small',
            onClick: () => handleStartInstance(row.id),
            loading: loading.value,
            disabled: row.status === 'running'
          }, { default: () => t('instances.start') }),
          h(NButton, {
            type: 'warning',
            size: 'small',
            onClick: () => handleStopInstance(row.id),
            loading: loading.value,
            disabled: row.status === 'stopped'
          }, { default: () => t('instances.stop') }),
          h(NButton, {
            type: 'error',
            size: 'small',
            onClick: () => handleDeleteInstance(row.id),
            loading: loading.value
          }, { default: () => t('instances.delete') })
        ]
      })
    }
  }
]

onMounted(async () => {
  const workspaceId = route.params.id
  loading.value = true
  try {
    // 获取工作空间详情
    const workspaceRes = await GetWorkspace(workspaceId)
    workspace.value = workspaceRes

    // 获取关联的服务实例列表
    const instancesRes = await GetWorkspaceServerInstances(workspaceId)
    instances.value = instancesRes
  } catch (error) {
    console.error('Failed to fetch workspace data:', error)
  } finally {
    loading.value = false
  }
})

async function handleAddServerConfig(data) {
  try {
    const workspaceId = route.params.id
    await SaveServerConfig({
      workspace: workspaceId,
      config: data
    })
    // 重新获取实例列表
    const instancesRes = await GetWorkspaceServerInstances(workspaceId)
    instances.value = instancesRes
  } catch (error) {
    console.error('Failed to add service:', error)
  }
}

async function handleStartAll() {
  const workspaceId = route.params.id
  loading.value = true
  try {
    await StartWorkspace(workspaceId)
    // 重新获取实例列表
    const instancesRes = await GetWorkspaceServerInstances(workspaceId)
    instances.value = instancesRes
  } catch (error) {
    console.error('Failed to start all services:', error)
  } finally {
    loading.value = false
  }
}

async function handleStopAll() {
  loading.value = true
  try {
    // 停止所有服务实例
    await Promise.all(instances.value.map(instance => StopServerInstance(instance.id)))
    // 重新获取实例列表
    const instancesRes = await GetWorkspaceServerInstances(route.params.id)
    instances.value = instancesRes
  } catch (error) {
    console.error('Failed to stop all services:', error)
  } finally {
    loading.value = false
  }
}

async function handleStartInstance(instanceId) {
  loading.value = true
  try {
    const err = await StartServerInstance(instanceId)
    if (err) {
      throw new Error(err)
    }
    const instancesRes = await GetWorkspaceServerInstances(route.params.id)
    instances.value = instancesRes
  } catch (error) {
    console.error('Failed to start instance:', error)
  } finally {
    loading.value = false
  }
}

async function handleStopInstance(instanceId) {
  loading.value = true
  try {
    await StopServerInstance(instanceId)
    const instancesRes = await GetWorkspaceServerInstances(route.params.id)
    instances.value = instancesRes
  } catch (error) {
    console.error('Failed to stop instance:', error)
  } finally {
    loading.value = false
  }
}

async function handleDeleteInstance(instanceId) {
  loading.value = true
  try {
    await DeleteServerConfig(instanceId)
    const instancesRes = await GetWorkspaceServerInstances(route.params.id)
    instances.value = instancesRes
  } catch (error) {
    console.error('Failed to delete instance:', error)
  } finally {
    loading.value = false
  }
}
</script>