<script lang="ts">
  import { tools, type ToolMeta } from "./lib/tools";

  let search = $state("");
  let activeId = $state(tools[0].id);

  // Case-insensitive filter on name or description (R19.3/19.4/19.5).
  let filtered = $derived(
    search.trim() === ""
      ? tools
      : tools.filter((t) => {
          const q = search.trim().toLowerCase();
          return (
            t.name.toLowerCase().includes(q) ||
            t.description.toLowerCase().includes(q)
          );
        })
  );

  let active = $derived(
    tools.find((t) => t.id === activeId) ?? tools[0]
  );

  // Group filtered tools by category for the sidebar.
  let grouped = $derived.by(() => {
    const map = new Map<string, ToolMeta[]>();
    for (const t of filtered) {
      const arr = map.get(t.category) ?? [];
      arr.push(t);
      map.set(t.category, arr);
    }
    return [...map.entries()];
  });

  // Theme toggle (flat design supports light/dark).
  let dark = $state(false);
  function toggleTheme() {
    dark = !dark;
    document.documentElement.setAttribute("data-theme", dark ? "dark" : "light");
  }

  const ActiveComponent = $derived(active.component);
</script>

<div class="layout">
  <aside class="sidebar">
    <div class="brand">
      <span class="logo">🛠️</span>
      <span class="title">DevToolkit</span>
      <button class="theme" onclick={toggleTheme} title="切换主题" aria-label="切换主题">
        {dark ? "☀️" : "🌙"}
      </button>
    </div>

    <input
      class="search"
      type="text"
      bind:value={search}
      placeholder="搜索工具…"
      aria-label="搜索工具"
    />

    <nav class="nav">
      {#if filtered.length === 0}
        <div class="empty">无匹配工具</div>
      {:else}
        {#each grouped as [category, items] (category)}
          <div class="group">
            <div class="group-title">{category}</div>
            {#each items as t (t.id)}
              <button
                class="nav-item"
                class:active={t.id === activeId}
                onclick={() => (activeId = t.id)}
              >
                <div class="nav-name">{t.name}</div>
                <div class="nav-desc">{t.description}</div>
              </button>
            {/each}
          </div>
        {/each}
      {/if}
    </nav>
  </aside>

  <main class="content">
    <header class="content-head">
      <h2>{active.name}</h2>
      <p>{active.description}</p>
    </header>
    <div class="content-body">
      {#key active.id}
        <ActiveComponent />
      {/key}
    </div>
  </main>
</div>

<style>
  .layout {
    display: grid;
    grid-template-columns: 280px 1fr;
    height: 100vh;
  }
  .sidebar {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 16px;
    background: var(--color-surface);
    border-right: 1px solid var(--color-border);
    overflow-y: auto;
  }
  .brand {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .brand .title {
    font-weight: 600;
    font-size: 16px;
  }
  .brand .theme {
    margin-left: auto;
    padding: 4px 8px;
    background: transparent;
  }
  .nav {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .group-title {
    font-size: 12px;
    color: var(--color-text-weak);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    margin-bottom: 4px;
  }
  .nav-item {
    display: block;
    width: 100%;
    text-align: left;
    background: transparent;
    border: none;
    border-radius: var(--radius);
    padding: 8px 10px;
    margin-bottom: 2px;
  }
  .nav-item:hover {
    background: var(--color-surface-2);
  }
  .nav-item.active {
    background: var(--color-accent-weak);
  }
  .nav-item.active .nav-name {
    color: var(--color-accent);
    font-weight: 600;
  }
  .nav-name {
    font-size: 14px;
  }
  .nav-desc {
    font-size: 12px;
    color: var(--color-text-weak);
  }
  .empty {
    color: var(--color-text-weak);
    font-size: 13px;
    padding: 8px 10px;
  }
  .content {
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .content-head {
    padding: 20px 24px 12px;
    border-bottom: 1px solid var(--color-border);
  }
  .content-head h2 {
    margin: 0;
    font-size: 18px;
  }
  .content-head p {
    margin: 4px 0 0;
    color: var(--color-text-weak);
    font-size: 13px;
  }
  .content-body {
    padding: 20px 24px;
    overflow-y: auto;
  }
</style>
