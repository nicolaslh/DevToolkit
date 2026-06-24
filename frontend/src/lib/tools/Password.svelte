<script lang="ts">
  import { SecurityService } from "../../../bindings/github.com/nic/devtoolkit";
  import { errMsg } from "../err";
  import Copy from "../components/Copy.svelte";
  import ErrorBar from "../components/ErrorBar.svelte";

  let upper = $state(true);
  let lower = $state(true);
  let digits = $state(true);
  let symbol = $state(false);
  let length = $state(16);
  let count = $state(5);
  let results: string[] = $state([]);
  let error = $state("");

  async function generate() {
    error = "";
    results = [];
    try {
      results = await SecurityService.GeneratePasswords({
        upper,
        lower,
        digits,
        symbol,
        length,
        count,
      });
    } catch (e) {
      error = errMsg(e);
    }
  }

  // Bcrypt
  let bcryptIn = $state("");
  let bcryptOut = $state("");
  let bcryptErr = $state("");
  async function doBcrypt() {
    bcryptErr = "";
    try {
      bcryptOut = await SecurityService.BcryptHash(bcryptIn);
    } catch (e) {
      bcryptErr = errMsg(e);
    }
  }

  let allText = $derived(results.join("\n"));
</script>

<div class="tool">
  <section class="card">
    <h3>强密码生成</h3>
    <div class="checks">
      <label><input type="checkbox" bind:checked={upper} /> 大写字母</label>
      <label><input type="checkbox" bind:checked={lower} /> 小写字母</label>
      <label><input type="checkbox" bind:checked={digits} /> 数字</label>
      <label><input type="checkbox" bind:checked={symbol} /> 特殊符号</label>
    </div>
    <div class="nums">
      <div>
        <label for="pw-len">长度（4–128）</label>
        <input id="pw-len" type="number" min="4" max="128" bind:value={length} />
      </div>
      <div>
        <label for="pw-cnt">数量（1–1000）</label>
        <input id="pw-cnt" type="number" min="1" max="1000" bind:value={count} />
      </div>
      <button class="primary" onclick={generate}>生成</button>
    </div>
    <ErrorBar message={error} />
    {#if results.length}
      <div class="out-head">
        <span class="lbl">结果（{results.length} 个）</span>
        <Copy text={allText} label="复制全部" />
      </div>
      <pre>{allText}</pre>
    {/if}
  </section>

  <section class="card">
    <h3>Bcrypt 哈希</h3>
    <label for="bc-in">明文（1–72 字节）</label>
    <input id="bc-in" type="text" bind:value={bcryptIn} placeholder="待哈希的明文…" />
    <div class="actions">
      <button class="primary" onclick={doBcrypt}>生成 Bcrypt</button>
    </div>
    <ErrorBar message={bcryptErr} />
    {#if bcryptOut}
      <div class="out-head">
        <span class="lbl">Bcrypt</span>
        <Copy text={bcryptOut} />
      </div>
      <input type="text" readonly value={bcryptOut} />
    {/if}
  </section>
</div>

<style>
  .tool {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .card {
    display: flex;
    flex-direction: column;
    gap: 10px;
    background: var(--color-surface);
    border-radius: var(--radius);
    padding: 14px;
  }
  .card h3 {
    margin: 0;
    font-size: 15px;
  }
  .checks {
    display: flex;
    flex-wrap: wrap;
    gap: 16px;
  }
  .checks label {
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--color-text);
  }
  .nums {
    display: flex;
    align-items: flex-end;
    gap: 12px;
  }
  .nums div {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .nums input {
    width: 120px;
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
  pre {
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: 12px;
    margin: 0;
    font-family: var(--font-mono);
    font-size: 13px;
    white-space: pre-wrap;
    word-break: break-all;
  }
</style>
