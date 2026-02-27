package data

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/charmbracelet/glamour/styles"
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

// MostSimilarGlamourStyle returns the name of the glamour built-in style that
// most closely matches the active gogh theme. It uses two heuristics:
//
//  1. Background luminance → decides dark vs. light family.
//  2. For dark themes, the circular HSL hue-distance between the theme's
//     accent color and each dark style's known dominant hue determines the
//     winner among dracula (~264°), tokyo-night (~220°), pink (~330°), and
//     dark (~195° neutral teal).
//
// This preserves glamour's hand-crafted, vibrant colour palettes and rich
// Chroma syntax highlighting instead of a flat programmatic approximation.
func MostSimilarGlamourStyle() string {
	t := CurrentTheme

	// Smart fallback by background luminance.
	// Perceived luminance formula (ITU-R BT.601): L = 0.299R + 0.587G + 0.114B
	if hexLuminance(t.Background) > 0.45 {
		return styles.LightStyle
	}

	name := strings.ToLower(CurrentThemeName)

	// Explicit name-based matches (Priority)
	if strings.Contains(name, "dracula") {
		return styles.DraculaStyle
	}
	if strings.Contains(name, "night") {
		return styles.TokyoNightStyle
	}
	if strings.Contains(name, "dark") {
		return styles.DarkStyle
	}
	if strings.Contains(name, "light") || strings.Contains(name, "day") {
		return styles.LightStyle
	}
	if strings.Contains(name, "rose") || strings.Contains(name, "red") || strings.Contains(name, "pink") || strings.Contains(name, "sun") {
		return styles.AutoStyle
	}

	// For dark themes, use the theme's BrightMagenta as the primary
	// accent (most distinctive per-palette hue) and find the nearest glamour
	// dark-style fingerprint by circular hue distance.
	// Fallback to Magenta if BrightMagenta is absent.
	accent := t.BrightMagenta
	if accent == "" {
		accent = t.Magenta
	}
	accentHue := hexHue(accent)

	type fingerprint struct {
		name string
		hue  float64
	}
	// Dominant hue of each dark glamour style (measured from their JSON palettes)
	fingerprints := []fingerprint{
		{styles.DraculaStyle, 264},    // BrightMagenta #bd93f9 → purple
		{styles.TokyoNightStyle, 220}, // dominant blue-indigo
		{styles.AutoStyle, 330},       // rose-pink
		{styles.DarkStyle, 195},       // neutral teal (ANSI 39)
	}

	best, bestDist := styles.DarkStyle, math.MaxFloat64
	for _, fp := range fingerprints {
		if d := hueCircularDist(accentHue, fp.hue); d < bestDist {
			best, bestDist = fp.name, d
		}
	}
	return best
}

// hexLuminance returns the perceived luminance [0,1] of a hex colour string.
func hexLuminance(hex string) float64 {
	var r, g, b int
	fmt.Sscanf(strings.TrimPrefix(hex, "#"), "%02x%02x%02x", &r, &g, &b)
	return (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 255.0
}

// hexHue returns the HSL hue [0°, 360°) of a hex colour string.
func hexHue(hex string) float64 {
	if hex == "" {
		return 0
	}
	var r, g, b int
	fmt.Sscanf(strings.TrimPrefix(hex, "#"), "%02x%02x%02x", &r, &g, &b)
	rf, gf, bf := float64(r)/255.0, float64(g)/255.0, float64(b)/255.0
	max := math.Max(rf, math.Max(gf, bf))
	min := math.Min(rf, math.Min(gf, bf))
	if max == min {
		return 0 // achromatic
	}
	delta := max - min
	var h float64
	switch max {
	case rf:
		h = (gf - bf) / delta
		if gf < bf {
			h += 6
		}
	case gf:
		h = 2 + (bf-rf)/delta
	case bf:
		h = 4 + (rf-gf)/delta
	}
	return h * 60
}

// hueCircularDist returns the shortest angular distance [0°, 180°] between
// two HSL hue values on the colour wheel.
func hueCircularDist(h1, h2 float64) float64 {
	d := math.Abs(h1 - h2)
	if d > 180 {
		d = 360 - d
	}
	return d
}
