<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type {
  PlaygroundModelRow,
  PlaygroundRouterHealth,
  PlaygroundServiceStatus,
} from '../../api/playgroundGateway'

export type PgMetaCell = {
  text: string
  title: string
  pillClass: string
  dotClass: string
}

const props = defineProps<{
  row: PlaygroundModelRow
  layout?: 'row' | 'column' | 'inline'
  variant?: 'full' | 'trigger' | 'triggerSummary'
  callable?: boolean
  showCatalogBadge?: boolean
  registryIds?: Set<string> | null
  modelKindLabel: (k: string) => string
  serviceStatusCell: (st: PlaygroundServiceStatus) => PgMetaCell
  routerHealthCell: (h: PlaygroundRouterHealth) => PgMetaCell
  registryCell: (m: PlaygroundModelRow) => PgMetaCell
}>()

const { t } = useI18n()

const layoutClass = () => {
  if (props.layout === 'column') return 'th-pg-model-picker__opt-aside'
  if (props.layout === 'inline') return 'th-pg-v2-model-meta--inline'
  return 'th-pg-model-picker__meta-row'
}

const catalogOnlySummaryCell = computed((): PgMetaCell => ({
  text: t('playground.catalogOnlyBadge'),
  title: t('playground.catalogOnlyOptionTitle'),
  pillClass: 'border border-amber-500/35 bg-amber-500/10 text-amber-200/90',
  dotClass: 'th-models-upstream-pill-dot bg-slate-600',
}))

const summaryStatusCell = computed((): PgMetaCell | null => {
  if (props.variant !== 'triggerSummary') return null
  const row = props.row

  if (props.callable === false) return catalogOnlySummaryCell.value

  if (row.routerHealth === 'unhealthy') return props.routerHealthCell('unhealthy')

  if (props.registryIds?.has(row.id)) {
    const upstream = (row.upstream_list_status ?? '').trim()
    if (upstream === 'not_listed' || upstream === 'mixed') return props.registryCell(row)
  }

  if (row.serviceStatus === 'degraded') return props.serviceStatusCell('degraded')
  if (row.serviceStatus === 'stale') return props.serviceStatusCell('stale')

  // 「仅登记」等为常见中性态，触发器上不重复展示；完整信息保留下拉列表与 title
  if (props.registryIds?.has(row.id) && (row.upstream_list_status ?? '').trim() === 'listed') {
    return props.registryCell(row)
  }

  return null
})

const statusCells = computed(() => {
  if (props.variant === 'triggerSummary') {
    const cell = summaryStatusCell.value
    return cell ? [cell] : []
  }

  const cells: PgMetaCell[] = []
  const registry = props.registryIds?.has(props.row.id) ? props.registryCell(props.row) : null
  const router = props.row.routerHealth ? props.routerHealthCell(props.row.routerHealth) : null
  const service = props.row.serviceStatus ? props.serviceStatusCell(props.row.serviceStatus) : null

  if (props.variant === 'trigger') {
    if (registry) cells.push(registry)
    if (router) cells.push(router)
    if (service) cells.push(service)
    return cells
  }

  if (service) cells.push(service)
  if (router) cells.push(router)
  if (registry) cells.push(registry)
  return cells
})

const detailTitle = computed(() => {
  const parts: string[] = []
  if (props.callable === false) {
    parts.push(t('playground.catalogOnlyOptionTitle'))
  }
  if (props.row.serviceStatus) {
    const c = props.serviceStatusCell(props.row.serviceStatus)
    parts.push(`${c.text}${c.title ? ` — ${c.title}` : ''}`)
  }
  if (props.row.routerHealth) {
    const c = props.routerHealthCell(props.row.routerHealth)
    parts.push(`${c.text}${c.title ? ` — ${c.title}` : ''}`)
  }
  if (props.registryIds?.has(props.row.id)) {
    const c = props.registryCell(props.row)
    parts.push(`${c.text}${c.title ? ` — ${c.title}` : ''}`)
  }
  return parts.join('\n')
})

defineExpose({ detailTitle })
</script>

<template>
  <div v-if="variant !== 'triggerSummary' || statusCells.length" :class="layoutClass()">
    <span
      v-if="showCatalogBadge && callable === false && variant !== 'triggerSummary'"
      class="th-pg-model-picker__meta-pill border border-amber-500/35 bg-amber-500/10 text-amber-200/90"
    >
      {{ $t('playground.catalogOnlyBadge') }}
    </span>
    <span
      v-if="variant !== 'trigger' && variant !== 'triggerSummary'"
      v-for="lab in row.labels ?? []"
      :key="`${row.id}-k-${lab}`"
      class="th-pg-model-picker__meta-pill th-pg-model-picker__kind-pill"
      :title="$t('modelsPage.labelsHint')"
    >
      {{ modelKindLabel(lab) }}
    </span>
    <span
      v-for="(cell, idx) in statusCells"
      :key="`${row.id}-st-${idx}`"
      class="th-pg-model-picker__meta-pill"
      :class="cell.pillClass"
      :title="variant === 'triggerSummary' ? detailTitle : cell.title"
    >
      <span class="h-1.5 w-1.5 shrink-0 rounded-full ring-1 ring-black/20" :class="cell.dotClass" aria-hidden="true" />
      <span class="min-w-0 truncate">{{ cell.text }}</span>
    </span>
  </div>
</template>
