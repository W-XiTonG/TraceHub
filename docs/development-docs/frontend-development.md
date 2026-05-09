# TraceHub 前端开发文档

**版本**：V1.0  
**更新时间**：2026-05-09  
**技术栈**：Vue 3.4+ / Vite 5+ / TailwindCSS / Pinia

---

## 一、前端整体架构

> **[sub2api]** 参考 Sub2API 的前端架构，Vue 3 + Vite + TailwindCSS 组合。

```
web/frontend/
├── public/                       # 静态资源（Favicon 等）
├── src/
│   ├── api/                      # API 调用封装（按模块划分）
│   │   ├── auth.js                # 登录/注册/Token 刷新
│   │   ├── user.js                # 用户 API 调用
│   │   ├── admin.js               # 管理员 API 调用
│   │   └── gateway.js            # 网关 API 调用（聊天测试）
│   ├── components/               # 通用组件
│   │   ├── layout/               # 布局组件（Header/Footer/Sidebar）
│   │   ├── auth/                 # 认证相关（LoginForm/RegisterForm）
│   │   ├── dashboard/            # 仪表盘组件（StatsCard/Chart）
│   │   ├── apikey/               # API Key 管理组件
│   │   ├── channel/              # 渠道管理组件（管理员）
│   │   └── common/               # 通用小组件（Pagination/StatusBadge/CopyButton）
│   ├── views/                    # 页面组件
│   │   ├── home/                 # 官网首页
│   │   │   ├── HeroSection.vue
│   │   │   ├── ModelsSection.vue
│   │   │   ├── PricingSection.vue
│   │   │   ├── FAQSection.vue
│   │   │   └── index.vue
│   │   ├── auth/                 # 认证页面
│   │   │   ├── Login.vue
│   │   │   ├── Register.vue
│   │   │   └── ResetPassword.vue
│   │   ├── dashboard/            # 用户中心仪表盘
│   │   │   ├── Overview.vue
│   │   │   ├── UsageChart.vue
│   │   │   └── index.vue
│   │   ├── apikeys/              # API Key 管理页面
│   │   │   ├── List.vue
│   │   │   ├── Create.vue
│   │   │   └── index.vue
│   │   ├── billing/              # 充值/余额页面
│   │   │   ├── Balance.vue
│   │   │   ├── Recharge.vue
│   │   │   ├── RedeemCode.vue
│   │   │   └── index.vue
│   │   ├── admin/                # 管理后台（子路由）
│   │   │   ├── Dashboard.vue
│   │   │   ├── UserManage.vue
│   │   │   ├── ChannelManage.vue
│   │   │   ├── GroupManage.vue
│   │   │   ├── LogQuery.vue
│   │   │   ├── RedeemCodeManage.vue
│   │   │   ├── SystemConfig.vue
│   │   │   └── index.vue
│   │   ├── docs/                 # API 文档页
│   │   │   ├── ApiReference.vue   # Swagger UI 集成
│   │   │   └── index.vue
│   │   └── NotFound.vue          # 404 页面
│   ├── router/                   # Vue Router 配置
│   │   └── index.js
│   ├── store/                    # Pinia 状态管理
│   │   ├── auth.js                # 认证状态（token/userInfo）
│   │   ├── dashboard.js           # 仪表盘数据
│   │   └── admin.js              # 管理后台数据
│   ├── utils/                    # 工具函数
│   │   ├── request.js            # Axios 封装（请求/响应拦截）
│   │   ├── auth.js                # Token 管理（存储/刷新）
│   │   ├── format.js              # 格式化工具（金额/时间/Token）
│   │   └── constants.js          # 常量定义
│   ├── App.vue                   # 根组件
│   └── main.js                  # 入口文件
├── package.json
├── vite.config.js
├── tailwind.config.js
└── postcss.config.js
```

---

## 二、关键设计决策

### 2.1 API 调用封装（统一拦截器）

