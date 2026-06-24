<script lang="ts">
  // Flat copy button with success/failure feedback (横切：复制反馈).
  let { text, label = "复制" }: { text: string; label?: string } = $props();
  let state: "idle" | "ok" | "err" = $state("idle");

  async function copy() {
    if (!text) return;
    try {
      await navigator.clipboard.writeText(text);
      state = "ok";
    } catch {
      state = "err";
    }
    setTimeout(() => (state = "idle"), 1500);
  }
</script>

<button class="copy" onclick={copy} disabled={!text} title="复制到剪贴板">
  {state === "ok" ? "已复制" : state === "err" ? "复制失败" : label}
</button>

<style>
  .copy {
    padding: 4px 10px;
    font-size: 12px;
  }
</style>
