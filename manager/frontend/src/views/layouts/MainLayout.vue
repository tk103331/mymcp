<template>
  <n-layout has-sider>
    <!-- 侧边栏 -->
    <n-layout-sider
      bordered
      collapse-mode="width"
      :collapsed-width="64"
      :width="160"
      collapsed
    >
      <!-- 主菜单 -->
      <n-menu
        collapsed
        :collapsed-width="64"
        :collapsed-icon-size="22"
        :options="menuOptions"
        :value="activeKey"
        style="height: calc(100vh - 64px)"
      />
      <!-- 底部菜单 -->
      <n-menu
        collapsed
        :collapsed-width="64"
        :collapsed-icon-size="22"
        :options="bottomMenuOptions"
        style="border-top: 1px solid var(--divider-color)"
      />
    </n-layout-sider>

    <!-- 主内容区 -->
    <n-layout-content content-style="padding: 12px;">
      <slot></slot>
    </n-layout-content>
  </n-layout>
</template>

<script setup>
import { ref, h } from 'vue'
import { RouterLink } from 'vue-router'
import { NIcon } from 'naive-ui'
import { 
  SearchOutline,
  BriefcaseOutline,
  GridOutline,
  SettingsOutline,
  PersonOutline
} from '@vicons/ionicons5'
import { useUserStore } from '../../stores'
import { useI18n } from 'vue-i18n'

const userStore = useUserStore()
const activeKey = ref('chat')
const { t } = useI18n()

function renderIcon(icon) {
  return () => h(NIcon, null, { default: () => h(icon) })
}

// 主菜单选项
const menuOptions = [
  {
    label: () => h(RouterLink, { to: '/workspace' }, { default: () => t('menu.workspace') }),
    key: 'workspace',
    icon: renderIcon(BriefcaseOutline)
  },
  {
    label: () => h(RouterLink, { to: '/instances' }, { default: () => t('menu.instances') }),
    key: 'instances',
    icon: renderIcon(GridOutline)
  },
  {
    label: () => h(RouterLink, { to: '/search' }, { default: () => t('menu.search') }),
    key: 'search',
    icon: renderIcon(SearchOutline)
  }
]

// 底部菜单选项
const bottomMenuOptions = [
  {
    label: () => h(RouterLink, { to: '/settings' }, { default: () => t('menu.settings') }),
    key: 'settings',
    icon: renderIcon(SettingsOutline)
  }
]
</script>

<style scoped>
.n-layout {
  height: 100vh;
}

.n-layout-sider {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
}
</style>