```js
// src/utils/request.js

import axios from 'axios'
import { useAuthStore } from '@/store/auth'
import { ElMessage } from 'element-plus'

const request = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api',
  timeout: 30000
})

// 请求拦截器：自动注入 Token
request.interceptors.request.use(config => {
  const authStore = useAuthStore()
  if (authStore.token) {
    config.headers.Authorization = `Bearer ${authStore.token}`
  }
  config.headers['X-Request-ID'] = crypto.randomUUID()
  return config
})

// 响应拦截器：统一错误处理 + Token 自动刷新
request.interceptors.response.use(
  response => {
    // OpenAI 标准错误格式
    if (response.data.error) {
      ElMessage.error(response.data.error.message)
      return Promise.reject(response.data.error)
    }
    return response.data
  },
  async error => {
    if (error.response?.status === 401) {
      // 尝试刷新 Token
      const authStore = useAuthStore()
      const refreshed = await authStore.refreshToken()
      if (refreshed) {
        return request(error.config)
      } else {
        authStore.logout()
        router.push('/login')
      }
    }
    return Promise.reject(error)
  }
)

export default request
```

### 2.2 Pinia 状态管理设计

```js
// src/store/auth.js

import { defineStore, defineSetupStore } from 'pinia'
import { ref, computed } from 'vue'
import authAPI from '@/api/auth'

export const useAuthStore = defineStore('auth', () => {
  // State
  const token = ref(localStorage.getItem('access_token') || '')
  const refreshToken = ref(localStorage.getItem('refresh_token') || '')
  const user = ref(JSON.parse(localStorage.getItem('user') || 'null'))
  
  // Getters
  const isLoggedIn = computed(() => !!token.value)
  const isAdmin = computed(() => user.value?.role === 'admin' || user.value?.role === 'superadmin')
  
  // Actions
  async function login(email, pasword) {
    const res = await authAPI.login({ email, pasword })
    token.value = res.data.access_token
    refreshToken.value = res.data.refresh_token
    user.value = res.data.user
    
    localStorage.setItem('access_token', token.value)
    localStorage.setItem('refresh_token', refreshToken.value)
    localStorage.setItem('user', JSON.stringify(user.value))
  }
  
  async function refreshToken() {
    try {
      const res = await authAPI.refresh({ refresh_token: refreshToken.value })
      token.value = res.data.access_token
      localStorage.setItem('access_token', token.value)
      return true
    } catch {
      return false
    }
  }
  
  function logout() {
    token.value = ''
    refreshToken.value = ''
    user.value = null
    localStorage.clear()
  }
  
  return { token, refreshToken, user, isLoggedIn, isAdmin, login, refreshToken, logout }
})
```

### 2.3 路由守卫（权限控制）

```js
// src/router/index.js

import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/store/auth'

const routes = [
  {
    path: '/',
    component: ()=> import('@/views/home/index.vue')
  },
  {
    path: '/dashboard',
    component: ()=> import('@/views/dashboard/index.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/admin',
    component: ()=> import('@/views/admin/index.vue'),
    meta: { requiresAuth: true, requiresAdmin: true }
  },
  // ...
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach((to, from, next) => {
  const authStore = useAuthStore()
  
  if (to.meta.requiresAuth && !authStore.isLoggedIn) {
    return next('/login')
  }
  
  if (to.meta.requiresAdmin && !authStore.isAdmin) {
    return next('/dashboard')
  }
  
  next()
})

export default router
```

---

## 三、关键页面开发指南

### 3.1 官网首页（`/views/home`）

> 参考产品文档 2.1 节。

**包含的 Section**：

| Section | 组件 | 说明 |
|:---|:---|:---|
| Hero | `HeroSection.vue` | 标题 + Slogan + CTA 按钮 |
| 支持模型 | `ModelsSection.vue` | 标签墙展示已接入模型 |
| 价格与计费 | `PricingSection.vue` | 分模型费率表 |
| API 文档入口 | `APIDocsSection.vue` | 跳转至 `/docs` |
| FAQ | `FAQSection.vue` | 折叠面板展示常见问题 |
| 联系方式 | `ContactSection.vue` | 提交工单/邮箱入口 |

