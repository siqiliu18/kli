package ui

import (
	"fmt"
	"strings"

	kt "github.com/siqiliu/kli/internal/types"
)

// PrintApplyResults prints per-resource apply results and a summary line.
//
// Example output:
//
//	✅  Deployment/nginx        created
//	🔄  ConfigMap/app-config    configured
//	⚡   Service/api             unchanged
//	❌  Deployment/worker       failed → image pull error
//
//	Applied 4 resources — ✅ 1 created  🔄 1 configured  ⚡ 1 unchanged  ❌ 1 failed
func PrintApplyResults(results []kt.ResourceResult) {
	var created, configured, unchanged, failed int

	for _, r := range results {
		label := fmt.Sprintf("%s/%s", r.Kind, r.Name)
		if r.Err != nil {
			fmt.Printf("  %s  %-40s failed → %v\n", SymbolFailed, label, r.Err)
			failed++
			continue
		}
		switch r.Action {
		case kt.ActionCreated:
			fmt.Printf("  %s  %-40s %s\n", SymbolSuccess, label, Green.Render("created"))
			created++
		case kt.ActionConfigured:
			fmt.Printf("  %s  %-40s %s\n", SymbolChanged, label, Yellow.Render("configured"))
			configured++
		case kt.ActionUnchanged:
			fmt.Printf("  %s  %-40s %s\n", SymbolUnchanged, label, Gray.Render("unchanged"))
			unchanged++
		}
	}

	fmt.Println()
	fmt.Printf("Applied %d resources — %s %d created  %s %d configured  %s %d unchanged  %s %d failed\n",
		len(results),
		SymbolSuccess, created,
		SymbolChanged, configured,
		SymbolUnchanged, unchanged,
		SymbolFailed, failed,
	)
}

// PrintUndeployResults prints per-resource undeploy results and a summary line.
//
// Example output:
//
//	🗑️  Deployment/nginx        deleted
//	⚡  Service/api              skipped (not found)
//	❌  ConfigMap/config        failed → forbidden
//
//	Undeployed 3 resources — 🗑️  1 deleted  ⚡ 1 skipped  ❌ 1 failed
func PrintUndeployResults(results []kt.ResourceResult) {
	var deleted, skipped, warned, failed int

	for _, r := range results {
		label := fmt.Sprintf("%s/%s", r.Kind, r.Name)
		switch {
		case r.Action == kt.ActionWarning:
			fmt.Printf("  %s  %-40s %s\n", SymbolWarning, label, Yellow.Render("warning → "+r.Err.Error()))
			warned++
		case r.Err != nil:
			fmt.Printf("  %s  %-40s %s\n", SymbolFailed, label, Red.Render("failed → "+r.Err.Error()))
			failed++
		case r.Action == kt.ActionDeleted:
			fmt.Printf("  %s  %-40s %s\n", SymbolDeleted, label, Red.Render("deleted"))
			deleted++
		case r.Action == kt.ActionSkipped:
			fmt.Printf("  %s  %-40s %s\n", SymbolUnchanged, label, Gray.Render("skipped (not found)"))
			skipped++
		}
	}

	fmt.Println()
	fmt.Printf("Undeployed %d resources — %s %d deleted  %s %d skipped  %s %d warned  %s %d failed\n",
		len(results),
		SymbolDeleted+" ", deleted,
		SymbolUnchanged, skipped,
		SymbolWarning+" ", warned,
		SymbolFailed, failed,
	)
}

