<script lang="ts">
  import { SecurityService } from "../../../bindings/github.com/nic/devtoolkit";
  import {
    Charset,
    Format,
    Mode,
    type Progress,
  } from "../../../bindings/github.com/nic/devtoolkit/internal/pkg/crackx/models";
  import { errMsg } from "../err";
  import Copy from "../components/Copy.svelte";
  import ErrorBar from "../components/ErrorBar.svelte";

  // --- file selection ---
  let fileName = $state("");
  let fileB64 = $state("");
  let fileSize = $state(0);

  // --- options ---
  let format = $state<Format>(Format.FormatAuto);
  let mode = $state<Mode>(Mode.ModeBrute);
  let lower = $state(true);
  let upper = $state(false);
  let digits = $state(true);
  let symbols = $state(false);
  let custom = $state("");
  let minLen = $state(1);
  let maxLen = $state(4);
  let wordlist = $state("");

  // --- run state ---
  let jobId = $state("");
  let running = $state(false);
  let progress = $state<Progress | null>(null);
  let error = $state("");
  let timer: ReturnType<typeof setInterval> | null = null;

  // --- resume state ---
  let resume = $state(true); // continue a previous run when a checkpoint exists
  let resumeTried = $state(0); // candidates already tried per the saved checkpoint
  let resumeCheckTimer: ReturnType<typeof setTimeout> | null = null;

  function buildOpts() {
    return {
      format,
      mode,
      minLen,
      maxLen,
      charset: new Charset({ lower, upper, digits, symbols, custom }),
      wordlist,
    };
  }

  // Whenever the file or any ordering-relevant option changes, ask the backend
  // (debounced) whether a resumable checkpoint exists for this exact setup.
  $effect(() => {
    // Touch every dependency so the effect re-runs when they change.
    void [fileB64, format, mode, minLen, maxLen, lower, upper, digits, symbols, custom, wordlist];
    if (resumeCheckTimer) clearTimeout(resumeCheckTimer);
    if (!fileB64 || running) {
      resumeTried = 0;
      return;
    }
    resumeCheckTimer = setTimeout(async () => {
      try {
        const info = await SecurityService.Resumable(fileB64, fileName, buildOpts());
        resumeTried = info.available ? info.tried : 0;
      } catch {
        resumeTried = 0;
      }
    }, 400);
  });

  function toBase64(buf: Uint8Array): string {
    let binary = "";
    const chunk = 0x8000;
    for (let i = 0; i < buf.length; i += chunk) {
      binary += String.fromCharCode(...buf.subarray(i, i + chunk));
    }
    return btoa(binary);
  }

  async function onFile(e: Event) {
    error = "";
    const file = (e.target as HTMLInputElement).files?.[0];
    if (!file) return;
    fileName = file.name;
    fileSize = file.size;
    const buf = new Uint8Array(await file.arrayBuffer());
    fileB64 = toBase64(buf);
  }

  async function onWordlistFile(e: Event) {
    const file = (e.target as HTMLInputElement).files?.[0];
    if (!file) return;
    wordlist = await file.text();
  }

  function stopPolling() {
    if (timer) {
      clearInterval(timer);
      timer = null;
    }
  }

  async function poll() {
    if (!jobId) return;
    try {
      const p = await SecurityService.CrackProgress(jobId);
      progress = p;
      if (p.done) {
        running = false;
        stopPolling();
      }
    } catch (e) {
      error = errMsg(e);
      running = false;
      stopPolling();
    }
  }

  async function start() {
    error = "";
    progress = null;
    if (!fileB64) {
      error = "请先选择要破解的文件";
      return;
    }
    if (mode === Mode.ModeDict && !wordlist.trim()) {
      error = "字典模式下请提供口令字典";
      return;
    }
    const opts = buildOpts();
    try {
      running = true;
      jobId = await SecurityService.StartCrack(fileB64, fileName, opts, resume);
      progress = null;
      stopPolling();
      timer = setInterval(poll, 250);
      poll();
    } catch (e) {
      error = errMsg(e);
      running = false;
    }
  }

  async function cancel() {
    if (!jobId) return;
    try {
      await SecurityService.CancelCrack(jobId);
    } catch (e) {
      error = errMsg(e);
    }
    running = false;
    stopPolling();
  }

  function fmtSize(n: number): string {
    if (n < 1024) return `${n} B`;
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
    return `${(n / 1024 / 1024).toFixed(1)} MB`;
  }

  function fmtNum(n: number): string {
    return n.toLocaleString("en-US");
  }

  function fmtDuration(sec: number): string {
    if (!isFinite(sec) || sec < 0) return "—";
    const s = Math.round(sec);
    if (s < 60) return `${s}秒`;
    const m = Math.floor(s / 60);
    if (m < 60) return `${m}分${s % 60}秒`;
    const h = Math.floor(m / 60);
    if (h < 24) return `${h}时${m % 60}分`;
    const d = Math.floor(h / 24);
    if (d < 365) return `${d}天${h % 24}时`;
    const y = Math.floor(d / 365);
    return `约${y}年`;
  }

  let percent = $derived(
    progress && progress.total > 0
      ? Math.min(100, (progress.tried / progress.total) * 100)
      : null,
  );
