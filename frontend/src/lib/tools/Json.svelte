<script lang="ts">
  import Copy from "../components/Copy.svelte";
  import ErrorBar from "../components/ErrorBar.svelte";
  import JsonTree from "../components/JsonTree.svelte";

  type ParseResult =
    | { ok: true; value: unknown; usedHexEscapes: boolean }
    | { ok: false; message: string };

  function errorMessage(e: unknown) {
    return e instanceof Error ? e.message : String(e);
  }

  function decodeHexEscapes(raw: string) {
    return raw.replace(/(?:\\x[0-9a-fA-F]{2})+/g, (run) => {
      const bytes = [...run.matchAll(/\\x([0-9a-fA-F]{2})/g)].map((m) =>
        Number.parseInt(m[1], 16),
      );

      try {
        return new TextDecoder("utf-8", { fatal: true }).decode(new Uint8Array(bytes));
      } catch {
        return String.fromCharCode(...bytes);
      }
    });
  }

  function parseJsonInput(raw: string): ParseResult {
    try {
      return { ok: true, value: JSON.parse(raw), usedHexEscapes: false };
    } catch (e) {
      if (!/\\x[0-9a-fA-F]{2}/.test(raw)) {
        return { ok: false, message: errorMessage(e) };
      }
    }

    try {
      return { ok: true, value: JSON.parse(decodeHexEscapes(raw)), usedHexEscapes: true };
    } catch (e) {
      return { ok: false, message: errorMessage(e) };
    }
  }

  let input = $state("");
  let error = $state("");
  let indent = $state(2);
  let view = $state<"tree" | "text">("tree");

  // Parsed value (undefined when invalid). Drives the tree view.
  let parseResult = $derived.by<ParseResult | undefined>(() => {
    if (input.trim() === "") return undefined;
    return parseJsonInput(input);
  });
  let parsed = $derived(parseResult?.ok ? parseResult.value : undefined);
  let usedHexEscapes = $derived(parseResult?.ok === true ? parseResult.usedHexEscapes : false);

  // Validate on every change so the error bar stays in sync.
  $effect(() => {
    if (input.trim() === "") {
      error = "";
      return;
    }
    error = parseResult?.ok ? "" : (parseResult?.message ?? "");
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
    const result = parseJsonInput(input);
    if (result.ok && typeof result.value === "string") {
      input = result.value;
      error = "";
    } else if (result.ok) {
      error = "去转义需要输入一个 JSON 字符串";
    } else {
      error = result.message;
    }
  }

  function clear() {
    input = "";
    error = "";
  }

  let valid = $derived(input.trim() !== "" && parseResult?.ok === true);
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
  {#if usedHexEscapes}
    <div class="parse-hint">已自动解码 \xNN 十六进制转义。</div>
  {/if}

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
        <div class="result-actions">
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
          <button class="compact-action" onclick={minify} disabled={!valid}>压缩</button>
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
  .parse-hint {
    margin-top: -6px;
    font-size: 12px;
    color: var(--color-text-weak);
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

  .result-actions {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
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
  .compact-action {
    padding: 4px 10px;
    font-size: 13px;
  }
</style>
