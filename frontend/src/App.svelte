<script lang="ts">
  import { tools, type ToolMeta } from "./lib/tools";

  // Explicit category order + icon, mirroring json.cn's top category bar.
  const categories: { name: string; icon: string }[] = [
    { name: "JSON 工具", icon: "🧩" },
    { name: "文本与代码", icon: "📝" },
    { name: "网络与接口", icon: "🌐" },
    { name: "系统运维", icon: "⚙️" },
    { name: "前端与视觉", icon: "🎨" },
    { name: "安全与身份验证", icon: "🔒" },
    { name: "编码与加解密", icon: "🔣" },
  ];

  // Group tools by category, preserving the order above.
  const grouped = categories
    .map((c) => ({
      ...c,
      items: tools.filter((t) => t.category === c.name),
    }))
    .filter((g) => g.items.length > 0);

  let activeId = $state(tools[0].id);
  let active = $derived(tools.find((t) => t.id === activeId) ?? tools[0]);
  let activeCategory = $derived(active.category);

  // Which dropdown is currently open (by category name), null = none.
  let openCategory = $state<string | null>(null);
  // Close timer lets the pointer travel from trigger to panel without flicker.
  let closeTimer: ReturnType<typeof setTimeout> | undefined;

  function openMenu(name: string) {
    clearTimeout(closeTimer);
    openCategory = name;
    searchOpen = false;
  }
  function scheduleClose() {
    clearTimeout(closeTimer);
    closeTimer = setTimeout(() => (openCategory = null), 120);
  }
  function toggleCategory(name: string) {
    clearTimeout(closeTimer);
    openCategory = openCategory === name ? null : name;
    searchOpen = false;
  }

  // Search across all tools; results shown in a dropdown panel.
  let search = $state("");
  let searchOpen = $state(false);
  let searchResults = $derived.by(() => {
    const q = search.trim().toLowerCase();
    if (q === "") return [] as ToolMeta[];
    return tools.filter(
      (t) =>
        t.name.toLowerCase().includes(q) ||
        t.description.toLowerCase().includes(q),
    );
  });

  function selectTool(id: string) {
    activeId = id;
    openCategory = null;
    searchOpen = false;
    search = "";
  }

  // Close any open menu when clicking outside the navbar.
  function handleWindowClick(e: MouseEvent) {
    const target = e.target as HTMLElement;
    if (!target.closest(".navbar")) {
      openCategory = null;
      searchOpen = false;
    }
  }
  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      openCategory = null;
      searchOpen = false;
    }
  }

  // Theme toggle (flat design supports light/dark).
  let dark = $state(false);
  function toggleTheme() {
    dark = !dark;
    document.documentElement.setAttribute("data-theme", dark ? "dark" : "light");
  }

  const ActiveComponent = $derived(active.component);
</script>

<svelte:window onclick={handleWindowClick} onkeydown={handleKeydown} />

