<script lang="ts">
  import { DevOpsService } from "../../../bindings/github.com/nic/devtoolkit";
  import { errMsg } from "../err";
  import Copy from "../components/Copy.svelte";
  import ErrorBar from "../components/ErrorBar.svelte";

  type Triad = { read: boolean; write: boolean; exec: boolean };
  const empty = (): Triad => ({ read: false, write: false, exec: false });

  let owner = $state(empty());
  let group = $state(empty());
  let other = $state(empty());
  let numeric = $state("000");
  let symbolic = $state("---------");
  let error = $state("");

  async function fromPerms() {
    error = "";
    try {
      const r = await DevOpsService.ChmodFromPerms({ owner, group, other });
      numeric = r.numeric;
      symbolic = r.symbolic;
    } catch (e) {
      error = errMsg(e);
    }
  }

  async function fromNumeric() {
    error = "";
    try {
      const r = await DevOpsService.ChmodFromNumeric(numeric);
      numeric = r.numeric;
      symbolic = r.symbolic;
      owner = r.perms.owner;
      group = r.perms.group;
      other = r.perms.other;
    } catch (e) {
      error = errMsg(e);
    }
  }

  fromPerms();
</script>

<div class="tool">
  <div class="grid">
    {#each [["所有者", owner], ["群组", group], ["其他", other]] as [title, t] (title)}
      <div class="triad">
        <div class="lbl">{title}</div>
        <label><input type="checkbox" bind:checked={(t as Triad).read} onchange={fromPerms} /> r 读</label>
        <label><input type="checkbox" bind:checked={(t as Triad).write} onchange={fromPerms} /> w 写</label>
        <label><input type="checkbox" bind:checked={(t as Triad).exec} onchange={fromPerms} /> x 执行</label>
      </div>
    {/each}
  </div>

  <ErrorBar message={error} />

  <div class="results">
    <div class="field">
      <label for="chmod-num">数字权限</label>
      <div class="line">
        <input id="chmod-num" type="text" bind:value={numeric} oninput={fromNumeric} maxlength="3" />
        <Copy text={numeric} />
      </div>
    </div>
    <div class="field">
      <span class="lbl">符号权限</span>
      <div class="line">
        <code class="sym">{symbolic}</code>
        <Copy text={symbolic} />
      </div>
    </div>
  </div>
</div>

<style>
  .tool {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 12px;
  }
  .triad {
    background: var(--color-surface);
    border-radius: var(--radius);
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .triad label {
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--color-text);
  }
  .results {
    display: flex;
    gap: 24px;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .line {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .field input {
    width: 100px;
    font-family: var(--font-mono);
  }
  .sym {
    font-family: var(--font-mono);
    font-size: 16px;
    background: var(--color-surface);
    padding: 8px 12px;
    border-radius: var(--radius);
    letter-spacing: 2px;
  }
</style>
