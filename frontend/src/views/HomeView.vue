<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { AppService } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { RecentRepo } from '../../bindings/github.com/brogli/nebegit/internal/settings'

const recentRepos = ref<RecentRepo[]>([])

onMounted(async () => {
  recentRepos.value = (await AppService.RecentRepos()) ?? []
})

function pickAndOpen(): void {
  void AppService.PickAndOpen()
}

function openRecent(path: string): void {
  void AppService.OpenRecent(path)
}
</script>

<template>
  <main>
    <h1>nebegit</h1>
    <button type="button" @click="pickAndOpen">Open folder…</button>
    <ul>
      <li v-for="repo in recentRepos" :key="repo.path">
        <button type="button" @click="openRecent(repo.path)">{{ repo.path }}</button>
      </li>
    </ul>
  </main>
</template>
