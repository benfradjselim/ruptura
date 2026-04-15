// Widget registry — maps type string to component and metadata
import TimeseriesWidget  from './TimeseriesWidget.svelte'
import GaugeWidget       from './GaugeWidget.svelte'
import KpiWidget         from './KpiWidget.svelte'
import StatWidget        from './StatWidget.svelte'
import PredictionWidget  from './PredictionWidget.svelte'
import AlertsWidget      from './AlertsWidget.svelte'
import TopNWidget        from './TopNWidget.svelte'

export const WIDGET_TYPES = {
  timeseries: {
    component: TimeseriesWidget,
    label: 'Time Series',
    icon: '📈',
    defaults: { title: 'Metric', metric: '', host: '' },
    fields: ['title', 'metric', 'kpi', 'host', 'threshold'],
  },
  gauge: {
    component: GaugeWidget,
    label: 'Gauge',
    icon: '◎',
    defaults: { title: 'KPI Gauge', kpi: '', host: '' },
    fields: ['title', 'kpi', 'metric', 'host', 'max'],
  },
  kpi: {
    component: KpiWidget,
    label: 'KPI Value',
    icon: '⬡',
    defaults: { title: 'KPI', kpi: '', host: '' },
    fields: ['title', 'kpi', 'metric', 'host'],
  },
  stat: {
    component: StatWidget,
    label: 'Stat',
    icon: '◻',
    defaults: { title: 'Stat', metric: '', host: '' },
    fields: ['title', 'metric', 'kpi', 'host'],
  },
  prediction: {
    component: PredictionWidget,
    label: 'Prediction',
    icon: '🔮',
    defaults: { title: 'Forecast', metric: '', host: '', horizon: 60 },
    fields: ['title', 'metric', 'kpi', 'host', 'horizon'],
  },
  alerts: {
    component: AlertsWidget,
    label: 'Active Alerts',
    icon: '⚡',
    defaults: { title: 'Alerts', severity: '' },
    fields: ['title', 'severity'],
  },
  topn: {
    component: TopNWidget,
    label: 'Top-N',
    icon: '🔝',
    defaults: { title: 'Top Sources', label_key: 'src_ip', limit: 10 },
    fields: ['title', 'label_key', 'limit'],
  },
}

export function getWidget(type) {
  return WIDGET_TYPES[type] ?? WIDGET_TYPES.stat
}
