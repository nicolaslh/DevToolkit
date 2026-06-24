# DevToolkit

[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**面向开发者与 IT 专业人员的桌面级"瑞士军刀"工具箱**

DevToolkit 是一款桌面应用，整合了开发者和 IT 专业人员日常工作中的高频工具，涵盖文本处理、网络调试、系统运维、前端开发、安全验证等多个领域。所有工具默认本地执行，保护数据隐私。

## 功能特性

### 一、文本与代码处理

- **Diff Viewer** - 文本/代码/JSON 双栏比对，支持行级别和字符级别差异标注
- **Regex Tester** - 正则表达式测试器，内置常用正则库（邮箱、手机号、身份证号、IPv4/IPv6）
- **Mock Generator** - 批量生成随机模拟数据（姓名、地址、手机号、邮箱、银行卡号、Lorem Ipsum）

### 二、网络与接口

- **URL Parser** - URL 拆解解析，可视化编辑查询参数
- **cURL Converter** - cURL 命令转换为 Python/JavaScript/Go/Java 请求代码

### 三、系统运维

- **Cron Tool** - Cron 表达式解析与生成，可视化配置定时任务
- **Chmod Calculator** - Linux 权限计算器，数字权限与符号权限实时转换
- **Env/Path Viewer** - 环境变量查看、去重、比较

### 四、前端与视觉

- **Color Converter** - 颜色值转换（HEX/RGB/RGBA/HSL）
- **SVG Optimizer** - SVG 优化与预览
- **QR Code Tool** - 二维码生成与解码

### 五、安全与身份验证

- **JWT Decoder** - JWT 解码与格式化，自动识别过期状态
- **Password Generator** - 强密码生成器，支持批量生成和 Bcrypt 哈希

### 六、编码与加解密

- **Encoding Tool** - Base64/URL/Hex 编码转换
- **Crypto Tool** - MD5/SHA-1/SHA-256 哈希计算，AES 对称加解密

## 技术栈

- **后端**: Go + Wails v3
- **前端**: TypeScript + Svelte + Vite
- **桌面框架**: Wails v3 (跨平台)

## 快速开始

### 环境要求

- Go 1.21+
- Node.js 18+
- Wails v3 CLI

### 安装 Wails CLI

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
```

### 运行开发模式

```bash
# 安装前端依赖
cd frontend && npm install && cd ..

# 启动开发服务器
task dev
```

应用将自动启动并支持热重载。

### 构建生产版本

```bash
# 构建当前平台
task build

# 打包应用
task package
```

构建产物位于 `bin/` 目录。

## 项目结构

```
DevToolkit/
├── main.go                 # 应用入口
├── services_*.go           # 后端服务层
├── internal/pkg/           # 核心功能包
│   ├── chmod/              # Linux 权限计算
│   ├── colorx/             # 颜色转换
│   ├── cronx/              # Cron 解析
│   ├── curlconv/           # cURL 转换
│   ├── differ/             # 文本比对
│   ├── jwtx/               # JWT 解码
│   ├── mockgen/            # Mock 数据生成
│   ├── pathx/              # PATH 变量处理
│   ├── pwdgen/             # 密码生成
│   ├── qrx/                # 二维码
│   ├── regexx/             # 正则测试
│   ├── svgopt/             # SVG 优化
│   ├── urlparse/           # URL 解析
│   ├── codecx/             # 编码转换
│   └── cryptox/            # 加解密
├── frontend/               # 前端代码
│   ├── src/                # Svelte 组件
│   ├── bindings/           # TypeScript 类型绑定
│   └── dist/               # 构建输出
├── build/                  # 平台构建配置
└── Taskfile.yml            # 任务定义
```

## 常用命令

| 命令 | 说明 |
|------|------|
| `task dev` | 启动开发服务器（热重载） |
| `task build` | 构建应用 |
| `task package` | 打包应用 |
| `task run` | 运行已构建的应用 |
| `task build:server` | 构建服务器模式（无 GUI） |
| `task run:server` | 运行服务器模式 |

## 跨平台构建

支持以下平台：

- **macOS**: `task darwin:build`
- **Windows**: `task windows:build`
- **Linux**: `task linux:build`
- **iOS**: `task ios:build` (实验性)
- **Android**: `task android:build` (实验性)

## 数据隐私

DevToolkit 设计原则：

- ✅ 所有工具默认本地执行，不向外部传输用户数据
- ✅ 无需网络连接即可使用大部分功能
- ⚠️ 部分功能（HTTP 请求、IP 地理位置查询、DNS 解析）需要网络连接

## 开发指南

### 添加新工具模块

1. 在 `internal/pkg/` 创建功能包
2. 在根目录创建对应的 `services_*.go` 服务层
3. 在 `main.go` 注册服务
4. 运行 `task dev` 自动生成前端绑定

### 前端开发

前端使用 Svelte + TypeScript，位于 `frontend/` 目录：

```bash
cd frontend
npm install       # 安装依赖
npm run dev       # 启动 Vite 开发服务器
npm run build     # 构建生产版本
```

## 文档

详细需求规格请参阅 [.kiro/specs/dev-toolkit/requirements.md](.kiro/specs/dev-toolkit/requirements.md)

## 许可证

本项目基于 [MIT License](LICENSE) 开源。

## 致谢

- [Wails](https://wails.io/) - Go 桌面应用框架
- [Svelte](https://svelte.dev/) - 前端框架
- [Task](https://taskfile.dev/) - 任务运行器
