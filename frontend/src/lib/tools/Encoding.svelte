<script lang="ts">
  import { CodecService } from "../../../bindings/github.com/nic/devtoolkit";
  import { errMsg } from "../err";
  import Copy from "../components/Copy.svelte";
  import ErrorBar from "../components/ErrorBar.svelte";

  let kind = $state("base64");
  let input = $state("");
  let output = $state("");
  let error = $state("");

  async function run(op: "encode" | "decode") {
    error = "";
    try {
      output =
        op === "encode"
          ? await CodecService.Encode(input, kind)
          : await CodecService.Decode(input, kind);
    } catch (e) {
      error = errMsg(e);
    }
  }
</script>

<div class="tool">
  <div class="row">
    <label for="enc-kind">编码类型</label>
    <select id="enc-kind" bind:value={kind}>
      <option value="base64">Base64</option>
      <option value="url">URL 编码</option>
      <option value="hex">十六进制 Hex</option>
    </select>
  </div>

  <label for="enc-in">输入</label>
  <textarea id="enc-in" bind:value={input} placeholder="在此粘贴文本…"></textarea>

  <div class="actions">
    <button class="primary" onclick={() => run("encode")}>编码</button>
    <button onclick={() => run("decode")}>解码</button>
  </div>

  <ErrorBar message={error} />

  <div class="out-head">
    <label for="enc-out">输出</label>
    <Copy text={output} />
  </div>
  <textarea id="enc-out" readonly value={output}></textarea>
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
