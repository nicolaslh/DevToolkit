<script lang="ts">
  import { TextService } from "../../../bindings/github.com/nic/devtoolkit";
  import { errMsg } from "../err";
  import ErrorBar from "../components/ErrorBar.svelte";

  type Preset = { name: string; pattern: string };
  type Match = { text: string; start: number; end: number; groups: string[] };

  let pattern = $state("");
  let text = $state("");
  let global = $state(true);
  let ignoreCase = $state(false);
  let multiline = $state(false);
  let matches: Match[] = $state([]);
  let count = $state(0);
  let error = $state("");
  let ran = $state(false);
  let presets: Preset[] = $state([]);

  TextService.RegexLibrary().then((p: any) => (presets = p));

  async function run() {
    error = "";
    ran = true;
    matches = [];
    try {
      const r = await TextService.TestRegex(pattern, text, {
        global,
        ignoreCase,
        multiline,
      });
      matches = r.matches;
      count = r.count;
    } catch (e) {
      error = errMsg(e);
    }
  }

  function applyPreset(e: Event) {
    const v = (e.target as HTMLSelectElement).value;
    if (v) {
      pattern = v;
      run();
    }
  }

  // Build highlighted segments from match offsets.
  let segments = $derived.by(() => {
    if (!matches.length) return [{ text, hit: false }];
    const segs: { text: string; hit: boolean }[] = [];
    let pos = 0;
    for (const m of matches) {
      if (m.start > pos) segs.push({ text: text.slice(pos, m.start), hit: false });
      segs.push({ text: text.slice(m.start, m.end), hit: true });
      pos = m.end;
    }
    if (pos < text.length) segs.push({ text: text.slice(pos), hit: false });
    return segs;
  });
</script>

<div class="tool">
  <div class="row">
    <label for="rx-lib">常用正则库</label>
    <select id="rx-lib" onchange={applyPreset}>
      <option value="">— 选择套用 —</option>
      {#each presets as p (p.name)}
        <option value={p.pattern}>{p.name}</option>
      {/each}
    </select>
  </div>

  <label for="rx-pat">正则表达式（RE2 语法）</label>
  <input id="rx-pat" type="text" bind:value={pattern} placeholder="例如 \d+" />

  <div class="flags">
    <label><input type="checkbox" bind:checked={global} /> 全局 g</label>
    <label><input type="checkbox" bind:checked={ignoreCase} /> 忽略大小写 i</label>
    <label><input type="checkbox" bind:checked={multiline} /> 多行 m</label>
  </div>

  <label for="rx-text">待测试文本</label>
  <textarea id="rx-text" bind:value={text} placeholder="在此粘贴待匹配文本…"></textarea>

  <div class="actions">
    <button class="primary" onclick={run}>匹配</button>
  </div>

  <ErrorBar message={error} />

  {#if ran && !error}
    {#if count === 0}
      <div class="nores">无匹配结果</div>
    {:else}
      <span class="lbl">高亮（{count} 处匹配）</span>
      <div class="highlight">
        {#each segments as s (s)}
          {#if s.hit}<mark>{s.text}</mark>{:else}{s.text}{/if}
        {/each}
      </div>
      <span class="lbl">匹配项</span>
      <ul class="list">
        {#each matches as m, i (i)}
          <li>
            <span class="idx">@{m.start}</span>
            <code>{m.text}</code>
            {#if m.groups.length}
              <span class="groups">组: {m.groups.join(" | ")}</span>
            {/if}
          </li>
        {/each}
      </ul>
    {/if}
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
  #rx-pat {
    font-family: var(--font-mono);
  }
  .flags {
    display: flex;
    gap: 16px;
  }
  .flags label {
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--color-text);
  }
  .actions {
    display: flex;
    gap: 8px;
  }
  .nores {
    color: var(--color-text-weak);
    padding: 8px 12px;
    background: var(--color-surface);
    border-radius: var(--radius);
  }
  .highlight {
    background: var(--color-surface);
    border-radius: var(--radius);
    padding: 12px;
    font-family: var(--font-mono);
    font-size: 13px;
    white-space: pre-wrap;
    word-break: break-all;
  }
  mark {
    background: var(--color-accent-weak);
    color: var(--color-accent);
    border-radius: 3px;
    padding: 0 1px;
  }
  .list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .list li {
    background: var(--color-surface);
    border-radius: var(--radius);
    padding: 6px 10px;
    font-family: var(--font-mono);
    font-size: 13px;
  }
  .idx {
    color: var(--color-text-weak);
    margin-right: 8px;
  }
  .groups {
    color: var(--color-text-weak);
    margin-left: 10px;
  }
</style>
