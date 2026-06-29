<script lang="ts">
  import { FrontendService } from "../../../bindings/github.com/nic/devtoolkit";
  import { errMsg } from "../err";
  import Copy from "../components/Copy.svelte";
  import ErrorBar from "../components/ErrorBar.svelte";

  let picking = $state(false);
  let error = $state("");
  let needPermission = $state(false);

  // Live color under the cursor (updated while picking).
  let liveHex = $state("");
  let liveX = $state(0);
  let liveY = $state(0);

  // Locked color (after confirming), in all four formats.
  let hex = $state("");
  let rgb = $state("");
  let rgba = $state("");
  let hsl = $state("");

  const REGION = 19; // odd side length sampled around the cursor for the loupe
  const LOUPE_SIZE = 152; // on-screen px

  let loupe: HTMLCanvasElement | null = $state(null);
  let timer: ReturnType<typeof setInterval> | null = null;
  let inFlight = false;

  async function poll() {
    if (inFlight) return; // avoid overlapping calls if a tick is slow
    inFlight = true;
    try {
      const p = await FrontendService.PickColorAtCursor(REGION);
      liveHex = p.hex;
      liveX = p.x;
      liveY = p.y;
      drawLoupe(p.dataUri, p.region);
      error = "";
    } catch (e) {
      error = errMsg(e);
      stop(); // typically a permission error; stop hammering the backend
    } finally {
      inFlight = false;
    }
  }

  function drawLoupe(dataUri: string, region: number) {
    if (!loupe) return;
    const ctx = loupe.getContext("2d");
    if (!ctx) return;
    const img = new Image();
    img.onload = () => {
      ctx.imageSmoothingEnabled = false;
      ctx.clearRect(0, 0, LOUPE_SIZE, LOUPE_SIZE);
      ctx.drawImage(img, 0, 0, region, region, 0, 0, LOUPE_SIZE, LOUPE_SIZE);
      // Crosshair on the center pixel.
      const cell = LOUPE_SIZE / region;
      const c = Math.floor(region / 2);
      ctx.strokeStyle = "rgba(0,0,0,0.6)";
      ctx.lineWidth = 1;
      ctx.strokeRect(c * cell, c * cell, cell, cell);
      ctx.strokeStyle = "rgba(255,255,255,0.95)";
      ctx.strokeRect(c * cell + 1, c * cell + 1, cell - 2, cell - 2);
    };
    img.src = dataUri;
  }

  function start() {
    if (picking) return;
    error = "";
    startWithAccessCheck();
  }

  async function startWithAccessCheck() {
    try {
      const ok = await FrontendService.EnsureScreenAccess();
      if (!ok) {
        needPermission = true;
        return;
      }
    } catch (e) {
      error = errMsg(e);
      return;
    }
    needPermission = false;
    picking = true;
    poll();
    timer = setInterval(poll, 120);
    window.addEventListener("keydown", onKey, true);
  }

  function stop() {
    picking = false;
    if (timer) {
      clearInterval(timer);
      timer = null;
    }
    window.removeEventListener("keydown", onKey, true);
  }

  function onKey(e: KeyboardEvent) {
    if (!picking) return;
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      lock();
    } else if (e.key === "Escape") {
      e.preventDefault();
      stop();
    }
  }

  async function lock() {
    const value = liveHex;
    stop();
    if (!value) return;
    try {
      const set = await FrontendService.ConvertColor(value, "hex");
      hex = set.hex;
      rgb = set.rgb;
      rgba = set.rgba;
      hsl = set.hsl;
    } catch (e) {
      error = errMsg(e);
    }
  }

  // Clean up if the component unmounts mid-pick.
  $effect(() => () => stop());
</script>

