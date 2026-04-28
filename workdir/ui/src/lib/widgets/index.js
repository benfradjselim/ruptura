// Widget registry — maps type string to component and metadata
import TimeseriesWidget  from './TimeseriesWidget.svelte'
import GaugeWidget       from './GaugeWidget.svelte'
import KpiWidget         from './KpiWidget.svelte'
import StatWidget        from './StatWidget.svelte'
import PredictionWidget  from './PredictionWidget.svelte'
import AlertsWidget      from './AlertsWidget.svelte'
import TopNWidget        from './TopNWidget.svelte'
import QueryWidget       from './QueryWidget.svelte'
import SLOWidget         from './SLOWidget.svelte'

// SVG paths (Lucide-style, viewBox 0 0 24 24, stroke-based)
export const WIDGET_SVG = {
  timeseries: 'M3 3v18h18M8 17l4-8 4 8M16 9l2-2 2 2',
  gauge:      'M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0zM12 9v4M12 17h.01',
  kpi:        'M22 12h-4l-3 9L9 3l-3 9H2',
  stat:       'M13 2L3 14h9l-1 8 10-12h-9l1-8z',
  prediction: 'M23 6l-9.5 9.5-5-5L1 18M17 6h6v6',
  alerts:     'M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0zM12 9v4M12 17h.01',
  topn:       'M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01',
  query:      'M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0zM10 7v3m0 0v3m0-3h3m-3 0H7',
  slo:        'M9 12l2 2 4-4M7.835 4.697a3.42 3.42 0 001.946-.806 3.42 3.42 0 014.438 0 3.42 3.42 0 001.946.806 3.42 3.42 0 013.138 3.138 3.42 3.42 0 00.806 1.946 3.42 3.42 0 010 4.438 3.42 3.42 0 00-.806 1.946 3.42 3.42 0 01-3.138 3.138 3.42 3.42 0 00-1.946.806 3.42 3.42 0 01-4.438 0 3.42 3.42 0 00-1.946-.806 3.42 3.42 0 01-3.138-3.138 3.42 3.42 0 00-.806-1.946 3.42 3.42 0 010-4.438 3.42 3.42 0 00.806-1.946 3.42 3.42 0 013.138-3.138z',
}

export const WIDGET_TYPES = {
  timeseries: {
    component: TimeseriesWidget,
    label: 'Time Series',
    icon: WIDGET_SVG.timeseries,
    defaults: { title: 'Metric', metric: '', host: '' },
    fields: ['title', 'metric', 'kpi', 'host', 'threshold'],
  },
  gauge: {
    component: GaugeWidget,
    label: 'Gauge',
    icon: WIDGET_SVG.gauge,
    defaults: { title: 'KPI Gauge', kpi: '', host: '' },
    fields: ['title', 'kpi', 'metric', 'host', 'max'],
  },
  kpi: {
    component: KpiWidget,
    label: 'KPI Value',
    icon: WIDGET_SVG.kpi,
    defaults: { title: 'KPI', kpi: '', host: '' },
    fields: ['title', 'kpi', 'metric', 'host'],
  },
  stat: {
    component: StatWidget,
    label: 'Stat',
    icon: WIDGET_SVG.stat,
    defaults: { title: 'Stat', metric: '', host: '' },
    fields: ['title', 'metric', 'kpi', 'host'],
  },
  prediction: {
    component: PredictionWidget,
    label: 'Prediction',
    icon: WIDGET_SVG.prediction,
    defaults: { title: 'Forecast', metric: '', host: '', horizon: 60 },
    fields: ['title', 'metric', 'kpi', 'host', 'horizon'],
  },
  alerts: {
    component: AlertsWidget,
    label: 'Active Alerts',
    icon: WIDGET_SVG.alerts,
    defaults: { title: 'Alerts', severity: '' },
    fields: ['title', 'severity'],
  },
  topn: {
    component: TopNWidget,
    label: 'Top-N',
    icon: WIDGET_SVG.topn,
    defaults: { title: 'Top Sources', label_key: 'src_ip', limit: 10 },
    fields: ['title', 'label_key', 'limit'],
  },
  query: {
    component: QueryWidget,
    label: 'PromQL Query',
    icon: WIDGET_SVG.query,
    defaults: { title: 'PromQL', options: { datasource_id: '', query: '', range: '1h', step: 15 } },
    fields: ['title', 'datasource_id', 'query', 'range', 'step'],
  },
  slo: {
    component: SLOWidget,
    label: 'SLO Status',
    icon: WIDGET_SVG.slo,
    defaults: { title: 'SLO', options: { slo_id: '' } },
    fields: ['title', 'slo_id'],
  },
}

export function getWidget(type) {
  return WIDGET_TYPES[type] ?? WIDGET_TYPES.stat
}