</script>

<div class="tool">
  <div class="notice">
    🔒 仅用于找回<strong>你本人拥有或已授权</strong>的文件口令。全部计算在本地完成，文件与口令不会上传。
  </div>

  <section class="card">
    <h3>1 · 选择文件</h3>
    <input
      type="file"
      accept=".zip,.pdf,.docx,.xlsx,.pptx,.docm,.xlsm,.pptm"
      onchange={onFile}
    />
    {#if fileName}
      <div class="filemeta">{fileName} · {fmtSize(fileSize)}</div>
    {/if}

    <div class="row">
      <label for="ck-fmt">文件格式</label>
      <select id="ck-fmt" bind:value={format}>
        <option value={Format.FormatAuto}>自动识别</option>
        <option value={Format.FormatZIP}>ZIP 压缩包</option>
        <option value={Format.FormatPDF}>PDF 文档</option>
        <option value={Format.FormatOffice}>Office（Word / Excel / PPT）</option>
      </select>
    </div>
  </section>

  <section class="card">
    <h3>2 · 攻击方式</h3>
    <div class="modes">
      <label class:active={mode === Mode.ModeBrute}>
        <input type="radio" bind:group={mode} value={Mode.ModeBrute} /> 暴力枚举
      </label>
      <label class:active={mode === Mode.ModeDict}>
        <input type="radio" bind:group={mode} value={Mode.ModeDict} /> 字典匹配
      </label>
    </div>

    {#if mode === Mode.ModeBrute}
      <div class="charset">
        <label><input type="checkbox" bind:checked={lower} /> 小写 a-z</label>
        <label><input type="checkbox" bind:checked={upper} /> 大写 A-Z</label>
        <label><input type="checkbox" bind:checked={digits} /> 数字 0-9</label>
        <label><input type="checkbox" bind:checked={symbols} /> 符号</label>
      </div>
      <div class="row">
        <label for="ck-custom">自定义字符</label>
        <input id="ck-custom" type="text" bind:value={custom} placeholder="额外要尝试的字符（可选）" />
      </div>
      <div class="lens">
        <label>最小长度 <input type="number" min="1" max="12" bind:value={minLen} /></label>
        <label>最大长度 <input type="number" min="1" max="12" bind:value={maxLen} /></label>
      </div>
      <p class="hint">长度越长、字符集越大，组合数呈指数增长，破解时间可能极长。</p>
    {:else}
      <input type="file" accept=".txt,.lst,.dic" onchange={onWordlistFile} />
      <textarea
        bind:value={wordlist}
        placeholder="每行一个候选口令，或上传字典文件…"
      ></textarea>
    {/if}

    <div class="actions">
      {#if running}
        <button class="danger" onclick={cancel}>停止</button>
      {:else}
        <button class="primary" onclick={start}>开始破解</button>
      {/if}
    </div>

    {#if resumeTried > 0 && !running}
      <label class="resume">
        <input type="checkbox" bind:checked={resume} />
        继续上次中断的进度（已尝试 {fmtNum(resumeTried)} 个候选）
      </label>
    {/if}
  </section>

  <ErrorBar message={error} />

  {#if progress}
    <section class="card">
      <h3>破解进度</h3>

      {#if progress.resumedFrom > 0}
        <div class="resumed">↻ 已从上次中断处继续，跳过 {fmtNum(progress.resumedFrom)} 个候选</div>
      {/if}

      <div class="bar">
        <div
          class="bar-fill"
          class:indet={percent === null && !progress.done}
          style={percent !== null ? `width:${percent}%` : "width:100%"}
        ></div>
      </div>

      <div class="stats">
        <div><span>已尝试</span><b>{fmtNum(progress.tried)}</b></div>
        <div>
          <span>候选总数</span>
          <b>{progress.total < 0 ? "极大" : fmtNum(progress.total)}</b>
        </div>
        <div><span>速度</span><b>{fmtNum(Math.round(progress.rate))}/s</b></div>
        <div><span>耗时</span><b>{progress.elapsed.toFixed(1)}s</b></div>
        <div>
          <span>预计剩余</span>
          <b>{progress.done ? "—" : fmtDuration(progress.eta)}</b>
        </div>
      </div>

      {#if !progress.done && progress.current}
        <div class="current">正在尝试：<code>{progress.current}</code></div>
      {/if}

      {#if progress.found}
        <div class="result ok">
          ✅ 破解成功，口令为：
          <code>{progress.password}</code>
          <Copy text={progress.password} />
        </div>
      {:else if progress.canceled}
        <div class="result warn">⏹ 已手动停止</div>
      {:else if progress.done && progress.error}
        <div class="result warn">❌ {progress.error}</div>
      {/if}
    </section>
  {/if}
</div>

<style>
  .tool {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .notice {
    background: var(--color-accent-weak);
    color: var(--color-accent);
    border-radius: var(--radius);
    padding: 10px 12px;
    font-size: 13px;
    line-height: 1.5;
  }
  .card {
    display: flex;
    flex-direction: column;
    gap: 10px;
    background: var(--color-surface);
    border-radius: var(--radius);
    padding: 14px;
  }
  .card h3 {
    margin: 0;
    font-size: 15px;
  }
  .filemeta {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--color-text);
    opacity: 0.7;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .row label {
    min-width: 72px;
  }
  .row input,
  .row select,
  select {
    flex: 1;
    padding: 6px 8px;
    border-radius: var(--radius);
    border: 1px solid var(--color-border);
    background: var(--color-bg);
    color: var(--color-text);
  }
  .modes {
    display: flex;
    gap: 10px;
  }
  .modes label {
    flex: 1;
    text-align: center;
    padding: 8px;
    border-radius: var(--radius);
    border: 1px solid var(--color-border);
    cursor: pointer;
  }
  .modes label.active {
    border-color: var(--color-accent);
    color: var(--color-accent);
    background: var(--color-accent-weak);
  }
  .charset {
    display: flex;
    flex-wrap: wrap;
    gap: 14px;
  }
  .charset label {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .lens {
    display: flex;
    gap: 16px;
  }
  .lens input {
    width: 64px;
    margin-left: 6px;
    padding: 4px 6px;
    border-radius: var(--radius);
    border: 1px solid var(--color-border);
    background: var(--color-bg);
    color: var(--color-text);
  }
  .hint {
    margin: 0;
    font-size: 12px;
    opacity: 0.65;
  }
  textarea {
    min-height: 120px;
    padding: 8px;
    border-radius: var(--radius);
    border: 1px solid var(--color-border);
    background: var(--color-bg);
    color: var(--color-text);
    font-family: var(--font-mono);
    font-size: 13px;
    resize: vertical;
  }
  .actions {
    display: flex;
    gap: 8px;
  }
  .resume {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 13px;
    opacity: 0.85;
  }
  .resumed {
    font-size: 13px;
    color: var(--color-accent);
    background: var(--color-accent-weak);
    border-radius: var(--radius);
    padding: 6px 10px;
  }
  .danger {
    background: var(--color-error, #e5484d);
    color: #fff;
    border: none;
    padding: 8px 16px;
    border-radius: var(--radius);
    cursor: pointer;
  }
  .bar {
    height: 10px;
    border-radius: 6px;
    background: var(--color-bg);
    overflow: hidden;
  }
  .bar-fill {
    height: 100%;
    background: var(--color-accent);
    transition: width 0.2s ease;
  }
  .bar-fill.indet {
    animation: pulse 1s ease-in-out infinite;
  }
  @keyframes pulse {
    0%,
    100% {
      opacity: 0.4;
    }
    50% {
      opacity: 1;
    }
  }
  .stats {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(88px, 1fr));
    gap: 10px;
  }
  .stats div {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .stats span {
    font-size: 12px;
    opacity: 0.6;
  }
  .stats b {
    font-family: var(--font-mono);
    font-size: 14px;
  }
  .current {
    font-size: 13px;
    opacity: 0.8;
  }
  .current code,
  .result code {
    font-family: var(--font-mono);
    background: var(--color-bg);
    padding: 1px 6px;
    border-radius: 4px;
  }
  .result {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 12px;
    border-radius: var(--radius);
    font-size: 14px;
  }
  .result.ok {
    background: var(--color-accent-weak);
    color: var(--color-accent);
  }
  .result.warn {
    background: var(--color-error-weak, #fdeaea);
    color: var(--color-error, #e5484d);
  }
</style>
