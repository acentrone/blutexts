<script>
  import { onMount } from 'svelte';
  import { IsConfigured, SaveSetup, GetStatus, GetActivityLog, CheckDiskAccess, OpenSystemPrefs, CheckForUpdate, DownloadAndInstallUpdate, RestartApp, GetCurrentVersion } from '../wailsjs/go/main/App.js';

  let configured = false;
  let loading = true;
  let hasDiskAccess = false;

  // Setup form
  let token = '';
  let endpoint = 'wss://api.blutexts.com';
  let deviceName = '';
  let setupError = '';
  let saving = false;

  // Dashboard state
  let status = { connected: false, uptime: '', handles: [], device_name: '' };
  let activityLog = [];
  let pollInterval;

  // Update state
  let update = { available: false, version: '', notes: '', required: false, error: '' };
  let updating = false;
  let updateDone = false;
  let appVersion = '';

  onMount(async () => {
    configured = await IsConfigured();
    hasDiskAccess = await CheckDiskAccess();
    appVersion = await GetCurrentVersion();
    loading = false;

    if (configured) {
      startPolling();
      // Check for updates after a short delay
      setTimeout(checkUpdate, 3000);
    }
  });

  async function checkUpdate() {
    update = await CheckForUpdate();
  }

  async function doUpdate() {
    updating = true;
    const result = await DownloadAndInstallUpdate();
    updating = false;
    if (result.error) {
      update = { ...update, error: result.error };
    } else {
      updateDone = true;
    }
  }

  function startPolling() {
    poll();
    pollInterval = setInterval(poll, 2000);
  }

  async function poll() {
    status = await GetStatus();
    activityLog = await GetActivityLog();
  }

  async function handleSetup() {
    setupError = '';
    saving = true;
    try {
      await SaveSetup(token, endpoint, deviceName);
      configured = true;
      startPolling();
    } catch (e) {
      setupError = e;
    }
    saving = false;
  }
</script>

