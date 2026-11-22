package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

const defaultMaxItems = 100

type commandConfig struct {
	directory string
	ruleName  string
	planID    string
	apply     bool
	maxItems  int
	auditPath string
	lockPath  string
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		_, _ = fmt.Fprintf(os.Stderr, "错误：%v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	dataDir, err := locateRuntimeDataDir()
	if err != nil {
		return err
	}
	config := commandConfig{
		auditPath: runtimeAuditLogPath(dataDir),
		lockPath:  runtimeLockPath(dataDir),
	}
	flags := flag.NewFlagSet("local_file_rename", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&config.directory, "dir", "", "本地目录路径，支持相对路径，会解析为实际绝对路径（必填）")
	flags.StringVar(&config.ruleName, "rule", "all", "重命名方案；默认 all 会依次尝试四种内置视频规则")
	flags.BoolVar(&config.apply, "apply", false, "实际执行重命名；必须与 --plan 一起使用")
	flags.StringVar(&config.planID, "plan", "", "预演输出的完整计划指纹；执行时必填")
	flags.IntVar(&config.maxItems, "max-items", defaultMaxItems, "单次最多重命名文件数，1-10000")
	flags.StringVar(&config.auditPath, "audit-log", config.auditPath, "执行审计日志路径")
	flags.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "本地指定目录文件批量重命名（默认预演，不递归，不处理目录）")
		_, _ = fmt.Fprintln(stderr, "\n用法：")
		_, _ = fmt.Fprintln(stderr, "  go run ./tools/local_file_rename --dir '/path/to/directory'")
		_, _ = fmt.Fprintln(stderr, "  默认使用 all 规则：依次尝试 cloudrecord、cloud-video、timestamp-hash、live")
		_, _ = fmt.Fprintln(stderr, "  目标名称格式：YYYYMMDD_HHmmss_SSS_<12位随机HEXID>.原扩展名")
		_, _ = fmt.Fprintln(stderr, "  确认预演结果后追加 --apply --plan '<预演输出的计划指纹>'")
		_, _ = fmt.Fprintln(stderr, "  导出的二进制可在任意目录运行，不要求存在 Go 源码项目")
		_, _ = fmt.Fprintln(stderr, "  运行时数据目录：设置 PERSONAL_ROOT 时为 <PERSONAL_ROOT>/.local_file_rename")
		_, _ = fmt.Fprintln(stderr, "  源码项目内开发运行时自动使用项目根下的 .local_file_rename")
		_, _ = fmt.Fprintln(stderr, "  导出二进制默认使用系统用户配置目录下的 local_file_rename")
		_, _ = fmt.Fprintln(stderr, "\n参数：")
		flags.PrintDefaults()
		_, _ = fmt.Fprintf(stderr, "\n已注册规则：%s\n", strings.Join(availableRuleNames(), ", "))
		_, _ = fmt.Fprintln(stderr, "新增方案：在本目录 Go 文件中实现 RenameRule，并在 init() 中调用 registerRule。")
		_, _ = fmt.Fprintln(stderr, "\n安全约定：")
		_, _ = fmt.Fprintln(stderr, "  默认预演模式不修改任何本地文件。")
		_, _ = fmt.Fprintln(stderr, "  只处理目录下的直接普通文件；不递归，目录、符号链接和特殊文件不会被重命名。")
		_, _ = fmt.Fprintln(stderr, "  执行前重新扫描并核对计划指纹和文件标识；目录或源文件变化时整体中止。")
		_, _ = fmt.Fprintln(stderr, "  计划指纹绑定规则、源文件路径和文件标识；12位随机 HEXID 每次生成都会变化。")
		_, _ = fmt.Fprintln(stderr, "  未匹配任何规则的普通文件会逐条打印跳过日志，不会修改。")
		_, _ = fmt.Fprintln(stderr, "  已符合目标命名格式的普通文件会逐条打印跳过日志，不会重复重命名。")
		_, _ = fmt.Fprintln(stderr, "  生成目标名与原名一致的文件会逐条打印跳过日志，不会修改。")
		_, _ = fmt.Fprintln(stderr, "  单文件串行执行；源文件缺失、被替换或目标已存在时立即停止，不覆盖现有文件。")
		_, _ = fmt.Fprintln(stderr, "  每次执行逐条写入审计日志；请勿并发运行多个实例。")
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("不支持位置参数: %s", strings.Join(flags.Args(), " "))
	}
	rule, err := newRuleByName(config.ruleName)
	if err != nil {
		return err
	}
	if config.apply && strings.TrimSpace(config.planID) == "" {
		return fmt.Errorf("实际执行前必须先预演，并提供 --plan '<预演输出的计划指纹>'")
	}
	if !config.apply && strings.TrimSpace(config.planID) != "" {
		return fmt.Errorf("--plan 只能与 --apply 一起使用")
	}
	if config.maxItems < 1 || config.maxItems > 10000 {
		return fmt.Errorf("--max-items 必须在 1-10000 之间")
	}
	if config.auditPath == "" || config.lockPath == "" || sameFilePath(config.auditPath, config.lockPath) {
		return fmt.Errorf("审计日志与运行锁路径不能为空且不能指向同一文件")
	}
	config.directory, err = normalizeLocalDirectory(config.directory)
	if err != nil {
		return err
	}
	if err := ensureDataDir(dataDir); err != nil {
		return err
	}

	releaseLock, err := acquireProcessLock(config.lockPath)
	if err != nil {
		return err
	}
	defer releaseLock()

	_, _ = fmt.Fprintln(stdout, "正在读取本地目录并生成计划...")
	entries, err := listLocalEntries(config.directory)
	if err != nil {
		return err
	}
	plans, skippedFiles, err := BuildPlan(entries, config.directory, rule, config.maxItems)
	if err != nil {
		return err
	}
	for _, plan := range plans {
		if sameFilePath(config.auditPath, plan.SourcePath) {
			return fmt.Errorf("审计日志路径不能指向待重命名文件 %s", plan.SourcePath)
		}
	}
	currentPlanID := planFingerprint(config.directory, rule.Fingerprint(), plans)
	unmatchedCount := 0
	compliantCount := 0
	unchangedCount := 0
	otherCount := 0
	for _, skipped := range skippedFiles {
		switch skipped.Reason {
		case skipReasonNoRuleMatch:
			unmatchedCount++
		case skipReasonAlreadyCompliant:
			compliantCount++
		case skipReasonNoChange:
			unchangedCount++
		default:
			otherCount++
		}
	}
	_, _ = fmt.Fprintf(stdout, "目录：%s\n规则：%s\n扫描条目：%d；待重命名文件：%d；跳过条目：%d（未匹配规则 %d；已符合规范 %d；无需修改 %d；其他 %d）\n", config.directory, config.ruleName, len(entries), len(plans), len(skippedFiles), unmatchedCount, compliantCount, unchangedCount, otherCount)
	for _, skipped := range skippedFiles {
		_, _ = fmt.Fprintf(stdout, "跳过：%s（%s）\n", skipped.Name, skipped.Reason)
	}
	for index, plan := range plans {
		_, _ = fmt.Fprintf(stdout, "%3d. %s -> %s\n", index+1, plan.OldName, plan.NewName)
	}
	if config.apply && subtle.ConstantTimeCompare([]byte(strings.TrimSpace(config.planID)), []byte(currentPlanID)) != 1 {
		return fmt.Errorf("当前计划指纹与 --plan 不一致；目录、规则或源文件可能已变化，未执行任何重命名，请重新预演")
	}
	if len(plans) == 0 {
		_, _ = fmt.Fprintln(stdout, "没有需要修改的文件，未执行重命名。")
		return nil
	}
	if !config.apply {
		_, _ = fmt.Fprintf(stdout, "计划指纹：%s\n", currentPlanID)
		_, _ = fmt.Fprintln(stdout, "当前为预演模式，未修改本地文件。确认所有目标名称后使用 --apply --plan '<计划指纹>'。")
		return nil
	}

	logger, err := openAuditLogger(config.auditPath)
	if err != nil {
		return err
	}
	defer logger.close()
	for _, plan := range plans {
		if err := logger.write(auditEntry{Action: "rename", SourcePath: plan.SourcePath, NewName: plan.NewName, Status: "planned", PlanID: currentPlanID}); err != nil {
			return fmt.Errorf("计划审计写入失败，尚未执行重命名: %w", err)
		}
	}

	succeeded := 0
	for index, plan := range plans {
		if ctx.Err() != nil {
			entry := auditEntry{Action: "rename", SourcePath: plan.SourcePath, NewName: plan.NewName, Status: "interrupted", PlanID: currentPlanID, Error: "收到退出信号，当前项未执行"}
			if auditErr := logger.write(entry); auditErr != nil {
				return fmt.Errorf("已收到退出信号，已确认成功 %d/%d；审计写入失败: %w", succeeded, len(plans), auditErr)
			}
			return fmt.Errorf("已收到退出信号，已确认成功 %d/%d；当前项未执行", succeeded, len(plans))
		}
		_, _ = fmt.Fprintf(stdout, "[%d/%d] 正在重命名 %s\n", index+1, len(plans), plan.OldName)
		renameErr := renameLocalFile(plan, config.directory)
		if renameErr != nil {
			entry := auditEntry{Action: "rename", SourcePath: plan.SourcePath, NewName: plan.NewName, Status: "failed", PlanID: currentPlanID, Error: renameErr.Error()}
			if auditErr := logger.write(entry); auditErr != nil {
				return fmt.Errorf("已停止，已确认成功 %d/%d；%s；审计写入失败: %w", succeeded, len(plans), renameErr, auditErr)
			}
			return fmt.Errorf("已停止，已确认成功 %d/%d；%s", succeeded, len(plans), renameErr)
		}
		entry := auditEntry{Action: "rename", SourcePath: plan.SourcePath, NewName: plan.NewName, Status: "success", PlanID: currentPlanID}
		if err := logger.write(entry); err != nil {
			return fmt.Errorf("重命名已成功但审计写入失败，已停止，已确认成功 %d/%d: %w", succeeded+1, len(plans), err)
		}
		succeeded++
	}
	_, _ = fmt.Fprintf(stdout, "完成：已确认成功 %d 个，失败 0 个。审计日志：%s\n", succeeded, config.auditPath)
	return nil
}
