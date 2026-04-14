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
    } catch (e) {
      createErr = e.json?.error?.message || e.message
    } finally {
      creating = false
    }
  }

  async function deleteUser(u) {
    await api.userDelete(u.username).catch(() => {})
    loadUsers()
  }

  async function logout() {
    await api.logout().catch(() => {})
    token.set('')
    user.set(null)
  }

  onMount(loadUsers)
</script>

<div class="page">
  <div class="header">
    <h1>Settings</h1>
    <button class="btn-ghost" on:click={logout}>Sign out</button>
  </div>

  <div class="tabs">
    <button class:active={activeTab==='users'} on:click={() => activeTab='users'}>Users</button>
  </div>

  {#if activeTab === 'users'}
    <div class="card">
      <h2>Create User</h2>
      <div class="form-row">
        <input bind:value={newUser.username} placeholder="Username" class="inp"/>
        <input bind:value={newUser.password} type="password" placeholder="Password (min 8)" class="inp"/>
        <select bind:value={newUser.role} class="inp">
          <option value="viewer">viewer</option>
          <option value="operator">operator</option>
          <option value="admin">admin</option>
        </select>
        <button class="btn-primary" on:click={createUser} disabled={!newUser.username || !newUser.password || creating}>
          Create
        </button>
      </div>
      {#if createErr}<p class="err">{createErr}</p>{/if}
    </div>

    <div class="card" style="margin-top:0.75rem">
      <h2>Users <span class="badge">{users.length}</span></h2>
      {#if loading}
        <p class="muted">Loading…</p>
      {:else if users.length === 0}
        <p class="muted">No users</p>
      {:else}
        <table>
          <thead><tr><th>Username</th><th>Role</th><th>ID</th><th></th></tr></thead>
          <tbody>
            {#each users as u}
              <tr>
                <td>{u.username}</td>
                <td><span class="role-badge role-{u.role}">{u.role}</span></td>
                <td class="uid">{u.id}</td>
                <td>
                  <button class="btn-sm danger" on:click={() => deleteUser(u)}>Delete</button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </div>
  {/if}
</div>

<style>
  .page { padding: 0; }
  .header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 1rem; }
  h1 { margin: 0; font-size: 1.2rem; color: #e2e8f0; }
  h2 { margin: 0 0 0.75rem; font-size: 0.85rem; color: #64748b; text-transform: uppercase; letter-spacing: 0.05em; }
  .card { background: #1e293b; border: 1px solid #334155; border-radius: 8px; padding: 1rem; }
  .tabs { display: flex; gap: 0.25rem; margin-bottom: 0.75rem; }
  .tabs button { background: transparent; border: 1px solid #334155; color: #94a3b8; padding: 0.3rem 0.75rem; border-radius: 5px; cursor: pointer; font-size: 0.85rem; }
  .tabs button.active { background: #0284c7; border-color: #0284c7; color: #fff; }
  .form-row { display: flex; gap: 0.5rem; flex-wrap: wrap; align-items: center; }
  .inp { background: #0f172a; border: 1px solid #334155; color: #e2e8f0; padding: 0.4rem 0.6rem; border-radius: 5px; font-size: 0.85rem; }
  .btn-primary { background: #0284c7; border: none; color: #fff; padding: 0.4rem 0.75rem; border-radius: 5px; cursor: pointer; font-size: 0.85rem; }
  .btn-ghost { background: transparent; border: 1px solid #334155; color: #94a3b8; padding: 0.35rem 0.75rem; border-radius: 5px; cursor: pointer; font-size: 0.85rem; }
  .btn-sm { background: #334155; border: none; color: #e2e8f0; padding: 2px 8px; border-radius: 4px; cursor: pointer; font-size: 0.75rem; }
  .btn-sm.danger { background: #b91c1c; color: #fff; }
  table { width: 100%; border-collapse: collapse; font-size: 0.85rem; }
  th { text-align: left; padding: 0.4rem 0.5rem; color: #64748b; font-weight: 500; }
  td { padding: 0.4rem 0.5rem; border-top: 1px solid #0f172a; color: #cbd5e1; }
  .uid { font-family: monospace; font-size: 0.75rem; color: #475569; }
  .badge { background: #334155; border-radius: 10px; padding: 0 6px; font-size: 0.75rem; }
  .role-badge { font-size: 0.7rem; padding: 1px 6px; border-radius: 10px; background: #334155; color: #94a3b8; }
  .role-badge.role-admin { background: #1e3a5f; color: #60a5fa; }
  .role-badge.role-operator { background: #1a2f1a; color: #4ade80; }
  .muted { color: #64748b; font-size: 0.85rem; }
  .err { color: #f87171; font-size: 0.8rem; }
</style>
