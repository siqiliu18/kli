package ui

import "github.com/charmbracelet/lipgloss"

// Color palette used across all commands.
// All terminal styling goes through these styles so colors stay consistent
// and can be updated in one place.

var (
	// Status colors — used by apply, undeploy, and status
	Green  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	Yellow = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	Red    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	Gray   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	// Bold variants — used for section headers in status output
	GreenBold  = Green.Bold(true)
	YellowBold = Yellow.Bold(true)
	RedBold    = Red.Bold(true)
)

// Symbols paired with their colors — used in apply/undeploy result rows
// and status health indicators.
var (
	SymbolSuccess   = Green.Render("✅")  // created / healthy
	SymbolChanged   = Yellow.Render("🔄") // configured
	SymbolUnchanged = Gray.Render("⚡")   // unchanged / skipped
	SymbolFailed    = Red.Render("❌")    // failed
	SymbolDeleted   = Red.Render("🗑️")   // deleted
	SymbolWarning   = Yellow.Render("⚠️") // warning (finalizer present)
	SymbolUnknown   = Gray.Render("?")   // unknown (e.g. DaemonSet with no nodes)
)

// Log level colors — used by logs command to color-code log output
var (
	LogInfo  = lipgloss.NewStyle() // INFO: no color (default terminal)
	LogWarn  = Yellow              // WARN: yellow
	LogError = Red                 // ERROR: red
)