<script lang="ts">
  import { DevOpsService } from "../../../bindings/github.com/nic/devtoolkit";
  import { errMsg } from "../err";
  import Copy from "../components/Copy.svelte";
  import ErrorBar from "../components/ErrorBar.svelte";

  type DiffEntry = { path: string; state: string };

  let raw = $state("");
  let entries: string[] = $state([]);
  let error = $state("");

  // compare mode
  let compareB = $state("");
  let diff: DiffEntry[] = $state([]);

  let joined = $derived(entries.join("\n"));

  async function split() {
    error = "";
    diff = [];
    try {
      entries = await DevOpsService.SplitPath(raw);
    } catch (e) {
      error = errMsg(e);
    }
  }

  async function dedup() {
    error = "";
    try {
      entries = await DevOpsService.DedupPath(entries);
    } catch (e) {
      error = errMsg(e);
    }
  }

  async function compare() {
    error = "";
    try {
      diff = await DevOpsService.ComparePath(raw, compareB);
    } catch (e) {
      error = errMsg(e);
    }
  }

  const stateLabel: Record<string, string> = {
    left: "仅左侧",
    right: "仅右侧",
    both: "两侧共有",
  };
</script>

<div class="tool">
  <label for="ep-in">PATH 变量</label>
  <textarea id="ep-in" bind:value={raw} placeholder="粘贴 PATH 变量…"></textarea>
  <div class="actions">
    <button class="primary" onclick={split}>拆分</button>
    <button onclick={dedup} disabled={!entries.length}>去重</button>
  </div>

  <ErrorBar message={error} />

  {#if entries.length}
    <div class="out-head">
      <span class="lbl">路径条目（{entries.length}）</span>
      <Copy text={joined} label="复制重组" />
    </div>
    <ol class="entries">
      {#each entries as e (e)}
        <li>{e}</li>
      {/each}
    </ol>
  {/if}

  <details class="compare">
    <summary>与另一个 PATH 比较</summary>
    <label for="ep-b">第二个 PATH</label>
    <textarea id="ep-b" bind:value={compareB} placeholder="粘贴用于比较的 PATH…"></textarea>
    <button class="primary" onclick={compare}>比较</button>
    {#if diff.length}
      <ul class="diff">
        {#each diff as d (d.path)}
          <li class={d.state}>
            <span class="badge">{stateLabel[d.state]}</span>{d.path}
          </li>
        {/each}
      </ul>
    {/if}
  </details>
</div>

<style>
  .tool {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .actions {
    display: flex;
    gap: 8px;
  }
  .out-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .entries,
  .diff {
    margin: 0;
    font-family: var(--font-mono);
    font-size: 13px;
  }
  .entries {
    padding-left: 22px;
  }
  .diff {
    list-style: none;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .diff li {
    padding: 4px 8px;
    border-radius: var(--radius);
  }
  .diff li.left {
    background: var(--color-error-weak);
  }
  .diff li.right {
    background: var(--color-accent-weak);
  }
  .diff li.both {
    background: var(--color-surface);
  }
  .badge {
    display: inline-block;
    min-width: 64px;
    margin-right: 8px;
    font-size: 11px;
    color: var(--color-text-weak);
  }
  .compare {
    background: var(--color-surface);
    border-radius: var(--radius);
    padding: 12px;
  }
  .compare summary {
    cursor: pointer;
    color: var(--color-accent);
  }
  .compare textarea {
    margin-top: 8px;
  }
  .compare button {
    margin-top: 8px;
  }
</style>
