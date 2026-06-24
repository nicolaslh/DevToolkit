<script lang="ts">
  import { TextService } from "../../../bindings/github.com/nic/devtoolkit";
  import { errMsg } from "../err";
  import Copy from "../components/Copy.svelte";
  import ErrorBar from "../components/ErrorBar.svelte";

  let kind = $state("name");
  let count = $state(10);
  let results: string[] = $state([]);
  let error = $state("");

  const kinds = [
    { v: "name", label: "姓名" },
    { v: "address", label: "地址" },
    { v: "phone", label: "手机号" },
    { v: "email", label: "邮箱" },
    { v: "bankcard", label: "银行卡号" },
    { v: "lorem", label: "Lorem 占位文本" },
  ];

  let joined = $derived(results.join("\n"));

  async function generate() {
    error = "";
    results = [];
    try {
      results = await TextService.GenerateMock(kind, count);
    } catch (e) {
      error = errMsg(e);
    }
  }
</script>

<div class="tool">
  <div class="row">
    <label for="mk-kind">数据类型</label>
    <select id="mk-kind" bind:value={kind}>
      {#each kinds as k (k.v)}
        <option value={k.v}>{k.label}</option>
      {/each}
    </select>
    <label for="mk-cnt">数量</label>
    <input id="mk-cnt" type="number" min="1" max="10000" bind:value={count} />
    <button class="primary" onclick={generate}>生成</button>
  </div>

  <ErrorBar message={error} />

  {#if results.length}
    <div class="out-head">
      <span class="lbl">结果（{results.length} 条）</span>
      <Copy text={joined} label="复制全部" />
    </div>
    <pre>{joined}</pre>
  {/if}
</div>

<style>
  .tool {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .row select,
  .row input {
    width: auto;
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
    max-height: 420px;
    overflow-y: auto;
    font-family: var(--font-mono);
    font-size: 13px;
    white-space: pre-wrap;
    word-break: break-all;
  }
</style>
