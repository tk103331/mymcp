<template>
  <n-modal :show="show" @update:show="emit('update:show', $event)" :title="t('install.title')" preset="dialog" :style="{ width: '600px' }">
    <n-form ref="formRef" :model="formValue" :rules="rules">
      <!-- 工作空间选择 -->
      <n-form-item :label="t('install.workspace')" path="workspace">
        <n-select
          v-model:value="formValue.workspace"
          :options="workspaceOptions"
        />
      </n-form-item>

      <!-- 安装类型选择 -->
      <n-form-item :label="t('install.type')" path="type">
        <n-select
          v-model:value="formValue.type"
          :options="installationTypes"
          @update:value="handleTypeChange"
        />
      </n-form-item>

      <!-- 命令预览 -->
      <template v-if="selectedInstallation">
        <n-form-item :label="t('install.command')">
          <n-input
            type="textarea"
            v-model:value="fullCommand"
            :autosize="{ minRows: 2, maxRows: 4 }"
          />
        </n-form-item>

        <!-- 参数输入 -->
        <template v-if="selectedArguments">
          <n-form-item
            v-for="(arg, key) in selectedArguments"
            :key="key"
            :label="key"
            :path="`arguments.${key}`"
            :required="arg.required"
          >
            <n-input
              v-model:value="formValue.params[key]"
              :placeholder="arg.example || ''"
            />
            <template #feedback>
              {{ arg.description }}
            </template>
          </n-form-item>
        </template>
      </template>
    </n-form>

    <template #action>
      <n-space justify="end">
        <n-button @click="handleCancel">{{ t('install.cancel') }}</n-button>
        <n-button type="primary" @click="handleConfirm" :loading="installing">
          {{ t('install.confirm') }}
        </n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage } from 'naive-ui'
import { LoadWorkspaces, SaveServerConfig } from '../../wailsjs/go/bind/Data'

const props = defineProps({
  show: Boolean,
  serverData: Object
})

const emit = defineEmits(['update:show', 'confirm'])

const { t } = useI18n()
const message = useMessage()
const formRef = ref(null)
const installing = ref(false)

// 表单数据
const formValue = ref({
  type: null,
  workspace: null,
  args: [],
  env: {},
  arguments: {}
})

// 工作空间列表
const workspaces = ref([])

// 安装类型选项
const installationTypes = computed(() => {
  if (!props.serverData?.installations) return []
  return Object.entries(props.serverData.installations).map(([key, value]) => ({
    label: key.toUpperCase(),
    value: key
  }))
})

// 当前选择的安装配置
const selectedInstallation = computed(() => {
  if (!formValue.value.type || !props.serverData?.installations) return null
  return props.serverData.installations[formValue.value.type]
})

// 参数说明
const selectedArguments = computed(() => {
  return props.serverData?.arguments || {}
})

// 表单验证规则
const rules = {
  type: {
    required: true,
    message: t('install.type_required')
  },
  workspace: {
    required: true,
    message: t('install.workspace_required')
  }
}

// 完整命令预览
const fullCommand = computed(() => {
  if (!selectedInstallation.value) return ''
  const { command = '', args = [] } = selectedInstallation.value
  return [command, ...args].join('\n').trim()
})

// 工作空间选项
const workspaceOptions = computed(() => {
  return workspaces.value.map(workspace => ({
    label: workspace.name,
    value: workspace.id
  }))
})

// 加载工作空间列表
async function loadWorkspaces() {
  try {
    workspaces.value = await LoadWorkspaces()
  } catch (err) {
    message.error(t('install.load_workspace_error'))
  }
}

// 处理安装类型变更
function handleTypeChange(type) {
  const installation = props.serverData?.installations?.[type]
  if (!installation) return

  // 重置表单数据
  formValue.value.args = [...(installation.args || [])]
  formValue.value.env = {}
  formValue.value.params = {}
  
  // 初始化参数
  if (props.serverData?.params) {
    Object.keys(props.serverData.params).forEach(key => {
      formValue.value.params[key] = ''
    })
  }
}

// 组件挂载时加载工作空间列表
onMounted(() => {
  loadWorkspaces()
})

// 处理取消
function handleCancel() {
  emit('update:show', false)
}

// 处理确认
async function handleConfirm() {
  try {
    await formRef.value?.validate()
    installing.value = true

    // 构建服务配置数据
    const serverConfig = {
      workspace: formValue.value.workspace,
      transport: 'stdio',
      name: props.serverData.name,
      type: formValue.value.type,
      cmd: fullCommand.value.split('\n').join(' '),
      env: Object.entries(formValue.value.env).map(([key, value]) => `${key}=${value}`),
      ur: '',
      params: formValue.value.params
    }

    // 保存服务配置
    await SaveServerConfig(serverConfig)

    message.success(t('install.success'))
    emit('update:show', false)
  } catch (err) {
    message.error(err.message || t('install.save_error'))
  } finally {
    installing.value = false
  }
}
</script>

<style scoped>
.argument-item {
  margin-bottom: 8px;
}
.argument-description {
  margin-top: 4px;
  margin-left: 16px;
  font-size: 14px;
}
</style>