<script lang="ts">
  import { CodecService } from "../../../bindings/github.com/nic/devtoolkit";
  import { errMsg } from "../err";
  import Copy from "../components/Copy.svelte";
  import ErrorBar from "../components/ErrorBar.svelte";

  // Hash
  let hashAlgo = $state("sha256");
  let hashInput = $state("");
  let hashOut = $state("");
  let hashErr = $state("");

  async function doHash() {
    hashErr = "";
    try {
      hashOut = await CodecService.Hash(hashInput, hashAlgo);
    } catch (e) {
      hashErr = errMsg(e);
    }
  }

  // AES
  let aesKey = $state("");
  let aesInput = $state("");
  let aesOut = $state("");
  let aesErr = $state("");

  async function doEncrypt() {
    aesErr = "";
    try {
      aesOut = await CodecService.Encrypt(aesInput, aesKey);
    } catch (e) {
      aesErr = errMsg(e);
    }
  }
  async function doDecrypt() {
    aesErr = "";
    try {
      aesOut = await CodecService.Decrypt(aesInput, aesKey);
    } catch (e) {
      aesErr = errMsg(e);
    }
  }
</script>

<div class="tool">
  <section class="card">
    <h3>哈希</h3>
    <div class="row">
      <label for="h-algo">算法</label>
      <select id="h-algo" bind:value={hashAlgo}>
        <option value="md5">MD5</option>
        <option value="sha1">SHA-1</option>
        <option value="sha256">SHA-256</option>
      </select>
      <button class="primary" onclick={doHash}>计算哈希</button>
    </div>
    <textarea bind:value={hashInput} placeholder="待哈希内容…"></textarea>
    <div class="out-head">
      <label for="h-out">哈希值</label>
      <Copy text={hashOut} />
    </div>
    <input id="h-out" type="text" readonly value={hashOut} />
    <ErrorBar message={hashErr} />
  </section>

  <section class="card">
    <h3>AES 对称加解密（AES-GCM）</h3>
    <label for="aes-key">密钥（16 / 24 / 32 字节）</label>
    <input id="aes-key" type="text" bind:value={aesKey} placeholder="例如 16 个字符的密钥" />
    <label for="aes-in">明文 / 密文</label>
    <textarea id="aes-in" bind:value={aesInput} placeholder="加密填明文，解密填 Base64 密文…"></textarea>
    <div class="actions">
      <button class="primary" onclick={doEncrypt}>加密</button>
      <button onclick={doDecrypt}>解密</button>
    </div>
    <ErrorBar message={aesErr} />
    <div class="out-head">
      <label for="aes-out">结果</label>
      <Copy text={aesOut} />
    </div>
    <textarea id="aes-out" readonly value={aesOut}></textarea>
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
    gap: 8px;
    background: var(--color-surface);
    border-radius: var(--radius);
    padding: 14px;
  }
  .card h3 {
    margin: 0 0 4px;
    font-size: 15px;
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
