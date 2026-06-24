<script lang="ts">
  import Copy from "../components/Copy.svelte";

  // Current Unix timestamp (real-time update)
  let currentTimestamp = $state(Math.floor(Date.now() / 1000));
  let isRunning = $state(true);
  let timer: ReturnType<typeof setInterval> | undefined;

  // Timezone settings
  let timezone = $state<string>("local");

  // Common timezones
  const commonTimezones = [
    { value: "local", label: "本地时间" },
    { value: "UTC", label: "UTC" },
    { value: "Asia/Shanghai", label: "北京 (UTC+8)" },
    { value: "Asia/Tokyo", label: "东京 (UTC+9)" },
    { value: "America/New_York", label: "纽约 (UTC-5/-4)" },
    { value: "America/Los_Angeles", label: "洛杉矶 (UTC-8/-7)" },
    { value: "Europe/London", label: "伦敦 (UTC+0/+1)" },
    { value: "Europe/Paris", label: "巴黎 (UTC+1/+2)" },
  ];

  // Unix timestamp -> Human readable
  let timestampInput = $state("");
  let humanResult = $state("");
  let humanResultOther: Array<{ tz: string; label: string; time: string }> = $state([]);

  // Human readable -> Unix timestamp
  let year = $state(new Date().getFullYear());
  let month = $state(new Date().getMonth() + 1);
  let day = $state(new Date().getDate());
  let hour = $state(new Date().getHours());
  let minute = $state(new Date().getMinutes());
  let second = $state(new Date().getSeconds());
  let timestampResult = $state("");
  let timestampResultMs = $state("");

  // Get timezone offset info
  function getTimezoneOffset(tz: string): string {
    if (tz === "local") {
      const offset = -new Date().getTimezoneOffset();
      const hours = Math.floor(Math.abs(offset) / 60);
      const mins = Math.abs(offset) % 60;
      const sign = offset >= 0 ? "+" : "-";
      return `UTC${sign}${hours}${mins > 0 ? `:${mins.toString().padStart(2, "0")}` : ""}`;
    }
    try {
      const now = new Date();
      const formatter = new Intl.DateTimeFormat("en-US", {
        timeZone: tz,
        timeZoneName: "shortOffset",
      });
      const parts = formatter.formatToParts(now);
      const tzPart = parts.find((p) => p.type === "timeZoneName");
      return tzPart?.value || "";
    } catch {
      return "";
    }
  }

  // Format date in timezone
  function formatDateInTimezone(date: Date, tz: string): string {
    const options: Intl.DateTimeFormatOptions = {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
    };

    if (tz === "local") {
      return date.toLocaleString("zh-CN", options);
    }

    try {
      options.timeZone = tz;
      return date.toLocaleString("zh-CN", options);
    } catch {
      return date.toLocaleString("zh-CN", options);
    }
  }

  // Real-time update of current timestamp
  $effect(() => {
    if (isRunning) {
      timer = setInterval(() => {
        currentTimestamp = Math.floor(Date.now() / 1000);
      }, 1000);
    }
    return () => {
      if (timer) clearInterval(timer);
    };
  });

  // Convert timestamp to human readable
  function convertToHuman() {
    const ts = parseInt(timestampInput, 10);
    if (isNaN(ts)) {
      humanResult = "";
      humanResultOther = [];
      return;
    }

    const ms = ts > 1e12 ? ts : ts * 1000;
    const date = new Date(ms);

    if (isNaN(date.getTime())) {
      humanResult = "";
      humanResultOther = [];
      return;
    }

    const formatted = formatDateInTimezone(date, timezone);
    const iso = date.toISOString();
    const utc = date.toUTCString();

    humanResult = `${formatted}\nISO: ${iso}\nUTC: ${utc}`;

    humanResultOther = commonTimezones
      .filter((tz) => tz.value !== timezone && tz.value !== "local")
      .map((tz) => ({
        tz: tz.value,
        label: tz.label,
        time: formatDateInTimezone(date, tz.value),
      }));
  }

  // Convert human readable to timestamp
  function convertToTimestamp() {
    const date = new Date(year, month - 1, day, hour, minute, second);
    if (isNaN(date.getTime())) {
      timestampResult = "";
      timestampResultMs = "";
      return;
    }

    timestampResult = String(Math.floor(date.getTime() / 1000));
    timestampResultMs = String(date.getTime());
  }

  // Use current timestamp
  function useCurrentTimestamp() {
    timestampInput = String(currentTimestamp);
    convertToHuman();
  }

  // Use current date/time
  function useCurrentDateTime() {
    const now = new Date();
    year = now.getFullYear();
    month = now.getMonth() + 1;
    day = now.getDate();
    hour = now.getHours();
    minute = now.getMinutes();
    second = now.getSeconds();
    convertToTimestamp();
  }

  // Toggle real-time update
  function toggleTimer() {
    isRunning = !isRunning;
  }

  // Format timestamp with separators
  let formattedTimestamp = $derived(currentTimestamp.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ","));

  // Current time in selected timezone
  let currentTimeInTz = $derived(() => {
    return formatDateInTimezone(new Date(), timezone);
  });
