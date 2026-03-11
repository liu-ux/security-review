#!/bin/bash

# 漏洞报告合并脚本
# 功能：将指定目录下的所有漏洞 JSON 文件合并为一个完整的 JSONL 报告

set -e

# --- 帮助信息 ---
usage() {
    echo "用法: $0 <报告输出目录>"
    echo ""
    echo "功能:"
    echo "  查找指定目录下所有漏洞 JSON 文件，合并为一个 full_report.jsonl 文件"
    echo ""
    echo "参数:"
    echo "  报告输出目录 : 包含各个漏洞 JSON 文件的目标目录"
    echo ""
    echo "输出:"
    echo "  full_report.jsonl : 合并后的完整 JSONL 报告文件"
    echo ""
    echo "说明:"
    echo "  1. 查找所有匹配 *-report-*.json 的文件"
    echo "  2. 自动去除文件中的 \`\`\`json 包裹标记"
    echo "  3. 去除所有换行符，每个 JSON 对象作为一行写入"
    echo "  4. 生成标准的 JSONL 格式（每行一个 JSON 对象）"
    echo "  5. 不合并 full_report.jsonl 自身"
    exit 1
}

# --- 参数解析 ---

if [ $# -eq 0 ]; then
    usage
fi

REPORT_DIR="$1"
OUTPUT_FILE="${REPORT_DIR}/full_report.jsonl"

# 验证报告目录是否存在
if [ ! -d "$REPORT_DIR" ]; then
    echo "错误: 报告目录 [$REPORT_DIR] 不存在"
    exit 1
fi

# 验证目录是否可读
if [ ! -r "$REPORT_DIR" ]; then
    echo "错误: 报告目录 [$REPORT_DIR] 不可读"
    exit 1
fi

# 验证目录是否可写（用于创建 full_report.jsonl）
if [ ! -w "$REPORT_DIR" ]; then
    echo "错误: 报告目录 [$REPORT_DIR] 不可写"
    exit 1
fi

echo "正在合并报告..."
echo "报告目录: $REPORT_DIR"
echo "输出文件: $OUTPUT_FILE"
echo ""

# 查找所有漏洞 JSON 文件（排除 full_report.jsonl 自身）
# 使用 find 处理文件名包含空格的特殊情况
mapfile -t md_files < <(find "$REPORT_DIR" -type f -name "*-report-*.json" ! -name "full_report.jsonl" 2>/dev/null | sort)

# 检查是否找到文件
if [ ${#md_files[@]} -eq 0 ]; then
    echo "警告: 在目录 [$REPORT_DIR] 中未找到任何 .json 文件"
    exit 0
fi

echo "找到 ${#md_files[@]} 个报告文件"
echo ""

# 清空输出文件
> "$OUTPUT_FILE"

# 合并所有 JSON 文件为 JSONL 格式
for json_file in "${md_files[@]}"; do
    # 读取文件内容
    content=$(cat "$json_file")
    
    # 去除可能的 ```json``` 包裹标记
    content=$(echo "$content" | sed '1s/^```json\s*//' | sed '$s/```\s*$//')
    
    # 去除所有换行符，转换为单行 JSON
    content=$(echo "$content" | tr -d '\n' | tr -d '\r')
    
    # 写入输出文件作为一行
    echo "$content" >> "$OUTPUT_FILE"
done

echo ""
echo "----------------------------------------"
echo "报告合并完成!"
echo "合并后文件: $OUTPUT_FILE"
echo "合并文件数: ${#md_files[@]}"
echo "----------------------------------------"

exit 0
