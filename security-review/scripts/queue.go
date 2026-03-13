package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// 常量定义
const (
	queueFilename     = ".queue"
	defaultExcludeDir = ".git"   // 默认排除的目录
	defaultExcludeFile = ".queue" // 默认排除的文件
)

// StoredFilePath 表示队列存储格式的文件路径（适配 <路径>:<开始行>:<行数>）
type StoredFilePath struct {
	FilePath string // 纯文件绝对路径
	StartLine int   // 开始行号（0 = 无行号）
	LineCount int   // 行数（0 = 无行号）
	RawStored string// 存储的原始字符串（<路径> 或 <路径>:<开始行>:<行数>）
	RawInput  string// 用户输入的原始字符串（用于日志）
}

// ReadableRange 返回可读的行范围字符串（如 "10-20" 或 "无"）
func (s *StoredFilePath) ReadableRange() string {
	if s.StartLine == 0 || s.LineCount == 0 {
		return "无"
	}
	endLine := s.StartLine + s.LineCount - 1
	if s.LineCount == 1 {
		return fmt.Sprintf("%d", s.StartLine)
	}
	return fmt.Sprintf("%d-%d", s.StartLine, endLine)
}

// EndLine 返回结束行号（计算值）
func (s *StoredFilePath) EndLine() int {
	if s.StartLine == 0 || s.LineCount == 0 {
		return 0
	}
	return s.StartLine + s.LineCount - 1
}

// Config 存储全局配置
type Config struct {
	excludeFiles []string // 文件排除规则（glob 模式）
	excludeDirs  []string // 目录排除规则（glob 模式）
	queueFile    string   // 队列文件绝对路径
	scanDir      string   // 扫描目录绝对路径
}

// 主函数：解析参数并分发子命令
func main() {
	// 初始化配置
	cfg := &Config{
		queueFile: filepath.Join(getCwd(), queueFilename),
	}

	// 解析命令行参数
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "init":
		// init 命令：扫描目录生成队列（纯路径格式）
		scanDirInput := "."
		if len(os.Args) >= 3 {
			scanDirInput = os.Args[2]
		}
		absDir, err := filepath.Abs(scanDirInput)
		if err != nil {
			fmt.Printf("错误：解析扫描目录失败 - %v\n", err)
			os.Exit(1)
		}
		cfg.scanDir = absDir
		loadExcludeFromEnv(cfg)
		if err := initCmd(cfg); err != nil {
			fmt.Printf("init 命令执行失败：%v\n", err)
			os.Exit(1)
		}

	case "status":
		// status 命令：查看队列剩余数量
		if err := statusCmd(cfg); err != nil {
			fmt.Printf("status 命令执行失败：%v\n", err)
			os.Exit(1)
		}

	case "push":
		// push 命令：追加文件/目录/带行号路径到队列
		if len(os.Args) < 3 {
			fmt.Println("错误：push 命令需要指定至少一个文件/目录/带行号路径")
			usage()
			os.Exit(1)
		}
		paths := os.Args[2:]
		loadExcludeFromEnv(cfg)
		if err := pushCmd(cfg, paths); err != nil {
			fmt.Printf("push 命令执行失败：%v\n", err)
			os.Exit(1)
		}

	case "pop":
		// pop 命令：弹出并输出结构化信息（适配 LLM）
		if err := popCmd(cfg); err != nil {
			fmt.Printf("pop 命令执行失败：%v\n", err)
			os.Exit(1)
		}

	case "diff":
		// diff 命令：将 git diff 结果（转换为存储格式）加入队列
		gitDiffArgs := os.Args[2:]
		if err := diffCmd(cfg, gitDiffArgs); err != nil {
			fmt.Printf("diff 命令执行失败：%v\n", err)
			os.Exit(1)
		}

	case "clean":
		// clean 命令：清空队列
		if err := cleanCmd(cfg); err != nil {
			fmt.Printf("clean 命令执行失败：%v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Printf("错误：未知命令 '%s'\n", cmd)
		usage()
		os.Exit(1)
	}
}

// --- 基础工具函数 ---

// usage 打印帮助信息（适配新格式说明）
func usage() {
	progName := filepath.Base(os.Args[0])
	fmt.Printf("用法: %s <命令> [扫描目录]\n", progName)
	fmt.Println()
	fmt.Println("命令:")
	fmt.Println("  init   : 扫描指定目录，生成 .queue 文件到当前工作目录（纯路径格式）")
	fmt.Println("  status : 查看当前工作目录下队列的剩余数量")
	fmt.Println("  push <路径1> [路径2...]: 将指定的文件/目录/带行号路径追加到队列")
	fmt.Println("           支持的行号格式：")
	fmt.Println("             - <路径>:<开始>-<结束> （如 /a/b.go:10-20）")
	fmt.Println("             - <路径>:<开始>:<结束> （如 /a/b.go:10:20）")
	fmt.Println("  pop    : 弹出并输出结构化文件信息（适配 LLM，包含行号/范围详情）")
	fmt.Println("  diff [git diff 参数...] : 将 git diff 的结果（转换为 <路径>:<开始>:<行数>）加入队列")
	fmt.Println("  clean  : 清空队列文件")
	fmt.Println()
	fmt.Println("环境变量:")
	fmt.Println("  EXCLUDE_SCAN_FILES : 设置空格分隔的 glob 模式列表，用于排除文件 (如 '*.o *.log')")
	fmt.Println("  EXCLUDE_SCAN_DIRS  : 设置空格分隔的 glob 模式列表，用于排除目录 (如 'node_modules .git')")
	fmt.Println()
	fmt.Println("注意:")
	fmt.Println("  扫描目录支持相对路径（基于当前工作目录）。")
	fmt.Println("  队列文件 (.queue) 始终生成在当前工作目录下。")
	fmt.Println("  队列存储格式：纯路径 或 <绝对路径>:<开始行>:<行数>")
}

// getCwd 获取当前工作目录（带错误处理）
func getCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("错误：获取当前工作目录失败 - %v\n", err)
		os.Exit(1)
	}
	return cwd
}

