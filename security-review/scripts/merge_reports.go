package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// --- 帮助信息 ---
func usage() {
	fmt.Println("用法: merge_reports <报告输出目录>")
	fmt.Println("")
	fmt.Println("功能:")
	fmt.Println("  查找指定目录下所有漏洞 JSON 文件，合并为一个 full_report.jsonl 文件")
	fmt.Println("")
	fmt.Println("参数:")
	fmt.Println("  报告输出目录 : 包含各个漏洞 JSON 文件的目标目录")
	fmt.Println("")
	fmt.Println("输出:")
	fmt.Println("  full_report.jsonl : 合并后的完整 JSONL 报告文件")
	fmt.Println("")
	fmt.Println("说明:")
	fmt.Println("  1. 查找所有匹配 *-report-*.json 的文件")
	fmt.Println("  2. 自动去除文件中的 ```json 包裹标记")
	fmt.Println("  3. 去除所有换行符，每个 JSON 对象作为一行写入")
	fmt.Println("  4. 生成标准的 JSONL 格式（每行一个 JSON 对象）")
	fmt.Println("  5. 不合并 full_report.jsonl 自身")
	os.Exit(1)
}

func main() {
	// 参数解析
	if len(os.Args) < 2 {
		usage()
	}

	reportDir := os.Args[1]
	outputFile := filepath.Join(reportDir, "full_report.jsonl")

	// 验证报告目录是否存在
	if _, err := os.Stat(reportDir); os.IsNotExist(err) {
		fmt.Printf("错误: 报告目录 [%s] 不存在\n", reportDir)
		os.Exit(1)
	}

	// 验证目录是否可读
	if !isReadable(reportDir) {
		fmt.Printf("错误: 报告目录 [%s] 不可读\n", reportDir)
		os.Exit(1)
	}

	// 验证目录是否可写（用于创建 full_report.jsonl）
	if !isWritable(reportDir) {
		fmt.Printf("错误: 报告目录 [%s] 不可写\n", reportDir)
		os.Exit(1)
	}

	fmt.Println("正在合并报告...")
	fmt.Printf("报告目录: %s\n", reportDir)
	fmt.Printf("输出文件: %s\n", outputFile)
	fmt.Println("")

	// 查找所有漏洞 JSON 文件（排除 full_report.jsonl 自身）
	jsonFiles, err := findJsonFiles(reportDir)
	if err != nil {
		fmt.Printf("错误: 查找文件失败: %v\n", err)
		os.Exit(1)
	}

	// 检查是否找到文件
	if len(jsonFiles) == 0 {
		fmt.Printf("警告: 在目录 [%s] 中未找到任何 .json 文件\n", reportDir)
		os.Exit(0)
	}

	fmt.Printf("找到 %d 个报告文件\n", len(jsonFiles))
	fmt.Println("")

	// 清空输出文件
	output, err := os.Create(outputFile)
	if err != nil {
		fmt.Printf("错误: 创建输出文件失败: %v\n", err)
		os.Exit(1)
	}
	defer output.Close()

	// 合并所有 JSON 文件为 JSONL 格式
	for _, jsonFile := range jsonFiles {
		// 读取文件内容
		content, err := ioutil.ReadFile(jsonFile)
		if err != nil {
			fmt.Printf("警告: 读取文件失败 [%s]: %v\n", jsonFile, err)
			continue
		}

		// 转换为字符串
		contentStr := string(content)

		// 去除可能的 ```json``` 包裹标记
		contentStr = removeJsonMarkers(contentStr)

		// 去除所有换行符，转换为单行 JSON
		contentStr = removeNewlines(contentStr)

		// 写入输出文件作为一行
		if _, err := fmt.Fprintln(output, contentStr); err != nil {
			fmt.Printf("警告: 写入文件失败 [%s]: %v\n", jsonFile, err)
			continue
		}
	}

	fmt.Println("")
	fmt.Println("----------------------------------------")
	fmt.Println("报告合并完成!")
	fmt.Printf("合并后文件: %s\n", outputFile)
	fmt.Printf("合并文件数: %d\n", len(jsonFiles))
	fmt.Println("----------------------------------------")

	os.Exit(0)
}

// isReadable 检查目录是否可读
func isReadable(path string) bool {
	// 尝试打开目录
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// isWritable 检查目录是否可写
func isWritable(path string) bool {
	// 尝试在目录中创建临时文件
	testFile := filepath.Join(path, ".write_test")
	f, err := os.Create(testFile)
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(testFile)
	return true
}

// findJsonFiles 查找目录下所有匹配的 JSON 文件
func findJsonFiles(dir string) ([]string, error) {
	var files []string

	// 使用正则匹配 *-report-*.json
	pattern := regexp.MustCompile(`-report-.*\.json$`)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// 只处理文件
		if !info.IsDir() {
			// 排除 full_report.jsonl
			if info.Name() == "full_report.jsonl" {
				return nil
			}

			// 检查是否匹配 *-report-*.json
			if pattern.MatchString(info.Name()) {
				files = append(files, path)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// 排序
	sort.Strings(files)

	return files, nil
}

// removeJsonMarkers 去除 ```json``` 包裹标记
func removeJsonMarkers(content string) string {
	// 去除开头的 ```json 和可能的空格
	pattern := regexp.MustCompile("(?i)^```json\\s*")
	content = pattern.ReplaceAllString(content, "")

	// 去除结尾的 ``` 和可能的空白
	pattern = regexp.MustCompile("(?i)\\s*```\\s*$")
	content = pattern.ReplaceAllString(content, "")

	return content
}

// removeNewlines 去除所有换行符
func removeNewlines(content string) string {
	// 去除 \r\n 和 \n
	content = strings.ReplaceAll(content, "\r\n", "")
	content = strings.ReplaceAll(content, "\n", "")
	// 去除 \r
	content = strings.ReplaceAll(content, "\r", "")

	return content
}