<div class="tool">
  <div class="bar">
    {#if picking}
      <button class="primary" onclick={lock}>锁定颜色</button>
      <button class="ghost" onclick={stop}>停止</button>
      <span class="hint">移动鼠标到屏幕任意位置，按 Enter / 空格 或点「锁定颜色」取色，Esc 取消</span>
    {:else}
      <button class="primary" onclick={start}>开始取色</button>
      <span class="hint">点击后移动鼠标到屏幕任意位置即可实时取色</span>
    {/if}
  </div>

  <ErrorBar message={error} />

  {#if needPermission}
    <div class="perm">
      <strong>需要「屏幕录制」权限</strong>
      <p>
        没有该权限时，取色只能读取到桌面壁纸，无法读取其他应用窗口的颜色。
        请到「系统设置 › 隐私与安全性 › 屏幕录制」中勾选 DevToolkit，然后<strong>重启应用</strong>再试。
      </p>
    </div>
  {/if}

  {#if picking}
    <div class="live">
      <canvas
        bind:this={loupe}
        class="loupe"
        width={LOUPE_SIZE}
        height={LOUPE_SIZE}
        aria-label="光标处放大预览"
      ></canvas>
      <div class="live-info">
        <div class="swatch" style="background:{liveHex || '#ffffff'}"></div>
        <div class="live-text">
          <strong>{liveHex || "—"}</strong>
          <span class="coords">({liveX}, {liveY})</span>
        </div>
      </div>
    </div>
  {/if}

  {#if hex}
    <div class="result">
      <div class="result-swatch" style="background:{hex}"></div>
      <div class="grid">
        <div class="field">
          <label for="cp-hex">HEX</label>
          <div class="line">
            <input id="cp-hex" type="text" value={hex} readonly />
            <Copy text={hex} />
          </div>
        </div>
        <div class="field">
          <label for="cp-rgb">RGB</label>
          <div class="line">
            <input id="cp-rgb" type="text" value={rgb} readonly />
            <Copy text={rgb} />
          </div>
        </div>
        <div class="field">
          <label for="cp-rgba">RGBA</label>
          <div class="line">
            <input id="cp-rgba" type="text" value={rgba} readonly />
            <Copy text={rgba} />
          </div>
        </div>
        <div class="field">
          <label for="cp-hsl">HSL</label>
          <div class="line">
            <input id="cp-hsl" type="text" value={hsl} readonly />
            <Copy text={hsl} />
          </div>
        </div>
      </div>
    </div>
  {:else if !picking && !error}
    <p class="empty">
      点击「开始取色」后，把鼠标移到屏幕上任意位置，实时读取该处颜色。
      <br />
      macOS 首次使用需在「系统设置 › 隐私与安全性 › 屏幕录制」中授予权限。
    </p>
  {/if}
</div>

<style>
  .tool {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .bar {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-wrap: wrap;
  }
  button.primary {
    padding: 8px 14px;
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    background: var(--color-accent, #2f6feb);
    color: #fff;
    cursor: pointer;
  }
  button.ghost {
    padding: 8px 14px;
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    background: transparent;
    cursor: pointer;
  }
  .hint {
    color: var(--color-text-muted, #6b7280);
    font-size: 13px;
  }
  .empty {
    color: var(--color-text-muted, #6b7280);
    line-height: 1.7;
    margin: 0;
  }
  .perm {
    border: 1px solid var(--color-border);
    border-left: 3px solid #e0a106;
    border-radius: var(--radius);
    padding: 10px 14px;
    background: rgba(224, 161, 6, 0.08);
  }
  .perm p {
    margin: 6px 0 0;
    line-height: 1.7;
    color: var(--color-text-muted, #6b7280);
  }
  .live {
    display: flex;
    align-items: center;
    gap: 16px;
  }
  .loupe {
    width: 152px;
    height: 152px;
    border-radius: 50%;
    border: 2px solid var(--color-border);
    box-shadow: 0 2px 10px rgba(0, 0, 0, 0.25);
    image-rendering: pixelated;
    background: #fff;
  }
  .live-info {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .swatch {
    width: 56px;
    height: 48px;
    border-radius: var(--radius);
    border: 1px solid var(--color-border);
  }
  .live-text {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .live-text strong {
    font-family: var(--font-mono, monospace);
    font-size: 18px;
  }
  .coords {
    color: var(--color-text-muted, #6b7280);
    font-size: 12px;
  }
  .result {
    display: flex;
    gap: 14px;
    align-items: stretch;
  }
  .result-swatch {
    width: 72px;
    border-radius: var(--radius);
    border: 1px solid var(--color-border);
  }
  .grid {
    flex: 1;
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .line {
    display: flex;
    gap: 8px;
    align-items: center;
  }
</style>