// loadExcludeFromEnv 从环境变量加载排除规则
func loadExcludeFromEnv(cfg *Config) {
	// 解析文件排除规则
	if excludeFiles := os.Getenv("EXCLUDE_SCAN_FILES"); excludeFiles != "" {
		cfg.excludeFiles = strings.Fields(excludeFiles)
	}
	// 解析目录排除规则
	if excludeDirs := os.Getenv("EXCLUDE_SCAN_DIRS"); excludeDirs != "" {
		cfg.excludeDirs = strings.Fields(excludeDirs)
	}
	// 追加默认排除规则（确保不被覆盖）
	cfg.excludeFiles = append(cfg.excludeFiles, defaultExcludeFile)
	cfg.excludeDirs = append(cfg.excludeDirs, defaultExcludeDir)
}

// parseInputToStored 解析用户输入的路径（修复 Windows 带行号路径解析错误）
// 支持的输入格式：
//   1. 纯路径：C:\test\file.txt /a/b.txt
//   2. 行范围（-分隔，开始-结束）：C:\test\file.txt:10-20 /a/b.txt:10-20
//   3. 行范围（:分隔，开始:行数）：C:\test\file.txt:10:20 /a/b.txt:10:20
// 注意：不支持单行号格式（如 file.txt:10）
func parseInputToStored(rawInput string) (StoredFilePath, error) {
	result := StoredFilePath{
		RawInput: rawInput,
	}

	// 正则表达式预编译
	// 匹配行号格式：冒号后跟数字和分隔符
	// 格式1: - 分隔（开始-结束）
	lineRangeDashRe := regexp.MustCompile(`:(\d+)-(\d+)$`)
	// 格式2: : 分隔（开始:行数）
	lineRangeColonRe := regexp.MustCompile(`:(\d+):(\d+)$`)
	// 匹配单行号（不支持，检测用）
	singleLineRe := regexp.MustCompile(`:(\d+)$`)

	// ========== 步骤1：检测并提取行号部分 ==========
	var (
		pathPartRaw string   // 原始文件路径部分
		linePart    string   // 行号/范围部分
		startLine   int
		lineCount   int
		err         error
	)

	// 尝试匹配 - 分隔格式（如 file.txt:10-20）
	if matches := lineRangeDashRe.FindStringSubmatch(rawInput); matches != nil {
		// 找到 - 分隔格式：开始-结束
		pathPartRaw = rawInput[:len(rawInput)-len(matches[0])]  // 移除行号部分得到路径
		linePart = matches[0][1:]  // 去掉开头的冒号，得到 "10-20"

		startLine, err = strconv.Atoi(matches[1])
		if err != nil {
			return result, fmt.Errorf("解析开始行号失败（行号：%s）：%v", linePart, err)
		}
		endLine, err := strconv.Atoi(matches[2])
		if err != nil {
			return result, fmt.Errorf("解析结束行号失败（行号：%s）：%v", linePart, err)
		}
		// 校验行号合法性
		if startLine <= 0 || endLine < startLine {
			return result, fmt.Errorf("行号范围不合法（开始行%d，结束行%d）", startLine, endLine)
		}
		lineCount = endLine - startLine + 1
	} else if matches := lineRangeColonRe.FindStringSubmatch(rawInput); matches != nil {
		// 找到 : 分隔格式（如 file.txt:10:20）
		pathPartRaw = rawInput[:len(rawInput)-len(matches[0])]
		linePart = matches[0][1:]  // 得到 "10:20"

		startLine, err = strconv.Atoi(matches[1])
		if err != nil {
			return result, fmt.Errorf("解析开始行号失败（行号：%s）：%v", linePart, err)
		}
		lineCount, err = strconv.Atoi(matches[2])
		if err != nil {
			return result, fmt.Errorf("解析行数失败（行号：%s）：%v", linePart, err)
		}
		// 校验行号合法性
		if startLine <= 0 || lineCount <= 0 {
			return result, fmt.Errorf("行号或行数不合法（开始行%d，行数%d）", startLine, lineCount)
		}
	} else if singleLineRe.MatchString(rawInput) {
		// 单行号格式，不支持
		return result, fmt.Errorf("不支持单行号格式（请使用 行范围-结束 或 行范围:行数）：%s", rawInput)
	} else {
		// 无行号：纯路径格式
		absPath, err := filepath.Abs(rawInput)
		if err != nil {
			return result, fmt.Errorf("解析纯路径失败：%v", err)
		}
		result.FilePath = absPath
		result.RawStored = absPath
		return result, nil
	}

	// ========== 步骤2：验证文件路径是否存在 ==========
	absPath, err := filepath.Abs(pathPartRaw)
	if err != nil {
		return result, fmt.Errorf("解析文件路径失败（路径部分：%s）：%v", pathPartRaw, err)
	}
	// 检查路径是否存在（提前校验，避免后续无效解析）
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return result, fmt.Errorf("文件路径不存在：%s", absPath)
	}
	result.FilePath = absPath

	// ========== 步骤3：构建存储格式 ==========
	result.StartLine = startLine
	result.LineCount = lineCount
	result.RawStored = fmt.Sprintf("%s:%d:%d", absPath, startLine, lineCount)

	return result, nil
}

