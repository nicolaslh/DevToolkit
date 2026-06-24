<script lang="ts">
  import { SecurityService } from "../../../bindings/github.com/nic/devtoolkit";
  import { errMsg } from "../err";
  import ErrorBar from "../components/ErrorBar.svelte";

  let token = $state("");
  let header = $state("");
  let payload = $state("");
  let expHuman = $state("");
  let expired: boolean | null = $state(null);
  let error = $state("");

  async function decode() {
    error = "";
    header = payload = expHuman = "";
    expired = null;
    if (!token.trim()) {
      error = "JWT 为空：请粘贴一个 JWT";
      return;
    }
    try {
      const r = await SecurityService.DecodeJWT(token);
      header = r.header;
      payload = r.payload;
      expHuman = r.expHuman;
      expired = r.expired ?? null;
    } catch (e) {
      error = errMsg(e);
    }
  }
</script>

<div class="tool">
  <label for="jwt-in">JWT</label>
  <textarea id="jwt-in" bind:value={token} placeholder="粘贴 JWT（header.payload.signature）…"></textarea>
  <div class="actions">
    <button class="primary" onclick={decode}>解码</button>
  </div>

  <ErrorBar message={error} />

  {#if expHuman}
    <div class="exp" class:expired={expired}>
      过期时间 exp：{expHuman}
      <span class="tag">{expired ? "已过期" : "未过期"}</span>
    </div>
  {/if}

  {#if header}
    <div class="cols">
      <div>
        <span class="lbl">Header</span>
        <pre>{header}</pre>
      </div>
      <div>
        <span class="lbl">Payload</span>
        <pre>{payload}</pre>
      </div>
    </div>
  {/if}
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
  .exp {
    background: var(--color-accent-weak);
    color: var(--color-accent);
    border-radius: var(--radius);
    padding: 8px 12px;
    font-family: var(--font-mono);
    font-size: 13px;
  }
  .exp.expired {
    background: var(--color-error-weak);
    color: var(--color-error);
  }
  .tag {
    margin-left: 8px;
    font-weight: 600;
  }
  .cols {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
  }
  pre {
    background: var(--color-surface);
    border-radius: var(--radius);
    padding: 12px;
    margin: 4px 0 0;
    font-family: var(--font-mono);
    font-size: 13px;
    white-space: pre-wrap;
    word-break: break-all;
    overflow-x: auto;
  }
</style>
