package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

// FileEntry contains only the metadata needed to build a safe local rename plan.
// FileID binds a plan to a concrete filesystem object, playing the role of the
// netdisk fs_id so a replaced file cannot silently inherit an old plan.
type FileEntry struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	IsDir       bool   `json:"is_dir"`
	Unsupported bool   `json:"unsupported,omitempty"`
	FileID      string `json:"file_id,omitempty"`
}

// RenamePlan describes one non-overwriting rename operation.
type RenamePlan struct {
	SourcePath string `json:"source_path"`
	OldName    string `json:"old_name"`
	NewName    string `json:"new_name"`
	FileID     string `json:"file_id"`
}

const (
	skipReasonNoRuleMatch      = "未匹配当前重命名规则"
	skipReasonAlreadyCompliant = "已符合预期规范"
	skipReasonNoChange         = "目标名称未变化"
	skipReasonUnsupportedEntry = "仅支持普通文件，已跳过符号链接或特殊文件"
)

// SkippedFile is a non-directory entry that is intentionally left unchanged.
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
		FileID     string `json:"file_id"`
	}
	sources := make([]source, 0, len(plans))
	for _, plan := range plans {
		sources = append(sources, source{SourcePath: plan.SourcePath, OldName: plan.OldName, FileID: plan.FileID})
	}
	payload := struct {
		Version   int      `json:"version"`
		Tool      string   `json:"tool"`
		Directory string   `json:"directory"`
		Rule      string   `json:"rule"`
		Sources   []source `json:"sources"`
	}{Version: 1, Tool: "local_file_rename", Directory: directory, Rule: ruleFingerprint, Sources: sources}
	raw, _ := json.Marshal(payload)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

// BuildPlan plans only direct child regular files. Existing names, including
// directories and unsupported entries, are considered conflicts. Files already
// in the expected final format, unmatched ordinary files, and files whose
// generated name is unchanged are returned as skipped, with separate reasons.
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
		if filepath.Dir(entry.Path) != dir {
			return nil, nil, fmt.Errorf("读取到目标目录之外的条目: %s", entry.Path)
		}
		baseName := filepath.Base(entry.Path)
		if entry.Name != baseName {
			return nil, nil, fmt.Errorf("条目文件名与路径不一致: %s", entry.Path)
		}
		foldedName := foldName(baseName)
		existing[foldedName] = append(existing[foldedName], baseName)
		if entry.IsDir {
			continue
		}
		if entry.Unsupported {
			skipped = append(skipped, SkippedFile{Name: baseName, Reason: skipReasonUnsupportedEntry})
			continue
		}
		if entry.FileID == "" {
			return nil, nil, fmt.Errorf("普通文件 %q 缺少稳定文件标识，已停止", baseName)
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
			FileID:     entry.FileID,
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
