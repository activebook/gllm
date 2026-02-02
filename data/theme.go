package data

import (
	"fmt"
	"sort"

	"github.com/muesli/termenv"
	goghthemes "github.com/willyv3/gogh-themes"
)

const (
	DefaultThemeName string = "Dracula"

	// Formatting Utilities
	BoldSeq      string = "\033[1m"
	UnderlineSeq string = "\033[4m"
	ResetSeq     string = "\033[0m"
)

var (
	// Current Theme Name
	CurrentThemeName string = DefaultThemeName
	CurrentTheme     goghthemes.Theme

	// --- Semantic Color Variables (Exported) ---

	// Roles
	RoleSystemColor    string
	RoleUserColor      string
	RoleAssistantColor string

	// Tools
	ToolCallColor     string
	ToolResponseColor string

	// Status
	StatusErrorColor   string
	StatusSuccessColor string
	StatusWarnColor    string
	StatusInfoColor    string
	StatusDebugColor   string

	// Reasoning
	ReasoningActiveColor string
	ReasoningDoneColor   string
	ReasoningOffColor    string
	ReasoningLowColor    string
	ReasoningMedColor    string
	ReasoningHighColor   string

	// UI & Interactive
	SwitchOnColor     string
	SwitchOffColor    string
	TaskCompleteColor string

	// Media
	MediaColor string

	// Workflow
	WorkflowColor  string
	AgentRoleColor string
	ModelColor     string
	DirectoryColor string
	PromptColor    string
	BoolTrueColor  string
	BoolFalseColor string

	// Additional UI Colors (ANSI Sequences)
	BorderColor      string
	SectionColor     string
	KeyColor         string
	HighlightColor   string
	ForegroundColor  string
	BackgroundColor  string
	LabelColor       string
	DetailColor      string
	ShellOutputColor string

	// Diff Colors
	DiffAddedColor     string
	DiffRemovedColor   string
	DiffHeaderColor    string
	DiffSeparatorColor string
	DiffAddedBgColor   string
	DiffRemovedBgColor string

	// Hex Codes (for lipgloss or other UI libs)
	BorderHex     string
	SectionHex    string
	KeyHex        string
	LabelHex      string
	DetailHex     string
	SpinnerHex    string
	BackgroundHex string

	HighCachedHex string
	MedCachedHex  string
	LowCachedHex  string
	OffCachedHex  string

	// Functional Helpers (for backwards compatibility or convenience)
	// These might wrap the strings above
)

// init initializes the default theme.
func init() {
	// Defer loading until config is likely ready, or load default immediately.
	// Since init() runs before main(), config might not be loaded yet.
	LoadTheme(DefaultThemeName)
}

// LoadTheme loads a theme by name from gogh-themes and updates all color variables.
func LoadTheme(name string) error {
	if name == "" {
		name = DefaultThemeName
	}

	theme, ok := goghthemes.Get(name)
	if !ok {
		return fmt.Errorf("theme '%s' not found", name)
	}

	CurrentThemeName = name
	CurrentTheme = theme
	applyTheme(theme)
	return nil
}

