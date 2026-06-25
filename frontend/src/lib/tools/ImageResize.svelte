<script lang="ts">
  import Copy from "../components/Copy.svelte";
  import ErrorBar from "../components/ErrorBar.svelte";

  // State
  let originalImage: HTMLImageElement | null = $state(null);
  let originalDataUrl = $state("");
  let originalWidth = $state(0);
  let originalHeight = $state(0);
  let fileName = $state("image");

  // Resize options
  let targetWidth = $state(0);
  let targetHeight = $state(0);
  let maintainRatio = $state(true);
  let quality = $state(0.92);
  let outputFormat = $state<"image/png" | "image/jpeg" | "image/webp">("image/png");

  // Result
  let resizedDataUrl = $state("");
  let resizedWidth = $state(0);
  let resizedHeight = $state(0);
  let resizedSize = $state(0);
  let originalSize = $state(0);
  let error = $state("");

  // Preset sizes
  const presets: { name: string; width: number; height: number }[] = [
    { name: "自定义", width: 0, height: 0 },
    { name: "头像 (200×200)", width: 200, height: 200 },
    { name: "缩略图 (150×150)", width: 150, height: 150 },
    { name: "社交媒体 (1200×630)", width: 1200, height: 630 },
    { name: "Instagram (1080×1080)", width: 1080, height: 1080 },
    { name: "网页横幅 (1920×600)", width: 1920, height: 600 },
    { name: "图标 (64×64)", width: 64, height: 64 },
    { name: "图标 (128×128)", width: 128, height: 128 },
    { name: "图标 (256×256)", width: 256, height: 256 },
  ];
  let selectedPreset = $state(0);

  // Handle file selection
  async function onFileSelect(e: Event) {
    error = "";
    resizedDataUrl = "";
    originalImage = null;
    originalDataUrl = "";

    const file = (e.target as HTMLInputElement).files?.[0];
    if (!file) return;

    fileName = file.name.replace(/\.[^.]+$/, "");
    originalSize = file.size;

    const reader = new FileReader();
    reader.onload = (event) => {
      const dataUrl = event.target?.result as string;
      originalDataUrl = dataUrl;

      const img = new Image();
      img.onload = () => {
        originalImage = img;
        originalWidth = img.width;
        originalHeight = img.height;
        targetWidth = img.width;
        targetHeight = img.height;
      };
      img.onerror = () => {
        error = "无法加载图片，请选择有效的图片文件";
      };
      img.src = dataUrl;
    };
    reader.onerror = () => {
      error = "读取文件失败";
    };
    reader.readAsDataURL(file);
  }

  // Handle width change with aspect ratio
  function onWidthChange(value: string) {
    const w = parseInt(value) || 0;
    targetWidth = w;
    if (maintainRatio && originalWidth > 0 && w > 0) {
      targetHeight = Math.round((w / originalWidth) * originalHeight);
    }
    selectedPreset = 0;
  }

  function onHeightChange(value: string) {
    const h = parseInt(value) || 0;
    targetHeight = h;
    if (maintainRatio && originalHeight > 0 && h > 0) {
      targetWidth = Math.round((h / originalHeight) * originalWidth);
    }
    selectedPreset = 0;
  }

  function onPresetChange(index: number) {
    selectedPreset = index;
    if (index === 0) return;

    const preset = presets[index];
    targetWidth = preset.width;
    targetHeight = preset.height;

    if (maintainRatio && originalWidth > 0 && originalHeight > 0) {
      // Fit within preset dimensions while maintaining aspect ratio
      const ratio = Math.min(preset.width / originalWidth, preset.height / originalHeight);
      targetWidth = Math.round(originalWidth * ratio);
      targetHeight = Math.round(originalHeight * ratio);
    }
  }

  // Resize the image
  function resizeImage() {
    if (!originalImage) {
      error = "请先选择图片";
      return;
    }

    if (targetWidth <= 0 || targetHeight <= 0) {
      error = "请输入有效的目标尺寸";
      return;
    }

    error = "";
    resizedDataUrl = "";

    try {
      const canvas = document.createElement("canvas");
      canvas.width = targetWidth;
      canvas.height = targetHeight;
      const ctx = canvas.getContext("2d");

      if (!ctx) {
        error = "无法创建画布上下文";
        return;
      }

      // Enable image smoothing for better quality
      ctx.imageSmoothingEnabled = true;
      ctx.imageSmoothingQuality = "high";

      // Draw resized image
      ctx.drawImage(originalImage, 0, 0, targetWidth, targetHeight);

      // Convert to data URL
      canvas.toBlob(
        (blob) => {
          if (!blob) {
            error = "生成图片失败";
            return;
          }
          resizedSize = blob.size;
          const reader = new FileReader();
          reader.onload = (e) => {
            resizedDataUrl = e.target?.result as string;
            resizedWidth = targetWidth;
            resizedHeight = targetHeight;
          };
          reader.readAsDataURL(blob);
        },
        outputFormat,
        quality
      );
    } catch (e) {
      error = "处理图片时发生错误";
      console.error(e);
    }
  }

  // Download the resized image
  function download() {
    if (!resizedDataUrl) return;
    const ext = outputFormat.split("/")[1];
    const a = document.createElement("a");
    a.href = resizedDataUrl;
    a.download = `${fileName}_resized.${ext}`;
    a.click();
  }

  // Format file size
  function formatSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / 1024 / 1024).toFixed(2)} MB`;
  }
</script>

<div class="tool">
  <section class="card">
    <h3>选择图片</h3>
    <input type="file" accept="image/*" onchange={onFileSelect} />

    {#if originalImage}
      <div class="preview-row">
        <div class="preview-item">
          <span class="label">原始图片</span>
          <img src={originalDataUrl} alt="原始图片" class="preview-img" />
          <span class="info">
            {originalWidth} × {originalHeight} · {formatSize(originalSize)}
          </span>
        </div>
        {#if resizedDataUrl}
          <div class="preview-item">
            <span class="label">调整后</span>
            <img src={resizedDataUrl} alt="调整后图片" class="preview-img" />
            <span class="info">
              {resizedWidth} × {resizedHeight} · {formatSize(resizedSize)}
            </span>
          </div>
        {/if}
      </div>
    {/if}
  </section>

  {#if originalImage}
    <section class="card">
      <h3>调整设置</h3>

      <div class="form-row">
        <label>
          <input type="checkbox" bind:checked={maintainRatio} />
          保持宽高比
        </label>
      </div>

      <div class="form-row">
        <select value={selectedPreset} onchange={(e) => onPresetChange(parseInt((e.target as HTMLSelectElement).value))}>
          {#each presets as preset, i}
            <option value={i}>{preset.name}</option>
          {/each}
        </select>
      </div>

      <div class="form-row dims">
        <div class="dim-input">
          <label for="width">宽度 (px)</label>
          <input id="width" type="number" min="1" value={targetWidth} oninput={(e) => onWidthChange((e.target as HTMLInputElement).value)} />
        </div>
        <span class="times">×</span>
        <div class="dim-input">
          <label for="height">高度 (px)</label>
          <input id="height" type="number" min="1" value={targetHeight} oninput={(e) => onHeightChange((e.target as HTMLInputElement).value)} />
        </div>
      </div>

      <div class="form-row">
        <div class="dim-input">
          <label for="format">输出格式</label>
          <select id="format" bind:value={outputFormat}>
            <option value="image/png">PNG (无损)</option>
            <option value="image/jpeg">JPEG (有损)</option>
            <option value="image/webp">WebP (高效)</option>
          </select>
        </div>
        {#if outputFormat !== "image/png"}
          <div class="dim-input">
            <label for="quality">质量 ({Math.round(quality * 100)}%)</label>
            <input id="quality" type="range" min="0.1" max="1" step="0.01" bind:value={quality} />
          </div>
        {/if}
      </div>

      <ErrorBar message={error} />

      <div class="actions">
        <button class="primary" onclick={resizeImage}>调整尺寸</button>
        {#if resizedDataUrl}
          <button onclick={download}>下载图片</button>
        {/if}
      </div>
    </section>
  {/if}
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
    gap: 12px;
    background: var(--color-surface);
    border-radius: var(--radius);
    padding: 14px;
  }
  .card h3 {
    margin: 0;
    font-size: 15px;
  }
  .preview-row {
    display: flex;
    gap: 20px;
    flex-wrap: wrap;
  }
  .preview-item {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .preview-item .label {
    font-size: 12px;
    color: var(--color-text-weak);
  }
  .preview-img {
    max-width: 200px;
    max-height: 200px;
    border-radius: var(--radius);
    border: 1px solid var(--color-border);
    object-fit: contain;
    background: repeating-conic-gradient(#f0f0f0 0% 25%, #fff 0% 50%) 50% / 10px 10px;
  }
  :global([data-theme="dark"]) .preview-img {
    background: repeating-conic-gradient(#333 0% 25%, #222 0% 50%) 50% / 10px 10px;
  }
  .preview-item .info {
    font-size: 12px;
    color: var(--color-text-weak);
  }
  .form-row {
    display: flex;
    align-items: center;
    gap: 12px;
    flex-wrap: wrap;
  }
  .form-row label {
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--color-text);
    font-size: 13px;
  }
  .dims {
    align-items: flex-end;
  }
  .dim-input {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .dim-input label {
    font-size: 12px;
    color: var(--color-text-weak);
  }
  .dim-input input[type="number"],
  .dim-input select,
  .card input[type="file"],
  .card select {
    padding: 8px 10px;
    border-radius: var(--radius);
    border: 1px solid var(--color-border);
    background: var(--color-bg);
    color: var(--color-text);
    font-size: 13px;
    min-width: 100px;
  }
  .dim-input input[type="number"]:focus,
  .dim-input select:focus,
  .card select:focus {
    outline: none;
    border-color: var(--color-accent);
  }
  .times {
    font-size: 14px;
    color: var(--color-text-weak);
    padding-bottom: 8px;
  }
  .dim-input input[type="range"] {
    width: 120px;
  }
  .actions {
    display: flex;
    gap: 10px;
  }
  .actions button {
    padding: 8px 16px;
    border-radius: var(--radius);
    font-size: 13px;
  }
</style>
