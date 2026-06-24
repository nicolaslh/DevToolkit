<script lang="ts">
  import { NetworkService } from "../../../bindings/github.com/nic/devtoolkit";
  import { errMsg } from "../err";
  import Copy from "../components/Copy.svelte";
  import ErrorBar from "../components/ErrorBar.svelte";

  interface QP {
    key: string;
    value: string;
  }

  let raw = $state("");
  let protocol = $state("");
  let host = $state("");
  let port = $state("");
  let path = $state("");
  let query: QP[] = $state([]);
  let rebuilt = $state("");
  let error = $state("");
  let parsed = $state(false);

  async function parse() {
    error = "";
    try {
      const p = await NetworkService.ParseURL(raw);
      protocol = p.protocol;
      host = p.host;
      port = p.port;
      path = p.path;
      query = (p.query ?? []).map((q: any) => ({ key: q.key, value: q.value }));
      parsed = true;
      await rebuild();
    } catch (e) {
      parsed = false;
      error = errMsg(e);
    }
  }

  async function rebuild() {
    error = "";
    try {
      rebuilt = await NetworkService.BuildURL({
        protocol,
        host,
        port,
        path,
        query,
      });
    } catch (e) {
      error = errMsg(e);
    }
  }
</script>

<div class="tool">
  <label for="url-in">URL</label>
  <div class="row">
    <input id="url-in" type="text" bind:value={raw} placeholder="https://example.com:8443/path?a=1&b=2" />
    <button class="primary" onclick={parse}>解析</button>
  </div>

  <ErrorBar message={error} />

  {#if parsed}
    <div class="fields">
      <div><span class="lbl">协议</span><input type="text" bind:value={protocol} oninput={rebuild} /></div>
      <div><span class="lbl">主机</span><input type="text" bind:value={host} oninput={rebuild} /></div>
      <div><span class="lbl">端口</span><input type="text" bind:value={port} oninput={rebuild} /></div>
      <div><span class="lbl">路径</span><input type="text" bind:value={path} oninput={rebuild} /></div>
    </div>

    {#if query.length}
      <span class="lbl">查询参数</span>
      <div class="params">
        {#each query as q (q)}
          <div class="param">
            <input type="text" bind:value={q.key} oninput={rebuild} placeholder="键" />
            <input type="text" bind:value={q.value} oninput={rebuild} placeholder="值" />
            <Copy text={q.value} label="复制值" />
          </div>
        {/each}
      </div>
    {/if}

    <div class="out-head">
      <span class="lbl">重组后的 URL</span>
      <Copy text={rebuilt} />
    </div>
    <input type="text" readonly value={rebuilt} />
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
    gap: 8px;
  }
  .row button {
    white-space: nowrap;
  }
  .fields {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 10px;
  }
  .fields div {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .params {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .param {
    display: grid;
    grid-template-columns: 1fr 1fr auto;
    gap: 8px;
    align-items: center;
  }
  .out-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
</style>
