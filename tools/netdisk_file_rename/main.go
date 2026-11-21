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
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const envAccessToken = "BAIDU_NETDISK_ACCESS_TOKEN"

type commandConfig struct {
	directory          string
	ruleName           string
	planID             string
	apply              bool
	maxItems           int
	requestInterval    time.Duration
	maxRequestsPerHour int
	timeout            time.Duration
	statePath          string
	auditPath          string
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
	projectRoot, err := locateProjectRoot()
	if err != nil {
		return err
	}
	if err := loadDotEnv(filepath.Join(projectRoot, ".env")); err != nil {
		return err
	}
	intervalDefault, err := time.ParseDuration(envOrDefault("BAIDU_NETDISK_REQUEST_INTERVAL", "1s"))
	if err != nil {
		return fmt.Errorf("BAIDU_NETDISK_REQUEST_INTERVAL 格式无效: %w", err)
	}
	timeoutDefault, err := time.ParseDuration(envOrDefault("BAIDU_NETDISK_TIMEOUT", "30s"))
	if err != nil {
		return fmt.Errorf("BAIDU_NETDISK_TIMEOUT 格式无效: %w", err)
	}
	maxRequestsDefault, err := strconv.Atoi(envOrDefault("BAIDU_NETDISK_MAX_REQUESTS_PER_HOUR", "3600"))
	if err != nil {
		return fmt.Errorf("BAIDU_NETDISK_MAX_REQUESTS_PER_HOUR 必须是整数")
	}
	maxItemsDefault, err := strconv.Atoi(envOrDefault("BAIDU_NETDISK_MAX_RENAMES", "100"))
	if err != nil {
		return fmt.Errorf("BAIDU_NETDISK_MAX_RENAMES 必须是整数")
	}

	config := commandConfig{
		statePath: filepath.Join(projectRoot, ".netdisk_file_rename_state.json"),
		auditPath: filepath.Join(projectRoot, "netdisk_rename_audit.jsonl"),
	}
	flags := flag.NewFlagSet("netdisk_file_rename", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&config.directory, "dir", "", "百度网盘目录，必须位于 /apps/{应用名} 下（必填）")
	flags.StringVar(&config.ruleName, "rule", "all", "重命名方案；默认 all 会依次尝试四种内置视频规则")
	flags.BoolVar(&config.apply, "apply", false, "实际执行重命名；必须与 --plan 一起使用")
	flags.StringVar(&config.planID, "plan", "", "预演输出的完整计划指纹；执行时必填")
	flags.IntVar(&config.maxItems, "max-items", maxItemsDefault, "单次最多重命名文件数，1-10000")
	flags.DurationVar(&config.requestInterval, "interval", intervalDefault, "两次 API 请求的最小间隔，不得低于 1s")
	flags.IntVar(&config.maxRequestsPerHour, "max-requests-per-hour", maxRequestsDefault, "本地滚动一小时 API 请求预算，1-10000")
	flags.DurationVar(&config.timeout, "timeout", timeoutDefault, "单次 HTTP 请求超时，5s-2m")
	flags.StringVar(&config.auditPath, "audit-log", config.auditPath, "执行审计日志路径")
	flags.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "百度网盘指定目录文件批量重命名（默认预演，不递归，不处理目录）")
		_, _ = fmt.Fprintln(stderr, "\n用法：")
		_, _ = fmt.Fprintln(stderr, "  go run ./tools/netdisk_file_rename --dir '/apps/应用名/目录'")
		_, _ = fmt.Fprintln(stderr, "  默认使用 all 规则：依次尝试 cloudrecord、cloud-video、timestamp-hash、live")
		_, _ = fmt.Fprintln(stderr, "  目标名称格式：YYYYMMDD_HHmmss_SSS_<12位随机HEXID>.原扩展名")
		_, _ = fmt.Fprintln(stderr, "  确认预演结果后追加 --apply --plan '<预演输出的计划指纹>'")
		_, _ = fmt.Fprintln(stderr, "\n参数：")
		flags.PrintDefaults()
		_, _ = fmt.Fprintf(stderr, "\n已注册规则：%s\n", strings.Join(availableRuleNames(), ", "))
		_, _ = fmt.Fprintln(stderr, "新增方案：在本目录 Go 文件中实现 RenameRule，并在 init() 中调用 registerRule。")
		_, _ = fmt.Fprintln(stderr, "\n安全约定：")
		_, _ = fmt.Fprintln(stderr, "  access_token 只从项目根目录 .env 或同名环境变量读取，不支持命令行传密钥。")
		_, _ = fmt.Fprintln(stderr, "  默认 1 秒一次请求、滚动一小时最多 3600 次；未审核应用官方限制为 10 次/小时，请勿直接使用默认值。")
		_, _ = fmt.Fprintln(stderr, "  执行前重新列举并核对计划指纹；目录变化导致源文件计划不同时整体中止。")
		_, _ = fmt.Fprintln(stderr, "  计划指纹绑定规则及源文件；12位随机 HEXID 属于随机目标部分，每次生成都会变化。")
		_, _ = fmt.Fprintln(stderr, "  未匹配任何规则的普通文件会逐条打印跳过日志，不会修改。")
		_, _ = fmt.Fprintln(stderr, "  已符合目标命名格式的普通文件会逐条打印跳过日志，不会重复重命名。")
		_, _ = fmt.Fprintln(stderr, "  生成目标名与原名一致的文件会逐条打印跳过日志，不会修改。")
		_, _ = fmt.Fprintln(stderr, "  单文件同步提交、ondup=fail、串行执行；限流或业务错误立即停止。")
		_, _ = fmt.Fprintln(stderr, "  查询请求仅对网络错误和 HTTP 5xx 做最多 2 次退避重试；重命名请求绝不自动重发。")
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("不支持位置参数: %s", strings.Join(flags.Args(), " "))
	}
	accessToken := strings.TrimSpace(os.Getenv(envAccessToken))
	if accessToken == "" {
		return fmt.Errorf("请在项目根目录 .env 中设置 %s", envAccessToken)
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
	if config.requestInterval < time.Second {
		return fmt.Errorf("--interval 不得低于 1s；请按应用控制台配额设置")
	}
	if config.maxRequestsPerHour < 1 || config.maxRequestsPerHour > 10000 {
		return fmt.Errorf("--max-requests-per-hour 必须在 1-10000 之间")
	}
	if config.timeout < 5*time.Second || config.timeout > 2*time.Minute {
		return fmt.Errorf("--timeout 必须在 5s-2m 之间")
	}
	if config.statePath == "" || config.auditPath == "" || sameFilePath(config.statePath, config.auditPath) {
		return fmt.Errorf("限速状态与审计日志路径不能为空且不能指向同一文件")
	}
	config.directory, err = normalizeCloudDir(config.directory)
	if err != nil {
		return err
	}

	releaseLock, err := acquireProcessLock(config.statePath)
	if err != nil {
		return err
	}
	defer releaseLock()
	limiter, err := newRateLimiter(config.requestInterval, config.maxRequestsPerHour, config.statePath)
	if err != nil {
		return err
	}
	client := NewClient(accessToken, limiter, config.timeout)

	_, _ = fmt.Fprintf(stdout, "正在读取授权目录并生成计划（请求间隔 %s，小时预算 %d）...\n", config.requestInterval, config.maxRequestsPerHour)
	entries, err := client.ListFiles(ctx, config.directory)
	if err != nil {
		return err
	}
	plans, skippedFiles, err := BuildPlan(entries, config.directory, rule, config.maxItems)
	if err != nil {
		return err
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
	_, _ = fmt.Fprintf(stdout, "目录：%s\n规则：%s\n扫描条目：%d；待重命名文件：%d；跳过文件：%d（未匹配规则 %d；已符合规范 %d；无需修改 %d；其他 %d）\n", config.directory, config.ruleName, len(entries), len(plans), len(skippedFiles), unmatchedCount, compliantCount, unchangedCount, otherCount)
	for _, skipped := range skippedFiles {
		_, _ = fmt.Fprintf(stdout, "跳过：%s（%s）\n", skipped.Name, skipped.Reason)
	}
	for index, plan := range plans {
		_, _ = fmt.Fprintf(stdout, "%3d. %s -> %s\n", index+1, plan.OldName, plan.NewName)
	}
	if config.apply && subtle.ConstantTimeCompare([]byte(strings.TrimSpace(config.planID)), []byte(currentPlanID)) != 1 {
		return fmt.Errorf("当前计划指纹与 --plan 不一致；目录或规则可能已变化，未执行任何重命名，请重新预演")
	}
	if len(plans) == 0 {
		_, _ = fmt.Fprintln(stdout, "没有需要修改的文件，未执行重命名。")
		return nil
	}
	if !config.apply {
		_, _ = fmt.Fprintf(stdout, "计划指纹：%s\n", currentPlanID)
		_, _ = fmt.Fprintln(stdout, "当前为预演模式，未修改网盘。确认所有目标名称后使用 --apply --plan '<计划指纹>'。")
		return nil
	}

	if sameFilePath(config.statePath, config.auditPath) {
		return fmt.Errorf("审计日志不能指向限速状态文件")
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
		_, _ = fmt.Fprintf(stdout, "[%d/%d] 正在重命名 %s\n", index+1, len(plans), plan.OldName)
		requestID, renameErr := client.RenameFile(ctx, plan.SourcePath, plan.NewName)
		if renameErr != nil {
			safeMessage := redactSecret(renameErr.Error(), accessToken)
			status := "failed"
			outcomeMessage := "当前项已失败"
			if renameOutcomeUnknown(renameErr) {
				status = "unknown"
				outcomeMessage = "当前项结果未知，请人工核对且不要直接重跑"
			}
			entry := auditEntry{Action: "rename", SourcePath: plan.SourcePath, NewName: plan.NewName, Status: status, PlanID: currentPlanID, RequestID: requestID, Error: safeMessage}
			if auditErr := logger.write(entry); auditErr != nil {
				return fmt.Errorf("已停止，已确认成功 %d/%d；%s；%s；%v", succeeded, len(plans), outcomeMessage, safeMessage, auditErr)
			}
			return fmt.Errorf("已停止，已确认成功 %d/%d；%s；%s", succeeded, len(plans), outcomeMessage, safeMessage)
		}
		entry := auditEntry{Action: "rename", SourcePath: plan.SourcePath, NewName: plan.NewName, Status: "success", PlanID: currentPlanID, RequestID: requestID}
		if err := logger.write(entry); err != nil {
			return fmt.Errorf("重命名已成功但审计写入失败，已停止，已确认成功 %d/%d: %w", succeeded+1, len(plans), err)
		}
		succeeded++
	}
	_, _ = fmt.Fprintf(stdout, "完成：已确认成功 %d 个，失败 0 个。审计日志：%s\n", succeeded, config.auditPath)
	return nil
}

func envOrDefault(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}
