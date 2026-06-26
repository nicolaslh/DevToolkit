<script lang="ts">
  import { NetworkService } from "../../../bindings/github.com/nic/devtoolkit";
  import type { Result } from "../../../bindings/github.com/nic/devtoolkit/internal/pkg/dnsx/models";
  import { errMsg } from "../err";
  import Copy from "../components/Copy.svelte";
  import ErrorBar from "../components/ErrorBar.svelte";

  let host = $state("");
  let result = $state<Result | null>(null);
  let error = $state("");
  let loading = $state(false);

  async function resolve() {
    error = "";
    result = null;
    if (!host.trim()) {
      error = "请输入要解析的域名或主机";
      return;
    }
    loading = true;
    try {
      result = await NetworkService.ResolveDNS(host);
    } catch (e) {
      error = errMsg(e);
    } finally {
      loading = false;
    }
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === "Enter") resolve();
  }

  // Record sections rendered in display order; only non-empty ones are shown.
  const sections = $derived(
    result
      ? [
          { type: "A", values: result.a ?? [] },
          { type: "AAAA", values: result.aaaa ?? [] },
          { type: "CNAME", values: result.cname ? [result.cname] : [] },
          { type: "MX", values: result.mx ?? [] },
          { type: "NS", values: result.ns ?? [] },
          { type: "TXT", values: result.txt ?? [] },
        ].filter((s) => s.values.length > 0)
      : [],
  );

  const allText = $derived(
    sections
      .map((s) => s.values.map((v: string) => `${s.type}\t${v}`).join("\n"))
      .join("\n"),
  );
</script>

<div class="tool">
  <label for="dns-in">域名 / 主机</label>
  <div class="row">
    <input
      id="dns-in"
      bind:value={host}
      onkeydown={onKey}
      placeholder="example.com"
      autocomplete="off"
      spellcheck="false"
    />
    <button class="primary" onclick={resolve} disabled={loading}>
      {loading ? "解析中…" : "解析"}
    </button>
  </div>

  <ErrorBar message={error} />

  {#if result}
    <div class="out-head">
      <span class="lbl">{result.host} 的 DNS 记录</span>
      <Copy text={allText} />
    </div>

    <table>
      <thead>
        <tr>
          <th>类型</th>
          <th>记录值</th>
        </tr>
      </thead>
      <tbody>
        {#each sections as s (s.type)}
          {#each s.values as v, i (v)}
            <tr>
              {#if i === 0}
                <td class="type" rowspan={s.values.length}>{s.type}</td>
              {/if}
              <td class="val">{v}</td>
            </tr>
          {/each}
        {/each}
      </tbody>
    </table>
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
  .row input {
    flex: 1;
  }
  .out-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
  }
  th,
  td {
    border: 1px solid var(--color-border, #e5e7eb);
    padding: 8px 10px;
    text-align: left;
    vertical-align: top;
  }
  th {
    background: var(--color-surface);
    font-weight: 600;
  }
  td.type {
    font-weight: 600;
    white-space: nowrap;
    width: 90px;
  }
  td.val {
    font-family: var(--font-mono);
    word-break: break-all;
  }
</style>
