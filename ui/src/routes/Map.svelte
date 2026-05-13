<script lang="ts">
  // Topology map — S2-1 (GAP-V7-01). Canvas rendered with Cytoscape.js.
  // This is a placeholder; the full implementation ships in S2-1.
</script>

<div class="placeholder">
  <div class="icon">⎋</div>
  <div class="title">Topology Map</div>
  <p>
    Service dependency graph built from OTLP trace data.<br>
    Nodes are coloured by HealthScore; edges show call rate and p99 latency.<br>
    <strong>Ships in S2-1 (GAP-V7-01).</strong>
  </p>
  <div class="badge">Coming in v7.0</div>
</div>

<style>
  .placeholder {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 16px;
    min-height: 60vh;
    text-align: center;
    color: var(--muted);
  }

  .icon {
    font-size: 56px;
    opacity: 0.3;
  }

  .title {
    font-size: 22px;
    font-weight: 700;
    color: var(--text);
  }

  p {
    max-width: 400px;
    line-height: 1.8;
    font-size: 13px;
  }

  .badge {
    background: rgba(88, 166, 255, 0.12);
    color: var(--blue);
    border: 1px solid var(--blue);
    border-radius: 20px;
    padding: 4px 14px;
    font-size: 12px;
    font-weight: 600;
  }
</style>
