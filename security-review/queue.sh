#!/bin/bash
# 队列文件名
QUEUE_FILENAME=".queue"

# --- 帮助信息 ---
usage() {
    echo "用法: $0 <命令> [扫描目录]"
    echo ""
    echo "命令:"
    echo "  init   : 扫描指定目录，生成 .queue 文件到当前工作目录"
    echo "  status : 查看当前工作目录下队列的剩余数量"
    echo "  push <路径1> [路径2...]: 将指定的文件/目录下文件追加到队列中 (支持多个路径)"
    echo "  pop    : 弹出并输出一个文件绝对路径"
    echo "  diff [git diff 参数...] : 将 git diff 的结果加入队列"
    echo "  clean  : 清理"
    echo ""
    echo "环境变量:"
    echo "  EXCLUDE_SCAN_FILES : 设置空格分隔的 glob 模式列表，用于排除文件 (如 '*.o *.log')"
    echo "  EXCLUDE_SCAN_DIRS  : 设置空格分隔的 glob 模式列表，用于排除目录 (如 'node_modules .git')"
    echo ""
    echo "注意:"
    echo "  扫描目录支持相对路径（基于当前工作目录）。"
    echo "  队列文件 (.queue) 始终生成在当前工作目录下。"
    echo "  写入队列的文件路径均为绝对路径。"
    exit 1
}

# --- 全局变量 ---
CMD=""
SCAN_DIR_INPUT=""      # 用户输入的扫描目录
SCAN_DIR_ABS=""        # 转换后的绝对扫描路径
QUEUE_FILE=""          # 队列文件路径 (固定为当前目录)
EXCLUDE_FILE_ARGS=()   # 文件排除参数
EXCLUDE_DIR_ARGS=()    # 目录排除参数

# --- 参数解析 ---

