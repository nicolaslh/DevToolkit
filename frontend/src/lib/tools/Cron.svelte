<script lang="ts">
  import { DevOpsService } from "../../../bindings/github.com/nic/devtoolkit";
  import { errMsg } from "../err";
  import Copy from "../components/Copy.svelte";
  import ErrorBar from "../components/ErrorBar.svelte";

  let minute = $state("*");
  let hour = $state("*");
  let day = $state("*");
  let month = $state("*");
  let weekday = $state("*");

  let expr = $state("* * * * *");
  let description = $state("");
  let nextRuns: string[] = $state([]);
  let error = $state("");

  async function build() {
    expr = await DevOpsService.BuildCron({ minute, hour, day, month, weekday });
    await parse();
  }

  async function parse() {
    error = "";
    description = "";
    nextRuns = [];
    try {
      const info = await DevOpsService.ParseCron(expr);
      description = info.description;
      nextRuns = info.nextRuns;
    } catch (e) {
      error = errMsg(e);
    }
  }

  build();
</script>

<div class="tool">
  <div class="fields">
    <div><label for="cr-min">分 (0-59)</label><input id="cr-min" type="text" bind:value={minute} oninput={build} /></div>
    <div><label for="cr-hr">时 (0-23)</label><input id="cr-hr" type="text" bind:value={hour} oninput={build} /></div>
    <div><label for="cr-day">日 (1-31)</label><input id="cr-day" type="text" bind:value={day} oninput={build} /></div>
    <div><label for="cr-mon">月 (1-12)</label><input id="cr-mon" type="text" bind:value={month} oninput={build} /></div>
    <div><label for="cr-wd">周 (0-6)</label><input id="cr-wd" type="text" bind:value={weekday} oninput={build} /></div>
  </div>

  <div class="expr-row">
    <input
      class="expr"
      type="text"
      bind:value={expr}
      oninput={parse}
      aria-label="Cron 表达式"
    />
    <Copy text={expr} />
  </div>

  <ErrorBar message={error} />

  {#if description}
    <div class="desc">{description}</div>
  {/if}

  {#if nextRuns.length}
    <div>
      <span class="lbl">接下来 5 次执行</span>
      <ul class="runs">
        {#each nextRuns as r (r)}
          <li>{r}</li>
        {/each}
      </ul>
    </div>
  {/if}
</div>

<style>
  .tool {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .fields {
    display: grid;
    grid-template-columns: repeat(5, 1fr);
    gap: 10px;
  }
  .fields div {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .fields input {
    font-family: var(--font-mono);
    text-align: center;
  }
  .expr-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .expr {
    font-family: var(--font-mono);
    font-size: 16px;
  }
  .desc {
    background: var(--color-accent-weak);
    color: var(--color-accent);
    border-radius: var(--radius);
    padding: 8px 12px;
  }
  .runs {
    margin: 6px 0 0;
    padding-left: 18px;
    font-family: var(--font-mono);
    font-size: 13px;
    color: var(--color-text);
  }
  .runs li {
    margin: 2px 0;
  }
</style>
