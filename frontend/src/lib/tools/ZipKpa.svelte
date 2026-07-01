<script lang="ts">
  import { SecurityService } from "../../../bindings/github.com/nic/devtoolkit";
  import {
    KPAPhase,
    ZipEncryption,
    Charset,
    type ZipInfo,
    type KPAProgress,
    type DecryptedEntry,
    type PasswordProgress,
    type PlaintextPrep,
  } from "../../../bindings/github.com/nic/devtoolkit/internal/pkg/crackx/models";
  import { errMsg } from "../err";
  import Copy from "../components/Copy.svelte";
  import ErrorBar from "../components/ErrorBar.svelte";

  // --- archive selection ---
  let archiveName = $state("");
  let archiveB64 = $state("");
  let archiveSize = $state(0);
  let info = $state<ZipInfo | null>(null);
  let targetEntry = $state("");

  // --- known plaintext ---
  // "auto": user supplies the original file; the backend re-compresses/verifies.
  // "manual": user supplies the already-compressed bytes directly.
  let plainMode = $state<"auto" | "manual">("auto");

  // auto mode
  let originalName = $state("");
  let originalSize = $state(0);
  let prep = $state<PlaintextPrep | null>(null);
  let preparing = $state(false);

  // manual mode
  let plainName = $state("");
  let plainB64 = $state("");
  let plainSize = $state(0);
  let offset = $state(0);

  // --- run state ---
  let jobId = $state("");
  let running = $state(false);
  let progress = $state<KPAProgress | null>(null);
  let results = $state<DecryptedEntry[]>([]);
  let error = $state("");
  let timer: ReturnType<typeof setInterval> | null = null;

  // --- optional password recovery ---
  let pwLower = $state(true);
  let pwUpper = $state(false);
  let pwDigits = $state(true);
  let pwSymbols = $state(false);
  let pwMinLen = $state(1);
  let pwMaxLen = $state(6);
  let pwJobId = $state("");
  let pwRunning = $state(false);
  let pwProgress = $state<PasswordProgress | null>(null);
  let pwTimer: ReturnType<typeof setInterval> | null = null;

  // ZipCrypto entries are the only ones a known-plaintext attack can target.
  let zipCryptoEntries = $derived(
    info ? info.entries.filter((e) => e.encryption === ZipEncryption.ZipEncZipCrypto) : [],
  );

  function toBase64(buf: Uint8Array): string {
    let binary = "";
    const chunk = 0x8000;
    for (let i = 0; i < buf.length; i += chunk) {
      binary += String.fromCharCode(...buf.subarray(i, i + chunk));
    }
    return btoa(binary);
  }

  function fromBase64(b64: string): Uint8Array {
    const bin = atob(b64);
    const out = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
    return out;
  }

  async function onArchive(e: Event) {
    error = "";
    info = null;
    results = [];
    progress = null;
    const file = (e.target as HTMLInputElement).files?.[0];
    if (!file) return;
    archiveName = file.name;
    archiveSize = file.size;
    const buf = new Uint8Array(await file.arrayBuffer());
    archiveB64 = toBase64(buf);
    try {
      info = await SecurityService.InspectZip(archiveB64);
      targetEntry = zipCryptoEntries[0]?.name ?? "";
    } catch (err) {
      error = errMsg(err);
    }
  }

  async function onOriginalFile(e: Event) {
    error = "";
    prep = null;
    const file = (e.target as HTMLInputElement).files?.[0];
    if (!file) return;
    originalName = file.name;
    originalSize = file.size;
    const buf = new Uint8Array(await file.arrayBuffer());
    const b64 = toBase64(buf);
    try {
      preparing = true;
      prep = await SecurityService.PrepareKnownPlaintext(archiveB64, targetEntry, b64);
    } catch (err) {
      error = errMsg(err);
      prep = null;
    } finally {
      preparing = false;
    }
  }

  async function onPlaintext(e: Event) {
    const file = (e.target as HTMLInputElement).files?.[0];
    if (!file) return;
    plainName = file.name;
    plainSize = file.size;
    const buf = new Uint8Array(await file.arrayBuffer());
    plainB64 = toBase64(buf);
  }

  // Changing the target entry invalidates any previously prepared plaintext.
  $effect(() => {
    void targetEntry;
    prep = null;
    originalName = "";
  });

  function stopPolling() {
    if (timer) {
      clearInterval(timer);
      timer = null;
    }
  }

  async function poll() {
    if (!jobId) return;
    try {
      const p = await SecurityService.KnownPlaintextProgress(jobId);
      progress = p;
      if (p.finished) {
        running = false;
        stopPolling();
        if (p.found) {
          try {
            results = await SecurityService.KnownPlaintextResult(jobId);
          } catch (err) {
            error = errMsg(err);
          }
        }
      }
    } catch (err) {
      error = errMsg(err);
      running = false;
      stopPolling();
    }
  }

  async function start() {
    error = "";
    results = [];
    progress = null;
    if (!archiveB64) {
      error = "请先选择加密压缩包";
      return;
    }
    if (!targetEntry) {
      error = "请选择一个 ZipCrypto 加密条目作为攻击目标";
      return;
    }

    let plaintext: string;
    let off: number;
    if (plainMode === "auto") {
      if (!prep) {
        error = "请先提供该条目对应的原始文件";
        return;
      }
      plaintext = prep.plaintext;
      off = prep.offset;
    } else {
      if (!plainB64) {
        error = "请提供该条目压缩后的已知明文字节";
        return;
      }
      plaintext = plainB64;
      off = offset;
    }

    try {
      running = true;
      jobId = await SecurityService.StartKnownPlaintextAttack(
        archiveB64,
        targetEntry,
        plaintext,
        off,
      );
      stopPolling();
      timer = setInterval(poll, 300);
      poll();
    } catch (err) {
      error = errMsg(err);
      running = false;
    }
  }

  async function cancel() {
    if (!jobId) return;
    try {
      await SecurityService.CancelKnownPlaintextAttack(jobId);
    } catch (err) {
      error = errMsg(err);
    }
    running = false;
    stopPolling();
  }

  function download(entry: DecryptedEntry) {
    const bytes = fromBase64(entry.content);
    const ab = new ArrayBuffer(bytes.length);
    new Uint8Array(ab).set(bytes);
    const blob = new Blob([ab]);
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = entry.name.split("/").pop() || "decrypted";
    a.click();
    URL.revokeObjectURL(url);
  }

  function stopPwPolling() {
    if (pwTimer) {
      clearInterval(pwTimer);
      pwTimer = null;
    }
  }

  async function pollPw() {
    if (!pwJobId) return;
    try {
      const p = await SecurityService.PasswordRecoveryProgress(pwJobId);
      pwProgress = p;
      if (p.finished) {
        pwRunning = false;
        stopPwPolling();
      }
    } catch (err) {
      error = errMsg(err);
      pwRunning = false;
      stopPwPolling();
    }
  }

  async function startPw() {
    if (!jobId) return;
    error = "";
    pwProgress = null;
    try {
      pwRunning = true;
      pwJobId = await SecurityService.StartPasswordRecovery(
        jobId,
        new Charset({
          lower: pwLower,
          upper: pwUpper,
          digits: pwDigits,
          symbols: pwSymbols,
          custom: "",
        }),
        pwMinLen,
        pwMaxLen,
      );
      stopPwPolling();
      pwTimer = setInterval(pollPw, 300);
      pollPw();
    } catch (err) {
      error = errMsg(err);
      pwRunning = false;
    }
  }

  async function cancelPw() {
    if (!pwJobId) return;
    try {
      await SecurityService.CancelPasswordRecovery(pwJobId);
    } catch (err) {
      error = errMsg(err);
    }
    pwRunning = false;
    stopPwPolling();
  }

  function fmtSize(n: number): string {
    if (n < 1024) return `${n} B`;
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
    return `${(n / 1024 / 1024).toFixed(1)} MB`;
  }

  function fmtNum(n: number): string {
    return n.toLocaleString("en-US");
  }

  const encLabels: Record<string, string> = {
    none: "未加密",
    zipcrypto: "ZipCrypto（可攻击）",
    aes128: "AES-128（免疫）",
    aes192: "AES-192（免疫）",
    aes256: "AES-256（免疫）",
    aes: "AES（免疫）",
  };

  const phaseLabels: Record<string, string> = {
    reducing: "正在收窄候选密钥（Z 规约）",
    attacking: "正在还原内部密钥",
    decrypting: "正在用密钥解密压缩包",
    done: "完成",
  };

  let percent = $derived(
    progress && progress.total > 0
      ? Math.min(100, (progress.done / progress.total) * 100)
      : null,
  );
