<script lang="ts">
  import { TextService } from "../../../bindings/github.com/nic/devtoolkit";
  import { errMsg } from "../err";
  import ErrorBar from "../components/ErrorBar.svelte";

  type Segment = { type: string; text: string };

  let left = $state("");
  let right = $state("");
  let mode = $state("line");
  let jsonMode = $state(false);
  let leftSegs: Segment[] = $state([]);
  let rightSegs: Segment[] = $state([]);
  let identical = $state(false);
  let error = $state("");
  let ran = $state(false);

  async function run() {
    error = "";
    ran = true;
    leftSegs = [];
    rightSegs = [];
    identical = false;
    try {
      const r = await TextService.Diff(left, right, { mode, jsonMode });
      leftSegs = r.left;
      rightSegs = r.right;
      identical = r.identical;
    } catch (e) {
      error = errMsg(e);
    }
  }
</script>

<div class="tool">
  <div class="inputs">
    <div>
      <label for="d-left">左侧</label>
      <textarea id="d-left" bind:value={left} placeholder="原始内容…"></textarea>
    </div>
    <div>
      <label for="d-right">右侧</label>
      <textarea id="d-right" bind:value={right} placeholder="对比内容…"></textarea>
    </div>
  </div>

  <div class="controls">
    <label>比对模式
      <select bind:value={mode}>
        <option value="line">按行</option>
        <option value="char">按字符</option>
      </select>
    </label>
    <label><input type="checkbox" bind:checked={jsonMode} /> JSON 结构比对</label>
    <button class="primary" onclick={run}>比对</button>
  </div>

  <ErrorBar message={error} />

  {#if ran && !error}
    {#if identical}
      <div class="same">无差异</div>
    {:else}
      <div class="panes">
        <pre class="pane">{#each leftSegs as s (s)}<span class={s.type}>{s.text}</span>{/each}</pre>
        <pre class="pane">{#each rightSegs as s (s)}<span class={s.type}>{s.text}</span>{/each}</pre>
      </div>
    {/if}
  {/if}
</div>

<style>
  .tool {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .inputs {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
  }
  .inputs div {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .controls {
    display: flex;
    align-items: center;
    gap: 16px;
  }
  .controls label {
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--color-text);
  }
  .controls select {
    width: auto;
  }
  .same {
    background: var(--color-surface);
    color: var(--color-text-weak);
    border-radius: var(--radius);
    padding: 8px 12px;
  }
  .panes {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
  }
  .pane {
    background: var(--color-surface);
    border-radius: var(--radius);
    padding: 12px;
    margin: 0;
    font-family: var(--font-mono);
    font-size: 13px;
    white-space: pre-wrap;
    word-break: break-all;
    overflow-x: auto;
  }
  .pane :global(.delete) {
    background: var(--color-error-weak);
    color: var(--color-error);
  }
  .pane :global(.insert) {
    background: rgba(31, 157, 87, 0.16);
    color: var(--color-success);
  }
</style>
