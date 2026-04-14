<script>
  import { api } from '../lib/api.js'
  import { token, user } from '../lib/store.js'

  let username = '', password = '', error = '', loading = false, isSetup = false

  async function checkSetup() {
    try {
      await api.health()
      // Try a protected endpoint; if 401 with no users, offer setup
    } catch {}
    // Try setup — if it returns 409 (already configured), show login
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
        // After setup, log in
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

<div class="login-wrap">
  <div class="card">
    <div class="logo">
      <span class="logo-text">OHE</span>
      <span class="logo-sub">Observability Holistic Engine</span>
    </div>

    {#if isSetup}
      <p class="hint">No admin account found. Create the first admin account below.</p>
    {/if}

    <form on:submit|preventDefault={submit}>
      <label>
        Username
        <input bind:value={username} type="text" autocomplete="username" required placeholder="admin"/>
      </label>
      <label>
        Password
        <input bind:value={password} type="password" autocomplete="current-password" required placeholder="••••••••"/>
      </label>
      {#if error}
        <p class="err">{error}</p>
      {/if}
      <button type="submit" disabled={loading}>
        {loading ? 'Please wait…' : isSetup ? 'Create account & sign in' : 'Sign in'}
      </button>
    </form>
  </div>
</div>

<style>
  .login-wrap {
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    background: #0f172a;
  }
  .card {
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 12px;
    padding: 2rem;
    width: 340px;
  }
  .logo { text-align: center; margin-bottom: 1.5rem; }
  .logo-text { font-size: 2rem; font-weight: 800; color: #38bdf8; display: block; }
  .logo-sub { font-size: 0.75rem; color: #64748b; }
  label { display: block; margin-bottom: 1rem; font-size: 0.85rem; color: #94a3b8; }
  input {
    display: block; width: 100%; margin-top: 4px;
    background: #0f172a; border: 1px solid #334155; border-radius: 6px;
    color: #e2e8f0; padding: 0.5rem 0.75rem; font-size: 0.9rem;
    box-sizing: border-box;
  }
  input:focus { outline: none; border-color: #38bdf8; }
  button {
    width: 100%; padding: 0.6rem; background: #0284c7; color: #fff;
    border: none; border-radius: 6px; font-size: 0.9rem; font-weight: 600;
    cursor: pointer; margin-top: 0.5rem;
  }
  button:disabled { opacity: 0.6; cursor: not-allowed; }
  .err { color: #f87171; font-size: 0.8rem; margin: 0.5rem 0; }
  .hint { font-size: 0.8rem; color: #fbbf24; background: #1c1400; padding: 0.5rem; border-radius: 4px; margin-bottom: 1rem; }
</style>
