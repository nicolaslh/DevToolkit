<script lang="ts">
  // Recursive, collapsible JSON tree node (json.cn style visual parse).
  import Self from "./JsonTree.svelte";

  let {
    data,
    name = null,
    isIndex = false,
  }: { data: unknown; name?: string | null; isIndex?: boolean } = $props();

  let collapsed = $state(false);

  const type = $derived(
    data === null
      ? "null"
      : Array.isArray(data)
        ? "array"
        : typeof data,
  );
  const isContainer = $derived(type === "array" || type === "object");

  const entries = $derived.by(() => {
    if (type === "array")
      return (data as unknown[]).map((v, i) => [String(i), v] as const);
    if (type === "object")
      return Object.entries(data as Record<string, unknown>);
    return [] as (readonly [string, unknown])[];
  });

  function primitive(v: unknown, t: string): string {
    if (t === "string") return `"${v}"`;
    if (t === "null") return "null";
    return String(v);
  }
</script>

<div class="node">
  {#if isContainer}
    <div class="line">
      <button
        class="toggle"
        onclick={() => (collapsed = !collapsed)}
        aria-label={collapsed ? "展开" : "折叠"}
      >
        {collapsed ? "▶" : "▼"}
      </button>
      {#if name !== null}
        <span class="key" class:index={isIndex}>{name}</span><span class="colon">:</span>
      {/if}
      <span class="bracket">{type === "array" ? "[" : "{"}</span>
      {#if collapsed}
        <span class="summary">{entries.length} {type === "array" ? "项" : "字段"}</span>
        <span class="bracket">{type === "array" ? "]" : "}"}</span>
      {/if}
    </div>

    {#if !collapsed}
      <div class="children">
        {#each entries as [k, v] (k)}
          <Self data={v} name={k} isIndex={type === "array"} />
        {/each}
      </div>
      <div class="bracket-close">{type === "array" ? "]" : "}"}</div>
    {/if}
  {:else}
    <div class="line leaf">
      {#if name !== null}
        <span class="key" class:index={isIndex}>{name}</span><span class="colon">:</span>
      {/if}
      <span class="val {type}">{primitive(data, type)}</span>
    </div>
  {/if}
</div>

<style>
  .node {
    font-family: var(--font-mono);
    font-size: 13px;
    line-height: 1.7;
  }
  .line {
    display: flex;
    align-items: baseline;
    gap: 4px;
    white-space: nowrap;
  }
  .toggle {
    background: transparent;
    border: none;
    padding: 0;
    width: 16px;
    font-size: 10px;
    color: var(--color-text-weak);
    cursor: pointer;
    flex-shrink: 0;
  }
  .toggle:hover {
    color: var(--color-accent);
    background: transparent;
  }
  .leaf {
    padding-left: 16px;
  }
  .children {
    margin-left: 8px;
    padding-left: 8px;
    border-left: 1px dashed var(--color-border);
  }
  .bracket-close {
    padding-left: 16px;
    color: var(--color-text-weak);
  }
  .key {
    color: var(--color-accent);
  }
  .key.index {
    color: var(--color-text-weak);
  }
  .colon {
    color: var(--color-text-weak);
  }
  .bracket {
    color: var(--color-text-weak);
  }
  .summary {
    color: var(--color-text-weak);
    font-style: italic;
    opacity: 0.8;
  }
  .val.string {
    color: var(--color-success);
  }
  .val.number {
    color: var(--color-warning);
  }
  .val.boolean {
    color: var(--color-accent);
  }
  .val.null {
    color: var(--color-text-weak);
  }
</style>
