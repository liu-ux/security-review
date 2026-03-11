# 代码安全审计 SKILL

自动化识别和分析代码中的安全漏洞，提供详细的审计报告。

## 功能特性

- 批量代码安全扫描
- 多语言代码分析（Python、Java、Go、C++等）
- 常见漏洞检测（SQL注入、路径遍历、反序列化等）
- 队列式文件管理
- 报告自动合并

## 安装

### 安装步骤

1. 将项目克隆到本地 `security-review`

```bash
git clone <repository-url> security-review
```

2. 拷贝到 skills 目录（选择以下任一目录）：
   - costrict项目配置：`.costrict/skills/security-review/`
   - costrict全局配置：`~/.config/costrict/skills/security-review/`
   - Claude项目配置：`.claude/skills/security-review/`
   - Claude全局配置：`~/.claude/skills/security-review/`

```bash
cp -r security-review /path/to/skills/
```

## 使用方法

### 基础用法

使用 `/security-review` 命令对目录或文件进行安全审计：

```
/security-review 对 /path/to/project 目录进行代码审计
```

对指定文件进行审计：

```
/security-review 审计文件 path/to/file1 path/to/file2
```

### 配置选项

- `EXCLUDE_SCAN_FILES`：排除文件模式（空格分隔，如 `*.o *.log`）
- `EXCLUDE_SCAN_DIRS`：排除目录模式（空格分隔，如 `node_modules .git`）

## 脚本说明

### queue.sh - 文件队列管理

管理待审计文件的队列。

### merge_reports.sh - 报告合并

合并多个 Markdown 报告文件为一个完整报告。

## 输出说明

- 单独报告：`<输出目录>/<文件路径>-report.md`
- 合并报告：`<输出目录>/full_report.md`

## 参考

[3stoneBrother/code-audit](https://github.com/3stoneBrother/code-audit/tree/main)