**Hero Section 示例代码**：

```vue
<!-- src/views/home/HeroSection.vue -->

<template>
  <section class="bg-gradient-to-br from-blue-600 to-purple-700 text-white py-20">
    <div class="container mx-auto px-4 text-center">
      <h1 class="text-5xl font-bold mb-4">TraceHub</h1>
      <p class="text-xl mb-8 opacity-90">一站式 AI API 网关 · 统一接入 · 智能调度 · 精确计费</p>
      <div class="space-x-4">
        <router-link to="/register" class="bg-white text-blue-600 px-6 py-3 rounded-lg font-semibold hover:bg-gray-100">
          立即开始
        </router-link>
        <router-link to="/docs" class="border-2 border-white px-6 py-3 rounded-lg font-semibold hover:bg-white hover:text-blue-600">
          查看文档
        </router-link>
      </div>
    </div>
  </section>
</template>
```

### 3.2 用户中心仪表盘（`/views/dashboard`）

> 参考产品文档 2.2 节。

**页面布局**：

```
┌────────────────────────────────────────────────────────┐
│  Header（Logo + 导航 + 用户菜单）                    │
├───────────────────────────────────────────────────────┤
│  Stats Cards（今日消耗 / 本月消耗 / 余额 / API Key数） │
├───────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐                │
│  │  用量趋势图   │  │  模型分布图   │                │
│  │  (Chart.js)   │  │  (Chart.js)   │                │
│  └──────────────┘  └──────────────┘                │
├───────────────────────────────────────────────────────┤
│  API Key 列表（简要）                                │
│  [生成新 Key] 按钮                                   │
└───────────────────────────────────────────────────────┘
```

### 3.3 管理后台（`/views/admin`）

> 参考产品文档 3.2 节 + Sub2API 管理后台 UI 设计。

**管理后台布局**（Sidebar + Main Content）：

```vue
<!-- src/views/admin/index.vue -->

<template>
  <div class="flex h-screen">
    <!-- Sidebar -->
    <aside class="w-64 bg-gray-900 text-white flex flex-col">
      <div class="p-4 border-b border-gray-700">
        <h1 class="text-xl font-bold">TraceHub 管理后台</h1>
      </div>
      <nav class="flex-1 p-4 space-y-2">
        <router-link to="/admin/dashboard" class="block px-4 py-2 rounded hover:bg-gray-800">
          📊 总览仪表盘
        </router-link>
        <router-link to="/admin/users" class="block px-4 py-2 rounded hover:bg-gray-800">
          👥 用户管理
        </router-link>
        <router-link to="/admin/channels" class="block px-4 py-2 rounded hover:bg-gray-800">
          🔌 渠道管理
        </router-link>
        <router-link to="/admin/groups" class="block px-4 py-2 rounded hover:bg-gray-800">
          👥 分组管理
        </router-link>
        <router-link to="/admin/logs" class="block px-4 py-2 rounded hover:bg-gray-800">
          📝 调用日志
        </router-link>
        <router-link to="/admin/redeem-codes" class="block px-4 py-2 rounded hover:bg-gray-800">
          🎫 兑换码管理
        </router-link>
        <router-link to="/admin/config" class="block px-4 py-2 rounded hover:bg-gray-800">
          ⚙️ 系统配置
        </router-link>
      </nav>
    </aside>
    
    <!-- Main Content -->
    <main class="flex-1 overflow-auto bg-gray-100 p-6">
      <router-view />
    </main>
  </div>
</template>
```

---

## 四、Swagger UI 集成（API 文档页）

> 参考产品文档 2.1 节的 API 文档入口要求。

```js
// vite.config.js

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      },
      '/swagger': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  }
})
```

后端提供 Swagger UI 静态文件（通过 `swag` 工具生成）：

```go
// internal/handler/docs.go

import "github.com/swag/gin-swagger"
import "github.com/swag/gin-swagger/swaggerFiles"

// @title TraceHub API
// @version 1.0
// @description TraceHub AI API Gateway API 文档

r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
```

---

## 五、构建与部署