// PrintStatus prints deployment health and pods grouped under each resource.
//
// Example output:
//
//	Namespace: kli1
//
//	DEPLOYMENTS
//	  ✅  nginx           2/2  Healthy
//	      nginx-abc123         Running
//	      nginx-def456         Running
//	  ⚠️  frontend        1/3  Degraded
//	      frontend-aaa         Running
//	      frontend-bbb         Pending
func PrintStatus(namespace string, results []kt.ResourceStatus) {
	fmt.Printf("Namespace: %s\n", Green.Render(namespace))

	// Group by kind for section headers
	kinds := []string{"Deployment", "StatefulSet", "DaemonSet"}
	for _, kind := range kinds {
		var section []kt.ResourceStatus
		for _, r := range results {
			if r.Kind == kind {
				section = append(section, r)
			}
		}
		if len(section) == 0 {
			continue
		}

		// strings.ToUpper + "S" → "DEPLOYMENTS", "STATEFULSETS", "DAEMONSETS"
		fmt.Printf("\n%s\n", Gray.Render(strings.ToUpper(kind)+"S"))
		for _, r := range section {
			symbol, healthLabel := healthDisplay(r.Health)
			// symbol is pre-padded to a consistent visual width in healthDisplay,
			// so %-28s on the name column stays aligned across all health states.
			fmt.Printf("  %s  %-25s  %2d/%-2d  %s\n",
				symbol, r.Name, r.Ready, r.Total, healthLabel)
			for _, p := range r.Pods {
				colored, hint := podPhaseDisplay(p.Name, p.Phase)
				fmt.Printf("      %-25s         %s\n", p.Name, colored)
				if hint != "" {
					fmt.Println(hint)
				}
			}
		}
	}
}

// healthDisplay returns the symbol and colored label for a HealthState.
// Symbols are padded with trailing spaces so they occupy the same visual
// width regardless of emoji — ⚠️ renders 1-wide while ✅/❌ render 2-wide,
// so without padding the name column shifts by 1 char on degraded/unknown rows.
func healthDisplay(h kt.HealthState) (symbol, label string) {
	switch h {
	case kt.Healthy:
		return SymbolSuccess, GreenBold.Render("Healthy") // 2-wide emoji + 1 space = 3
	case kt.Degraded:
		return SymbolWarning + " ", YellowBold.Render("Degraded") // 1-wide emoji + 2 spaces = 3
	case kt.Failed:
		return SymbolFailed, RedBold.Render("Failed") // 2-wide emoji + 1 space = 3
	default:
		return SymbolUnknown + " ", Gray.Render("Unknown") // 1-wide char  + 2 spaces = 3
	}
}

// describePhases are pod phases/reasons where logs are unavailable and
// kubectl describe is the right next step to diagnose the problem.
var describePhases = map[string]bool{
	"ContainerCreating":          true, // init containers or volumes not ready yet
	"PodInitializing":            true, // init containers still running
	"ImagePullBackOff":           true, // image can't be pulled (bad tag, rate limit, no pull secret)
	"ErrImagePull":               true, // transient image pull failure
	"InvalidImageName":           true, // malformed image reference
	"CreateContainerConfigError": true, // bad env vars, missing configmap/secret
	"RunContainerError":          true, // container runtime error at start
	"OOMKilled":                  true, // killed by kernel OOM — check resource limits
	"Error":                      true, // generic container error
}

// podPhaseDisplay returns the colored phase string and an optional hint line.
// For phases where logs are unavailable and kubectl describe is needed,
// a hint is returned as a second string — otherwise hint is empty.
func podPhaseDisplay(podName, phase string) (colored, hint string) {
	switch phase {
	case "Running":
		return Green.Render(phase), ""
	case "Pending":
		return Yellow.Render(phase), ""
	case "Succeeded":
		return Gray.Render(phase), ""
	default:
		colored = Red.Render(phase)
		if describePhases[phase] {
			hint = Gray.Render(fmt.Sprintf("          → kubectl describe pod %s", podName))
		}
		return colored, hint
	}
}

// ColorLogLine applies color to a log line based on its level keyword.
// Log formats vary widely across apps, so we do a case-insensitive substring
// match rather than strict parsing — covers JSON (\"level\":\"error\"),
// logfmt (level=warn), and plain text ([ERROR], INFO:) styles.
func ColorLogLine(line string) string {
	upper := strings.ToUpper(line)
	switch {
	case strings.Contains(upper, "ERROR") || strings.Contains(upper, "FATAL"):
		return LogError.Render(line)
	case strings.Contains(upper, "WARN"):
		return LogWarn.Render(line)
	default:
		// INFO and unrecognised levels — default terminal color
		return line
	}
}
