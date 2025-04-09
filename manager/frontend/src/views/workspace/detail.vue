<template>
  <n-card>
    <template #header>
      <n-space justify="space-between" align="center">
        <n-space vertical>
          <span class="title">{{ workspace.name }}</span>
          <span class="description">{{ workspace.description }}</span>
        </n-space>
        <n-space>
          <n-space v-if="workspace.managedClients" align="center">
            <n-button type="info" @click="syncAllManagedClientConfig" circle size="small">
              <n-icon>
                <RefreshOutline />
              </n-icon>
            </n-button>
            <n-text>托管配置：</n-text>
            <n-tooltip v-for="client in supportClients.filter(c => isClientManaged(c))" :key="client.name">
              <template #trigger>
                <img 
                  :src="client.logo" 
                  alt="logo" 
                  style="width: 22px; height: 22px; margin-right: 4px;cursor: pointer;"
                />
              </template>
              <template #content>
                <div>
                  <img 
                    :src="client.logo" 
                    alt="logo" 
                    style="width: 16px; height:16px; margin-right: 4px;"
                  />
                  <n-text>{{ client.label }}</n-text>
                </div>
                <n-text type="info">{{ workspace.managedClients[client.name].config }}</n-text>
              </template>
            </n-tooltip>
          </n-space>
          <n-button type="info" @click="showConfigDialog = true">
            {{ t('workspace.config') }}
          </n-button>
        </n-space>
      </n-space>
    </template>

    <n-space vertical size="large">
      <n-space justify="space-between" align="center">
        <n-button type="primary" @click="showAddDialog = true">
            {{ t('workspace.add_server') }}
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
    <StepDialog v-model:show="showAddDialog" @confirm="handleAddServerConfig"/>
    <n-modal v-model:show="showConfigDialog" :title="t('workspace.config_title')" style="width: 800px">
      <n-card>
        <n-tabs type="line">
          <n-tab-pane 
            v-for="client in supportClients" 
            :key="client.name"
            :name="client.name" 
            :tab="client.label"
          >
            <template #tab>
              <n-space align="center">
                <img :src="client.logo" alt="logo" style="width: 18px; height: 18px;">
                <n-text>{{ client.label }}</n-text>
              </n-space>
            </template>
            <n-space vertical>
              <n-space align="center" justify="space-between" style="margin-bottom: 12px;">
                <n-space align="center">
                  <n-switch :value="isClientManaged(client)" @update:value="(val) => updateManagedClients(client, val)" :disabled="!client.configFile"/>
                  <n-text>{{ t('workspace.manage_config') }}</n-text>
                </n-space>
                <n-text v-if="client.configFile" type="info">
                  {{ client.configFile[osInfo.os]}}
                </n-text>
                <n-text v-else type="warning">
                  暂不支持配置托管
                </n-text>
              </n-space>
              <div style="position:relative;">
                <n-button 
                  style="position:absolute;top:8px;right:8px" 
                  type="primary" 
                  size="small"
                  @click="copyJsonConfig(JSON.stringify(client.configGenerator(instances), null, 2))"
                >
                  {{ t('instances.copy') }}
                </n-button>
                <div style="overflow: auto;border: solid 1px lightgray;height:500px;">
                  <n-code :code="JSON.stringify(client.configGenerator(instances), null, 2)" language="json" :show-line="true" />
                </div>
              </div>
            </n-space>
          </n-tab-pane>
        </n-tabs>
      </n-card>
    </n-modal>
    <n-modal v-model:show="showInstanceConnectDialog" :title="t('instances.connnect_config')" style="width: 600px">
      <n-card>
        <n-text>{{ t('instances.endpoint') }}</n-text>
        <n-input :value="currentInstance.endpoint" readonly style="width: 300px" />
        <n-button @click="copyInstanceEndpoint">
          {{ t('instances.copy') }}
        </n-button>
        <n-tabs type="line" >
          <n-tab-pane 
            v-for="client in supportClients" 
            :key="client.name"
            :name="client.name" 
            :tab="client.label"
          >
            <template #tab>
              <n-space align="center">
                <img :src="client.logo" alt="logo" style="width: 18px; height: 18px;">
                <n-text>{{ client.label }}</n-text>
              </n-space>
            </template>
            <n-space vertical>
              <n-space vertical>
                <div style="position:relative;">
                  <n-button 
                    style="position:absolute;top:8px;right:8px" 
                    type="primary" 
                    size="small"
                    @click="copyJsonConfig(JSON.stringify(client.configGenerator([currentInstance]), null, 2))"
                  >
                    {{ t('instances.copy') }}
                  </n-button>
                  <div style="overflow: auto;border: solid 1px lightgray;height:300px;">
                    <n-code :code="JSON.stringify(client.configGenerator([currentInstance]), null, 2)" language="json" />
                  </div>
                </div>
              </n-space>
            </n-space>
          </n-tab-pane>
        </n-tabs>
      </n-card>
    </n-modal>
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
import {h, onMounted, ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRoute} from 'vue-router'
import {NButton, NCard, NDataTable, NSpace, NTooltip, NTabs, NTabPane, useMessage, NIcon} from 'naive-ui'
import StepDialog from '@/components/StepDialog.vue'
import {DeleteServerConfig, GetWorkspace, SaveServerConfig, SaveWorkspace} from '../../../wailsjs/go/bind/Data'
import {
  GetWorkspaceServerInstances,
  StartServerInstance,
  StartWorkspace,
  StopServerInstance
} from '../../../wailsjs/go/bind/Manager'
import {GetOS, GetArch, ReadFile, WriteFile} from "../../../wailsjs/go/bind/Common";
import { RefreshOutline } from '@vicons/ionicons5'

