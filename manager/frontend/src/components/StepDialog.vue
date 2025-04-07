<template>
  <n-modal
    :show="show"
    @update:show="emit('update:show', $event)"
    :title="t('workspace.add_service')"
    preset="dialog"
    style="width: 800px"
  >
    <n-steps :current="currentStep" :status="stepStatus">
      <n-step :title="t('workspace.step_search')" />
      <n-step :title="t('workspace.step_config')" />
    </n-steps>

    <div class="step-content" style="margin-top: 20px">
      <!-- 第一步：搜索服务 -->
      <div v-if="currentStep === 0">
        <n-space vertical>
          <n-space align="center">
            <n-select
              v-model:value="selectedCategory"
              :options="categoryOptions"
              :placeholder="t('search.select_category')"
              clearable
              style="width: 200px"
            />
            <n-input-group>
              <n-input
                v-model:value="searchQuery"
                :placeholder="t('search.placeholder')"
                @keyup.enter="performSearch"
              />
              <n-button type="primary" @click="performSearch">
                {{ t('search.button') }}
              </n-button>
            </n-input-group>
          </n-space>

          <n-list v-if="searchResults.length > 0">
            <n-list-item v-for="result in searchResults" :key="result.name">
              <n-thing :title="result.display_name" :description="result.description">
                <template #description>
                  <n-space vertical size="small">
                    <n-text>{{ result.description }}</n-text>
                    <n-space v-if="result.tags && result.tags.length > 0">
                      <n-tag
                        v-for="tag in result.tags"
                        :key="tag"
                        size="small"
                        :type="getTagType(tag)"
                      >
                        {{ tag }}
                      </n-tag>
                    </n-space>
                  </n-space>
                </template>
                <template #header-extra>
                  <n-tag :type="result.type === 'official' ? 'success' : 'warning'">
                    {{ result.type || 'community' }}
                  </n-tag>
                </template>
                <template #footer>
                  <n-space justify="end">
                    <n-button
                      size="small"
                      type="primary"
                      @click="selectService(result)"
                    >
                      {{ t('workspace.select_service') }}
                    </n-button>
                  </n-space>
                </template>
              </n-thing>
            </n-list-item>
          </n-list>
          <n-empty v-else-if="searched" :description="t('search.no_results')" />
        </n-space>
      </div>

      <!-- 第二步：配置参数 -->
      <div v-if="currentStep === 1">
        <n-form ref="formRef" :model="formValue" :rules="rules">
          <n-form-item :label="t('install.type')" path="type">
            <n-select
              v-model:value="formValue.type"
              :options="installationTypes"
              @update:value="handleTypeChange"
            />
          </n-form-item>

          <template v-if="selectedInstallation">
            <n-form-item :label="t('install.command')">
              <n-input
                type="textarea"
                v-model:value="fullCommand"
                :autosize="{ minRows: 2, maxRows: 4 }"
              />
            </n-form-item>

            <template v-if="selectedArguments">
              <n-form-item
                v-for="(arg, key) in selectedArguments"
                :key="key"
                :label="key"
                :path="`arguments.${key}`"
                :required="arg.required"
              >
                <n-input
                  v-model:value="formValue.arguments[key]"
                  :placeholder="arg.example || ''"
                />
                <template #feedback>
                  {{ arg.description }}
                </template>
              </n-form-item>
            </template>
          </template>
        </n-form>
      </div>
    </div>

    <template #action>
      <n-space justify="end">
        <n-button @click="handleCancel">{{ t('install.cancel') }}</n-button>
        <n-button
          v-if="currentStep > 0"
          @click="currentStep--"
        >
          {{ t('workspace.previous') }}
        </n-button>
        <n-button
          type="primary"
          @click="currentStep === 0 ? nextStep() : handleConfirm()"
          :loading="installing"
          :disabled="currentStep === 0 && !selectedService"
        >
          {{ currentStep === 0 ? t('workspace.next') : t('install.confirm') }}
        </n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage } from 'naive-ui'
import servers from '@/assets/data/servers.json'

const props = defineProps({
  show: Boolean
})

const emit = defineEmits(['update:show', 'confirm'])

