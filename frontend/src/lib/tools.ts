import type { Component } from "svelte";
import Json from "./tools/Json.svelte";
import Encoding from "./tools/Encoding.svelte";
import Crypto from "./tools/Crypto.svelte";
import Jwt from "./tools/Jwt.svelte";
import Password from "./tools/Password.svelte";
import UrlParser from "./tools/UrlParser.svelte";
import Color from "./tools/Color.svelte";
import Diff from "./tools/Diff.svelte";
import Regex from "./tools/Regex.svelte";
import Cron from "./tools/Cron.svelte";
import Chmod from "./tools/Chmod.svelte";
import EnvPath from "./tools/EnvPath.svelte";
import Mock from "./tools/Mock.svelte";
import Curl from "./tools/Curl.svelte";
import Qr from "./tools/Qr.svelte";
import Svg from "./tools/Svg.svelte";
import Timestamp from "./tools/Timestamp.svelte";

export interface ToolMeta {
  id: string;
  name: string;
  description: string;
  category: string;
  icon: string;
  component: Component;
}

// MVP tool registry. Categories mirror the requirements document.
export const tools: ToolMeta[] = [
  {
    id: "json",
    name: "JSON 在线解析",
    description: "JSON 校验、格式化、压缩、转义与树形视图",
    category: "常用工具",
    icon: "🧩",
    component: Json,
  },
  {
    id: "encoding",
    name: "编码转码",
    description: "Base64 / URL / Hex 编码与解码",
    category: "编码与加解密",
    icon: "🔁",
    component: Encoding,
  },
  {
    id: "crypto",
    name: "哈希与加解密",
    description: "MD5 / SHA 哈希与 AES 对称加解密",
    category: "编码与加解密",
    icon: "🔐",
    component: Crypto,
  },
  {
    id: "jwt",
    name: "JWT 解码",
    description: "解码 Header / Payload 并标注过期时间",
    category: "安全与身份验证",
    icon: "🎫",
    component: Jwt,
  },
  {
    id: "password",
    name: "强密码生成",
    description: "可配置字符集的强密码与 Bcrypt 哈希",
    category: "安全与身份验证",
    icon: "🔑",
    component: Password,
  },
  {
    id: "url",
    name: "URL 解析",
    description: "拆解 URL 与查询参数并可编辑重组",
    category: "网络与接口",
    icon: "🔗",
    component: UrlParser,
  },
  {
    id: "color",
    name: "颜色转换",
    description: "HEX / RGB / RGBA / HSL 实时互转",
    category: "前端与视觉",
    icon: "🎨",
    component: Color,
  },
  {
    id: "diff",
    name: "文本比对",
    description: "文本 / 代码 / JSON 双栏差异高亮",
    category: "常用工具",
    icon: "📑",
    component: Diff,
  },
  {
    id: "regex",
    name: "正则测试",
    description: "正则匹配高亮与常用正则库",
    category: "常用工具",
    icon: "🔎",
    component: Regex,
  },
  {
    id: "mock",
    name: "Mock 数据",
    description: "批量生成姓名 / 地址 / 手机号 / 邮箱 / 银行卡 / Lorem",
    category: "常用工具",
    icon: "🎲",
    component: Mock,
  },
  {
    id: "timestamp",
    name: "时间戳转换",
    description: "Unix 时间戳与普通时间互转，实时显示当前时间戳",
    category: "常用工具",
    icon: "⏱️",
    component: Timestamp,
  },
  {
    id: "curl",
    name: "cURL 转换",
    description: "cURL 命令转 Python / JS / Go / Java 代码",
    category: "网络与接口",
    icon: "📡",
    component: Curl,
  },
  {
    id: "cron",
    name: "Cron 表达式",
    description: "可视化生成、解析与预测执行时间",
    category: "系统运维",
    icon: "⏰",
    component: Cron,
  },
  {
    id: "chmod",
    name: "权限计算器",
    description: "数字与符号 Linux 权限实时互转",
    category: "系统运维",
    icon: "🛡️",
    component: Chmod,
  },
  {
    id: "envpath",
    name: "环境变量 PATH",
    description: "PATH 拆分、去重与比较",
    category: "系统运维",
    icon: "🧭",
    component: EnvPath,
  },
  {
    id: "qr",
    name: "二维码",
    description: "文本生成二维码与图片解码",
    category: "前端与视觉",
    icon: "🔲",
    component: Qr,
  },
  {
    id: "svg",
    name: "SVG 优化",
    description: "清理压缩 SVG 并实时预览",
    category: "前端与视觉",
    icon: "🖼️",
    component: Svg,
  },
];