</script>

<div class="tool">
  <div class="notice">
    🔒 仅用于解开<strong>你本人拥有或已授权</strong>的压缩包。攻击只对传统 <strong>ZipCrypto</strong> 加密有效，对 AES 加密无效。全部计算在本地完成。
  </div>

  <section class="card">
    <h3>1 · 选择加密压缩包</h3>
    <input type="file" accept=".zip" onchange={onArchive} />
    {#if archiveName}
      <div class="filemeta">{archiveName} · {fmtSize(archiveSize)}</div>
    {/if}

    {#if info}
      {#if info.entries.length === 0}
        <div class="result warn">该压缩包没有可读取的条目</div>
      {:else}
        <table class="entries">
          <thead>
            <tr><th>条目</th><th>加密方式</th></tr>
          </thead>
          <tbody>
            {#each info.entries as e (e.name)}
              <tr>
                <td class="mono">{e.name}</td>
                <td class:vuln={e.encryption === ZipEncryption.ZipEncZipCrypto}>
                  {encLabels[e.encryption] ?? e.encryption}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>

        {#if !info.hasZipCrypto}
          <div class="result warn">
            ⚠️ 未发现 ZipCrypto 条目{info.hasAES ? "（全部为 AES 加密，已知明文攻击不适用）" : ""}。
          </div>
        {/if}
      {/if}
    {/if}
  </section>

  {#if info && info.hasZipCrypto}
    <section class="card">
      <h3>2 · 选择攻击目标条目</h3>
      <div class="row">
        <label for="kpa-entry">目标条目</label>
        <select id="kpa-entry" bind:value={targetEntry}>
          {#each zipCryptoEntries as e (e.name)}
            <option value={e.name}>{e.name}</option>
          {/each}
        </select>
      </div>
    </section>

    <section class="card">
      <h3>3 · 提供已知明文</h3>
      <div class="explain">
        <p>
          攻击的原理是：加密包里<strong>某一个文件</strong>的原始内容如果你也有一份，就能用它反推出密钥，从而解开<strong>整个包</strong>——不需要知道密码。
        </p>
        <p class="eg">
          举例：包里有个 <code>readme.txt</code>，而你手上正好有同一个 <code>readme.txt</code> 的原件；或者包里某张图片是从网上下载的公开文件，你能再下一份。把那个原件提供到这里即可。
        </p>
      </div>

      <div class="modes">
        <label class:active={plainMode === "auto"}>
          <input type="radio" bind:group={plainMode} value="auto" /> 自动（提供原始文件，推荐）
        </label>
        <label class:active={plainMode === "manual"}>
          <input type="radio" bind:group={plainMode} value="manual" /> 手动（提供压缩后字节）
        </label>
      </div>

      {#if plainMode === "auto"}
        <p class="hint">
          提供该条目<strong>解压后</strong>的原始文件即可。工具会用 CRC-32 校验它是否与包内是同一个文件，并在需要时自动按相同方式压缩对齐（无需你手动处理压缩）。
        </p>
        <input type="file" onchange={onOriginalFile} />
        {#if originalName}
          <div class="filemeta">{originalName} · {fmtSize(originalSize)}</div>
        {/if}
        {#if preparing}
          <div class="hint">正在校验并准备明文…</div>
        {/if}
        {#if prep}
          <div class="result" class:ok={prep.ready} class:warn={!prep.ready}>
            {prep.message}
          </div>
        {/if}
      {:else}
        <p class="hint">
          直接提供该条目<strong>压缩后</strong>的原始字节（Store 条目即原文件本身；Deflate 条目需以相同方式压缩后的数据）。明文至少 12 字节，越多越快。
        </p>
        <input type="file" onchange={onPlaintext} />
        {#if plainName}
          <div class="filemeta">{plainName} · {fmtSize(plainSize)}</div>
        {/if}
        <div class="row">
          <label for="kpa-offset">明文偏移</label>
          <input id="kpa-offset" type="number" min="0" bind:value={offset} />
          <span class="hint inline">已知明文在该条目数据中的起始位置，通常为 0</span>
        </div>
      {/if}

      <div class="actions">
        {#if running}
          <button class="danger" onclick={cancel}>停止</button>
        {:else}
          <button class="primary" onclick={start}>开始攻击</button>
        {/if}
      </div>
    </section>
  {/if}

  <ErrorBar message={error} />

  {#if progress}
    <section class="card">
      <h3>攻击进度</h3>
      <div class="phase">{phaseLabels[progress.phase] ?? progress.phase}</div>

      <div class="bar">
        <div
          class="bar-fill"
          class:indet={percent === null && !progress.finished}
          style={percent !== null ? `width:${percent}%` : "width:100%"}
        ></div>
      </div>

      <div class="stats">
        <div><span>进度</span><b>{fmtNum(progress.done)} / {fmtNum(progress.total)}</b></div>
        <div><span>耗时</span><b>{progress.elapsed.toFixed(1)}s</b></div>
        {#if progress.found}
          <div><span>已解密文件</span><b>{progress.fileCount}</b></div>
        {/if}
      </div>

      {#if progress.found}
        <div class="result ok">
          ✅ 已还原内部密钥：
          <code>{progress.keys}</code>
          <Copy text={progress.keys} />
        </div>
      {:else if progress.canceled}
        <div class="result warn">⏹ 已手动停止</div>
      {:else if progress.finished && progress.error}
        <div class="result warn">❌ {progress.error}</div>
      {/if}
    </section>
  {/if}

  {#if results.length > 0}
    <section class="card">
      <h3>解密结果（{results.length} 个文件）</h3>
      <ul class="files">
        {#each results as f (f.name)}
          <li>
            <span class="mono">{f.name}</span>
            <button class="link" onclick={() => download(f)}>下载</button>
          </li>
        {/each}
      </ul>
    </section>
  {/if}

  {#if progress && progress.found}
    <section class="card">
      <h3>（可选）反推明文密码</h3>
      <p class="hint">
        解密整包<strong>无需</strong>密码。如需还原出可复用的原始密码，可在此按字符集与长度范围搜索。长度越长耗时越久。
      </p>
      <div class="charset">
        <label><input type="checkbox" bind:checked={pwLower} /> 小写 a-z</label>
        <label><input type="checkbox" bind:checked={pwUpper} /> 大写 A-Z</label>
        <label><input type="checkbox" bind:checked={pwDigits} /> 数字 0-9</label>
        <label><input type="checkbox" bind:checked={pwSymbols} /> 符号</label>
      </div>
      <div class="lens">
        <label>最小长度 <input type="number" min="1" max="12" bind:value={pwMinLen} /></label>
        <label>最大长度 <input type="number" min="1" max="12" bind:value={pwMaxLen} /></label>
      </div>
      <div class="actions">
        {#if pwRunning}
          <button class="danger" onclick={cancelPw}>停止</button>
        {:else}
          <button class="primary" onclick={startPw}>反推密码</button>
        {/if}
      </div>

      {#if pwProgress}
        <div class="stats">
          <div><span>当前长度</span><b>{pwProgress.currentLength}</b></div>
          <div><span>耗时</span><b>{pwProgress.elapsed.toFixed(1)}s</b></div>
        </div>
        {#if pwProgress.passwords.length > 0}
          <div class="result ok">
            ✅ 找到密码：
            {#each pwProgress.passwords as pw (pw)}
              <code>{pw}</code>
              <Copy text={pw} />
            {/each}
          </div>
        {:else if pwProgress.finished && pwProgress.canceled}
          <div class="result warn">⏹ 已手动停止</div>
        {:else if pwProgress.finished}
          <div class="result warn">该字符集与长度范围内未找到密码，可扩大范围重试。</div>
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
  .row select {
    flex: 1;
    padding: 6px 8px;
    border-radius: var(--radius);
    border: 1px solid var(--color-border);
    background: var(--color-bg);
    color: var(--color-text);
  }
  .row input[type="number"] {
    width: 90px;
    padding: 6px 8px;
    border-radius: var(--radius);
    border: 1px solid var(--color-border);
    background: var(--color-bg);
    color: var(--color-text);
  }
  .hint {
    margin: 0;
    font-size: 12px;
    opacity: 0.7;
    line-height: 1.5;
  }
  .hint.inline {
    flex: 1;
  }
  .explain {
    background: var(--color-bg);
    border-radius: var(--radius);
    padding: 10px 12px;
    font-size: 13px;
    line-height: 1.6;
  }
  .explain p {
    margin: 0;
  }
  .explain .eg {
    margin-top: 6px;
    opacity: 0.8;
  }
  .explain code {
    font-family: var(--font-mono);
    background: var(--color-surface-2, rgba(127, 127, 127, 0.12));
    padding: 1px 5px;
    border-radius: 4px;
  }
  .modes {
    display: flex;
    gap: 10px;
    flex-wrap: wrap;
  }
  .modes label {
    flex: 1;
    min-width: 180px;
    text-align: center;
    padding: 8px;
    border-radius: var(--radius);
    border: 1px solid var(--color-border);
    cursor: pointer;
    font-size: 13px;
  }
  .modes label.active {
    border-color: var(--color-accent);
    color: var(--color-accent);
    background: var(--color-accent-weak);
  }
  .entries {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
  }
  .entries th,
  .entries td {
    text-align: left;
    padding: 6px 8px;
    border-bottom: 1px solid var(--color-border);
  }
  .entries th {
    font-size: 12px;
    opacity: 0.6;
  }
  .mono {
    font-family: var(--font-mono);
    font-size: 12px;
  }
  .vuln {
    color: var(--color-accent);
    font-weight: 600;
  }
  .actions {
    display: flex;
    gap: 8px;
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
  .danger {
    background: var(--color-error, #e5484d);
    color: #fff;
    border: none;
    padding: 8px 16px;
    border-radius: var(--radius);
    cursor: pointer;
  }
  .phase {
    font-size: 13px;
    opacity: 0.85;
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
    grid-template-columns: repeat(auto-fit, minmax(100px, 1fr));
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
  .result {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 12px;
    border-radius: var(--radius);
    font-size: 14px;
    flex-wrap: wrap;
  }
  .result code {
    font-family: var(--font-mono);
    background: var(--color-bg);
    padding: 1px 6px;
    border-radius: 4px;
  }
  .result.ok {
    background: var(--color-accent-weak);
    color: var(--color-accent);
  }
  .result.warn {
    background: var(--color-error-weak, #fdeaea);
    color: var(--color-error, #e5484d);
  }
  .files {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .files li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    padding: 6px 8px;
    border-radius: var(--radius);
    background: var(--color-bg);
  }
  .link {
    background: transparent;
    border: 1px solid var(--color-accent);
    color: var(--color-accent);
    border-radius: var(--radius);
    padding: 3px 12px;
    cursor: pointer;
    font-size: 13px;
  }
</style>
