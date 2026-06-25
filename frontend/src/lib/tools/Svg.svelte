<script lang="ts">
  import { FrontendService } from "../../../bindings/github.com/nic/devtoolkit";
  import { errMsg } from "../err";
  import Copy from "../components/Copy.svelte";
  import ErrorBar from "../components/ErrorBar.svelte";

  let input = $state("");
  let url = $state("");
  let optimized = $state("");
  let beforeBytes = $state(0);
  let afterBytes = $state(0);
  let reductionPct = $state("");
  let error = $state("");
  let ran = $state(false);
  let loading = $state(false);

  // Live preview source: optimized result if available, else raw input.
  // Basic sanitization before rendering via {@html} to avoid script execution
  // inside the webview (defense-in-depth; the design calls for sanitized preview).
  function sanitizeSvg(svg: string): string {
    return svg
      .replace(/<script[\s\S]*?<\/script>/gi, "")
      .replace(/\son\w+\s*=\s*"[^"]*"/gi, "")
      .replace(/\son\w+\s*=\s*'[^']*'/gi, "")
      .replace(/(href|xlink:href)\s*=\s*("|\')\s*javascript:[^"']*\2/gi, "");
  }
  let previewSrc = $derived(sanitizeSvg(optimized || input));

  async function fetchFromUrl() {
    if (!url.trim()) {
      error = "请输入 SVG 网络地址";
      return;
    }

    loading = true;
    error = "";
    ran = false;
    optimized = "";

    try {
      const response = await fetch(url.trim());
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }
      const contentType = response.headers.get("content-type") || "";
      const text = await response.text();

      // 验证是否为 SVG 内容
      if (!contentType.includes("svg") && !text.trim().match(/<svg[\s>]/i)) {
        throw new Error("返回的内容不是 SVG");
      }

      input = text;
    } catch (e) {
      error = `加载失败: ${errMsg(e)}`;
    } finally {
      loading = false;
    }
  }

  async function optimize() {
    error = "";
    ran = true;
    optimized = "";
    try {
      const r = await FrontendService.OptimizeSVG(input);
      optimized = r.optimized;
      beforeBytes = r.beforeBytes;
      afterBytes = r.afterBytes;
      reductionPct = r.reductionPct;
    } catch (e) {
      error = errMsg(e);
    }
  }
</script>

<div class="tool">
  <div class="cols">
    <div class="left">
      <label for="svg-url">网络地址</label>
      <div class="url-row">
        <input
          id="svg-url"
          type="url"
          bind:value={url}
          placeholder="输入 SVG 文件的 URL..."
          disabled={loading}
        />
        <button class="secondary" onclick={fetchFromUrl} disabled={loading}>
          {loading ? "加载中..." : "加载"}
        </button>
      </div>

      <label for="svg-in">SVG 代码</label>
      <textarea id="svg-in" bind:value={input} placeholder="粘贴 SVG 代码或从网络地址加载…"></textarea>
      <button class="primary" onclick={optimize}>优化</button>
      <ErrorBar message={error} />
      {#if ran && !error}
        <div class="stats">
          优化前 {beforeBytes} 字节 → 优化后 {afterBytes} 字节
          <span class="pct">减少 {reductionPct}%</span>
        </div>
        <div class="out-head">
          <span class="lbl">优化后代码</span>
          <Copy text={optimized} />
        </div>
        <pre>{optimized}</pre>
      {/if}
    </div>
    <div class="right">
      <span class="lbl">实时预览</span>
      <!-- eslint-disable-next-line svelte/no-at-html-tags -->
      <div class="preview">{@html previewSrc}</div>
    </div>
  </div>
</div>

<style>
  .tool {
    display: flex;
    flex-direction: column;
  }
  .cols {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px;
  }
  .left {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .right {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .url-row {
    display: flex;
    gap: 8px;
  }
  .url-row input {
    flex: 1;
  }
  .stats {
    background: var(--color-surface);
    border-radius: var(--radius);
    padding: 8px 12px;
    font-size: 13px;
  }
  .pct {
    color: var(--color-success);
    font-weight: 600;
    margin-left: 8px;
  }
  .out-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  pre {
    background: var(--color-surface);
    border-radius: var(--radius);
    padding: 12px;
    margin: 0;
    max-height: 240px;
    overflow: auto;
    font-family: var(--font-mono);
    font-size: 12px;
    white-space: pre-wrap;
    word-break: break-all;
  }
  .preview {
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: 12px;
    min-height: 240px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: #fff;
  }
  .preview :global(svg) {
    max-width: 100%;
    max-height: 360px;
  }
</style>