// parseStoredLine 解析队列中存储的行，支持两种格式：
// 1. 纯路径格式：如 "C:\test\file.txt"（含Windows盘符）、"/test/file.txt"
// 2. 带行号格式：如 "C:\test\file.txt:10:5"、"/test/file.txt:10:5"（<路径>:<开始行>:<行数>）
// 核心优化：
// - 移除多余的反斜杠替换，直接处理原始字符串
// - 精准识别Windows盘符冒号，避免纯路径被误判为非法格式
func parseStoredLine(storedLine string) (StoredFilePath, error) {
	// 初始化返回结果，RawStored 固定为原始存储行
	result := StoredFilePath{
		RawStored: storedLine,
	}

	// 步骤1：辅助函数 - 判断是否为Windows盘符开头（如 C: D:）
	isWindowsDrive := func(s string) bool {
		// 匹配规则：长度≥2 + 第2个字符是冒号 + 第1个字符是字母
		if len(s) < 2 {
			return false
		}
		return (s[1] == ':') && ((s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z'))
	}

	// 步骤2：计算「有效冒号」位置（排除Windows盘符的冒号）
	var (
		drivePrefixLen = 0                // 盘符前缀长度（如 C: → 2）
		baseLine       = storedLine       // 排除盘符后的基础字符串
	)
	if isWindowsDrive(storedLine) {
		drivePrefixLen = 2
		baseLine = storedLine[drivePrefixLen:]
	}

	// 步骤3：在基础字符串中查找最后两个冒号的位置（核心逻辑）
	// 最后一个冒号（行号/行数分隔符）
	lastColonInBase := strings.LastIndex(baseLine, ":")
	if lastColonInBase == -1 {
		// 情况1：排除盘符后无任何冒号 → 纯路径格式
		absPath, err := filepath.Abs(storedLine)
		if err != nil {
			return result, fmt.Errorf("解析纯路径失败：%v（原始行：%s）", err, storedLine)
		}
		result.FilePath = absPath
		// 纯路径无行号，StartLine 和 LineCount 保持 0
		return result, nil
	}

	// 第二个最后冒号（路径/开始行分隔符）
	secondLastColonInBase := strings.LastIndex(baseLine[:lastColonInBase], ":")
	if secondLastColonInBase == -1 {
		// 情况2：排除盘符后仅单个冒号 → 纯路径（如 C:\test:file.txt），非带行号格式
		absPath, err := filepath.Abs(storedLine)
		if err != nil {
			return result, fmt.Errorf("解析纯路径失败：%v（原始行：%s）", err, storedLine)
		}
		result.FilePath = absPath
		return result, nil
	}

	// 情况3：排除盘符后有两个及以上冒号 → 带行号格式
	// 3.1 计算原始字符串中冒号的实际位置（还原盘符前缀）
	rawSecondLastColonIdx := drivePrefixLen + secondLastColonInBase
	rawLastColonIdx := drivePrefixLen + lastColonInBase

	// 3.2 拆分路径部分（原始字符串中 0 → 第二个最后冒号）
	pathPart := storedLine[:rawSecondLastColonIdx]
	absPath, err := filepath.Abs(pathPart)
	if err != nil {
		return result, fmt.Errorf("解析路径部分失败：%v（路径部分：%s，原始行：%s）", err, pathPart, storedLine)
	}
	result.FilePath = absPath

	// 3.3 解析开始行号（第二个最后冒号 → 最后冒号 之间的部分）
	startLineStr := storedLine[rawSecondLastColonIdx+1 : rawLastColonIdx]
	startLine, err := strconv.Atoi(startLineStr)
	if err != nil {
		return result, fmt.Errorf("解析开始行号失败：%v（行号部分：%s，原始行：%s）", err, startLineStr, storedLine)
	}

	// 3.4 解析行数（最后冒号之后的部分）
	lineCountStr := storedLine[rawLastColonIdx+1:]
	lineCount, err := strconv.Atoi(lineCountStr)
	if err != nil {
		return result, fmt.Errorf("解析行数失败：%v（行数部分：%s，原始行：%s）", err, lineCountStr, storedLine)
	}

	// 步骤4：行号合法性校验
	if startLine <= 0 {
		return result, fmt.Errorf("开始行号非法（需为正整数）：%d（原始行：%s）", startLine, storedLine)
	}
	if lineCount < 0 {
		return result, fmt.Errorf("行数非法（需为非负整数）：%d（原始行：%s）", lineCount, storedLine)
	}

	// 赋值行号和行数
	result.StartLine = startLine
	result.LineCount = lineCount

	return result, nil
}

// isExcludedFile 判断文件是否被排除（基于 glob 规则）
func isExcludedFile(filePath string, excludePatterns []string) bool {
	filename := filepath.Base(filePath)
	for _, pattern := range excludePatterns {
		matched, err := filepath.Match(pattern, filename)
		if err == nil && matched {
			return true
		}
	}
	return false
}

// isExcludedDir 判断目录是否被排除（基于 glob 规则）
func isExcludedDir(dirPath string, excludePatterns []string) bool {
	dirname := filepath.Base(dirPath)
	for _, pattern := range excludePatterns {
		matched, err := filepath.Match(pattern, dirname)
		if err == nil && matched {
			return true
		}
	}
	return false
}

// deduplicateAndSort 去重并排序队列元素（按存储的原始字符串）
func deduplicateAndSort(elements []StoredFilePath) []StoredFilePath {
	uniqueMap := make(map[string]StoredFilePath)
	for _, elem := range elements {
		uniqueMap[elem.RawStored] = elem
	}

	uniqueSlice := make([]StoredFilePath, 0, len(uniqueMap))
	for _, elem := range uniqueMap {
		uniqueSlice = append(uniqueSlice, elem)
	}

	// 按存储字符串排序
	sort.Slice(uniqueSlice, func(i, j int) bool {
		return uniqueSlice[i].RawStored < uniqueSlice[j].RawStored
	})

	return uniqueSlice
}

// readQueue 读取队列文件并解析为 StoredFilePath 切片
func readQueue(queueFile string) ([]StoredFilePath, error) {
	var elements []StoredFilePath

	// 检查文件是否存在
	if _, err := os.Stat(queueFile); os.IsNotExist(err) {
		return elements, nil
	}

	// 打开文件
	file, err := os.Open(queueFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// 逐行读取
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		elem, err := parseStoredLine(line)
		if err != nil {
			fmt.Printf("警告：解析队列行 '%s' 失败，跳过 - %v\n", line, err)
			continue
		}
		elements = append(elements, elem)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return elements, nil
}

// writeQueue 将 StoredFilePath 切片写入队列文件（覆盖/追加）
func writeQueue(queueFile string, elements []StoredFilePath, appendMode bool) error {
	// 确定文件打开模式
	flag := os.O_WRONLY | os.O_CREATE
	if appendMode {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}

	// 打开文件
	file, err := os.OpenFile(queueFile, flag, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// 逐行写入（存储格式）
	writer := bufio.NewWriter(file)
	for _, elem := range elements {
		_, err := writer.WriteString(elem.RawStored + "\n")
		if err != nil {
			return err
		}
	}
	return writer.Flush()
}

// --- 子命令实现 ---

// initCmd 处理 init 命令逻辑（纯路径格式）
func initCmd(cfg *Config) error {
	fmt.Printf("当前工作目录: %s\n", getCwd())
	fmt.Printf("扫描目标目录: %s\n", cfg.scanDir)

	// 校验扫描目录是否存在
	if _, err := os.Stat(cfg.scanDir); os.IsNotExist(err) {
		return fmt.Errorf("扫描目录 [%s] 不存在", cfg.scanDir)
	}

	// 打印排除规则
	if len(cfg.excludeFiles) > 0 {
		fmt.Printf("排除文件规则: %s\n", strings.Join(cfg.excludeFiles, " "))
	}
	if len(cfg.excludeDirs) > 0 {
		fmt.Printf("排除目录规则: %s\n", strings.Join(cfg.excludeDirs, " "))
	}

	// 清空队列文件
	if err := os.Truncate(cfg.queueFile, 0); err != nil {
		return fmt.Errorf("清空队列文件失败：%v", err)
	}

	// 扫描目录获取所有符合条件的文件（纯路径）
	var files []StoredFilePath
	err := filepath.Walk(cfg.scanDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("警告：访问路径 %s 失败，跳过 - %v\n", path, err)
			return nil
		}

		// 排除目录
		if info.IsDir() {
			if isExcludedDir(path, cfg.excludeDirs) {
				return filepath.SkipDir
			}
			return nil
		}

		// 排除文件
		if isExcludedFile(path, cfg.excludeFiles) {
			return nil
		}

		// 只处理普通文件
		if !info.Mode().IsRegular() {
			return nil
		}

		// 转换为绝对路径并加入列表
		absPath, err := filepath.Abs(path)
		if err != nil {
			fmt.Printf("警告：转换路径 %s 为绝对路径失败，跳过 - %v\n", path, err)
			return nil
		}
		files = append(files, StoredFilePath{
			FilePath:  absPath,
			RawStored: absPath,
		})
		return nil
	})
	if err != nil {
		return fmt.Errorf("扫描目录失败：%v", err)
	}

	// 去重并排序
	files = deduplicateAndSort(files)

	// 写入队列文件
	if err := writeQueue(cfg.queueFile, files, false); err != nil {
		return fmt.Errorf("写入队列文件失败：%v", err)
	}

	// 统计信息（完全对标原 bash 格式）
	totalCount := len(files)
	suffixStats := countFileSuffix(files)

	fmt.Println("----------------------------------------")
	fmt.Printf("文件总数: %d\n", totalCount)
	fmt.Println()
	fmt.Println("文件后缀统计 (Top 10):")
	for _, stat := range suffixStats {
		fmt.Printf("%5d %s\n", stat.count, stat.suffix)
	}
	fmt.Println("----------------------------------------")

	return nil
}

// statusCmd 处理 status 命令逻辑
func statusCmd(cfg *Config) error {
	elements, err := readQueue(cfg.queueFile)
	if err != nil {
		return fmt.Errorf("读取队列文件失败：%v", err)
	}

	fmt.Printf("待审计文件数量: %d\n", len(elements))
	return nil
}

// pushCmd 处理 push 命令逻辑（支持多种行号输入格式）
func pushCmd(cfg *Config, paths []string) error {
	var newElements []StoredFilePath

	// 遍历所有传入的路径
	for _, rawPath := range paths {
		// 解析用户输入为存储格式
		elem, err := parseInputToStored(rawPath)
		if err != nil {
			fmt.Printf("警告：解析路径 '%s' 失败，跳过 - %v\n", rawPath, err)
			continue
		}

		// 检查路径是否存在（仅检查基础文件路径）
		if _, err := os.Stat(elem.FilePath); os.IsNotExist(err) {
			fmt.Printf("警告：路径 '%s' 不存在，跳过\n", elem.FilePath)
			continue
		}

		// 如果是目录：递归扫描（纯路径格式）
		info, err := os.Stat(elem.FilePath)
		if err != nil {
			fmt.Printf("警告：获取路径信息失败，跳过 - %v\n", err)
			continue
		}
		if info.IsDir() {
			// 递归扫描目录
			var dirElements []StoredFilePath
			err := filepath.Walk(elem.FilePath, func(path string, info fs.FileInfo, err error) error {
				if err != nil {
					fmt.Printf("警告：访问目录路径 %s 失败，跳过 - %v\n", path, err)
					return nil
				}
				if info.IsDir() {
					if isExcludedDir(path, cfg.excludeDirs) {
						return filepath.SkipDir
					}
					return nil
				}
				if isExcludedFile(path, cfg.excludeFiles) || !info.Mode().IsRegular() {
					return nil
				}
				absPath, _ := filepath.Abs(path)
				dirElements = append(dirElements, StoredFilePath{
					FilePath:  absPath,
					RawStored: absPath,
				})
				return nil
			})
			if err != nil {
				fmt.Printf("警告：扫描目录 '%s' 失败，跳过 - %v\n", elem.FilePath, err)
				continue
			}
			newElements = append(newElements, dirElements...)
			continue
		}

		// 如果是文件：检查是否被排除
		if isExcludedFile(elem.FilePath, cfg.excludeFiles) {
			fmt.Printf("警告：文件 '%s' 被排除规则命中，跳过\n", elem.FilePath)
			continue
		}

		// 加入新元素列表
		newElements = append(newElements, elem)
	}

	// 读取现有队列
	existingElements, err := readQueue(cfg.queueFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("读取现有队列失败：%v", err)
	}

	// 合并并去重排序
	allElements := append(existingElements, newElements...)
	allElements = deduplicateAndSort(allElements)

	// 写入队列
	if err := writeQueue(cfg.queueFile, allElements, false); err != nil {
		return fmt.Errorf("写入队列失败：%v", err)
	}

	fmt.Printf("成功追加 %d 个新元素到队列，队列总数：%d\n", len(newElements), len(allElements))
	return nil
}

// popCmd 处理 pop 命令逻辑（对齐原 bash 输出格式，区分两种格式）
func popCmd(cfg *Config) error {
	// 读取队列
	elements, err := readQueue(cfg.queueFile)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("队列为空（队列文件不存在）")
		}
		return fmt.Errorf("读取队列失败：%v", err)
	}

	if len(elements) == 0 {
		return fmt.Errorf("队列为空")
	}

	// 弹出第一个元素
	popElem := elements[0]
	remainingElements := elements[1:]

	// ========== 核心逻辑：区分两种格式输出 ==========
	if popElem.StartLine > 0 && popElem.LineCount >= 0 {
		// --- Diff 格式：带两个冒号的格式 ---
		fmt.Printf("下一个审计文件路径：`%s`\n", popElem.FilePath)
		fmt.Printf("> [Diff 范围] 起始行: %d, 行数: %d\n", popElem.StartLine, popElem.LineCount)

		// 行数为0的特殊提示
		if popElem.LineCount == 0 {
			fmt.Println("> [Hint] 行数为0，意味着仅删除了该行附近代码，未新增代码，应谨慎检查函数逻辑")
		}
		
		// 计算相对路径（严格对齐原 bash 逻辑）
		scanDirAbs := cfg.scanDir       // 扫描目录绝对路径（init 时的目录，默认当前目录）
		pwd := getCwd()                 // 当前工作目录
		filePath := popElem.FilePath    // 文件绝对路径
		relPath := ""

		// 1. 优先匹配扫描目录前缀
		if strings.HasPrefix(filePath, scanDirAbs) {
			relPath = strings.TrimPrefix(filePath, scanDirAbs+"/")
		} else if strings.HasPrefix(filePath, pwd) {
			// 2. 匹配当前工作目录前缀
			relPath = strings.TrimPrefix(filePath, pwd+"/")
		} else {
			// 3. 都不匹配则取文件名
			relPath = filepath.Base(filePath)
		}

		// 清理相对路径（去掉开头的/和./）
		relPath = strings.TrimPrefix(relPath, "/")
		relPath = strings.TrimPrefix(relPath, "./")

		// 输出提示信息
		fmt.Printf("> [Hint] 该待审计文件对应的中间报告输出路径为: `<输出目录>/%s-L%d-report-0.json`\n", relPath, popElem.StartLine)
	} else {
		// --- 普通格式：纯绝对路径 ---
		fmt.Printf("下一个审计文件路径：`%s`\n", popElem.FilePath)

		// 计算相对路径（严格对齐原 bash 逻辑）
		scanDirAbs := cfg.scanDir       // 扫描目录绝对路径（init 时的目录，默认当前目录）
		pwd := getCwd()                 // 当前工作目录
		filePath := popElem.FilePath    // 文件绝对路径
		relPath := ""

		// 1. 优先匹配扫描目录前缀
		if strings.HasPrefix(filePath, scanDirAbs) {
			relPath = strings.TrimPrefix(filePath, scanDirAbs+"/")
		} else if strings.HasPrefix(filePath, pwd) {
			// 2. 匹配当前工作目录前缀
			relPath = strings.TrimPrefix(filePath, pwd+"/")
		} else {
			// 3. 都不匹配则取文件名
			relPath = filepath.Base(filePath)
		}

		// 清理相对路径（去掉开头的/和./）
		relPath = strings.TrimPrefix(relPath, "/")
		relPath = strings.TrimPrefix(relPath, "./")

		// 输出提示信息
		fmt.Printf("> [Hint] 该待审计文件对应的中间报告输出路径为: `<输出目录>/%s-report-0.json`\n", relPath)
	}

	// 剩余元素写回队列
	if err := writeQueue(cfg.queueFile, remainingElements, false); err != nil {
		return fmt.Errorf("更新队列失败：%v", err)
	}

	return nil
}

