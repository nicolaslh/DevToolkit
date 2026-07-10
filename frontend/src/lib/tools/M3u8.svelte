<script lang="ts">
  import { MediaService } from "../../../bindings/github.com/nic/devtoolkit";
  import {
    Container,
    Options,
    type FFmpegInfo,
    type BatchProgress,
  } from "../../../bindings/github.com/nic/devtoolkit/internal/pkg/mediax/models";
  import { errMsg } from "../err";
  import Copy from "../components/Copy.svelte";
  import ErrorBar from "../components/ErrorBar.svelte";

  // --- ffmpeg availability ---
  let ffmpeg = $state<FFmpegInfo | null>(null);

  function loadFFmpeg() {
    MediaService.CheckFFmpeg()
      .then((info) => (ffmpeg = info))
      .catch(
        () =>
          (ffmpeg = {
            available: false,
            path: "",
            version: "",
            os: "",
            installCmd: "",
            downloadURL: "https://ffmpeg.org/download.html",
          }),
      );
  }
  $effect(loadFFmpeg);

  function recheckFFmpeg() {
    ffmpeg = null;
    loadFFmpeg();
  }

  // --- sources + output ---
  let sources = $state<string[]>([]);
  let urlInput = $state("");
  let outputDir = $state("");
  let format = $state<Container>(Container.MP4);
  let reEncode = $state(false);

  // --- run state ---
  let jobId = $state("");
  let running = $state(false);
  let progress = $state<BatchProgress | null>(null);
  let error = $state("");
  let timer: ReturnType<typeof setInterval> | null = null;

  // Adds one or more network addresses from the URL box. Accepts multiple
  // entries separated by newlines, commas or spaces so a whole list can be
  // pasted at once. Only http(s) URLs are accepted; duplicates are ignored.
  function addUrls() {
    error = "";
    const candidates = urlInput
      .split(/[\s,]+/)
      .map((s) => s.trim())
      .filter(Boolean);
    if (candidates.length === 0) return;

    const merged = [...sources];
    let added = 0;
    let skipped = 0;
    for (const u of candidates) {
      if (!/^https?:\/\//i.test(u)) {
        skipped++;
        continue;
      }
      if (!merged.includes(u)) {
        merged.push(u);
        added++;
      }
    }
    sources = merged;
    urlInput = "";
    if (added === 0 && skipped > 0) {
      error = "请输入以 http:// 或 https:// 开头的有效地址";
    } else if (skipped > 0) {
      error = `已添加 ${added} 个地址，忽略 ${skipped} 个无效条目`;
    }
  }

  function onUrlKey(e: KeyboardEvent) {
    // Ctrl/Cmd+Enter adds; plain Enter inserts a newline (multi-URL paste).
    if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
      e.preventDefault();
      addUrls();
    }
  }

  async function addFiles() {
    error = "";
    try {
      const paths = await MediaService.PickInputFiles();
      if (paths && paths.length) {
        const merged = [...sources];
        for (const p of paths) if (!merged.includes(p)) merged.push(p);
        sources = merged;
      }
    } catch (e) {
      error = errMsg(e);
    }
  }

  function removeSource(i: number) {
    sources = sources.filter((_, idx) => idx !== i);
  }

  function clearSources() {
    sources = [];
  }

  async function browseOutputDir() {
    error = "";
    try {
      const dir = await MediaService.PickOutputDir();
      if (dir) outputDir = dir;
    } catch (e) {
      error = errMsg(e);
    }
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
      const p = await MediaService.BatchProgress(jobId);
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
    if (sources.length === 0) {
      error = "请至少添加一个 m3u8 地址或文件";
      return;
    }
    if (!outputDir.trim()) {
      error = "请选择输出文件夹";
      return;
    }
    try {
      running = true;
      const opts = new Options({ format, reEncode });
      jobId = await MediaService.StartBatchConvert(sources, outputDir, opts);
      stopPolling();
      timer = setInterval(poll, 400);
      poll();
    } catch (e) {
      error = errMsg(e);
      running = false;
    }
  }

  async function cancel() {
    if (!jobId) return;
    try {
      await MediaService.CancelBatchConvert(jobId);
    } catch (e) {
      error = errMsg(e);
    }
    running = false;
    stopPolling();
  }

  function shortSource(s: string): string {
    const clean = s.split("?")[0].split("#")[0];
    return clean.split(/[\\/]/).pop() || s;
  }

  function itemPercent(p: number): string {
    return p >= 0 ? `${Math.min(100, p).toFixed(0)}%` : "…";
  }

  // Formats a seconds value as a compact clock (e.g. 1:05, 3:20:00).
  function fmtDuration(sec: number): string {
    if (!isFinite(sec) || sec < 0) return "—";
    const s = Math.round(sec);
    const h = Math.floor(s / 3600);
    const m = Math.floor((s % 3600) / 60);
    const ss = s % 60;
    const pad = (n: number) => String(n).padStart(2, "0");
    return h > 0 ? `${h}:${pad(m)}:${pad(ss)}` : `${m}:${pad(ss)}`;
  }

  // Estimated time remaining, rendered as "约 X" or "—" when unknown.
  function fmtETA(sec: number): string {
    if (!isFinite(sec) || sec < 0) return "—";
    if (sec < 1) return "即将完成";
    return `约 ${fmtDuration(sec)}`;
  }