const { t } = useI18n()
const route = useRoute()
const message = useMessage()

const osInfo = ref({
  os: '',
  arch: '',
})
const workspace = ref({
  managedClients: []
})
const instances = ref([])
const loading = ref(true)
const showAddDialog = ref(false)
const showInstanceConnectDialog = ref(false)
const showConfigDialog = ref(false)
const currentInstance = ref({})
const instanceConnectConfig = ref({
  cursor: '',
  claude: '',
  chatwise: '',
  cherrystudio: ''
})

const columns = [
  {
    title: t('instances.name'),
    key: 'config.name',
    render(row) {
      if (row.status === 'started') {
        return h(NSpace,{}, {
          default: () => [
            h(NTooltip, {}, {
              trigger: () => [
                row.config.name
              ],
              content: () => {

              }
            })
          ]
        });
      } else {
        return row.config.name;
      }
    }
  },
  {
    title: t('instances.type'),
    key: 'config.transport'
  },
  {
    title: t('instances.status'),
    key: 'status'
  },
  {
    title: t('instances.actions'),
    key: 'actions',
    width: 280,
    render(row) {
      return h(NSpace, {}, {
        default: () => [
          h(NButton, {
            type: 'success',
            size: 'small',
            onClick: () => handleStartInstance(row.id),
            loading: loading.value,
            disabled: row.status === 'started'
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
            loading: loading.value,
            disabled: row.status === 'started'
          }, { default: () => t('instances.delete') }),
          h(NButton, {
            type: 'info',
            size: 'small',
            onClick: () => showInstanceConnectConfig(row),
            loading: loading.value
          }, { default: () => t('instances.config') })
        ]
      })
    }
  }
]

onMounted(async () => {
  const workspaceId = route.params.id
  loading.value = true
  try {
    GetOS().then(os => osInfo.value.os = (os ==='windows'?'win':'mac'))
    GetArch().then(arch => osInfo.value.arch = arch)

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
      ...data
    })
    // 重新获取实例列表
    instances.value = await GetWorkspaceServerInstances(workspaceId)
  } catch (error) {
    console.error('Failed to add server:', error)
  }
  await syncAllManagedClientConfig()
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
    console.error('Failed to start all servers:', error)
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
    console.error('Failed to stop all servers:', error)
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

