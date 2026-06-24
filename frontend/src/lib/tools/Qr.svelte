<script lang="ts">
  import { FrontendService } from "../../../bindings/github.com/nic/devtoolkit";
  import { errMsg } from "../err";
  import Copy from "../components/Copy.svelte";
  import ErrorBar from "../components/ErrorBar.svelte";

  // Generate
  let text = $state("https://wails.io");
  let foreground = $state("#000000");
  let background = $state("#ffffff");
  let dataUri = $state("");
  let genErr = $state("");

  async function generate() {
    genErr = "";
    dataUri = "";
    try {
      dataUri = await FrontendService.GenerateQR(text, {
        foreground,
        background,
        size: 256,
      });
    } catch (e) {
      genErr = errMsg(e);
    }
  }

  // Decode
  let decoded = $state("");
  let decErr = $state("");

  async function onFile(e: Event) {
    decErr = "";
    decoded = "";
    const file = (e.target as HTMLInputElement).files?.[0];
    if (!file) return;
    try {
      const buf = new Uint8Array(await file.arrayBuffer());
      let binary = "";
      for (let i = 0; i < buf.length; i++) binary += String.fromCharCode(buf[i]);
      const base64 = btoa(binary);
      decoded = await FrontendService.DecodeQR(base64);
    } catch (err) {
      decErr = errMsg(err);
    }
  }

  generate();
</script>

<div class="tool">
  <section class="card">
    <h3>生成二维码</h3>
    <label for="qr-text">文本 / 链接</label>
    <textarea id="qr-text" bind:value={text} placeholder="输入文本或链接…"></textarea>
    <div class="colors">
      <label>前景色 <input type="color" bind:value={foreground} /></label>
      <label>背景色 <input type="color" bind:value={background} /></label>
      <button class="primary" onclick={generate}>生成</button>
    </div>
    <ErrorBar message={genErr} />
    {#if dataUri}
      <img class="qr" src={dataUri} alt="二维码" />
    {/if}
  </section>

  <section class="card">
    <h3>解码二维码</h3>
    <input type="file" accept="image/png,image/jpeg" onchange={onFile} />
    <ErrorBar message={decErr} />
    {#if decoded}
      <div class="out-head">
        <span class="lbl">识别内容</span>
        <Copy text={decoded} />
      </div>
      <pre>{decoded}</pre>
    {/if}
  </section>
</div>

<style>
  .tool {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .card {
    display: flex;
    flex-direction: column;
    gap: 10px;
    background: var(--color-surface);
    border-radius: var(--radius);
    padding: 14px;
  }
  .card h3 {
    margin: 0;
    font-size: 15px;
  }
  .colors {
    display: flex;
    align-items: center;
    gap: 16px;
  }
  .colors label {
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--color-text);
  }
  .qr {
    width: 256px;
    height: 256px;
    border-radius: var(--radius);
    background: #fff;
    align-self: flex-start;
  }
  .out-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  pre {
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: 12px;
    margin: 0;
    font-family: var(--font-mono);
    font-size: 13px;
    white-space: pre-wrap;
    word-break: break-all;
  }
</style>