// applyTheme maps the Gogh theme colors to our semantic variables.
func applyTheme(t goghthemes.Theme) {
	p := termenv.ColorProfile()

	// Helper to convert hex to ANSI sequence
	toAnsi := func(hex string) string {
		if hex == "" {
			return ""
		}
		c := p.Color(hex)
		return fmt.Sprintf("%s%sm", termenv.CSI, c.Sequence(false))
	}
	toAnsiBg := func(hex string) string {
		if hex == "" {
			return ""
		}
		c := p.Color(hex)
		return fmt.Sprintf("%s%sm", termenv.CSI, c.Sequence(true)) // true for background
	}

	// blend mixes two hex colors with a given ratio (0.0 to 1.0)
	blend := func(hex1, hex2 string, ratio float64) string {
		var r1, g1, b1, r2, g2, b2 int
		fmt.Sscanf(hex1, "#%02x%02x%02x", &r1, &g1, &b1)
		fmt.Sscanf(hex2, "#%02x%02x%02x", &r2, &g2, &b2)

		r := int(float64(r1)*ratio + float64(r2)*(1-ratio))
		g := int(float64(g1)*ratio + float64(g2)*(1-ratio))
		b := int(float64(b1)*ratio + float64(b2)*(1-ratio))

		return fmt.Sprintf("#%02x%02x%02x", r, g, b)
	}

	// 1. Roles
	RoleSystemColor = toAnsi(t.Yellow)  // Traditional System color
	RoleUserColor = toAnsi(t.Green)     // User usually Green
	RoleAssistantColor = toAnsi(t.Blue) // Assistant usually Blue

	// 2. Tools
	ToolCallColor = toAnsi(t.Cyan)        // Cyan for tools
	ToolResponseColor = toAnsi(t.Magenta) // Purple for output

	// 3. Status
	StatusErrorColor = toAnsi(t.Red)
	StatusSuccessColor = toAnsi(t.Green)
	StatusWarnColor = toAnsi(t.Yellow)
	StatusInfoColor = toAnsi(t.Blue)
	StatusDebugColor = toAnsi(t.BrightBlack) // Gray

	// 4. Reasoning
	ReasoningActiveColor = toAnsi(t.Green)
	ReasoningDoneColor = toAnsi(t.BrightBlack) // Dimmed out

	// Reasoning Levels (Heatmap style)
	ReasoningOffColor = toAnsi(t.BrightBlack)
	ReasoningLowColor = toAnsi(t.Red)
	ReasoningMedColor = toAnsi(t.Yellow)
	ReasoningHighColor = toAnsi(t.Green)

	// 5. UI
	SwitchOnColor = toAnsi(t.BrightGreen)
	SwitchOffColor = toAnsi(t.BrightBlack)
	TaskCompleteColor = toAnsi(t.BrightGreen)

	// 6. Media
	MediaColor = toAnsi(t.BrightCyan)

	// 7. Workflow
	WorkflowColor = toAnsi(t.Magenta)
	AgentRoleColor = toAnsi(t.Cyan)
	ModelColor = toAnsi(t.BrightGreen)
	DirectoryColor = toAnsi(t.Yellow)
	PromptColor = toAnsi(t.BrightCyan)
	BoolTrueColor = toAnsi(t.BrightGreen)
	BoolFalseColor = toAnsi(t.BrightBlack)

	// Additional UI
	BorderColor = toAnsi(t.BrightMagenta)
	SectionColor = toAnsi(t.BrightCyan)
	KeyColor = toAnsi(t.BrightMagenta)
	HighlightColor = toAnsi(t.BrightGreen)
	ForegroundColor = toAnsi(t.Foreground)
	BackgroundColor = toAnsi(t.Background)
	LabelColor = toAnsi(t.Foreground)
	DetailColor = toAnsi(t.BrightBlack)
	ShellOutputColor = toAnsi(t.BrightBlack)

	// 7. Diff
	DiffAddedColor = toAnsi(t.Green)
	DiffRemovedColor = toAnsi(t.Red)
	DiffHeaderColor = toAnsi(t.BrightCyan)
	DiffSeparatorColor = toAnsi(t.BrightBlack)

	// Use blending to create faint backgrounds (approx 15% opacity)
	DiffAddedBgColor = toAnsiBg(blend(t.Green, t.Background, 0.15))
	DiffRemovedBgColor = toAnsiBg(blend(t.Red, t.Background, 0.15))

	// 8. Hex Codes
	BorderHex = t.Foreground
	SectionHex = t.BrightCyan
	KeyHex = t.BrightMagenta
	LabelHex = t.Foreground
	DetailHex = t.BrightBlack
	SpinnerHex = t.BrightMagenta
	BackgroundHex = t.Background

	HighCachedHex = t.Green
	MedCachedHex = t.Yellow
	LowCachedHex = t.Red
	OffCachedHex = t.BrightBlack
}

// ListThemes returns a sorted list of all available theme names.
func ListThemes() []string {
	names := goghthemes.Names()
	sort.Strings(names)
	return names
}

// SaveThemeConfig persists the theme selection to the configuration file.
func SaveThemeConfig(name string) error {
	store := GetSettingsStore()
	return store.SetTheme(name)
}

// GetThemeFromConfig retrieves the configured theme name.
func GetThemeFromConfig() string {
	store := GetSettingsStore()
	return store.GetTheme()
}
