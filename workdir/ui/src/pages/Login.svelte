<script>
  import { api } from '../lib/api.js'
  import { token, user } from '../lib/store.js'

  let username = '', password = '', error = '', loading = false, isSetup = false

  async function checkSetup() {
    try {
      const r = await api.setup('__probe__', '__probe__').catch(e => e)
      isSetup = r?.status !== 409
    } catch {}
  }
  checkSetup()

  async function submit() {
    error = ''
    loading = true
    try {
      let data
      if (isSetup) {
        data = await api.setup(username, password)
        data = await api.login(username, password)
      } else {
        data = await api.login(username, password)
      }
      token.set(data.data.token)
      user.set(data.data.user)
    } catch (e) {
      error = e.json?.error?.message || e.message || 'Authentication failed'
    } finally {
      loading = false
    }
  }
</script>

<!-- Müller-Brockmann: full-bleed, single column centred, large numeral as hero -->
<div class="login-spread">
  <!-- Left col: brand + large numeral FRI concept -->
  <div class="login-left">
    <div class="brand-mark">
      <svg width="36" height="36" viewBox="0 0 36 36" fill="none">
        <path d="M18 3L33 30H3L18 3Z" stroke="currentColor" stroke-width="2" fill="none"/>
        <path d="M18 12L26 26H10L18 12Z" fill="currentColor" opacity="0.4"/>
      </svg>
      <span>Ruptura</span>
    </div>
    <div class="hero-numeral" aria-hidden="true">0.0</div>
    <p class="hero-label">Fused Rupture Index</p>
    <p class="hero-sub">
      Predictive AIOps for Kubernetes.<br>
      Detects divergence hours before failure.
    </p>
    <ul class="feature-list">
      <li><span class="dot healthy"></span>5-model adaptive ensemble</li>
      <li><span class="dot accent"></span>10 composite KPI signals</li>
      <li><span class="dot violet"></span>HealthScore forecast +30min</li>
    </ul>
  </div>

  <!-- Right col: login form -->
  <div class="login-right">
    <div class="form-card">
      <h1>{isSetup ? 'Create admin account' : 'Sign in'}</h1>
      {#if isSetup}
        <p class="setup-hint">No admin account found. Create the first one.</p>
      {/if}
      <form on:submit|preventDefault={submit} novalidate>
        <div class="field">
          <label for="username">Username</label>
          <input
            id="username"
            type="text"
            bind:value={username}
            autocomplete="username"
            placeholder="admin"
            required
            spellcheck="false"
          />
        </div>
        <div class="field">
          <label for="password">Password</label>
          <input
            id="password"
            type="password"
            bind:value={password}
            autocomplete="current-password"
            placeholder="••••••••"
            required
          />
        </div>
        {#if error}
          <p class="error" role="alert">{error}</p>
        {/if}
        <button type="submit" class="submit-btn" disabled={loading}>
          {loading ? 'Verifying…' : isSetup ? 'Create account' : 'Sign in'}
        </button>
      </form>
      <p class="version-note">Community edition · v7</p>
    </div>
  </div>
</div>

<style>
  /* ── Layout: full-bleed split grid ── */
  .login-spread {
    display: grid;
    grid-template-columns: 1fr 1fr;
    min-height: 100vh;
    background: var(--bg, #0F172A);
  }

  /* ── Left: brand panel ── */
  .login-left {
    display: flex;
    flex-direction: column;
    justify-content: center;
    padding: 64px 48px;
    background: var(--surface, #1E293B);
    border-right: 1px solid var(--border, rgba(148,163,184,0.10));
  }

  .brand-mark {
    display: flex;
    align-items: center;
    gap: 10px;
    font-size: 18px;
    font-weight: 700;
    color: var(--accent, #38BDF8);
    margin-bottom: 64px;
  }
  .brand-mark svg { color: var(--accent, #38BDF8); }

  /* Swiss big numeral — the signature move */
  .hero-numeral {
    font-family: "DM Mono", "Fira Code", monospace;
    font-size: clamp(72px, 10vw, 120px);
    font-weight: 500;
    line-height: 1;
    color: var(--text, #E2E8F0);
    letter-spacing: -0.02em;
    font-variant-numeric: tabular-nums;
    margin-bottom: 8px;
    /* Optical ink alignment: nudge left by side-bearing */
    margin-left: -4px;
  }

  .hero-label {
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--accent, #38BDF8);
    margin-bottom: 24px;
  }

  .hero-sub {
    font-size: 14px;
    line-height: 24px;
    color: var(--text-2, #94A3B8);
    margin-bottom: 40px;
    max-width: 320px;
  }

  .feature-list {
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .feature-list li {
    display: flex;
    align-items: center;
    gap: 10px;
    font-size: 13px;
    color: var(--text-2, #94A3B8);
  }
  .dot {
    width: 6px; height: 6px;
    border-radius: 50%;
    flex-shrink: 0;
  }
  .dot.healthy { background: var(--green, #22C55E); }
  .dot.accent  { background: var(--accent, #38BDF8); }
  .dot.violet  { background: var(--violet, #8B5CF6); }

  /* ── Right: form panel ── */
  .login-right {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 48px 32px;
    background: var(--bg, #0F172A);
  }

  .form-card {
    width: 100%;
    max-width: 360px;
  }

  h1 {
    font-size: 22px;
    font-weight: 700;
    color: var(--text, #E2E8F0);
    margin-bottom: 8px;
    letter-spacing: -0.01em;
  }

  .setup-hint {
    font-size: 12px;
    color: var(--amber, #F59E0B);
    background: var(--amber-dim, rgba(245,158,11,0.10));
    border: 1px solid rgba(245,158,11,0.20);
    border-radius: 4px;
    padding: 8px 12px;
    margin-bottom: 24px;
    line-height: 1.5;
  }

  .field {
    margin-bottom: 16px;
  }
  label {
    display: block;
    font-size: 11px;
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--text-3, #3F4D5C);
    margin-bottom: 6px;
  }
  input {
    display: block;
    width: 100%;
    background: var(--surface, #1E293B);
    border: 1px solid var(--border-2, rgba(148,163,184,0.18));
    border-radius: 4px;
    color: var(--text, #E2E8F0);
    padding: 10px 14px;
    font-size: 14px;
    font-family: inherit;
    transition: border-color 0.12s;
  }
  input:focus { outline: none; border-color: var(--accent, #38BDF8); }
  input::placeholder { color: var(--text-3, #3F4D5C); }

  .error {
    font-size: 12px;
    color: var(--red, #EF4444);
    background: var(--red-dim, rgba(239,68,68,0.10));
    border: 1px solid rgba(239,68,68,0.20);
    border-radius: 4px;
    padding: 8px 12px;
    margin-bottom: 16px;
    line-height: 1.5;
  }

  .submit-btn {
    width: 100%;
    padding: 11px;
    background: var(--accent, #38BDF8);
    color: #000;
    border: none;
    border-radius: 4px;
    font-size: 14px;
    font-weight: 700;
    cursor: pointer;
    transition: background 0.12s, opacity 0.12s;
    margin-top: 8px;
    letter-spacing: 0.01em;
  }
  .submit-btn:hover:not(:disabled) { background: #67CFFA; }
  .submit-btn:disabled { opacity: 0.5; cursor: not-allowed; }

  .version-note {
    margin-top: 24px;
    font-size: 11px;
    color: var(--text-3, #3F4D5C);
    text-align: center;
  }

  /* ── Responsive ── */
  @media (max-width: 640px) {
    .login-spread { grid-template-columns: 1fr; }
    .login-left { display: none; }
    .login-right { padding: 32px 24px; min-height: 100vh; }
  }
</style>
