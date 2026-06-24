<script lang="ts">
  import { FrontendService } from "../../../bindings/github.com/nic/devtoolkit";
  import { errMsg } from "../err";
  import Copy from "../components/Copy.svelte";
  import ErrorBar from "../components/ErrorBar.svelte";

  let hex = $state("#2f6feb");
  let rgb = $state("");
  let rgba = $state("");
  let hsl = $state("");
  let error = $state("");

  async function convert(value: string, format: string) {
    error = "";
    try {
      const set = await FrontendService.ConvertColor(value, format);
      hex = set.hex;
      rgb = set.rgb;
      rgba = set.rgba;
      hsl = set.hsl;
    } catch (e) {
      error = errMsg(e);
    }
  }

  // initialize with the literal default to avoid referencing reactive state at init
  convert("#2f6feb", "hex");

  function onPicker(e: Event) {
    const v = (e.target as HTMLInputElement).value;
    convert(v, "hex");
  }
</script>

<div class="tool">
  <div class="picker-row">
    <input class="picker" type="color" value={hex} oninput={onPicker} aria-label="取色面板" />
    <div class="swatch" style="background:{hex}"></div>
  </div>

  <ErrorBar message={error} />

  <div class="grid">
    <div class="field">
      <label for="c-hex">HEX</label>
      <div class="line">
        <input id="c-hex" type="text" bind:value={hex} onchange={() => convert(hex, "hex")} />
        <Copy text={hex} />
      </div>
    </div>
    <div class="field">
      <label for="c-rgb">RGB</label>
      <div class="line">
        <input id="c-rgb" type="text" bind:value={rgb} onchange={() => convert(rgb, "rgb")} />
        <Copy text={rgb} />
      </div>
    </div>
    <div class="field">
      <label for="c-rgba">RGBA</label>
      <div class="line">
        <input id="c-rgba" type="text" bind:value={rgba} onchange={() => convert(rgba, "rgba")} />
        <Copy text={rgba} />
      </div>
    </div>
    <div class="field">
      <label for="c-hsl">HSL</label>
      <div class="line">
        <input id="c-hsl" type="text" bind:value={hsl} onchange={() => convert(hsl, "hsl")} />
        <Copy text={hsl} />
      </div>
    </div>
  </div>
</div>

<style>
  .tool {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .picker-row {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .picker {
    width: 64px;
    height: 48px;
    padding: 0;
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    background: none;
    cursor: pointer;
  }
  .swatch {
    flex: 1;
    height: 48px;
    border-radius: var(--radius);
    border: 1px solid var(--color-border);
  }
  .grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .line {
    display: flex;
    gap: 8px;
    align-items: center;
  }
</style>
