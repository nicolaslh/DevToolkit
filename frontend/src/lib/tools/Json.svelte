<script lang="ts">
  import Copy from "../components/Copy.svelte";
  import ErrorBar from "../components/ErrorBar.svelte";
  import JsonTree from "../components/JsonTree.svelte";

  let input = $state(`{
  "name": "DevToolkit",
  "version": 3,
  "offline": true,
  "tags": ["json", "format", "parse"],
  "author": { "name": "nic", "url": "https://github.com" }
}`);
  let error = $state("");
  let indent = $state(2);
  let view = $state<"tree" | "text">("tree");

  // Parsed value (null when invalid). Drives the tree view.
  let parsed = $derived.by<unknown>(() => {
    if (input.trim() === "") return undefined;
    try {
      return JSON.parse(input);
    } catch {
      return undefined;
    }
  });

  // Validate on every change so the error bar stays in sync.
  $effect(() => {
    if (input.trim() === "") {
      error = "";
      return;
    }
    try {
      JSON.parse(input);
      error = "";
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  });

  // Pretty-printed text used by the text view and copy button.
  let pretty = $derived.by(() => {
    if (parsed === undefined) return "";
    try {
      return JSON.stringify(parsed, null, indent);
    } catch {
      return "";
    }
  });

  function format() {
    if (parsed === undefined) return;
    input = JSON.stringify(parsed, null, indent);
  }

  function minify() {
    if (parsed === undefined) return;
    input = JSON.stringify(parsed);
  }

  function escape() {
    input = JSON.stringify(input);
  }

  function unescape() {
    try {
      const v = JSON.parse(input);
      if (typeof v === "string") {
        input = v;
        error = "";
      } else {
        error = "去转义需要输入一个 JSON 字符串";
      }
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }

  function clear() {
    input = "";
    error = "";
  }

  let valid = $derived(input.trim() !== "" && parsed !== undefined);
</script>

<div class="tool">
  <div class="toolbar">
    <button class="primary" onclick={format} disabled={!valid}>格式化</button>
    <button onclick={minify} disabled={!valid}>压缩</button>
    <button onclick={escape} disabled={input.trim() === ""}>转义</button>
    <button onclick={unescape} disabled={input.trim() === ""}>去转义</button>
    <button onclick={clear} disabled={input.trim() === ""}>清空</button>

    <label class="indent">
      缩进
      <select bind:value={indent}>
        <option value={2}>2 空格</option>
        <option value={4}>4 空格</option>
        <option value={0}>压缩</option>
      </select>
    </label>

    <div class="spacer"></div>

    <div class="status" class:ok={valid} class:bad={input.trim() !== "" && !valid}>
      {input.trim() === "" ? "待输入" : valid ? "✓ 合法 JSON" : "✗ 非法 JSON"}
    </div>
  </div>

  <ErrorBar message={error} />

  <div class="panes">
    <div class="pane">
      <div class="pane-head">
        <span class="lbl">输入</span>
      </div>
      <textarea
        class="editor"
        bind:value={input}
        spellcheck="false"
        placeholder="在此粘贴 JSON…"
        aria-label="JSON 输入"
      ></textarea>
    </div>

    <div class="pane">
      <div class="pane-head">
        <div class="view-switch" role="tablist" aria-label="结果视图">
          <button
            class="seg"
            class:active={view === "tree"}
            role="tab"
            aria-selected={view === "tree"}
            onclick={() => (view = "tree")}
          >
            树形
          </button>
          <button
            class="seg"
            class:active={view === "text"}
            role="tab"
            aria-selected={view === "text"}
            onclick={() => (view = "text")}
          >
            文本
          </button>
        </div>
        <Copy text={pretty} />
      </div>

      <div class="result">
        {#if !valid}
          <div class="placeholder">解析结果将显示在这里</div>
        {:else if view === "tree"}
          <JsonTree data={parsed} />
        {:else}
          <pre class="text-out">{pretty}</pre>
        {/if}
      </div>
    </div>
  </div>
</div>

<style>
  .tool {
    display: flex;
    flex-direction: column;
    gap: 12px;
    height: 100%;
  }
  .toolbar {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }
  .indent {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 13px;
    color: var(--color-text-weak);
  }
  .indent select {
    width: auto;
  }
  .spacer {
    flex: 1;
  }
  .status {
    font-size: 13px;
    color: var(--color-text-weak);
  }
  .status.ok {
    color: var(--color-success);
  }
  .status.bad {
    color: var(--color-error);
  }

  .panes {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
    flex: 1;
    min-height: 360px;
  }
  .pane {
    display: flex;
    flex-direction: column;
    gap: 6px;
    min-width: 0;
  }
  .pane-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    min-height: 30px;
  }
  .editor {
    flex: 1;
    min-height: 320px;
    resize: none;
  }
  .result {
    flex: 1;
    min-height: 320px;
    overflow: auto;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: 12px;
  }
  .placeholder {
    color: var(--color-text-weak);
    font-size: 13px;
  }
  .text-out {
    margin: 0;
    font-family: var(--font-mono);
    font-size: 13px;
    line-height: 1.6;
    white-space: pre;
  }

  .view-switch {
    display: inline-flex;
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    overflow: hidden;
  }
  .seg {
    background: transparent;
    border: none;
    border-radius: 0;
    padding: 4px 12px;
    font-size: 13px;
  }
  .seg.active {
    background: var(--color-accent);
    color: #fff;
  }
</style>