function showInstanceConnectConfig(row) {
  currentInstance.value = row
  instanceConnectConfig.value = getInstanceConnectConfig(row)
  showInstanceConnectDialog.value = true
}

function getInstanceConnectConfig(instance) {
  const config = {};
  for (const client of supportClients) {
    if (client.name === instance.config.transport) {
      config[client.name] = client.configGenerator([instance])
    }
  }
  return config;
}

const supportClients = [
  {
    name: 'cursor',
    label: 'Cursor',
    logo: '/image/cursor.png',
    configFile: {
      win: '',
      mac: '~/.cursor/mcp.json',
      linux: ''
    },
    configGenerator: (instances) => {
      return {
        mcpServers: instances.reduce((acc, instance) => {
          acc[`${instance.config.name}`] = {
            url: instance.endpoint
          }
          return acc
        }, {})
      }
    }
  },
  {
    name: 'claude',
    label: 'Claude',
    logo: '/image/claude.svg',
    configFile: {
      win: '%APPDATA%\Claude\claude_desktop_config.json',
      mac: '~/Library/Application Support/Claude/claude_desktop_config.json',
      linux: ''
    },
    configGenerator: (instances) => {
      return {
        mcpServers: instances.reduce((acc, instance) => {
          acc[`${instance.config.name}`] = {
            url: instance.endpoint
          }
          return acc
        }, {})
      }
    }
  },
  {
    name: 'chatwise',
    label: 'ChatWise',
    logo: '/image/chatwise.png',
    configFile: null,
    configGenerator: (instances) => {
      return {
        mcpServers: instances.reduce((acc, instance) => {
          acc[`${instance.config.name}`] = {
            url: instance.endpoint
          }
          return acc
        }, {})
      }
    }
  },
  {
    name: 'cherrystudio',
    label: 'CherryStudio',
    logo: '/image/cherrystudio.png',
    configFile: null,
    configGenerator: (instances) => {
      return {
        mcpServers: instances.reduce((acc, instance) => {
          acc[`${instance.config.id}`] = {
            name: instance.config.name,
            isActive: true,
            baseUrl: instance.endpoint
          }
          return acc
        }, {})
      }
    }
  }
];

async function copyInstanceEndpoint() {
  try {
    await navigator.clipboard.writeText(currentInstance.value.endpoint)
    message.success(t('instances.copy_success'))
  } catch (error) {
    console.error('Failed to copy:', error)
    message.error(t('instances.copy_failed'))
  }
}

async function copyJsonConfig(jsonStr) {
  try {
    await navigator.clipboard.writeText(jsonStr)
    message.success(t('instances.copy_success'))
  } catch (error) {
    console.error('Failed to copy:', error)
    message.error(t('instances.copy_failed'))
  }
}
function isClientManaged(client) {
  if (!workspace.value.managedClients) {
    workspace.value.managedClients = {};
  }
  return workspace.value.managedClients[client.name] !== undefined;
}
async function updateManagedClients(client, isManaged) {
  if (!workspace.value.managedClients) {
    workspace.value.managedClients = {};
  }
  if (isManaged) {
    if (!workspace.value.managedClients[client.name]) {
      workspace.value.managedClients[client.name] = {
        config: client.configFile[osInfo.value.os]
      };
    }
    await saveManagedClientConfig(client)
    message.success(t('instances.managed_success'))
  } else {
    delete workspace.value.managedClients[client.name];
  }
  SaveWorkspace(workspace.value)
}

async function saveManagedClientConfig(client) {
  if (client && client.configFile) {
      const configFilePath = client.configFile[osInfo.value.os];
      const config = client.configGenerator(instances.value);
      await WriteFile(configFilePath, JSON.stringify(config, null, 2));
  }
}

async function syncAllManagedClientConfig() {
  Object.keys(workspace.value.managedClients).forEach(async clientName => {
    const client = supportClients.find(c => c.name === clientName);
    if (client && client.configFile) {
      await saveManagedClientConfig(client);
    }
  })
  message.success(t('instances.managed_success'))
}
</script>