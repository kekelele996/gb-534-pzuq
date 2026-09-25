<script setup lang="ts">
import * as echarts from 'echarts'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { AlignedPoint } from '../../types/deviation-analysis'
import type { SensorPoint } from '../../types/sensor-series'

const props = withDefaults(defineProps<{ points?: SensorPoint[]; aligned?: AlignedPoint[]; height?: number }>(), {
  points: () => [], aligned: () => [], height: 340,
})
const chartEl = ref<HTMLDivElement>()
const empty = computed(() => !props.points.length && !props.aligned.length)
let chart: echarts.ECharts | undefined
let observer: ResizeObserver | undefined
const palette = ['#25714f', '#237a78', '#35689a', '#69756f', '#604f8b', '#2f8563']

function option(): echarts.EChartsOption {
  const series: echarts.SeriesOption[] = []
  if (props.points.length) {
    const start = new Date(props.points[0].timestamp).getTime()
    const channels = [...new Set(props.points.flatMap((point) => Object.keys(point.values)))].sort()
    channels.forEach((channel, index) => series.push({
      name: channel, type: 'line', showSymbol: false, connectNulls: false,
      data: props.points.map((point) => [
        (new Date(point.timestamp).getTime() - start) / 3_600_000, point.values[channel],
      ]),
      lineStyle: { width: 2, color: palette[index % palette.length] },
      itemStyle: { color: palette[index % palette.length] },
    }))
  } else {
    const channels = [...new Set(props.aligned.map((point) => point.channel))].sort()
    channels.forEach((channel, index) => {
      const rows = props.aligned.filter((point) => point.channel === channel)
      series.push({
        name: `${channel} 实测`, type: 'line', showSymbol: false,
        data: rows.map((point) => [point.actual_elapsed_h, point.actual_value]),
        lineStyle: { width: 2, color: palette[index % palette.length] },
      })
      series.push({
        name: `${channel} 参考`, type: 'line', showSymbol: false,
        data: rows.map((point) => [point.reference_elapsed_h, point.reference_value]),
        lineStyle: { width: 1.5, type: 'dashed', color: palette[index % palette.length], opacity: .65 },
      })
    })
  }
  return {
    animationDuration: 240,
    color: palette,
    grid: { left: 48, right: 24, top: 54, bottom: 42, containLabel: false },
    legend: { top: 6, left: 6, type: 'scroll', textStyle: { color: '#52615a', fontSize: 11 } },
    tooltip: { trigger: 'axis', valueFormatter: (value) => typeof value === 'number' ? value.toFixed(3) : String(value) },
    xAxis: { type: 'value', name: 'elapsed h', nameLocation: 'middle', nameGap: 28, axisLine: { lineStyle: { color: '#9ca9a2' } } },
    yAxis: { type: 'value', scale: true, splitLine: { lineStyle: { color: '#e0e7e2' } } },
    series,
  }
}
async function render() {
  await nextTick()
  if (!chartEl.value || empty.value) return
  chart ??= echarts.init(chartEl.value)
  chart.setOption(option(), true)
}
watch(() => [props.points, props.aligned], render, { deep: true })
onMounted(() => {
  render()
  observer = new ResizeObserver(() => chart?.resize())
  if (chartEl.value) observer.observe(chartEl.value)
})
onBeforeUnmount(() => { observer?.disconnect(); chart?.dispose() })
</script>

<template>
  <div class="chart-frame" :style="{ height: `${height}px` }">
    <div v-if="empty" class="chart-empty">暂无可绘制的动力学证据</div>
    <div v-else ref="chartEl" class="chart-canvas" />
  </div>
</template>
