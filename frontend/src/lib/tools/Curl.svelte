<script lang="ts">
  import { NetworkService } from "../../../bindings/github.com/nic/devtoolkit";
  import { errMsg } from "../err";
  import Copy from "../components/Copy.svelte";
  import ErrorBar from "../components/ErrorBar.svelte";

  let cmd = $state("");
  let target = $state("python");
  let code = $state("");
  let error = $state("");

  const langs = [
    { v: "python", label: "Python (requests)" },
    { v: "javascript", label: "JavaScript (fetch)" },
    { v: "go", label: "Go (net/http)" },
    { v: "java", label: "Java (HttpClient)" },
  ];

  async function convert() {
    error = "";
    code = "";
    try {
      code = await NetworkService.ConvertCurl(cmd, target);
    } catch (e) {
      error = errMsg(e);
    }
  }
</script>

<div class="tool">
  <label for="cl-in">cURL 命令</label>
  <textarea id="cl-in" bind:value={cmd} placeholder={`curl -X POST https://api.example.com -H "Content-Type: application/json" -d '{"a":1}'`}></textarea>

  <div class="row">
    <label for="cl-lang">目标语言</label>
    <select id="cl-lang" bind:value={target} onchange={convert}>
      {#each langs as l (l.v)}
        <option value={l.v}>{l.label}</option>
      {/each}
    </select>
    <button class="primary" onclick={convert}>转换</button>
  </div>

  <ErrorBar message={error} />

  {#if code}
    <div class="out-head">
      <span class="lbl">生成代码</span>
      <Copy text={code} />
    </div>
    <pre>{code}</pre>
  {/if}
</div>

<style>
  .tool {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .row select {
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
    overflow-x: auto;
    font-family: var(--font-mono);
    font-size: 13px;
    white-space: pre-wrap;
    word-break: break-all;
  }
</style>
