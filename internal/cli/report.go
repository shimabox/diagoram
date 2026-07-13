package cli

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shimabox/diagoram/internal/diagram"
	"github.com/shimabox/diagoram/internal/gocode"
	"github.com/shimabox/diagoram/internal/render"
	"github.com/shimabox/diagoram/internal/render/mermaid"
)

// markdownReport renders a self-contained account of the analysis. Edge
// reasons are always enabled because they provide useful evidence for both
// human review and downstream generative-AI analysis.
func markdownReport(opts *Options, d *diagram.Diagram, warnings []gocode.Warning, modulePath string, selected *gocode.BuildContext) (string, error) {
	renderOptions := render.Options{
		HideUnexported:    opts.HideUnexported,
		ShowConstants:     opts.ShowConstants,
		ShowFunctions:     opts.ShowFunctions,
		DisableFields:     opts.DisableFields,
		DisableMethods:    opts.DisableMethods,
		DisableImplements: opts.DisableImplements,
		FunctionPatterns:  opts.FunctionPatterns,
		MethodPatterns:    opts.MethodPatterns,
		ReceiverPatterns:  opts.ReceiverPatterns,
		MaxMembers:        opts.MaxMembers,
		ShowEdgeReasons:   true,
	}
	diagramText, err := mermaid.New().Render(d, renderOptions)
	if err != nil {
		return "", err
	}
	summary := diagram.Summary(d, diagram.SummaryOptions{
		HideUnexported:    opts.HideUnexported,
		DisableFields:     opts.DisableFields,
		DisableMethods:    opts.DisableMethods,
		DisableImplements: opts.DisableImplements,
		FunctionPatterns:  opts.FunctionPatterns,
		MethodPatterns:    opts.MethodPatterns,
		ReceiverPatterns:  opts.ReceiverPatterns,
		ShowEdgeReasons:   true,
	})

	var b strings.Builder
	b.WriteString("# Go source analysis report\n\n")
	b.WriteString("## Scope\n\n")
	writeReportItem(&b, "Directory", filepath.Clean(opts.Dir))
	if modulePath == "" {
		writeReportItem(&b, "Module", "not detected")
	} else {
		writeReportItem(&b, "Module", modulePath)
	}
	writeReportItem(&b, "Build context", buildContextDescription(selected))
	writeReportItem(&b, "Diagnostics", strconv.Itoa(len(warnings)))

	b.WriteString("\n## Analysis settings\n\n")
	writeReportItem(&b, "Public API", boolText(opts.PublicAPI))
	writeReportItem(&b, "Unexported identifiers", visibilityText(opts.HideUnexported))
	writeReportItem(&b, "Type aliases", visibilityText(opts.HideAliases))
	writeReportItem(&b, "Fields", visibilityText(opts.DisableFields))
	writeReportItem(&b, "Methods", visibilityText(opts.DisableMethods))
	writeReportItem(&b, "Interface implementations", visibilityText(opts.DisableImplements))
	writeReportItem(&b, "Constants in diagram", enabledText(opts.ShowConstants))
	writeReportItem(&b, "Package functions in diagram", enabledText(opts.ShowFunctions))
	writeReportItem(&b, "Function filters", listText(opts.FunctionPatterns))
	writeReportItem(&b, "Method filters", listText(opts.MethodPatterns))
	writeReportItem(&b, "Receiver filters", listText(opts.ReceiverPatterns))
	writeReportItem(&b, "Include files", listText(defaultList(opts.Include, "*.go")))
	writeReportItem(&b, "Exclude files", listText(defaultList(opts.Exclude, "*_test.go")))
	writeReportItem(&b, "Include directories", listText(opts.IncludeDirs))
	writeReportItem(&b, "Exclude directories", listText(effectiveExcludeDirs(opts)))
	writeReportItem(&b, "Generated files", generatedFilesText(opts))
	writeReportItem(&b, "Relation targets", listText(opts.RelTargets))
	if len(opts.RelTargets) > 0 {
		writeReportItem(&b, "Relation depth", strconv.Itoa(opts.RelTargetDepth))
	}
	if opts.MaxMembers == 0 {
		writeReportItem(&b, "Members per diagram category", "unlimited")
	} else {
		writeReportItem(&b, "Members per diagram category", strconv.Itoa(opts.MaxMembers))
	}
	writeReportItem(&b, "Dependency reasons", "included")

	b.WriteString("\n## Structural summary\n\n```text\n")
	b.WriteString(strings.TrimRight(summary, "\n"))
	b.WriteString("\n```\n\n## Types and relationships\n\n```mermaid\n")
	b.WriteString(strings.TrimRight(diagramText, "\n"))
	b.WriteString("\n```\n\n## Diagnostics\n\n")
	if len(warnings) == 0 {
		b.WriteString("None\n")
	} else {
		for _, warning := range warnings {
			fmt.Fprintf(&b, "- %s\n", strings.ReplaceAll(warning.Error(), "\n", " "))
		}
	}
	return b.String(), nil
}

func writeReportItem(b *strings.Builder, label, value string) {
	fmt.Fprintf(b, "- %s `%s`\n", label, strings.ReplaceAll(value, "`", "'"))
}

func boolText(value bool) string {
	if value {
		return "enabled"
	}
	return "disabled"
}

func enabledText(value bool) string {
	if value {
		return "included"
	}
	return "not included"
}

func visibilityText(hidden bool) string {
	if hidden {
		return "hidden"
	}
	return "included"
}

func listText(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}

func defaultList(values []string, fallback string) []string {
	if len(values) == 0 {
		return []string{fallback}
	}
	return values
}

func effectiveExcludeDirs(opts *Options) []string {
	values := append([]string(nil), opts.ExcludeDirs...)
	if opts.PublicAPI {
		values = append(values, "internal", "tests", "example", "examples", "_examples", "benchmark")
	}
	return uniqueStrings(values)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func generatedFilesText(opts *Options) string {
	if opts.ExcludeGenerated {
		return "excluded"
	}
	if opts.GeneratedOnly {
		return "only generated files"
	}
	return "included"
}
