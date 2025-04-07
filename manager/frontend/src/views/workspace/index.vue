<template>
  <n-card>
    <n-space justify="space-between" align="center">
      <n-h1>{{ t('workspace.title') }}</n-h1>
      <n-button v-if="workspaces.length > 0" type="primary" @click="showCreateModal = true">
        {{ t('workspace.create') }}
      </n-button>
    </n-space>
    <n-space vertical size="large">
      <template v-if="workspaces.length > 0">
        <n-grid :cols="3" :x-gap="16" :y-gap="16">
          <n-grid-item v-for="workspace in workspaces" :key="workspace.id">
            <n-card hoverable>
              <n-space vertical>
                <n-h3>{{ workspace.name }}</n-h3>
                <n-text depth="3">{{ workspace.description }}</n-text>
                <n-space>
                  <n-button type="primary" @click="openWorkspace(workspace.id)">
                    {{ t('workspace.open') }}
                  </n-button>
                  <n-button type="error" @click="showDeleteConfirm(workspace)">
                    {{ t('workspace.delete') }}
                  </n-button>
                </n-space>
              </n-space>
            </n-card>
          </n-grid-item>
        </n-grid>
      </template>
      <template v-else>
        <n-empty :description="t('workspace.empty_description')">
          <template #extra>
            <n-button type="primary" @click="showCreateModal = true">
              {{ t('workspace.create') }}
            </n-button>
          </template>
        </n-empty>
      </template>
    </n-space>

    <n-modal v-model:show="showCreateModal" preset="dialog" :title="t('workspace.create_modal_title')">
      <n-form ref="formRef" :model="formModel" :rules="rules">
        <n-form-item :label="t('workspace.form.name_label')" path="name">
          <n-input v-model:value="formModel.name" :placeholder="t('workspace.form.name_placeholder')" />
        </n-form-item>
        <n-form-item :label="t('workspace.form.description_label')" path="description">
          <n-input v-model:value="formModel.description" type="textarea" :placeholder="t('workspace.form.description_placeholder')" />
        </n-form-item>
      </n-form>
      <template #action>
        <n-button @click="showCreateModal = false">{{ t('common.cancel') }}</n-button>
        <n-button type="primary" @click="handleCreateWorkspace">{{ t('common.create') }}</n-button>
      </template>
    </n-modal>
  </n-card>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useDialog } from 'naive-ui'
import { useRouter } from 'vue-router'
import { LoadWorkspaces, SaveWorkspace, DeleteWorkspace, SaveServerConfig } from '../../../wailsjs/go/bind/Data.js'

const { t } = useI18n()
const dialog = useDialog()
const router = useRouter()

const workspaces = ref([])
const showCreateModal = ref(false)
const formRef = ref(null)
const formModel = ref({
  name: '',
  description: ''
})

const rules = {
  name: {
    required: true,
    message: t('workspace.form.name_required'),
    trigger: 'blur'
  },
  description: {
    required: true,
    message: t('workspace.form.description_required'),
    trigger: 'blur'
  }
}

async function loadWorkspaces() {
  try {
    const data = await LoadWorkspaces()
    if (data) {
      workspaces.value = data
    }
  } catch (error) {
    console.error('Failed to load workspaces:', error)
  }
}

function openWorkspace(id) {
  const workspace = workspaces.value.find(w => w.id === id)
  if (workspace) {
    router.push(`/workspace/${id}`)
  }
}

async function handleCreateWorkspace() {
  try {
    await formRef.value?.validate()
    await SaveWorkspace(formModel.value)
    await loadWorkspaces()
    showCreateModal.value = false
    formModel.value = {
      name: '',
      description: ''
    }
  } catch (error) {
    if (error?.message) {
      console.error('Failed to create workspace:', error)
    }
  }
}

function showDeleteConfirm(workspace) {
  dialog.warning({
    title: t('workspace.delete_confirm_title'),
    content: t('workspace.delete_confirm_content', { name: workspace.name }),
    positiveText: t('workspace.delete_confirm_ok'),
    negativeText: t('workspace.delete_confirm_cancel'),
    onPositiveClick: async () => {
      try {
        await DeleteWorkspace(workspace.id)
        await loadWorkspaces()
      } catch (error) {
        console.error('Failed to delete workspace:', error)
      }
    }
  })
}

onMounted(() => {
  loadWorkspaces()
})
</script>