{#if loading}
  <div class="loading">
    <div class="spinner"></div>
  </div>
{:else if !configured}
  <!-- Setup Screen -->
  <div class="setup" style="padding-top: 28px;">
    <div class="setup-inner">
      <div class="logo">
        <div class="logo-icon">
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="white" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>
          </svg>
        </div>
        <span class="logo-text">BlueSend</span>
      </div>

      <h1>Connect this device</h1>
      <p class="subtitle">Paste the device token from your admin panel.</p>

      {#if !hasDiskAccess}
        <div class="warning no-drag">
          <strong>Full Disk Access required</strong>
          <p>BlueSend needs to read Messages data. Grant access in System Settings.</p>
          <button class="btn-link" on:click={OpenSystemPrefs}>Open System Settings</button>
        </div>
      {/if}

      <div class="form no-drag">
        <label>
          <span>Device Token</span>
          <input type="password" bind:value={token} placeholder="Paste from admin panel" />
        </label>
        <label>
          <span>API Endpoint</span>
          <input type="text" bind:value={endpoint} />
        </label>
        <label>
          <span>Device Name</span>
          <input type="text" bind:value={deviceName} placeholder="Auto-detected from hostname" />
        </label>

        {#if setupError}
          <div class="error">{setupError}</div>
        {/if}

        <button class="btn-primary" on:click={handleSetup} disabled={!token || saving}>
          {saving ? 'Connecting...' : 'Connect'}
        </button>
      </div>
    </div>
  </div>
{:else}
  <!-- Dashboard -->
  <div class="dashboard" style="padding-top: 28px;">
    <!-- Update banner -->
    {#if update.available && !updateDone}
      <div class="update-banner no-drag">
        <div class="update-info">
          <strong>BlueSend v{update.version} available</strong>
          {#if update.notes}<p>{update.notes}</p>{/if}
        </div>
        {#if update.error}
          <span class="update-error">{update.error}</span>
        {/if}
        <button class="btn-update" on:click={doUpdate} disabled={updating}>
          {updating ? 'Installing...' : 'Update Now'}
        </button>
      </div>
    {/if}
    {#if updateDone}
      <div class="update-banner update-done no-drag">
        <strong>Update installed!</strong>
        <button class="btn-update" on:click={RestartApp}>Restart Now</button>
      </div>
    {/if}

    <!-- Status bar -->
    <div class="status-bar no-drag">
      <div class="status-left">
        <div class="logo-small">
          <div class="logo-icon-sm">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="white" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
              <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>
            </svg>
          </div>
          <span>BlueSend</span>
        </div>
        <div class="status-badge" class:online={status.connected} class:offline={!status.connected}>
          <div class="dot"></div>
          {status.connected ? 'Connected' : 'Disconnected'}
        </div>
        {#if status.uptime}
          <span class="uptime">Uptime: {status.uptime}</span>
        {/if}
      </div>
      <div class="device-name">{status.device_name} <span class="version">v{appVersion}</span></div>
    </div>

    <div class="panels">
      <!-- Handles -->
      <div class="panel">
        <h3>iMessage Identities</h3>
        <p class="panel-desc">Phone numbers and emails tied to this device's Apple ID</p>
        {#if status.handles && status.handles.length > 0}
          {@const phones = status.handles.filter(h => h.startsWith('+') || /^\d/.test(h))}
          {@const emails = status.handles.filter(h => h.includes('@'))}
          {#if phones.length > 0}
            <div class="handle-section">Phone Numbers</div>
            <div class="handle-list">
              {#each phones as handle}
                <div class="handle-item">
                  <div class="handle-dot phone"></div>
                  {handle}
                </div>
              {/each}
            </div>
          {/if}
          {#if emails.length > 0}
            <div class="handle-section" class:mt={phones.length > 0}>Emails</div>
            <div class="handle-list">
              {#each emails as handle}
                <div class="handle-item">
                  <div class="handle-dot email"></div>
                  {handle}
                </div>
              {/each}
            </div>
          {/if}
        {:else}
          <div class="empty">No identities detected yet. Ensure Messages.app is signed in and has sent at least one message.</div>
        {/if}
      </div>

      <!-- Activity Log -->
      <div class="panel activity-panel">
        <h3>Activity</h3>
        <p class="panel-desc">Recent message and connection events</p>
        {#if activityLog.length > 0}
          <div class="log-list">
            {#each [...activityLog].reverse().slice(0, 50) as entry}
              <div class="log-entry" class:log-inbound={entry.type === 'inbound'} class:log-outbound={entry.type === 'outbound'} class:log-connection={entry.type === 'connection'}>
                <span class="log-time">{entry.time}</span>
                <span class="log-type">{entry.type}</span>
                <span class="log-msg">{entry.message}</span>
              </div>
            {/each}
          </div>
        {:else}
          <div class="empty">No activity yet. Waiting for messages...</div>
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  .loading { display: flex; align-items: center; justify-content: center; height: 100vh; }
  .spinner { width: 32px; height: 32px; border: 3px solid #e5e7eb; border-top-color: #007AFF; border-radius: 50%; animation: spin 0.6s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }

  /* Setup */
  .setup { display: flex; align-items: center; justify-content: center; height: 100vh; background: #f9fafb; }
  .setup-inner { width: 100%; max-width: 400px; padding: 0 24px; }
  .logo { display: flex; align-items: center; gap: 8px; margin-bottom: 24px; }
  .logo-icon { width: 32px; height: 32px; background: #007AFF; border-radius: 10px; display: flex; align-items: center; justify-content: center; }
  .logo-text { font-weight: 700; font-size: 18px; }
  h1 { font-size: 22px; font-weight: 700; margin-bottom: 4px; }
  .subtitle { color: #6b7280; font-size: 14px; margin-bottom: 24px; }
  .warning { background: #fef3c7; border: 1px solid #fde68a; border-radius: 12px; padding: 12px 16px; margin-bottom: 20px; font-size: 13px; }
  .warning strong { display: block; margin-bottom: 4px; color: #92400e; }
  .warning p { color: #78350f; margin-bottom: 8px; }
  .btn-link { background: none; border: none; color: #007AFF; font-size: 13px; font-weight: 600; cursor: pointer; padding: 0; }
  .form { display: flex; flex-direction: column; gap: 16px; }
  .form label { display: flex; flex-direction: column; gap: 4px; }
  .form label span { font-size: 13px; font-weight: 500; color: #374151; }
  .form input { padding: 10px 14px; border: 1px solid #d1d5db; border-radius: 10px; font-size: 14px; outline: none; font-family: inherit; }
  .form input:focus { border-color: #007AFF; box-shadow: 0 0 0 3px rgba(0,122,255,0.1); }
  .error { background: #fef2f2; border: 1px solid #fecaca; border-radius: 10px; padding: 10px 14px; font-size: 13px; color: #dc2626; }
  .btn-primary { background: #007AFF; color: white; border: none; border-radius: 10px; padding: 12px; font-size: 14px; font-weight: 600; cursor: pointer; font-family: inherit; margin-top: 4px; }
  .btn-primary:hover { background: #0066dd; }
  .btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }

  /* Dashboard */
  .dashboard { display: flex; flex-direction: column; height: 100vh; }
  .status-bar { display: flex; align-items: center; justify-content: space-between; padding: 12px 20px; background: white; border-bottom: 1px solid #e5e7eb; }
  .status-left { display: flex; align-items: center; gap: 12px; }
  .logo-small { display: flex; align-items: center; gap: 6px; font-weight: 600; font-size: 14px; }
  .logo-icon-sm { width: 22px; height: 22px; background: #007AFF; border-radius: 6px; display: flex; align-items: center; justify-content: center; }
  .status-badge { display: flex; align-items: center; gap: 6px; font-size: 12px; font-weight: 500; padding: 4px 10px; border-radius: 20px; border: 1px solid; }
  .status-badge.online { background: #f0fdf4; color: #15803d; border-color: #bbf7d0; }
  .status-badge.offline { background: #fef2f2; color: #dc2626; border-color: #fecaca; }
  .dot { width: 6px; height: 6px; border-radius: 50%; }
  .online .dot { background: #22c55e; animation: pulse 2s infinite; }
  .offline .dot { background: #ef4444; }
  @keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.5; } }
  .uptime { font-size: 12px; color: #9ca3af; }
  .device-name { font-size: 12px; color: #6b7280; }
  .version { color: #9ca3af; }

  .update-banner { display: flex; align-items: center; justify-content: space-between; padding: 10px 20px; background: #eff6ff; border-bottom: 1px solid #bfdbfe; gap: 12px; }
  .update-banner.update-done { background: #f0fdf4; border-color: #bbf7d0; }
  .update-info { flex: 1; font-size: 13px; }
  .update-info strong { font-size: 13px; }
  .update-info p { font-size: 12px; color: #6b7280; margin-top: 2px; }
  .update-error { font-size: 12px; color: #dc2626; }
  .btn-update { background: #007AFF; color: white; border: none; border-radius: 8px; padding: 6px 14px; font-size: 12px; font-weight: 600; cursor: pointer; white-space: nowrap; font-family: inherit; }
  .btn-update:hover { background: #0066dd; }
  .btn-update:disabled { opacity: 0.5; cursor: not-allowed; }

  .panels { display: flex; flex-direction: column; gap: 16px; padding: 20px; overflow-y: auto; flex: 1; }
  .panel { background: white; border: 1px solid #e5e7eb; border-radius: 14px; padding: 20px; }
  .panel h3 { font-size: 15px; font-weight: 600; margin-bottom: 2px; }
  .panel-desc { font-size: 12px; color: #9ca3af; margin-bottom: 14px; }
  .handle-list { display: flex; flex-direction: column; gap: 8px; }
  .handle-item { display: flex; align-items: center; gap: 8px; font-size: 14px; font-weight: 500; padding: 8px 12px; background: #f9fafb; border-radius: 8px; }
  .handle-section { font-size: 11px; font-weight: 600; color: #9ca3af; text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 6px; }
  .handle-section.mt { margin-top: 14px; }
  .handle-dot { width: 8px; height: 8px; border-radius: 50%; }
  .handle-dot.phone { background: #007AFF; }
  .handle-dot.email { background: #9ca3af; }
  .empty { font-size: 13px; color: #9ca3af; text-align: center; padding: 20px; }

  .activity-panel { flex: 1; display: flex; flex-direction: column; }
  .log-list { flex: 1; overflow-y: auto; max-height: 300px; display: flex; flex-direction: column; gap: 2px; }
  .log-entry { display: flex; gap: 8px; font-size: 12px; padding: 6px 10px; border-radius: 6px; font-family: 'SF Mono', 'Menlo', monospace; }
  .log-entry:hover { background: #f9fafb; }
  .log-time { color: #9ca3af; min-width: 60px; }
  .log-type { font-weight: 600; min-width: 80px; }
  .log-inbound .log-type { color: #007AFF; }
  .log-outbound .log-type { color: #22c55e; }
  .log-connection .log-type { color: #f59e0b; }
  .log-msg { color: #4b5563; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
