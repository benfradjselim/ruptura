// Built-in Security Dashboard template — client-side import + apply
export const SECURITY_TEMPLATE = {
  name: 'Security Overview',
  description: 'Cert expiry, auth failures, anomaly scores, top-N IPs',
  refresh_seconds: 60,
  widgets: [
    // Row 1: auth & anomaly KPIs
    {
      type: 'kpi',
      title: 'System Stress',
      kpi: 'stress',
      w: 1, h: 1,
    },
    {
      type: 'kpi',
      title: 'Contagion Index',
      kpi: 'contagion',
      w: 1, h: 1,
    },
    {
      type: 'kpi',
      title: 'Resilience',
      kpi: 'resilience',
      w: 1, h: 1,
    },
    {
      type: 'kpi',
      title: 'Entropy',
      kpi: 'entropy',
      w: 1, h: 1,
    },

    // Row 2: timeseries with thresholds
    {
      type: 'timeseries',
      title: 'Error Rate',
      metric: 'error_rate',
      threshold: 0.05,
      w: 2, h: 1,
    },
    {
      type: 'timeseries',
      title: 'Contagion over Time',
      kpi: 'contagion',
      threshold: 0.3,
      w: 2, h: 1,
    },

    // Row 3: prediction + top-N
    {
      type: 'prediction',
      title: 'Stress Forecast',
      kpi: 'stress',
      horizon: 60,
      w: 2, h: 1,
    },
    {
      type: 'topn',
      title: 'Top Source IPs',
      label_key: 'src_ip',
      limit: 10,
      w: 2, h: 1,
    },

    // Row 4: gauges
    {
      type: 'gauge',
      title: 'System Health',
      kpi: 'health_score',
      max: 1,
      w: 1, h: 1,
    },
    {
      type: 'gauge',
      title: 'Pressure',
      kpi: 'pressure',
      max: 1,
      w: 1, h: 1,
    },

    // Row 5: active alerts
    {
      type: 'alerts',
      title: 'Critical Alerts',
      severity: 'critical',
      w: 3, h: 1,
    },
    {
      type: 'alerts',
      title: 'Emergency Alerts',
      severity: 'emergency',
      w: 3, h: 1,
    },
  ],
}
