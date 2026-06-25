# DevToolkit 图标配置说明

## 图标文件结构

```
build/
├── appicon.png                    # 基础图标文件 (1024x1024 PNG)
├── appicon.icon/                  # macOS 图标配置目录
│   ├── icon.json                  # 图标配置文件
│   └── Assets/
│       └── devtoolkit_icon.svg    # 自定义 SVG 图标
├── darwin/
│   └── icons.icns                 # macOS 应用图标 (自动生成)
└── windows/
    └── icon.ico                   # Windows 应用图标 (自动生成)
```

## 图标生成步骤

### 1. 准备图标文件

#### 方法一：使用自定义 SVG 图标（推荐）
1. 将 SVG 图标放到 `build/appicon.icon/Assets/` 目录
2. 确保 `icon.json` 中的 `image-name` 与文件名匹配
3. 运行生成命令

#### 方法二：使用 PNG 图标
1. 准备一个 1024x1024 的 PNG 图标
2. 替换 `build/appicon.png` 文件
3. 运行生成命令

### 2. 生成图标

运行以下命令生成所有平台的图标：

```bash
# 生成图标
task common:generate:icons

# 或者使用完整命令
wails3 generate icons -input build/appicon.png \
  -macfilename build/darwin/icons.icns \
  -windowsfilename build/windows/icon.ico \
  -iconcomposerinput build/appicon.icon \
  -macassetdir build/darwin
```

### 3. 更新构建资源

如果修改了应用信息，运行：

```bash
task common:update:build-assets
```

### 4. 重新构建应用

```bash
task build
```

## 图标配置文件说明

### icon.json 配置项

```json
{
  "fill": {
    "automatic-gradient": "extended-gray:1.00000,1.00000"
  },
  "groups": [
    {
      "layers": [
        {
          "image-name": "devtoolkit_icon.svg",  // SVG 文件名
          "name": "devtoolkit_icon",             // 图层名称
          "position": {
            "scale": 1.0,                        // 缩放比例
            "translation-in-points": [0, 0]      // 位置偏移
          },
          "fill-specializations": [              // 适配不同外观模式
            {
              "appearance": "dark",
              "value": {
                "solid": "srgb:0.92143,0.92145,0.92144,1.00000"
              }
            }
          ]
        }
      ],
      "shadow": {                                // 阴影效果
        "kind": "neutral",
        "opacity": 0.5
      },
      "specular": true,                          // 高光效果
      "translucency": {                          // 半透明效果
        "enabled": true,
        "value": 0.5
      }
    }
  ]
}
```

## 图标设计建议

### 尺寸要求
- **基础图标**：1024x1024 像素
- **小尺寸**：确保在 16x16、32x32、64x64 下清晰可辨
- **macOS**：支持自适应暗色/亮色模式

### 设计风格
- 简洁明了，避免过多细节
- 高对比度，确保可识别性
- 符合平台设计规范：
  - **macOS**：圆角矩形，支持深色模式
  - **Windows**：传统方形图标
  - **Linux**：PNG 格式，支持多种尺寸

### 颜色建议
- 主色调：专业、稳重（如深蓝、深灰）
- 辅助色：与主色调和谐搭配
- 避免过于鲜艳的颜色

## 当前配置

已为 DevToolkit 配置：
- ✅ 自定义 SVG 图标：工具箱样式
- ✅ macOS 图标配置：支持深色模式
- ✅ 应用信息：DevToolkit v1.0.0
- ✅ 自动生成脚本：支持一键生成

## 下一步操作

1. **自定义图标**：替换 `devtoolkit_icon.svg` 为你的设计
2. **生成图标**：运行 `task common:generate:icons`
3. **测试验证**：构建应用并检查图标显示

## 常见问题

### 图标显示模糊
- 确保原始图标尺寸足够大（1024x1024）
- 检查 SVG 图标是否有足够的细节
- 验证生成的 `.icns` 和 `.ico` 文件

### macOS 图标不显示
- 检查 `config.yml` 中的 `cfBundleIconName` 设置
- 确保 `Assets.car` 文件已生成
- 重新构建应用

### Windows 图标不更新
- 删除旧的 `.ico` 文件
- 重新运行图标生成命令
- 清理构建缓存并重新构建
