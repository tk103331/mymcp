import { defineStore } from 'pinia'

export const useUserStore = defineStore('user', {
  state: () => ({
    name: '',
    isLoggedIn: false
  }),
  
  actions: {
    setUser(name) {
      this.name = name
      this.isLoggedIn = true
    },
    
    logout() {
      this.name = ''
      this.isLoggedIn = false
    }
  },
  
  getters: {
    userDisplayName: (state) => state.name || '游客'
  }
}) 