</script>

<div class="tool">
  <!-- Unix timestamp to human readable -->
  <section class="card">
    <div class="card-header">
      <h3>时间戳 → 时间</h3>
      <div class="timezone-select">
        <label for="tz-select">时区</label>
        <select id="tz-select" bind:value={timezone} onchange={convertToHuman}>
          {#each commonTimezones as tz}
            <option value={tz.value}>{tz.label}</option>
          {/each}
        </select>
      </div>
    </div>

    <div class="input-group">
      <input
        type="text"
        bind:value={timestampInput}
        placeholder="输入 Unix 时间戳，如 1782283288"
        oninput={convertToHuman}
      />
      <span class="input-hint">自动识别秒/毫秒</span>
    </div>

    <div class="quick-actions">
      <button class="btn-sm" onclick={useCurrentTimestamp}>使用当前时间戳</button>
    </div>

    {#if humanResult}
      <div class="result-section">
        <div class="result-header">
          <span class="result-label">{commonTimezones.find((t) => t.value === timezone)?.label}</span>
          <Copy text={humanResult} />
        </div>
        <pre class="result-main">{humanResult}</pre>

        {#if humanResultOther.length > 0}
          <div class="other-zones">
            <div class="zones-title">其他时区</div>
            <div class="zones-grid">
              {#each humanResultOther as item}
                <div class="zone-row">
                  <span class="zone-name">{item.label}</span>
                  <code class="zone-time">{item.time}</code>
                </div>
              {/each}
            </div>
          </div>
        {/if}
      </div>
    {/if}
  </section>

  <!-- Human readable to Unix timestamp -->
  <section class="card">
    <div class="card-header">
      <h3>时间 → 时间戳</h3>
      <span class="tz-hint">本地时区</span>
    </div>

    <div class="datetime-grid">
      <div class="dt-field">
        <input id="ts-year" type="number" bind:value={year} oninput={convertToTimestamp} min="1970" max="2100" />
        <label for="ts-year">年</label>
      </div>
      <div class="dt-field">
        <input id="ts-month" type="number" bind:value={month} oninput={convertToTimestamp} min="1" max="12" />
        <label for="ts-month">月</label>
      </div>
      <div class="dt-field">
        <input id="ts-day" type="number" bind:value={day} oninput={convertToTimestamp} min="1" max="31" />
        <label for="ts-day">日</label>
      </div>
      <div class="dt-field">
        <input id="ts-hour" type="number" bind:value={hour} oninput={convertToTimestamp} min="0" max="23" />
        <label for="ts-hour">时</label>
      </div>
      <div class="dt-field">
        <input id="ts-minute" type="number" bind:value={minute} oninput={convertToTimestamp} min="0" max="59" />
        <label for="ts-minute">分</label>
      </div>
      <div class="dt-field">
        <input id="ts-second" type="number" bind:value={second} oninput={convertToTimestamp} min="0" max="59" />
        <label for="ts-second">秒</label>
      </div>
    </div>

    <div class="quick-actions">
      <button class="btn-sm" onclick={useCurrentDateTime}>使用当前时间</button>
    </div>

    {#if timestampResult}
      <div class="result-section">
        <div class="result-grid">
          <div class="result-item">
            <div class="result-header">
              <span class="result-label">Unix 时间戳（秒）</span>
              <Copy text={timestampResult} />
            </div>
            <div class="result-box primary">
              <code class="result-value">{timestampResult}</code>
              <span class="result-unit">秒</span>
            </div>
          </div>

          <div class="result-item">
            <div class="result-header">
              <span class="result-label">Unix 时间戳（毫秒）</span>
              <Copy text={timestampResultMs} />
            </div>
            <div class="result-box">
              <code class="result-value">{timestampResultMs}</code>
              <span class="result-unit">毫秒</span>
            </div>
          </div>
        </div>
      </div>
    {/if}
  </section>

  <!-- Current timestamp display -->
  <section class="card current-card">
    <div class="current-main">
      <div class="current-info">
        <div class="current-header">
          <h3>当前时间戳</h3>
          <div class="status-badge" class:running={isRunning}>
            <span class="status-dot"></span>
            {isRunning ? "实时" : "暂停"}
          </div>
        </div>
        <div class="timestamp-row">
          <span class="timestamp-value">{formattedTimestamp}</span>
          <span class="timestamp-unit">秒</span>
        </div>
        <div class="time-info">
          <span class="time-value">{currentTimeInTz()}</span>
          <span class="tz-offset">{getTimezoneOffset(timezone)}</span>
        </div>
      </div>
      <div class="current-actions">
        <button class="btn-icon" onclick={toggleTimer} title={isRunning ? "暂停" : "开始"}>
          {isRunning ? "⏸" : "▶"}
        </button>
        <button class="btn-icon" onclick={() => (currentTimestamp = Math.floor(Date.now() / 1000))} title="刷新">
          ↻
        </button>
        <Copy text={String(currentTimestamp)} label="复制" />
      </div>
    </div>
  </section>
</div>

<style>
  .tool {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .card {
    display: flex;
    flex-direction: column;
    gap: 12px;
    background: var(--color-surface);
    border-radius: var(--radius);
    padding: 16px;
  }

  .card h3 {
    margin: 0;
    font-size: 15px;
    font-weight: 600;
  }

  .card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .timezone-select {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .timezone-select label {
    font-size: 13px;
    color: var(--color-text-weak);
  }

  .timezone-select select {
    width: auto;
    min-width: 130px;
  }

  .tz-hint {
    font-size: 12px;
    color: var(--color-text-weak);
    background: var(--color-surface-2);
    padding: 3px 8px;
    border-radius: 4px;
  }

  /* Input styles */
  .input-group {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .input-hint {
    font-size: 11px;
    color: var(--color-text-weak);
    opacity: 0.7;
  }

  /* Datetime grid */
  .datetime-grid {
    display: grid;
    grid-template-columns: repeat(6, 1fr);
    gap: 8px;
  }

  .dt-field {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .dt-field input {
    text-align: center;
    font-family: var(--font-mono);
    font-size: 14px;
  }

  .dt-field label {
    font-size: 11px;
    color: var(--color-text-weak);
    text-align: center;
  }

  /* Quick actions */
  .quick-actions {
    display: flex;
    gap: 8px;
  }

  .btn-sm {
    padding: 5px 12px;
    font-size: 13px;
  }

  /* Results */
  .result-section {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding-top: 8px;
    border-top: 1px solid var(--color-border);
  }

  .result-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .result-label {
    font-size: 12px;
    color: var(--color-text-weak);
    font-weight: 500;
  }

  .result-main {
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: 10px;
    margin: 0;
    font-family: var(--font-mono);
    font-size: 13px;
    white-space: pre-wrap;
    word-break: break-all;
    line-height: 1.6;
  }

  .result-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
  }

  .result-item {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .result-box {
    display: flex;
    align-items: center;
    gap: 8px;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: 10px 12px;
  }

  .result-box.primary {
    background: linear-gradient(135deg, var(--color-accent-weak) 0%, var(--color-bg) 100%);
    border-color: var(--color-accent);
  }

  .result-value {
    font-family: var(--font-mono);
    font-size: 15px;
    font-weight: 600;
    word-break: break-all;
  }

  .result-box.primary .result-value {
    font-size: 17px;
    color: var(--color-accent);
  }

  .result-unit {
    font-size: 12px;
    color: var(--color-text-weak);
    white-space: nowrap;
  }

  /* Other timezones */
  .other-zones {
    display: flex;
    flex-direction: column;
    gap: 8px;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: 12px;
  }

  .zones-title {
    font-size: 12px;
    color: var(--color-text-weak);
    text-transform: uppercase;
    letter-spacing: 0.5px;
    font-weight: 500;
  }

  .zones-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 8px;
  }

  .zone-row {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 6px;
    background: var(--color-surface);
    border-radius: 4px;
  }

  .zone-name {
    font-size: 11px;
    color: var(--color-text-weak);
  }

  .zone-time {
    font-family: var(--font-mono);
    font-size: 12px;
    background: none;
  }

  /* Current timestamp card */
  .current-card {
    background: linear-gradient(135deg, var(--color-accent-weak) 0%, var(--color-surface) 100%);
    border: 1px solid var(--color-accent);
    padding: 14px 16px;
  }

  .current-main {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
  }

  .current-info {
    display: flex;
    align-items: center;
    gap: 24px;
    flex: 1;
  }

  .current-header {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .status-badge {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    font-size: 11px;
    padding: 3px 8px;
    border-radius: 10px;
    background: var(--color-surface-2);
    color: var(--color-text-weak);
    font-weight: 500;
  }

  .status-badge.running {
    background: rgba(34, 197, 94, 0.15);
    color: var(--color-success);
  }

  .status-dot {
    width: 5px;
    height: 5px;
    border-radius: 50%;
    background: currentColor;
  }

  .status-badge.running .status-dot {
    animation: pulse 2s ease-in-out infinite;
  }

  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.4; }
  }

  .timestamp-row {
    display: flex;
    align-items: baseline;
    gap: 6px;
  }

  .timestamp-value {
    font-family: var(--font-mono);
    font-size: 28px;
    font-weight: 700;
    color: var(--color-accent);
    letter-spacing: -0.5px;
  }

  .timestamp-unit {
    font-size: 13px;
    color: var(--color-text-weak);
  }

  .time-info {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .time-value {
    font-family: var(--font-mono);
    font-size: 14px;
    font-weight: 500;
  }

  .tz-offset {
    font-size: 11px;
    color: var(--color-text-weak);
  }

  .current-actions {
    display: flex;
    gap: 6px;
    flex-shrink: 0;
  }

  .btn-icon {
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0;
    font-size: 16px;
  }

  /* Responsive */
  @media (max-width: 1100px) {
    .zones-grid {
      grid-template-columns: repeat(2, 1fr);
    }
  }

  @media (max-width: 900px) {
    .current-info {
      flex-direction: column;
      align-items: flex-start;
      gap: 12px;
    }

    .current-main {
      flex-direction: column;
      align-items: flex-start;
    }

    .current-actions {
      width: 100%;
      justify-content: flex-end;
    }
  }

  @media (max-width: 600px) {
    .datetime-grid {
      grid-template-columns: repeat(3, 1fr);
    }

    .result-grid {
      grid-template-columns: 1fr;
    }

    .zones-grid {
      grid-template-columns: 1fr;
    }

    .timestamp-value {
      font-size: 22px;
    }
  }
</style>