// diffCmd 处理 diff 命令逻辑（解析 git diff 转换为存储格式）
func diffCmd(cfg *Config, gitDiffArgs []string) error {
	// 构建 git diff 命令（追加 --no-color -p 确保能解析行号）
	cmdArgs := append([]string{"diff", "--no-color", "-p"}, gitDiffArgs...)
	cmd := exec.Command("git", cmdArgs...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	// 执行 git diff
	if err := cmd.Run(); err != nil {
		// 忽略 "无差异" 错误（exit code 1）
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			fmt.Println("git diff 未检测到变更文件")
			return nil
		}
		return fmt.Errorf("执行 git diff 失败：%v，错误输出：%s", err, errBuf.String())
	}

	// 解析 git diff 输出（提取文件路径+行号范围）
	diffOutput := outBuf.String()
	elements, err := parseGitDiffToStored(diffOutput)
	if err != nil {
		return fmt.Errorf("解析 git diff 输出失败：%v", err)
	}

	if len(elements) == 0 {
		fmt.Println("git diff 未检测到变更文件")
		return nil
	}

	// 读取现有队列
	existingElements, err := readQueue(cfg.queueFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("读取现有队列失败：%v", err)
	}

	// 合并并去重排序
	allElements := append(existingElements, elements...)
	allElements = deduplicateAndSort(allElements)

	// 写入队列
	if err := writeQueue(cfg.queueFile, allElements, false); err != nil {
		return fmt.Errorf("写入队列失败：%v", err)
	}

	fmt.Printf("成功将 %d 个 git diff 变更元素加入队列，队列总数：%d\n", len(elements), len(allElements))
	return nil
}

