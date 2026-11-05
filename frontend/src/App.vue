<script setup lang="ts">
import { onMounted, ref } from 'vue'

type ApiStatus = 'checking' | 'online' | 'offline'

const apiStatus = ref<ApiStatus>('checking')
const apiLabel = {
  checking: '检查中',
  online: '服务正常',
  offline: '连接失败',
}

async function checkApi() {
  apiStatus.value = 'checking'

  try {
    const response = await fetch('/api/v1/health')
    if (!response.ok) throw new Error('API response error')
    const data: { status?: string } = await response.json()
    apiStatus.value = data.status === 'ok' ? 'online' : 'offline'
  } catch {
    apiStatus.value = 'offline'
  }
}

onMounted(checkApi)
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar">
      <a class="brand" href="/" aria-label="Subscription Hub 首页">
        <span class="brand-mark">S</span>
        <span>Subscription Hub</span>
      </a>

      <div class="nav-label">工作区</div>
      <nav class="nav-menu" aria-label="主导航">
        <a class="nav-link active" href="/" aria-current="page">概览</a>
        <span class="nav-link disabled">订阅管理 <small>即将推出</small></span>
        <span class="nav-link disabled">资源管理 <small>即将推出</small></span>
        <span class="nav-link disabled">支出统计 <small>即将推出</small></span>
        <span class="nav-link disabled">提醒中心 <small>即将推出</small></span>
      </nav>

      <div class="sidebar-footer">开源项目 · MIT License</div>
    </aside>

    <main class="main">
      <header class="page-header">
        <div>
          <div class="eyebrow">Subscription Hub / Overview</div>
          <h1>概览</h1>
          <p>统一管理订阅、费用与资源有效期。</p>
        </div>
        <span class="preview-tag">开发预览</span>
      </header>

      <section class="metrics" aria-label="指标概览">
        <div class="metric">
          <span>活跃订阅</span>
          <strong>—</strong>
          <small>功能开发中</small>
        </div>
        <div class="metric">
          <span>月均支出</span>
          <strong>—</strong>
          <small>等待账单模块</small>
        </div>
        <div class="metric">
          <span>临近到期</span>
          <strong>—</strong>
          <small>等待提醒模块</small>
        </div>
      </section>

      <section class="content-grid">
        <article class="panel">
          <div class="panel-heading">
            <h2>近期续费</h2>
            <span>Coming soon</span>
          </div>
          <div class="empty-state">
            <div class="empty-illustration">↗</div>
            <h3>你的账本即将在这里呈现</h3>
            <p>后续版本可添加订阅、设置到期时间并追踪实际支出。</p>
          </div>
        </article>

        <article class="panel">
          <div class="panel-heading">
            <h2>系统状态</h2>
            <span class="status" :class="apiStatus">{{ apiLabel[apiStatus] }}</span>
          </div>
          <div class="system-detail">
            <div><span>后端 API</span><code>/api/v1/health</code></div>
            <div><span>项目阶段</span><strong>基础工程</strong></div>
            <div><span>业务存储</span><strong>尚未接入</strong></div>
          </div>
          <button class="outline-button" type="button" @click="checkApi">重新检查连接</button>
        </article>
      </section>
    </main>
  </div>
</template>
