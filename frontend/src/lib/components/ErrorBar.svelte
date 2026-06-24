<script lang="ts">
  // Flat semantic error bar (no shadow/gradient).
  let { message }: { message: string } = $props();
</script>

{#if message}
  <div class="error-bar" role="alert">{message}</div>
{/if}

<style>
  .error-bar {
    background: var(--color-error-weak);
    color: var(--color-error);
    border-radius: var(--radius);
    padding: 8px 12px;
    font-size: 13px;
  }
</style>
