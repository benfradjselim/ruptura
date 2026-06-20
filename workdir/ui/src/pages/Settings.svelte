<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { token, user } from '../lib/store.js'

  let users = [], activeTab = 'users', loading = false
  let newUser = { username: '', password: '', role: 'viewer' }, creating = false, createErr = ''

  async function loadUsers() {
    loading = true
    const r = await api.users().catch(() => ({ data: [] }))
    users = r.data || []
    loading = false
  }

  async function createUser() {
    creating = true; createErr = ''
    try {
      await api.userCreate(newUser)
      newUser = { username: '', password: '', role: 'viewer' }
      loadUsers()
    } catch(e) { createErr = e.message }
    finally { creating = false }
  }

  async function deleteUser(id) {
    if (!confirm('Delete this user?')) return
    await api.userDelete(id).catch(() => {})
    loadUsers()
  }

  onMount(loadUsers)
</script>

<div class="page-wrap">
  <div class="band page-header">
    <h1 class="page-title" style="grid-column:1/7">Settings</h1>
  </div>

  <div class="band tabs-band">
    <div class="tabs" style="grid-column:1/-1">
      {#each ['users','api'] as t}
        <button class="tab" class:active={activeTab===t} on:click={() => activeTab=t}>
          {t === 'users' ? 'Users' : 'API Key'}
        </button>
      {/each}
    </div>
  </div>

  {#if activeTab === 'users'}
    <div class="band">
      <!-- Create user form -->
      <div class="section-card" style="grid-column:1/6">
        <div class="section-label">Create user</div>
        <div class="form-row">
          <input class="input" placeholder="Username" bind:value={newUser.username} />
        </div>
        <div class="form-row">
          <input class="input" type="password" placeholder="Password" bind:value={newUser.password} />
        </div>
        <div class="form-row">
          <select class="input" bind:value={newUser.role}>
            <option value="viewer">Viewer</option>
            <option value="operator">Operator</option>
            <option value="admin">Admin</option>
          </select>
        </div>
        {#if createErr}<p class="err">{createErr}</p>{/if}
        <button class="btn btn-primary" disabled={creating} on:click={createUser}>
          {creating ? 'Creating…' : 'Create user'}
        </button>
      </div>

      <!-- Users list -->
      <div class="section-card" style="grid-column:6/13">
        <div class="section-label">Users</div>
        {#if loading}
          <div class="loading"><div class="spinner"></div></div>
        {:else if users.length === 0}
          <p class="empty-msg">No users yet</p>
        {:else}
          <table class="data-table">
            <thead><tr><th>Username</th><th>Role</th><th></th></tr></thead>
            <tbody>
              {#each users as u}
                <tr>
                  <td>{u.username}</td>
                  <td><span class="role-badge">{u.role}</span></td>
                  <td>
                    {#if u.id !== $user?.id}
                      <button class="btn btn-danger btn-sm" on:click={() => deleteUser(u.id)}>Delete</button>
                    {/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </div>
    </div>

  {:else}
    <div class="band">
      <div class="section-card" style="grid-column:1/8">
        <div class="section-label">Current API key</div>
        <div class="key-display">
          <span class="mono">{$token ? $token.substring(0,8) + '…' : '—'}</span>
        </div>
        <p class="hint-text">Keep your API key secret. Rotate it by creating a new user and logging in again.</p>
      </div>
    </div>
  {/if}
</div>

<style>
  .page-wrap { padding:32px 24px; overflow-y:auto; height:100%; display:grid; grid-template-columns:repeat(12,1fr); column-gap:20px; row-gap:0; align-content:start; }
  .band { grid-column:1/-1; display:grid; grid-template-columns:subgrid; column-gap:20px; margin-bottom:24px; align-items:start; }
  @supports not (grid-template-columns:subgrid) { .band { grid-template-columns:repeat(12,1fr); } }
  .page-title { font-size:22px; font-weight:700; letter-spacing:-0.02em; color:var(--text); margin-bottom:0; }
  .page-header { align-items:center; }

  .tabs { display:flex; gap:4px; border-bottom:1px solid var(--border); padding-bottom:0; }
  .tab { padding:8px 16px; background:none; border:none; color:var(--text-3); font-size:13px; font-weight:500; cursor:pointer; border-bottom:2px solid transparent; margin-bottom:-1px; font-family:inherit; transition:color .12s; }
  .tab:hover { color:var(--text-2); }
  .tab.active { color:var(--accent); border-bottom-color:var(--accent); }

  .section-card { background:var(--surface); border:1px solid var(--border); border-radius:4px; padding:20px; }
  .section-label { font-size:10px; font-weight:700; letter-spacing:0.10em; text-transform:uppercase; color:var(--text-3); margin-bottom:16px; }
  .form-row { margin-bottom:10px; }
  .input { width:100%; background:var(--surface-2); border:1px solid var(--border-2); border-radius:4px; color:var(--text); padding:8px 12px; font-size:13px; font-family:inherit; }
  .input:focus { outline:none; border-color:var(--accent); }
  .err { color:var(--red); font-size:12px; margin-bottom:8px; }
  .empty-msg { color:var(--text-3); font-size:13px; }

  .btn { display:inline-flex; align-items:center; gap:6px; padding:8px 16px; border-radius:4px; border:1px solid transparent; font-size:12px; font-weight:600; cursor:pointer; font-family:inherit; margin-top:8px; }
  .btn-primary { background:var(--accent); color:#000; }
  .btn-primary:hover:not(:disabled) { opacity:.9; }
  .btn-primary:disabled { opacity:.5; cursor:not-allowed; }
  .btn-ghost { background:transparent; color:var(--text-2); border-color:var(--border-2); }
  .btn-danger { background:var(--red-dim); color:var(--red); border-color:rgba(244,63,94,.2); }
  .btn-sm { padding:3px 10px; font-size:11px; margin-top:0; }

  .data-table { width:100%; border-collapse:collapse; font-size:12px; }
  th { text-align:left; padding:6px 8px; font-size:10px; font-weight:700; letter-spacing:0.08em; text-transform:uppercase; color:var(--text-3); border-bottom:1px solid var(--border); }
  td { padding:8px; border-bottom:1px solid var(--border); color:var(--text-2); vertical-align:middle; }
  .role-badge { font-size:9px; font-weight:700; padding:2px 6px; border-radius:3px; text-transform:uppercase; letter-spacing:.06em; background:var(--surface-3); color:var(--text-3); }

  .key-display { background:var(--surface-2); border:1px solid var(--border); border-radius:4px; padding:12px 16px; margin-bottom:12px; }
  .mono { font-family:"DM Mono",monospace; font-size:13px; color:var(--text-2); font-variant-numeric:tabular-nums; }
  .hint-text { font-size:12px; color:var(--text-3); line-height:1.6; }
  .loading { display:flex; justify-content:center; padding:32px; }
  .spinner { width:16px; height:16px; border-radius:50%; border:2px solid var(--border-2); border-top-color:var(--accent); animation:spin .7s linear infinite; }
  @keyframes spin { to{transform:rotate(360deg)} }
</style>
