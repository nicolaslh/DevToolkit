<script lang="ts">
  import { CodecService } from "../../../bindings/github.com/nic/devtoolkit";
  import { errMsg } from "../err";
  import Copy from "../components/Copy.svelte";
  import ErrorBar from "../components/ErrorBar.svelte";

  // Each direction maps to either an encode (文本 → \uXXXX) or a decode
  // (\uXXXX → 文本) operation. ASCII / 中文 share the same underlying codec;
  // the separate entries make the intent explicit for users.
  type Op = "encode" | "decode";
  const directions: { value: string; label: string; op: Op }[] = [
    { value: "ascii2unicode", label: "ASCII 转 Unicode", op: "encode" },
    { value: "unicode2ascii", label: "Unicode 转 ASCII", op: "decode" },
    { value: "unicode2cn", label: "Unicode 转 中文", op: "decode" },
    { value: "cn2unicode", label: "中文 转 Unicode", op: "encode" },
  ];

  let direction = $state("cn2unicode");
  let input = $state("");
  let output = $state("");
  let error = $state("");

  const placeholder = $derived(
    directions.find((d) => d.value === direction)?.op === "encode"
      ? "在此粘贴文本…"
      : "在此粘贴 \\uXXXX 序列…",
  );

  async function run() {
    error = "";
    const dir = directions.find((d) => d.value === direction)!;
    try {
      output =
        dir.op === "encode"
          ? await CodecService.Encode(input, "unicode")
          : await CodecService.Decode(input, "unicode");
    } catch (e) {
      error = errMsg(e);
    }
  }
</script>

<div class="tool">
  <div class="row">
    <label for="uni-dir">转换方向</label>
    <select id="uni-dir" bind:value={direction}>
      {#each directions as d}
        <option value={d.value}>{d.label}</option>
      {/each}
    </select>
  </div>

  <label for="uni-in">输入</label>
  <textarea
    id="uni-in"
    bind:value={input}
    {placeholder}
  ></textarea>

  <div class="actions">
    <button class="primary" onclick={run}>转换</button>
  </div>

  <ErrorBar message={error} />

  <div class="out-head">
    <label for="uni-out">输出</label>
    <Copy text={output} />
  </div>
  <textarea id="uni-out" readonly value={output}></textarea>
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
  .actions {
    display: flex;
    gap: 8px;
  }
  .out-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
</style>
