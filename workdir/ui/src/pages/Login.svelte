<script>
  import { api } from '../lib/api.js'
  import Mark from '../lib/components/Mark.svelte'
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
      token.set(data.token)
      user.set(data.user)
    } catch (e) {
      error = e.json?.error?.message || e.message || 'Authentication failed'
    } finally {
      loading = false
    }
  }
</script>

<div class="login-spread">
  <!-- Left col: brand + large numeral FRI concept -->
  <div class="login-left">
    <Mark size={84} />
    <h2 class="wordmark">RUPTURA</h2>
    <p class="promise">Know what will fail, when, and why.</p>
    <svg class="fault" viewBox="0 0 600 120" preserveAspectRatio="none" aria-hidden="true">
      <path d="M0 96 L238 96 L262 62 L246 46 L302 46 L322 26 L600 26" />
      <circle cx="262" cy="62" r="5" /><circle cx="246" cy="46" r="5" />
    </svg>
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
      <p class="version-note">Community edition</p>
    </div>
  </div>
</div>

<style>
  .login-spread { display: grid; grid-template-columns: 1.3fr 1fr; min-height: 100vh; }
  .login-left { position: relative; display: flex; flex-direction: column; justify-content: center; padding: 64px; overflow: hidden; }
  .wordmark { margin-top: 28px; font-size: 54px; line-height: 60px; letter-spacing: 0.2em; font-weight: 700; color: var(--text); }
  .promise { margin-top: 12px; font-size: 20px; line-height: 28px; color: var(--text-2); }
  .fault { position: absolute; left: 0; right: 0; bottom: 56px; width: 100%; height: 120px; }
  .fault path { fill: none; stroke: var(--accent); stroke-width: 2; vector-effect: non-scaling-stroke; }
  .fault circle { fill: var(--red); }
  .login-right { display: flex; align-items: center; justify-content: center; padding: 48px; background: var(--surface); border-left: 1px solid var(--border-2); }
  .form-card { width: 100%; max-width: 340px; }
  h1 { font-size: 22px; line-height: 28px; text-transform: uppercase; margin-bottom: 24px; }
  .setup-hint { color: var(--text-2); margin-bottom: 16px; }
  .field { display: flex; flex-direction: column; gap: 6px; margin-bottom: 16px; }
  label { font: 500 11px var(--font-mono); letter-spacing: 0.08em; text-transform: uppercase; color: var(--text-2); }
  input { padding: 10px 12px; background: var(--bg); border: 1px solid var(--border-2); color: var(--text); font-size: 15px; }
  input:focus { outline: none; border-color: var(--accent); }
  .error { color: var(--red); margin-bottom: 12px; }
  .submit-btn {
    width: 100%; padding: 12px; border: 0; background: var(--accent); color: #04080F;
    font: 700 14px var(--font-ui); letter-spacing: 0.1em; text-transform: uppercase; cursor: pointer;
    clip-path: polygon(0 0, calc(100% - 10px) 0, 100% 10px, 100% 100%, 10px 100%, 0 calc(100% - 10px));
  }
  .submit-btn:disabled { opacity: 0.6; cursor: wait; }
  .version-note { margin-top: 20px; font: 400 12px var(--font-mono); color: var(--text-3); }
  @media (max-width: 800px) {
    .login-spread { grid-template-columns: 1fr; }
    .login-left { padding: 32px; min-height: 280px; }
    .wordmark { font-size: 34px; line-height: 40px; }
  }
</style>

