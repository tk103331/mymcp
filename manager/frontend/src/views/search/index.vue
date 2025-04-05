<template>
  <n-card>
    <template #header>
      <n-space justify="space-between" align="center">
        <n-h1 style="margin: 0">{{ t('search.title') }}</n-h1>
        <n-space align="center">
          <n-text depth="3" style="font-size: 14px" v-if="searched">
            {{ searchResults.length }} {{ t('search.results_count') }}
          </n-text>
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
        </n-space>
      </n-space>
    </template>
    <n-space vertical size="small">
      <n-list v-if="searchResults.length > 0">
        <n-list-item v-for="result in searchResults" :key="result.name">
          <n-thing
            :title="result.display_name"
            :description="result.description"
          >
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
                <n-button size="small" @click="viewDetails(result.name)">
                  {{ t('search.view_details') }}
                </n-button>
                <n-button
                  size="small"
                  type="primary"
                  @click="installService(result.name)"
                >
                  {{ t('search.install') }}
                </n-button>
              </n-space>
            </template>
          </n-thing>
        </n-list-item>
      </n-list>
      <n-empty v-else-if="searched" :description="t('search.no_results')" />
    </n-space>

    <!-- 安装弹窗 -->
    <install-dialog
      v-model:show="showInstallDialog"
      :server-data="selectedServer"
      @confirm="handleInstall"
    />
  </n-card>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import servers from '@/assets/data/servers.json'
import InstallDialog from '@/components/InstallDialog.vue'

const router = useRouter()

const { t } = useI18n()

const searchQuery = ref('')
const searched = ref(false)
const selectedCategory = ref(null)

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

function viewDetails(name) {
  router.push(`/search/${name}`)
}

const showInstallDialog = ref(false)
const selectedServer = ref(null)

function installService(name) {
  selectedServer.value = servers[name]
  showInstallDialog.value = true
}

function handleInstall(installConfig) {
  console.log('Installing service:', selectedServer.value.name, installConfig)
}

// 初始加载时显示所有服务
performSearch()
</script>