</script>

<div class="tool">
  <div class="notice">
    🎬 批量将 m3u8 (HLS) 流转换为 MP4 / MKV / MOV 等格式。支持在线地址与本地播放列表，
    输出统一保存到指定文件夹，文件名沿用来源名称、仅更改后缀。转换在本地通过
    <code>ffmpeg</code> 完成，请仅转换你有权下载的内容。
  </div>

  {#if ffmpeg && !ffmpeg.available}
    <div class="warn-bar">
      <div class="warn-title">⚠️ 未检测到 <code>ffmpeg</code></div>
      <p>该功能依赖 ffmpeg 进行转换。请按下方命令安装，安装后重新打开本工具即可自动识别。</p>
      {#if ffmpeg.installCmd}
        <div class="cmd">
          <code>{ffmpeg.installCmd}</code>
          <Copy text={ffmpeg.installCmd} />
        </div>
      {/if}
      <p class="alt">
        其他平台：macOS <code>brew install ffmpeg</code> ·
        Windows <code>winget install Gyan.FFmpeg</code> ·
        Linux <code>sudo apt install ffmpeg</code>
        <br />
        或从官网下载：<a href={ffmpeg.downloadURL} target="_blank" rel="noreferrer">{ffmpeg.downloadURL}</a>
      </p>
      <button class="recheck" onclick={recheckFFmpeg}>重新检测</button>
    </div>
  {:else if ffmpeg && ffmpeg.available && ffmpeg.version}
    <div class="ok-bar">✓ 已检测到 ffmpeg：{ffmpeg.version}</div>
  {/if}

  <section class="card">
    <div class="head">
      <h3>1 · 输入源（{sources.length}）</h3>
      {#if sources.length > 0}
        <button class="link" onclick={clearSources}>清空</button>
      {/if}
    </div>

    <div class="url-add">
      <textarea
        bind:value={urlInput}
        onkeydown={onUrlKey}
        rows="2"
        placeholder="粘贴 m3u8 网络地址（http/https），可一行一个批量粘贴；Ctrl/⌘+Enter 添加"
      ></textarea>
      <div class="url-btns">
        <button onclick={addUrls}>添加地址</button>
        <button onclick={addFiles}>选择文件</button>
      </div>
    </div>

    {#if sources.length > 0}
      <ul class="sources">
        {#each sources as s, i (s)}
          <li>
            <span class="mono" title={s}>{shortSource(s)}</span>
            {#if !running}
              <button class="link" onclick={() => removeSource(i)}>移除</button>
            {/if}
          </li>
        {/each}
      </ul>
    {/if}
  </section>

  <section class="card">
    <h3>2 · 输出设置</h3>
    <div class="row">
      <label for="m3-fmt">输出格式</label>
      <select id="m3-fmt" bind:value={format}>
        <option value={Container.MP4}>MP4</option>
        <option value={Container.MKV}>MKV</option>
        <option value={Container.MOV}>MOV</option>
        <option value={Container.TS}>TS</option>
        <option value={Container.MP3}>MP3（仅音频）</option>
      </select>
    </div>

    {#if format !== Container.MP3}
      <label class="check">
        <input type="checkbox" bind:checked={reEncode} />
        重新编码为 H.264 / AAC（更慢，但兼容性更好；默认直接复制流，速度快）
      </label>
    {/if}

    <div class="row">
      <input
        type="text"
        bind:value={outputDir}
        placeholder="输出文件夹…"
        readonly
      />
      <button onclick={browseOutputDir}>选择文件夹</button>
    </div>

    <div class="actions">
      {#if running}
        <button class="danger" onclick={cancel}>停止</button>
      {:else}
        <button
          class="primary"
          onclick={start}
          disabled={!!ffmpeg && !ffmpeg.available}
        >
          开始批量转换
        </button>
      {/if}
    </div>
  </section>

  <ErrorBar message={error} />

  {#if progress}
    <section class="card">
      <div class="head">
        <h3>转换进度</h3>
        <span class="summary">
          {progress.completed}/{progress.total} 完成
          {#if progress.failed > 0}· <span class="fail">{progress.failed} 失败</span>{/if}
        </span>
      </div>

      <div class="bar">
        <div class="bar-fill" style={`width:${Math.min(100, progress.percent)}%`}></div>
      </div>

      <div class="ostats">
        <div><span>总进度</span><b>{progress.percent.toFixed(1)}%</b></div>
        <div><span>已用时</span><b>{fmtDuration(progress.elapsed)}</b></div>
        <div><span>预计剩余</span><b>{progress.done ? "—" : fmtETA(progress.eta)}</b></div>
      </div>

      <ul class="items">
        {#each progress.items as it, i (it.output)}
          <li class:running={progress.current === i && !it.done}>
            <div class="item-top">
              <span class="mono name" title={it.output}>{it.name}</span>
              <span class="state">
                {#if it.success}
                  ✅ 完成
                {:else if it.done}
                  ❌ 失败
                {:else if progress.current === i}
                  {itemPercent(it.percent)}
                  {#if it.eta >= 0}· 剩 {fmtDuration(it.eta)}{/if}
                {:else}
                  待处理
                {/if}
              </span>
            </div>
            {#if progress.current === i && !it.done}
              <div class="mini-bar">
                <div
                  class="mini-fill"
                  class:indet={it.percent < 0}
                  style={it.percent >= 0 ? `width:${Math.min(100, it.percent)}%` : "width:100%"}
                ></div>
              </div>
            {/if}
            {#if it.done && !it.success && it.error}
              <div class="item-error">{it.error}</div>
            {/if}
          </li>
        {/each}
      </ul>

      {#if progress.done}
        {#if progress.canceled}
          <div class="result warn">⏹ 已手动停止（{progress.succeeded} 个已完成）</div>
        {:else if progress.failed === 0}
          <div class="result ok">
            ✅ 全部完成，共 {progress.succeeded} 个文件
            <Copy text={outputDir} />
          </div>
        {:else}
          <div class="result warn">
            完成 {progress.succeeded} 个，失败 {progress.failed} 个（详见上方列表）
          </div>
        {/if}
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
    line-height: 1.6;
  }
  .warn-bar {
    background: var(--color-error-weak, #fdeaea);
    color: var(--color-error, #e5484d);
    border-radius: var(--radius);
    padding: 12px 14px;
    font-size: 13px;
    line-height: 1.6;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .warn-bar p {
    margin: 0;
  }
  .warn-title {
    font-weight: 600;
    font-size: 14px;
  }
  .warn-bar .cmd {
    display: flex;
    align-items: center;
    gap: 8px;
    background: var(--color-bg);
    border-radius: var(--radius);
    padding: 8px 10px;
  }
  .warn-bar .cmd code {
    flex: 1;
    font-family: var(--font-mono);
    color: var(--color-text);
  }
  .warn-bar .alt {
    font-size: 12px;
    opacity: 0.85;
    line-height: 1.7;
  }
  .warn-bar .alt a {
    color: inherit;
  }
  .recheck {
    align-self: flex-start;
    background: var(--color-bg);
    border: 1px solid currentColor;
    color: inherit;
  }
  .ok-bar {
    background: var(--color-accent-weak);
    color: var(--color-accent);
    border-radius: var(--radius);
    padding: 8px 12px;
    font-size: 12px;
    font-family: var(--font-mono);
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
  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .summary {
    font-size: 12px;
    opacity: 0.7;
    font-family: var(--font-mono);
  }
  .summary .fail {
    color: var(--color-error, #e5484d);
    opacity: 1;
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
  select {
    flex: 1;
    padding: 6px 8px;
    border-radius: var(--radius);
    border: 1px solid var(--color-border);
    background: var(--color-bg);
    color: var(--color-text);
    font-family: var(--font-mono);
    font-size: 13px;
  }
  button {
    padding: 6px 14px;
    border-radius: var(--radius);
    border: 1px solid var(--color-border);
    background: var(--color-bg);
    color: var(--color-text);
    cursor: pointer;
    white-space: nowrap;
  }
  button.link {
    border: none;
    background: none;
    color: var(--color-accent);
    padding: 2px 4px;
    font-size: 13px;
  }
  .url-add {
    display: flex;
    gap: 10px;
    align-items: stretch;
  }
  .url-add textarea {
    flex: 1;
    padding: 8px;
    border-radius: var(--radius);
    border: 1px solid var(--color-border);
    background: var(--color-bg);
    color: var(--color-text);
    font-family: var(--font-mono);
    font-size: 13px;
    resize: vertical;
    min-height: 44px;
  }
  .url-btns {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .url-btns button {
    height: 100%;
  }
  .sources {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
    max-height: 220px;
    overflow-y: auto;
  }
  .sources li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 6px 8px;
    background: var(--color-bg);
    border-radius: var(--radius);
  }
  .mono {
    font-family: var(--font-mono);
    font-size: 12px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .check {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    opacity: 0.85;
  }
  .actions {
    display: flex;
    gap: 8px;
  }
  .primary {
    background: var(--color-accent);
    color: #fff;
    border: none;
  }
  .primary:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .danger {
    background: var(--color-error, #e5484d);
    color: #fff;
    border: none;
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
    transition: width 0.3s ease;
  }
  .ostats {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 10px;
  }
  .ostats div {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .ostats span {
    font-size: 12px;
    opacity: 0.6;
  }
  .ostats b {
    font-family: var(--font-mono);
    font-size: 14px;
  }
  .items {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 6px;
    max-height: 320px;
    overflow-y: auto;
  }
  .items li {
    padding: 8px 10px;
    background: var(--color-bg);
    border-radius: var(--radius);
    border: 1px solid transparent;
  }
  .items li.running {
    border-color: var(--color-accent);
  }
  .item-top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }
  .item-top .name {
    flex: 1;
  }
  .state {
    font-size: 12px;
    font-family: var(--font-mono);
    white-space: nowrap;
    opacity: 0.85;
  }
  .mini-bar {
    height: 5px;
    border-radius: 3px;
    background: var(--color-surface);
    overflow: hidden;
    margin-top: 6px;
  }
  .mini-fill {
    height: 100%;
    background: var(--color-accent);
    transition: width 0.3s ease;
  }
  .mini-fill.indet {
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
  .item-error {
    margin-top: 6px;
    font-size: 12px;
    color: var(--color-error, #e5484d);
    word-break: break-all;
  }
  .result {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 12px;
    border-radius: var(--radius);
    font-size: 14px;
    word-break: break-all;
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
