# UI Components

OHE features a Svelte-based embedded UI that requires no CDN or external dependencies, ensuring full functionality in air-gapped environments.

## Dashboard Widgets
Dashboards are built on a responsive 4-column grid. Available widget types include:
- **Timeseries:** Line chart for metric ranges.
- **Gauge (300° Arc):** Visual representation of holistic KPIs.
- **Stat Card:** Single numeric value display.
- **KPI Card:** Summary view of specific KPIs.
- **Prediction Chart:** Shows historical data + ML forecast extrapolation.
- **Alert Feed:** List view of active/historical alerts.
- **Top-N Table:** Ranking of hosts/services by specific metric.
- **Query Widget:** Direct PromQL results table.
- **SLO Status:** Current compliance percentage and error budget burn rate.

## Interaction
- **Edit Mode:** Enables widget resizing (◀ ▶ ▲ ▼ controls) and movement via drag-and-drop.
- **Tabbed Views:** Dashboards can be grouped into tabs for better organization.
- **Auto-Refresh:** Configurable refresh intervals from 5 seconds to 5 minutes.
