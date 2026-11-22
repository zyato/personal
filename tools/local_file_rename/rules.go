package main

import (
	"fmt"
	"sort"
	"strings"
)

// RenameRule is the extension point for user-defined renaming schemes.
// Implement it in a new Go file in this package, then register a factory with
// registerRule in init(). Fingerprint must change whenever deterministic rule
// behavior changes; generated random suffixes are not part of the plan digest.
type RenameRule interface {
	Match(fileName string) bool
	Rename(fileName string) (string, error)
	Fingerprint() string
}

// expectedNameRule is optional. A rule that implements it can declare files
// already in its final target format so BuildPlan can skip them. Rules with a
// different target format are unaffected.
type expectedNameRule interface {
	IsExpectedName(fileName string) bool
}

func hasExpectedName(rule RenameRule, fileName string) bool {
	matcher, ok := rule.(expectedNameRule)
	return ok && matcher.IsExpectedName(fileName)
}

type RuleFactory func() RenameRule

var ruleFactories = make(map[string]RuleFactory)

func init() {
	registerRule("cloudrecord", newCloudRecordRule)
	registerRule("cloud-video", newCloudVideoRule)
	registerRule("timestamp-hash", newTimestampHashRule)
	registerRule("live", newLiveRule)
	registerRule("all", newAllVideoRules)
}

func registerRule(name string, factory RuleFactory) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" || factory == nil {
		panic("重命名规则名称和工厂不能为空")
	}
	if _, exists := ruleFactories[name]; exists {
		panic("重命名规则重复注册: " + name)
	}
	ruleFactories[name] = factory
}

// anyRule tries several rules in a fixed order and applies the first matching
// one. The default "all" rule covers all built-in video naming schemes so a
// directory can contain a mix of them.
type anyRule struct {
	name  string
	rules []RenameRule
}

func newAllVideoRules() RenameRule {
	return &anyRule{
		name: "all",
		rules: []RenameRule{
			newCloudRecordRule(),
			newCloudVideoRule(),
			newTimestampHashRule(),
			newLiveRule(),
		},
	}
}

func (rule *anyRule) Match(fileName string) bool {
	for _, candidate := range rule.rules {
		if candidate.Match(fileName) {
			return true
		}
	}
	return false
}

func (rule *anyRule) IsExpectedName(fileName string) bool {
	// All built-in video rules produce the same final format; one check is
	// enough and avoids parsing the same filename once per sub-rule.
	return isExpectedTargetName(fileName)
}

func (rule *anyRule) Rename(fileName string) (string, error) {
	for _, candidate := range rule.rules {
		if !candidate.Match(fileName) {
			continue
		}
		newName, err := candidate.Rename(fileName)
		if err != nil {
			return "", fmt.Errorf("规则 %s 处理失败: %w", candidate.Fingerprint(), err)
		}
		return newName, nil
	}
	return "", fmt.Errorf("文件名不符合任何 %s 规则: %q", rule.name, fileName)
}

func (rule *anyRule) Fingerprint() string {
	fingerprints := make([]string, 0, len(rule.rules))
	for _, candidate := range rule.rules {
		fingerprints = append(fingerprints, candidate.Fingerprint())
	}
	return rule.name + "/v2;rules=" + strings.Join(fingerprints, "|")
}

func newRuleByName(name string) (RenameRule, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	factory, ok := ruleFactories[name]
	if !ok {
		return nil, fmt.Errorf("未知重命名规则 %q；可用规则: %s", name, strings.Join(availableRuleNames(), ", "))
	}
	rule := factory()
	if rule == nil || strings.TrimSpace(rule.Fingerprint()) == "" {
		return nil, fmt.Errorf("规则 %q 的工厂或指纹无效", name)
	}
	return rule, nil
}

func availableRuleNames() []string {
	names := make([]string, 0, len(ruleFactories))
	for name := range ruleFactories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
