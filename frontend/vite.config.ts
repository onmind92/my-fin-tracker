import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { VitePWA } from 'vite-plugin-pwa'

export default defineConfig({ plugins: [react(),VitePWA({
  registerType:'autoUpdate',
  manifest:{
    name:'Личные финансы',short_name:'Финансы',description:'Безопасный трекер личных финансов',
    theme_color:'#0d3327',background_color:'#f5f7f4',display:'standalone',start_url:'/',scope:'/',lang:'ru',
    icons:[{src:'/finance-icon-192.png',sizes:'192x192',type:'image/png',purpose:'any'},{src:'/finance-icon-512.png',sizes:'512x512',type:'image/png',purpose:'any maskable'},{src:'/finance-icon.svg',sizes:'any',type:'image/svg+xml',purpose:'any'}],
  },
  workbox:{
    globPatterns:['**/*.{js,css,html,svg,png,woff2}'],
    globIgnores:['finance-icon-*.png','finance-icon.svg'],
    navigateFallback:'/index.html',
    navigateFallbackDenylist:[/^\/api\//],
    runtimeCaching:[{urlPattern:({url})=>url.pathname.startsWith('/api/'),handler:'NetworkOnly'}],
  },
})] })
