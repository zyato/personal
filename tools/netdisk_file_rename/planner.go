package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	pathpkg "path"
	"sort"
	"strings"
	"unicode/utf8"
)

// FileEntry contains only the metadata needed to build a safe rename plan.
type FileEntry struct {
	FSID  uint64 `json:"fs_id"`
	Path  string `json:"path"`
	Name  string `json:"server_filename"`
	IsDir int    `json:"isdir"`
}

// RenamePlan describes one synchronous, non-overwriting rename operation.
type RenamePlan struct {
	SourcePath string `json:"source_path"`
	OldName    string `json:"old_name"`
	NewName    string `json:"new_name"`
	FSID       uint64 `json:"fs_id,omitempty"`
}

const (
	skipReasonNoRuleMatch      = "未匹配当前重命名规则"
	skipReasonAlreadyCompliant = "已符合预期规范"
	skipReasonNoChange         = "目标名称未变化"
)

// SkippedFile is a non-directory file that is intentionally left unchanged.
type SkippedFile struct {
	Name   string
	Reason string
}

// planFingerprint binds an apply run to the exact reviewed sources and rule.
// NewName is deliberately excluded because rules may generate fresh random IDs.
func planFingerprint(directory, ruleFingerprint string, plans []RenamePlan) string {
	type source struct {
		SourcePath string `json:"source_path"`
		OldName    string `json:"old_name"`
		FSID       uint64 `json:"fs_id"`
	}
	sources := make([]source, 0, len(plans))
	for _, plan := range plans {
		sources = append(sources, source{SourcePath: plan.SourcePath, OldName: plan.OldName, FSID: plan.FSID})
	}
	payload := struct {
		Version   int      `json:"version"`
		Directory string   `json:"directory"`
		Rule      string   `json:"rule"`
		Sources   []source `json:"sources"`
	}{Version: 3, Directory: directory, Rule: ruleFingerprint, Sources: sources}
	raw, _ := json.Marshal(payload)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func normalizeCloudDir(dir string) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" || !strings.HasPrefix(dir, "/apps/") {
		return "", fmt.Errorf("目录必须是 /apps/{应用名} 下的绝对路径")
	}
	if !utf8.ValidString(dir) || strings.ContainsRune(dir, '\x00') || strings.ContainsRune(dir, '\\') {
		return "", fmt.Errorf("目录包含无效字符")
	}
	for _, part := range strings.Split(dir, "/") {
		if part == "." || part == ".." {
			return "", fmt.Errorf("目录不能包含 . 或 .. 路径段")
		}
	}
	cleaned := pathpkg.Clean(dir)
	if cleaned == "/apps" || cleaned == "/apps/" || !strings.HasPrefix(cleaned, "/apps/") {
		return "", fmt.Errorf("必须指定具体的 /apps/{应用名} 目录")
	}
	appName := strings.TrimPrefix(cleaned, "/apps/")
	if slash := strings.IndexByte(appName, '/'); slash >= 0 {
		appName = appName[:slash]
	}
	if appName == "" {
		return "", fmt.Errorf("应用名不能为空")
	}
	return cleaned, nil
}

// BuildPlan plans only direct child files. Existing names, including directories,
// are considered conflicts. Files already in the expected final format,
// unmatched ordinary files, and files whose generated name is unchanged are
// returned as skipped, with separate reasons.
func BuildPlan(entries []FileEntry, dir string, rule RenameRule, maxItems int) ([]RenamePlan, []SkippedFile, error) {
	if rule == nil {
		return nil, nil, fmt.Errorf("重命名规则不能为空")
	}
	if maxItems < 1 {
		return nil, nil, fmt.Errorf("单次重命名上限必须大于 0")
	}

	existing := make(map[string][]string, len(entries))
	plans := make([]RenamePlan, 0)
	skipped := make([]SkippedFile, 0)
	for _, entry := range entries {
		if pathpkg.Dir(entry.Path) != dir {
			return nil, nil, fmt.Errorf("接口返回了目标目录之外的条目: %s", entry.Path)
		}
		baseName := pathpkg.Base(entry.Path)
		if entry.Name != "" && entry.Name != baseName {
			return nil, nil, fmt.Errorf("接口返回的文件名与路径不一致: %s", entry.Path)
		}
		foldedName := foldName(baseName)
		existing[foldedName] = append(existing[foldedName], baseName)
		if entry.IsDir != 0 {
			continue
		}
		if hasExpectedName(rule, baseName) {
			skipped = append(skipped, SkippedFile{Name: baseName, Reason: skipReasonAlreadyCompliant})
			continue
		}
		if !rule.Match(baseName) {
			skipped = append(skipped, SkippedFile{Name: baseName, Reason: skipReasonNoRuleMatch})
			continue
		}
		newName, err := rule.Rename(baseName)
		if err != nil {
			return nil, nil, fmt.Errorf("生成文件 %q 的目标名称失败: %w", baseName, err)
		}
		if newName == baseName {
			skipped = append(skipped, SkippedFile{Name: baseName, Reason: skipReasonNoChange})
			continue
		}
		if err := validateNewName(newName); err != nil {
			return nil, nil, fmt.Errorf("文件 %q 的目标名称无效: %w", baseName, err)
		}
		plans = append(plans, RenamePlan{
			SourcePath: entry.Path,
			OldName:    baseName,
			NewName:    newName,
			FSID:       entry.FSID,
		})
	}

	if len(plans) > maxItems {
		return nil, nil, fmt.Errorf("匹配到 %d 个待重命名文件，超过单次上限 %d；为避免误操作，本次不执行部分计划", len(plans), maxItems)
	}

	targets := make(map[string]string, len(plans))
	for _, plan := range plans {
		foldedTarget := foldName(plan.NewName)
		for _, existingName := range existing[foldedTarget] {
			if existingName != plan.OldName {
				return nil, nil, fmt.Errorf("目标名称 %q 与现有条目 %q 冲突", plan.NewName, existingName)
			}
		}
		if previous, ok := targets[foldedTarget]; ok {
			return nil, nil, fmt.Errorf("目标名称冲突: %q 和 %q 都会重命名为 %q", previous, plan.OldName, plan.NewName)
		}
		targets[foldedTarget] = plan.OldName
	}

	sort.Slice(plans, func(i, j int) bool {
		return plans[i].OldName < plans[j].OldName
	})
	sort.Slice(skipped, func(i, j int) bool {
		if skipped[i].Name == skipped[j].Name {
			return skipped[i].Reason < skipped[j].Reason
		}
		return skipped[i].Name < skipped[j].Name
	})
	return plans, skipped, nil
}

func validateNewName(name string) error {
	if name == "" || name == "." || name == ".." {
		return fmt.Errorf("名称不能为空、. 或 ..")
	}
	if !utf8.ValidString(name) {
		return fmt.Errorf("名称不是有效 UTF-8")
	}
	if utf8.RuneCountInString(name) > 255 {
		return fmt.Errorf("名称超过 255 个字符")
	}
	if strings.TrimSpace(name) != name {
		return fmt.Errorf("名称不能以空白字符开头或结尾")
	}
	if strings.HasSuffix(name, ".") {
		return fmt.Errorf("名称不能以点号结尾")
	}
	if strings.ContainsAny(name, `/\\:*?"<>|`) {
		return fmt.Errorf("名称不能包含 / \\ : * ? \" < > |")
	}
	for _, char := range name {
		if char < 32 || char == 127 {
			return fmt.Errorf("名称不能包含控制字符")
		}
	}
	return nil
}

func foldName(name string) string {
	return strings.ToLower(name)
}