<div class="layout">
  <header class="navbar">
    <div class="brand">
      <span class="logo">🛠️</span>
      <span class="title">DevToolkit</span>
    </div>

    <nav class="menus">
      {#each grouped as g (g.name)}
        <div
          class="menu"
          role="presentation"
          onmouseenter={() => openMenu(g.name)}
          onmouseleave={scheduleClose}
        >
          <button
            class="menu-trigger"
            class:active={g.name === activeCategory}
            class:open={openCategory === g.name}
            onclick={() => toggleCategory(g.name)}
            aria-haspopup="true"
            aria-expanded={openCategory === g.name}
          >
            <span class="menu-icon">{g.icon}</span>
            <span class="menu-label">{g.name}</span>
            <span class="caret" aria-hidden="true">▾</span>
          </button>

          {#if openCategory === g.name}
            <div class="dropdown" role="menu">
              {#each g.items as t (t.id)}
                <button
                  class="dropdown-item"
                  class:selected={t.id === activeId}
                  role="menuitem"
                  onclick={() => selectTool(t.id)}
                >
                  <span class="dd-icon">{t.icon}</span>
                  <span class="dd-text">
                    <span class="dd-name">{t.name}</span>
                    <span class="dd-desc">{t.description}</span>
                  </span>
                </button>
              {/each}
            </div>
          {/if}
        </div>
      {/each}
    </nav>

    <div class="bar-right">
      <div class="search-wrap">
        <span class="search-icon" aria-hidden="true">🔍</span>
        <input
          class="search"
          type="text"
          bind:value={search}
          oninput={() => {
            searchOpen = true;
            openCategory = null;
          }}
          onfocus={() => (searchOpen = true)}
          placeholder="搜索工具…"
          aria-label="搜索工具"
        />
        {#if searchOpen && searchResults.length > 0}
          <div class="dropdown search-dropdown" role="menu">
            {#each searchResults as t (t.id)}
              <button
                class="dropdown-item"
                class:selected={t.id === activeId}
                role="menuitem"
                onclick={() => selectTool(t.id)}
              >
                <span class="dd-icon">{t.icon}</span>
                <span class="dd-text">
                  <span class="dd-name">{t.name}</span>
                  <span class="dd-desc">{t.description}</span>
                </span>
              </button>
            {/each}
          </div>
        {:else if searchOpen && search.trim() !== ""}
          <div class="dropdown search-dropdown">
            <div class="empty">无匹配工具</div>
          </div>
        {/if}
      </div>

      <button class="theme" onclick={toggleTheme} title="切换主题" aria-label="切换主题">
        {dark ? "☀️" : "🌙"}
      </button>
    </div>
  </header>

  <main class="content">
    <header class="content-head">
      <h2><span class="head-icon">{active.icon}</span>{active.name}</h2>
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
    display: flex;
    flex-direction: column;
    height: 100vh;
  }

  /* ---- Top navigation bar (json.cn style) ---- */
  .navbar {
    display: flex;
    align-items: center;
    gap: 20px;
    min-height: 54px;
    padding: 6px 16px;
    background: var(--color-surface);
    border-bottom: 1px solid var(--color-border);
    flex-shrink: 0;
  }
  .brand {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-shrink: 0;
  }
  .brand .logo {
    font-size: 18px;
  }
  .brand .title {
    font-weight: 600;
    font-size: 16px;
    white-space: nowrap;
  }

  .menus {
    display: flex;
    align-items: center;
    gap: 4px;
    flex: 1;
    min-width: 0;
    flex-wrap: wrap;
    overflow: visible;
  }
  .menu {
    position: relative;
  }
  .menu-trigger {
    display: flex;
    align-items: center;
    gap: 6px;
    background: transparent;
    border: none;
    border-radius: 18px;
    padding: 7px 14px;
    white-space: nowrap;
    color: var(--color-text);
  }
  .menu-trigger:hover {
    background: var(--color-surface-2);
  }
  .menu-trigger.active {
    background: var(--color-accent);
    color: #fff;
  }
  .menu-trigger.active:hover {
    background: var(--color-accent);
    opacity: 0.92;
  }
  .menu-icon {
    font-size: 14px;
    line-height: 1;
  }
  .menu-label {
    font-size: 14px;
  }
  .caret {
    font-size: 10px;
    opacity: 0.7;
    transition: transform 0.15s ease;
  }
  .menu-trigger.open .caret {
    transform: rotate(180deg);
  }

  /* ---- Dropdown panels ---- */
  .dropdown {
    position: absolute;
    top: calc(100% + 8px);
    left: 0;
    min-width: 260px;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: 10px;
    box-shadow: 0 8px 28px rgba(0, 0, 0, 0.14);
    padding: 6px;
    z-index: 100;
    animation: dropIn 0.14s ease;
  }
  @keyframes dropIn {
    from {
      opacity: 0;
      transform: translateY(-4px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }
  .dropdown-item {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    width: 100%;
    text-align: left;
    background: transparent;
    border: none;
    border-radius: 8px;
    padding: 9px 10px;
  }
  .dropdown-item:hover {
    background: var(--color-surface-2);
  }
  .dropdown-item.selected {
    background: var(--color-accent-weak);
  }
  .dropdown-item.selected .dd-name {
    color: var(--color-accent);
    font-weight: 600;
  }
  .dd-icon {
    font-size: 16px;
    line-height: 1.3;
    flex-shrink: 0;
  }
  .dd-text {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .dd-name {
    font-size: 14px;
  }
  .dd-desc {
    font-size: 12px;
    color: var(--color-text-weak);
  }

  /* ---- Right side: search + theme ---- */
  .bar-right {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-shrink: 0;
  }
  .search-wrap {
    position: relative;
    display: flex;
    align-items: center;
  }
  .search-icon {
    position: absolute;
    left: 10px;
    font-size: 12px;
    opacity: 0.6;
    pointer-events: none;
  }
  .search {
    width: 190px;
    padding: 6px 10px 6px 30px;
    border-radius: 18px;
  }
  .search-dropdown {
    left: auto;
    right: 0;
    min-width: 280px;
    max-height: 380px;
    overflow-y: auto;
  }
  .theme {
    padding: 6px 10px;
    background: transparent;
    border-radius: 18px;
  }
  .empty {
    color: var(--color-text-weak);
    font-size: 13px;
    padding: 8px 10px;
  }

  /* ---- Main content ---- */
  .content {
    display: flex;
    flex-direction: column;
    flex: 1;
    overflow: hidden;
  }
  .content-head {
    padding: 20px 24px 12px;
    border-bottom: 1px solid var(--color-border);
  }
  .content-head h2 {
    margin: 0;
    font-size: 18px;
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .content-head .head-icon {
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
    flex: 1;
  }
</style>
