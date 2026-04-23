# Alerting System

OHE provides a robust, KPI-aware alerting system integrated directly into the core platform. It eliminates the need for external alert managers by handling rule definition, evaluation, and notification delivery internally.

## Rule Engine
The alert rule engine evaluates system metrics and holistic KPIs continuously. Rules are defined with the following parameters:
- **Name:** Unique identifier for the rule.
- **Metric/KPI:** The source signal being evaluated (e.g., `cpu_usage`, `stress`, `fatigue`).
- **Condition:** Comparison logic (e.g., `> 80`, `< 0.3`).
- **Severity:** `info`, `warning`, `critical`, or `emergency`.

## Lifecycle Management
Alerts follow a strict lifecycle managed through the API:
1. **Active:** Rule condition met; alert created.
2. **Acknowledged:** An operator has accepted responsibility (`POST /api/v1/alerts/{id}/acknowledge`).
3. **Silenced:** Alert suppressed for a defined duration (`POST /api/v1/alerts/{id}/silence`).
4. **Resolved:** Condition no longer met; alert automatically closed.

## Notification Delivery
Notifications are routed to configured channels based on severity filtering:
- **Supported Channels:** Slack, PagerDuty, Webhook.
- **Severity Filtering:** Routes alerts only of specific severities to relevant channels (e.g., send only `critical`/`emergency` to PagerDuty).
