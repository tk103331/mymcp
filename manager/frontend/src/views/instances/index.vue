<template>
  <n-card>
    <n-h1>{{ t('instances.title') }}</n-h1>
    <n-space vertical size="large">
      <n-data-table
        :columns="columns"
        :data="instances"
        :pagination="pagination"
      />
      <n-button type="primary" @click="createInstance">
        {{ t('instances.create') }}
      </n-button>
    </n-space>
  </n-card>
</template>

<script setup>
import { ref, h } from 'vue'
import { useI18n } from 'vue-i18n'
import { NButton, NSpace } from 'naive-ui'

const { t } = useI18n()

const instances = ref([
  {
    id: 1,
    name: 'Instance 1',
    status: 'running',
    type: 'Minecraft Server',
    port: 25565
  },
  {
    id: 2,
    name: 'Instance 2',
    status: 'stopped',
    type: 'Minecraft Server',
    port: 25566
  }
])

const columns = [
  {
    title: t('instances.name'),
    key: 'name'
  },
  {
    title: t('instances.status'),
    key: 'status'
  },
  {
    title: t('instances.type'),
    key: 'type'
  },
  {
    title: t('instances.port'),
    key: 'port'
  },
  {
    title: t('instances.actions'),
    key: 'actions',
    render: (row) => {
      return h(NSpace, null, {
        default: () => [
          h(NButton, {
            size: 'small',
            onClick: () => manageInstance(row.id)
          }, { default: () => t('instances.manage') }),
          h(NButton, {
            size: 'small',
            type: row.status === 'running' ? 'warning' : 'primary',
            onClick: () => toggleInstance(row.id)
          }, { default: () => t(row.status === 'running' ? 'instances.stop' : 'instances.start') })
        ]
      })
    }
  }
]

const pagination = {
  pageSize: 10
}

function createInstance() {
  // TODO: Implement instance creation logic
  console.log('Creating new instance')
}

function manageInstance(id) {
  // TODO: Implement instance management logic
  console.log('Managing instance:', id)
}

function toggleInstance(id) {
  // TODO: Implement instance toggle logic
  console.log('Toggling instance:', id)
}
</script>