const { t } = useI18n()
const message = useMessage()
const formRef = ref(null)
const installing = ref(false)

// 步骤控制
const currentStep = ref(0)
const stepStatus = ref('process')

// 搜索相关
const searchQuery = ref('')
const searched = ref(false)
const selectedCategory = ref(null)
const selectedService = ref(null)

// 将servers对象转换为数组
const serverList = computed(() => {
  return Object.entries(servers).map(([name, server]) => ({
    ...server,
    name
  }))
})

// 获取所有唯一的类别
const categories = computed(() => {
  const categorySet = new Set()
  serverList.value.forEach(server => {
    if (server.categories) {
      server.categories.forEach(category => categorySet.add(category))
    }
  })
  return Array.from(categorySet)
})

// 类别选项
const categoryOptions = computed(() => {
  return categories.value.map(category => ({
    label: category,
    value: category
  }))
})

// 搜索结果
const searchResults = ref([])

// 根据标签类型返回不同的样式
function getTagType(tag) {
  const tagTypes = {
    'database': 'info',
    'web': 'success',
    'cache': 'warning',
    'message': 'error',
    'proxy': 'default'
  }
  return tagTypes[tag.toLowerCase()] || 'default'
}

function performSearch() {
  searched.value = true
  const query = searchQuery.value.toLowerCase()
  
  let filteredResults = serverList.value

  // 按类别过滤
  if (selectedCategory.value) {
    filteredResults = filteredResults.filter(server => 
      server.categories && server.categories.includes(selectedCategory.value)
    )
  }

  // 按搜索词过滤
  if (query) {
    filteredResults = filteredResults.filter(server => {
      return server.name.toLowerCase().includes(query) ||
             server.display_name.toLowerCase().includes(query) ||
             server.description.toLowerCase().includes(query)
    })
  }

  searchResults.value = filteredResults
}

// 选择服务
function selectService(service) {
  selectedService.value = service
}

// 下一步
function nextStep() {
  if (!selectedService.value) return
  currentStep.value++
}

// 表单数据
const formValue = ref({
  type: null,
  args: [],
  env: {},
  arguments: {}
})

// 安装类型选项
const installationTypes = computed(() => {
  if (!selectedService.value?.installations) return []
  return Object.entries(selectedService.value.installations).map(([key, value]) => ({
    label: key.toUpperCase(),
    value: key
  }))
})

// 当前选择的安装配置
const selectedInstallation = computed(() => {
  if (!formValue.value.type || !selectedService.value?.installations) return null
  return selectedService.value.installations[formValue.value.type]
})

// 参数说明
const selectedArguments = computed(() => {
  return selectedService.value?.arguments || {}
})

// 表单验证规则
const rules = {
  type: {
    required: true,
    message: t('install.type_required')
  }
}

// 完整命令预览
const fullCommand = computed(() => {
  if (!selectedInstallation.value) return ''
  const { command = '', args = [] } = selectedInstallation.value
  return [command, ...args].join('\n').trim()
})

// 处理安装类型变更
function handleTypeChange(type) {
  const installation = selectedService.value?.installations?.[type]
  if (!installation) return

  // 重置表单数据
  formValue.value.args = [...(installation.args || [])]
  formValue.value.env = {}
  formValue.value.arguments = {}
  
  // 初始化参数
  if (selectedService.value?.arguments) {
    Object.keys(selectedService.value.arguments).forEach(key => {
      formValue.value.arguments[key] = ''
    })
  }
}

// 处理取消
function handleCancel() {
  currentStep.value = 0
  selectedService.value = null
  formValue.value = {
    type: null,
    args: [],
    env: {},
    arguments: {}
  }
  emit('update:show', false)
}

// 处理确认
async function handleConfirm() {
  try {
    await formRef.value?.validate()
    installing.value = true

    // 发送安装事件
    emit('confirm', {
      service: selectedService.value,
      config: {
        type: formValue.value.type,
        args: formValue.value.args,
        env: formValue.value.env
      }
    })

    message.success(t('install.success'))
    handleCancel()
  } catch (err) {
    // 表单验证失败
  } finally {
    installing.value = false
  }
}

// 初始加载时显示所有服务
performSearch()
</script>