if [ $# -eq 0 ]; then
    usage
fi

CMD="$1"
# 获取扫描目录参数，默认为当前目录 "."
SCAN_DIR_INPUT="${2:-.}"

# --- 路径处理 ---

# 1. 确定队列文件位置：固定在当前工作目录
QUEUE_FILE="${PWD}/${QUEUE_FILENAME}"

# 2. 确定扫描目录的绝对路径
# readlink -f 会自动解析相对路径为绝对路径（基于当前 PWD）
if command -v readlink &> /dev/null; then
    SCAN_DIR_ABS=$(readlink -f "$SCAN_DIR_INPUT")
elif command -v realpath &> /dev/null; then
    SCAN_DIR_ABS=$(realpath "$SCAN_DIR_INPUT")
else
    # 兼容无 readlink 的情况
    if [[ "$SCAN_DIR_INPUT" = /* ]]; then
        SCAN_DIR_ABS="$SCAN_DIR_INPUT"
    else
        SCAN_DIR_ABS="$PWD/$SCAN_DIR_INPUT"
    fi
fi

# --- 解析环境变量 ---

# 解析文件排除规则
if [ -n "$EXCLUDE_SCAN_FILES" ]; then
    for pattern in $EXCLUDE_SCAN_FILES; do
        EXCLUDE_FILE_ARGS+=("--exclude=$pattern")
    done
fi

# 解析目录排除规则
if [ -n "$EXCLUDE_SCAN_DIRS" ]; then
    for pattern in $EXCLUDE_SCAN_DIRS; do
        EXCLUDE_DIR_ARGS+=("--exclude-dir=$pattern")
    done
fi

# --- 功能函数 ---

func_init() {
    echo "当前工作目录: $PWD"
    echo "扫描目标目录: $SCAN_DIR_ABS"

    # 校验扫描目录是否存在
    if [ ! -d "$SCAN_DIR_ABS" ]; then
        echo "错误: 扫描目录 [$SCAN_DIR_INPUT] 不存在"
        exit 1
    fi
    
    # 打印排除规则
    if [ ${#EXCLUDE_FILE_ARGS[@]} -gt 0 ]; then
        echo "排除文件规则: ${EXCLUDE_FILE_ARGS[*]}"
    fi
    if [ ${#EXCLUDE_DIR_ARGS[@]} -gt 0 ]; then
        echo "排除目录规则: ${EXCLUDE_DIR_ARGS[*]}"
    fi
    
    # 清空或创建队列文件
    > "$QUEUE_FILE"

    # 构建 grep 参数
    # 1. 基础参数：递归、排除.git目录（默认）、排除队列文件本身
    local base_opts=(-r --exclude-dir=.git --exclude="$QUEUE_FILENAME")

    # 执行搜索
    # -l: 显示文件名
    # -L: 显示空文件名
    # 结合用户自定义的排除参数
    
    # 查找非空文件
    # 使用 realpath/readlink 确保输出绝对路径
    if command -v realpath &> /dev/null; then
        grep -l "${base_opts[@]}" "${EXCLUDE_FILE_ARGS[@]}" "${EXCLUDE_DIR_ARGS[@]}" "" "$SCAN_DIR_ABS" 2>/dev/null | xargs realpath 2>/dev/null >> "$QUEUE_FILE"
    elif command -v readlink &> /dev/null; then
        grep -l "${base_opts[@]}" "${EXCLUDE_FILE_ARGS[@]}" "${EXCLUDE_DIR_ARGS[@]}" "" "$SCAN_DIR_ABS" 2>/dev/null | xargs readlink -f 2>/dev/null >> "$QUEUE_FILE"
    else
        # 兼容无 realpath/readlink 的情况，假设 grep 输出已为相对路径
        grep -l "${base_opts[@]}" "${EXCLUDE_FILE_ARGS[@]}" "${EXCLUDE_DIR_ARGS[@]}" "" "$SCAN_DIR_ABS" 2>/dev/null | while read -r file; do
            if [[ "$file" = /* ]]; then
                echo "$file"
            else
                echo "$PWD/$file"
            fi
        done >> "$QUEUE_FILE"
    fi
    
    # 查找空文件
    # grep -L "${base_opts[@]}" "${EXCLUDE_FILE_ARGS[@]}" "${EXCLUDE_DIR_ARGS[@]}" "" "$SCAN_DIR_ABS" 2>/dev/null >> "$QUEUE_FILE"
    
    # 排序去重
    sort -u "$QUEUE_FILE" -o "$QUEUE_FILE"
    
    # 统计
    local total_count=$(wc -l < "$QUEUE_FILE")
    local suffix_stats=$(awk -F. '{if(NF>1) print$NF}' "$QUEUE_FILE" | sort | uniq -c | sort -rn | head -n 10)
    
    echo "----------------------------------------"
    echo "文件总数: $total_count"
    echo ""
    echo "文件后缀统计 (Top 10):"
    echo "$suffix_stats"
    echo "----------------------------------------"
    echo "队列文件已生成: $QUEUE_FILE"
}


# add: 手动添加路径到队列
func_add() {
    if [ $# -eq 0 ]; then
        echo "错误: add 命令需要至少指定一个路径"
        usage
    fi

    local added_total=0
    
    for item in "$@"; do
        # 1. 判断是否为带行号范围的路径（格式: <文件路径>:<开始行>:<行数>）
        # 使用末尾匹配来识别，确保最后两部分都是数字
        if [[ "$item" =~ ^(.+):[0-9]+:[0-9]+$ ]]; then
            # --- 带行号范围的路径 ---
            # 使用参数扩展提取最后两部分（行号和行数）
            local remainder="$item"
            local line_count="${remainder##*:}"
            remainder="${remainder%:*}"
            local start_line="${remainder##*:}"
            local file_path="${remainder%:*}"
            
            # 转换文件路径为绝对路径
            local abs_path=""
            if command -v readlink &> /dev/null; then
                abs_path=$(readlink -f "$file_path")
            elif command -v realpath &> /dev/null; then
                abs_path=$(realpath "$file_path")
            else
                if [[ "$file_path" = /* ]]; then
                    abs_path="$file_path"
                else
                    abs_path="$PWD/$file_path"
                fi
            fi
            
            # 校验路径是否存在
            if [ ! -e "$abs_path" ]; then
                echo "警告: 路径 [$item] 不存在，已跳过"
                continue
            fi
            
            # 将带行号范围的路径写入队列
            path_with_range="${abs_path}:${start_line}:${line_count}"
            echo "$path_with_range" >> "$QUEUE_FILE"
            ((added_total++))
            
        elif [ -f "$item" ] || [ -d "$item" ] || (echo "$item" | grep -q ':' && [ ! -e "$item" ]); then
            # --- 纯文件或目录路径 ---
            # 注意：这里检查参数本身是否合法，而不是先转绝对路径再检查
            # 这样会导致带行号范围的参数被错误地当作"不存在"而跳过
            # 所以上面已经提前用正则匹配捕获了带行号范围的格式
            
            # 1. 转换为绝对路径
            local abs_path=""
            if command -v readlink &> /dev/null; then
                abs_path=$(readlink -f "$item")
            elif command -v realpath &> /dev/null; then
                abs_path=$(realpath "$item")
            else
                if [[ "$item" = /* ]]; then
                    abs_path="$item"
                else
                    abs_path="$PWD/$item"
                fi
            fi
            
            # 2. 校验路径是否存在
            if [ ! -e "$abs_path" ]; then
                echo "警告: 路径 [$item] 不存在，已跳过"
                continue
            fi
            
            # 3. 根据类型处理
            if [ -f "$abs_path" ]; then
                # --- 是文件 ---
                # 防止将队列文件本身加入队列
                if [ "$abs_path" == "$QUEUE_FILE" ]; then
                    continue
                fi
                echo "$abs_path" >> "$QUEUE_FILE"
                ((added_total++))
                
            elif [ -d "$abs_path" ]; then
                # --- 是目录 ---
                # 使用 find 递归查找目录下的所有文件 (-type f)
                # 排除队列文件本身
                # tee -a 追加写入并统计数量
                # 使用 realpath/readlink 确保输出绝对路径
                echo "正在扫描目录 [$item] ..."
                local count=$(find "$abs_path" -type f ! -path "$QUEUE_FILE" 2>/dev/null | while read -r file; do
                    if command -v realpath &> /dev/null; then
                        realpath "$file" 2>/dev/null || echo "$file"
                    elif command -v readlink &> /dev/null; then
                        readlink -f "$file" 2>/dev/null || echo "$file"
                    else
                        if [[ "$file" = /* ]]; then
                            echo "$file"
                        else
                            echo "$PWD/$file"
                        fi
                    fi
                done | tee -a "$QUEUE_FILE" | wc -l)
                if [ "$count" -gt 0 ]; then
                    echo "已添加 $count 个文件"
                    ((added_total+=count))
                else
                    echo "提示: 目录 [$item] 下未找到文件"
                fi
            else
                echo "警告: [$item] 不是普通文件或目录，已跳过"
            fi
        else
            echo "警告: 路径 [$item] 不存在，已跳过"
        fi
    done

    # 4. 统一去重
    if [ $added_total -gt 0 ]; then
        sort -u "$QUEUE_FILE" -o "$QUEUE_FILE"
        echo "----------------------------------------"
        echo "总计新增: $added_total"
        echo "队列总数: $(wc -l < "$QUEUE_FILE")"
    fi
}

func_status() {
    if [ ! -f "$QUEUE_FILE" ]; then
        echo "提示: 当前目录 ($PWD) 下未找到队列文件"
        return 1
    fi
    
    local count=$(wc -l < "$QUEUE_FILE")
    echo "当前工作目录: $PWD"
    echo "剩余文件数量: $count"
}

func_pop() {
    if [ ! -f "$QUEUE_FILE" ]; then
        echo "错误: 当前目录 ($PWD) 下队列为空或未初始化"
        return 1
    fi
    
    if [ $(wc -l < "$QUEUE_FILE") -eq 0 ]; then
        echo "队列为空，无法弹出"
        return 1
    fi

    # head 取第一行
    local entry=$(head -n 1 "$QUEUE_FILE")
    
    # 判断是否为 diff 格式 (是否包含两个冒号)
    if [[ "$entry" =~ ^(.+):([0-9]+):([0-9]+)$ ]]; then
        # --- diff 格式：绝对路径:起始行:行数 ---
        local file_path="${BASH_REMATCH[1]}"
        local start_line="${BASH_REMATCH[2]}"
        local line_count="${BASH_REMATCH[3]}"
        
        echo "下一个审计文件路径：\`$file_path\`"
        echo "> [Diff 范围] 起始行: ${start_line}, 行数: ${line_count}"

        if [ $line_count -eq 0 ]; then 
            echo "> [Hint] 行数为0，意味着仅删除了该行附近代码，未新增代码，应谨慎检查函数逻辑"
        fi
        
        # 计算相对路径
        local rel_path=""
        if [[ "$file_path" == "$SCAN_DIR_ABS"* ]]; then
            rel_path="${file_path#$SCAN_DIR_ABS/}"
        elif [[ "$file_path" == "$PWD"* ]]; then
            rel_path="${file_path#$PWD/}"
        else
            rel_path=$(basename "$file_path")
        fi
        rel_path="${rel_path#/}"
        rel_path="${rel_path#./}"
        
        echo "> [Hint] 该待审计文件对应的中间报告输出路径为: \`<输出目录>/${rel_path}-L${start_line}-report-0.json\`"
    else
        # --- 普通格式：绝对路径 ---
        local file_path="$entry"
        
        echo "下一个审计文件路径：\`$file_path\`"
        
        # 计算相对路径
        local rel_path=""
        if [[ "$file_path" == "$SCAN_DIR_ABS"* ]]; then
            rel_path="${file_path#"$SCAN_DIR_ABS"/}"
        elif [[ "$file_path" == "$PWD"* ]]; then
            rel_path="${file_path#$PWD/}"
        else
            rel_path=$(basename "$file_path")
        fi
        rel_path="${rel_path#/}"
        rel_path="${rel_path#./}"
        
        echo "> [Hint] 该待审计文件对应的中间报告输出路径为: \`<输出目录>/${rel_path}-report-0.json\`"
    fi
    
    # tail 删除第一行
    tail -n +2 "$QUEUE_FILE" > "${QUEUE_FILE}.tmp"
    mv "${QUEUE_FILE}.tmp" "$QUEUE_FILE"
}

# diff: 将 git diff 的结果(路径+行范围)加入队列
func_diff() {
    # 1. 检查 git 环境
    if ! git rev-parse --is-inside-work-tree > /dev/null 2>&1; then
        echo "错误: 当前目录不在 git 仓库中"
        exit 1
    fi

    # 2. 获取 Git 根目录
    local git_root
    git_root=$(git rev-parse --show-toplevel)

    # 3. 执行 git diff
    # --unified=0 : 紧凑输出，只要变更行，不要上下文
    # --diff-filter=d : 排除已删除文件 (因为我们要的是修改后文件的路径)
    # 注意：暂不使用 --diff-filter=d，因为有些重命名场景需要判断，我们在代码里过滤 /dev/null
    local diff_output
    diff_output=$(git diff --unified=0 "$@" 2>/dev/null)

    if [ -z "$diff_output" ]; then
        echo "未检测到变更内容。"
        return 0
    fi

    # 4. 解析输出
    local count=0
    local current_file=""
    
    # 清空队列
    > "$QUEUE_FILE"

    while IFS= read -r line; do
        # --- A. 解析文件路径 ---
        # 匹配: +++ b/README.md
        # 提取: README.md
        if [[ "$line" =~ ^\+\+\+\ b/(.*) ]]; then
            local rel_path="${BASH_REMATCH[1]}"
            
            # 如果是 /dev/null (出现在删除文件场景，但我们通常不处理删除，忽略)
            if [[ "$rel_path" == "/dev/null" ]]; then
                current_file=""
                continue
            else
                # 拼接绝对路径
                current_file="${git_root}/${rel_path}"
                
                # 二次校验文件存在性
                if [ ! -f "$current_file" ]; then
                    current_file=""
                fi
            fi
        fi

        # --- B. 解析行范围 ---
        # 匹配: @@ -old_start,old_count +new_start,new_count @@
        # 正则说明: 
        # @@ [^@]*  : 匹配 @@ 和中间可能有旧文件信息
        # \+([0-9]+) : 匹配加号后的起始行号
        # (,([0-9]+))? : 匹配可选的逗号和行数
        # \s*@@     : 匹配结尾的 @@
        if [[ -n "$current_file" && "$line" =~ @@\ [^@]*\ \+([0-9]+)(,([0-9]+))?\ @@ ]]; then
            local start_line="${BASH_REMATCH[1]}"
            local line_count="${BASH_REMATCH[3]}"
            
            # 如果没有逗号，表示行数为1
            if [ -z "$line_count" ]; then
                line_count=1
            fi
            
            # 写入队列: 绝对路径:起始行:行数
            echo "${current_file}:${start_line}:${line_count}" >> "$QUEUE_FILE"
            ((count++))
        fi

    done <<< "$diff_output"

    echo "----------------------------------------"
    echo "已写入 $count 个变更块到队列"
    echo "队列文件: $QUEUE_FILE"
    echo "----------------------------------------"
    
}

func_clean() {
    if [ ! -f "$QUEUE_FILE" ]; then
        echo "错误: 当前目录 ($PWD) 下队列为空或未初始化"
        return 1
    fi
    
    if [ $(wc -l < "$QUEUE_FILE") -eq 0 ]; then
        rm "$QUEUE_FILE" && echo "队列清理完成" || echo "清理失败" && return 1
        return 1
    fi

    local count=$(wc -l < "$QUEUE_FILE")
    echo "剩余待审计文件数量: $count， 无法清理"
}

# --- 主逻辑分发 ---
case "$CMD" in
    init)
        func_init
        ;;
    status)
        func_status
        ;;
    push)
        # 将 "add" 之后的所有参数传递给 func_add
        # shift 移除第一个参数($1/cmd)，剩下的$@ 就是文件列表
        shift 
        func_add "$@"
        ;;
    pop)
        func_pop
        ;;
    diff)
        # shift 移除 'diff' 命令，将剩余参数传给 func_diff
        shift
        func_diff "$@"
        ;;
    clean)
        func_clean
        ;;
    *)
        echo "错误: 未知命令 '$CMD'"
        usage
        ;;
esac