// cleanCmd 处理 clean 命令逻辑
func cleanCmd(cfg *Config) error {
	// 检查队列文件是否存在
	if _, err := os.Stat(cfg.queueFile); os.IsNotExist(err) {
		return fmt.Errorf("错误: 当前目录下队列为空或未初始化")
	}

	// 使用 readQueue 读取队列
	elements, err := readQueue(cfg.queueFile)
	if err != nil {
		return fmt.Errorf("读取队列失败：%v", err)
	}

	count := len(elements)
	if count == 0 {
		// 队列为空，删除队列文件
		if err := os.Remove(cfg.queueFile); err != nil {
			return fmt.Errorf("清理失败：%v", err)
		}
		fmt.Println("队列清理完成")
		return nil
	}

	// 队列中有内容，无法清理
	fmt.Printf("剩余待审计文件数量: %d， 无法清理\n", count)
	return nil
}

// --- 辅助函数（git diff 解析/统计）---

// parseGitDiffToStored 解析 git diff -p 输出，转换为存储格式
func parseGitDiffToStored(diffOutput string) ([]StoredFilePath, error) {
	var elements []StoredFilePath

	// 匹配文件头的正则：diff --git a/file b/file
	fileRe := regexp.MustCompile(`diff --git a/(.+) b/(.+)`)
	// 匹配行号的正则：@@ -10,5 +10,5 @@
	lineRe := regexp.MustCompile(`@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

	scanner := bufio.NewScanner(strings.NewReader(diffOutput))
	currentFile := ""

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// 匹配文件头
		fileMatches := fileRe.FindStringSubmatch(line)
		if fileMatches != nil {
			// 获取文件路径（a/file 和 b/file 一致）
			currentFile = fileMatches[1]
			continue
		}

		// 匹配行号范围
		if currentFile == "" {
			continue
		}
		lineMatches := lineRe.FindStringSubmatch(line)
		if lineMatches != nil {
			// 解析新文件行号（+表示新文件）
			startStr := lineMatches[3]
			lenStr := lineMatches[4]

			startLine, err := strconv.Atoi(startStr)
			if err != nil {
				fmt.Printf("警告：解析行号失败，跳过 - %v\n", err)
				continue
			}

			// 计算行数
			lineCount := 1
			if lenStr != "" {
				length, err := strconv.Atoi(lenStr)
				if err != nil {
					fmt.Printf("警告：解析行长度失败，跳过 - %v\n", err)
					continue
				}
				lineCount = length
			}

			// 转换为绝对路径
			absFile, err := filepath.Abs(currentFile)
			if err != nil {
				fmt.Printf("警告：转换路径 %s 为绝对路径失败，跳过 - %v\n", currentFile, err)
				continue
			}

			// 构建存储格式
			storedStr := fmt.Sprintf("%s:%d:%d", absFile, startLine, lineCount)
			elements = append(elements, StoredFilePath{
				FilePath:  absFile,
				StartLine: startLine,
				LineCount: lineCount,
				RawStored: storedStr,
				RawInput:  fmt.Sprintf("git_diff:%s:%d-%d", absFile, startLine, startLine+lineCount-1),
			})

			// 重置当前文件（避免重复解析）
			currentFile = ""
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return elements, nil
}

// SuffixStat 文件后缀统计项
type SuffixStat struct {
	count  int
	suffix string
}

// countFileSuffix 统计文件后缀（Top 10）
func countFileSuffix(elements []StoredFilePath) []SuffixStat {
	suffixMap := make(map[string]int)

	// 统计所有文件路径的后缀
	for _, elem := range elements {
		ext := filepath.Ext(elem.FilePath)
		if ext == "" {
			ext = "(无后缀)"
		} else {
			ext = strings.TrimPrefix(ext, ".")
		}
		suffixMap[ext]++
	}

	// 转换为切片并排序
	stats := make([]SuffixStat, 0, len(suffixMap))
	for suffix, count := range suffixMap {
		stats = append(stats, SuffixStat{count: count, suffix: suffix})
	}

	// 按数量降序排序
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].count > stats[j].count
	})

	// 取 Top 10
	if len(stats) > 10 {
		stats = stats[:10]
	}

	return stats
}