### 5.1 开发模式

```bash
cd web/frontend

# 安装依赖
pnpm install

# 启动开发服务器（热重载）
pnpm run dev

# 前端开发服务器默认运行在 http://localhost:5173
# API 请求通过 Vite proxy 转发到后端（http://localhost:8080）
```

### 5.2 生产构建

```bash
cd web/frontend

# 构建生产版本（输出到 dist/）
pnpm run build

# 预览生产构建结果
pnpm run preview
```

### 5.3 嵌入后端二进制（embed）

> **[sub2api]** 参考 Sub2API 的 embed 模式，前端构建产物嵌入 Go 二进制。

```bash
# 构建前端
cd web/frontend && pnpm run build

# 构建后端（embed 前端产物）
cd ../../
go build -tags embed -o tracehub ./cmd/server

# 验证（无需独立部署前端）
./tracehub
# 访问 http://localhost:8080/ 即可看到前端页面
```

```go
// internal/handler/static.go

package handler

import (
    "embed"
    "io/fs"
    "net/http"
    
    "github.com/gin-gonic/gin"
)

//go:embed all:../../web/frontend/dist
var frontendDist embed.FS

func RegisterStaticRoutes(r *gin.Engine) {
    distFS, _ := fs.Sub(frontendDist, "web/frontend/dist")
    
    // SPA 路由兜底（所有未匹配的路由返回 index.html）
    r.NoRoute(func(c *gin.Context) {
        c.FileFromFS("/", embed.FS(distFS))
    })
}
```

---

## 六、样式规范（TailwindCSS）

### 6.1 设计 Token

```js
// tailwind.config.js

export default {
  theme: {
    extend: {
      colors: {
        primary: {
          50: '#eff6ff',
          500: '#3b82f6',
          600: '#2563eb',
          700: '#1d4ed8',
        },
        gray: {
          50: '#f9fafb',
          100: '#f3f4f6',
          800: '#1f2937',
          900: '#111827',
        }
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', 'sans-serif'],
      }
    },
  },
  plugins: [
    require('@tailwindcss/forms'),
  ]
}
```

### 6.2 常用组件类约定

| 组件 | Tailwind 类约定 |
|:---|:---|
| 卡片容器 | `bg-white rounded-xl shadow p-6` |
| 主按钮 | `bg-primary-600 hover:bg-primary-700 text-white px-6 py-3 rounded-lg` |
| 次按钮 | `border-2 border-primary-600 text-primary-600 px-6 py-3 rounded-lg` |
| 输入框 | `w-full px-4 py-3 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500` |
| 状态徽章（成功） | `px-2 py-1 text-xs bg-green-100 text-green-800 rounded-full` |
| 状态徽章（危险） | `px-2 py-1 text-xs bg-red-100 text-red-800 rounded-full` |

---

## 七、环境变量配置

```bash
# .env.development

VITE_API_BASE_URL=http://localhost:8080/api
VITE_APP_NAME=TraceHub
VITE_STRIPE_PUBLISHABLE_KEY=pk_test_xxx
VITE_GA_ID=G-XXXXXXXXXX  # Google Analytics（可选）
```

```bash
# .env.production

VITE_API_BASE_URL=/api   # 生产环境使用相对路径（同源部署）
VITE_APP_NAME=TraceHub
```

---

## 八、总结

前端开发完全参考 Sub2API 的 Vue 3 + Vite + TailwindCSS 技术选型，重点页面覆盖产品文档要求的所有功能模块：

| 页面 | 覆盖功能 | 参考 |
|:---|:---|:---|
| 官网首页 | Hero / 模型展示 / 价格 / FAQ / API 文档入口 | 产品文档 2.1 |
| 用户中心 | 仪表盘 / API Key 管理 / 充值 / 调用日志 / 兑换码 | 产品文档 2.2 |
| 管理后台 | 总览 / 用户管理 / 渠道管理 / 分组 / 日志 / 兑换码 / 配置 | 产品文档 3.2 |
| API 文档 | Swagger UI 集成 | 产品文档 2.1 |
