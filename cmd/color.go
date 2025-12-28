package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
)

var (
	// Terminal ANSI colors (raw strings for direct concatenation)
	switchOffColor string
	switchOnColor  string
	resetColor     string

	cmdOutputColor string
	cmdErrorColor  string

	// Functional colors using SprintFunc
	highlightColor func(a ...interface{}) string
	sectionColor   func(a ...interface{}) string
	keyColor       func(a ...interface{}) string

	// Specialized UI colors
	memoryHeaderColor func(a ...interface{}) string
	memoryItemColor   func(a ...interface{}) string

	// Helper colors
	greenColor func(a ...interface{}) string
	grayColor  func(a ...interface{}) string
)

func init() {
	rootCmd.AddCommand(colorCmd)
	setupColors()
}

func setupColors() {
	p := termenv.ColorProfile()

	// 1. Raw ANSI strings for concatenation
	resetColor = "\033[0m"

	if p == termenv.TrueColor {
		// Vibrant TrueColor (24-bit)
		switchOffColor = p.Color("#808080").Sequence(false) // Grey
		switchOnColor = p.Color("#00FF7F").Sequence(false)  // Spring Green
		cmdOutputColor = p.Color("#FFFFE0").Sequence(false) // Light Yellow
		cmdErrorColor = p.Color("#FF69B4").Sequence(false)  // Hot Pink
	} else if p >= termenv.ANSI256 {
		// ANSI 256 Color (8-bit)
		switchOffColor = "\033[38;5;244m"
		switchOnColor = "\033[38;5;82m"
		cmdOutputColor = "\033[38;5;187m"
		cmdErrorColor = "\033[38;5;175m"
	} else {
		// Basic ANSI (4-bit)
		switchOffColor = "\033[90m"
		switchOnColor = "\033[92m"
		cmdOutputColor = "\033[33m"
		cmdErrorColor = "\033[35m"
	}

	// 2. Functional colors (SprintFunc style)
	style := func(color string, bold bool) func(a ...interface{}) string {
		return func(a ...interface{}) string {
			s := termenv.String(fmt.Sprint(a...)).Foreground(p.Color(color))
			if bold {
				s = s.Bold()
			}
			return s.String()
		}
	}

	if p == termenv.TrueColor {
		highlightColor = style("#00FF7F", true)
		sectionColor = style("#00CED1", true)
		keyColor = style("#FF69B4", true)
		memoryHeaderColor = style("#00BFFF", true)
		memoryItemColor = style("#ADFF2F", false)
		greenColor = style("#00FF00", false)
		grayColor = style("#808080", false)
	} else {
		// Fallback to fatih/color for consistent basic/256 output
		if p >= termenv.ANSI256 {
			highlightColor = color.New(color.FgHiGreen, color.Bold).SprintFunc()
			sectionColor = color.New(color.FgHiCyan, color.Bold).SprintFunc()
			keyColor = color.New(color.FgHiMagenta, color.Bold).SprintFunc()
		} else {
			highlightColor = color.New(color.FgGreen, color.Bold).SprintFunc()
			sectionColor = color.New(color.FgCyan, color.Bold).SprintFunc()
			keyColor = color.New(color.FgMagenta, color.Bold).SprintFunc()
		}
		memoryHeaderColor = color.New(color.FgCyan, color.Bold).SprintFunc()
		memoryItemColor = color.New(color.FgGreen).SprintFunc()
		greenColor = color.New(color.FgGreen).SprintFunc()
		grayColor = color.New(color.FgHiBlack).SprintFunc()
	}
}

var colorCmd = &cobra.Command{
	Use:    "color",
	Hidden: true, // hidden from help
	Short:  "Test different colors of gllm output",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Terminal Color Support: ")
		if supportsTrueColor() {
			color.New(color.FgGreen, color.Bold).Println("TRUE COLOR (24-bit)")
		} else {
			color.New(color.FgYellow).Println("256-color or basic")
		}
		fmt.Println()

		// Basic ANSI colors
		fmt.Println("=== Basic ANSI Colors ===")
		color.Black("Black Text")
		color.Red("Red Text")
		color.Green("Green Text")
		color.Yellow("Yellow Text")
		color.Blue("Blue Text")
		color.Magenta("Magenta Text")
		color.Cyan("Cyan Text")
		color.White("White Text")
		fmt.Println()

		// Bright colors
		fmt.Println("=== Bright Colors ===")
		color.HiBlack("Bright Black")
		color.HiRed("Bright Red")
		color.HiGreen("Bright Green")
		color.HiYellow("Bright Yellow")
		color.HiBlue("Bright Blue")
		color.HiMagenta("Bright Magenta")
		color.HiCyan("Bright Cyan")
		color.HiWhite("Bright White")
		fmt.Println()

		// Background colors
		fmt.Println("=== Background Colors ===")
		color.New(color.BgBlack, color.FgWhite).Print("Black BG")
		fmt.Print(" ")
		color.New(color.BgRed, color.FgWhite).Print("Red BG")
		fmt.Print(" ")
		color.New(color.BgGreen, color.FgBlack).Print("Green BG")
		fmt.Print(" ")
		color.New(color.BgBlue, color.FgWhite).Print("Blue BG")
		fmt.Print(" ")
		color.New(color.BgMagenta, color.FgWhite).Print("Magenta BG")
		fmt.Print(" ")
		color.New(color.BgCyan, color.FgBlack).Print("Cyan BG")
		fmt.Print(" ")
		color.New(color.BgYellow, color.FgBlack).Print("Yellow BG")
		fmt.Print(" ")
		color.New(color.BgWhite, color.FgBlack).Print("White BG")
		fmt.Println()
		fmt.Println()

		// True color examples (only if supported)
		if supportsTrueColor() {
			fmt.Println("=== TRUE COLOR (24-bit RGB) Examples ===")

			// Rainbow gradient
			fmt.Println("Rainbow Gradient:")
			rainbow := []struct{ r, g, b int }{
				{255, 0, 0}, {255, 165, 0}, {255, 255, 0}, {0, 255, 0},
				{0, 255, 255}, {0, 0, 255}, {128, 0, 128},
			}
			for _, rgb := range rainbow {
				color.RGB(rgb.r, rgb.g, rgb.b).Print("█")
			}
			fmt.Println()

			// Beautiful color palette
			fmt.Println("Color Palette:")
			palette := []struct {
				name    string
				r, g, b int
			}{
				{"Crimson", 220, 20, 60},
				{"Coral", 255, 127, 80},
				{"Gold", 255, 215, 0},
				{"Lime", 0, 255, 0},
				{"Aqua", 0, 255, 255},
				{"Royal Blue", 65, 105, 225},
				{"Purple", 128, 0, 128},
				{"Hot Pink", 255, 105, 180},
				{"Turquoise", 64, 224, 208},
				{"Orange Red", 255, 69, 0},
				{"Spring Green", 0, 255, 127},
				{"Deep Sky Blue", 0, 191, 255},
				{"Violet", 238, 130, 238},
				{"Tomato", 255, 99, 71},
				{"Chartreuse", 127, 255, 0},
				{"Sky Blue", 135, 206, 235},
			}

			for i, c := range palette {
				col := color.RGB(c.r, c.g, c.b)
				col.Printf("%-15s", c.name)
				if (i+1)%4 == 0 {
					fmt.Println()
				}
			}
			fmt.Println()

			// Background colors with true color
			fmt.Println("True Color Backgrounds:")
			trueBgColors := []struct {
				name    string
				r, g, b int
			}{
				{"Dark Red", 139, 0, 0},
				{"Dark Green", 0, 100, 0},
				{"Dark Blue", 0, 0, 139},
				{"Dark Purple", 75, 0, 130},
				{"Dark Orange", 255, 140, 0},
				{"Dark Cyan", 0, 139, 139},
			}

			for _, bg := range trueBgColors {
				color.BgRGB(bg.r, bg.g, bg.b).SprintFunc()(fmt.Sprintf(" %-12s ", bg.name))
			}
			fmt.Println()
			fmt.Println()

			// Gradient examples
			fmt.Println("Gradient Examples:")
			// Red to yellow gradient
			fmt.Print("Red→Yellow: ")
			for i := 0; i <= 20; i++ {
				r := 255
				g := (255 * i) / 20
				b := 0
				color.RGB(r, g, b).Print("█")
			}
			fmt.Println()

			// Blue to green gradient
			fmt.Print("Blue→Green: ")
			for i := 0; i <= 20; i++ {
				r := 0
				g := (255 * i) / 20
				b := 255 - (255*i)/20
				color.RGB(r, g, b).Print("█")
			}
			fmt.Println()
		} else {
			fmt.Println("=== Limited Color Support ===")
			fmt.Println("Your terminal doesn't support true color (24-bit).")
			fmt.Println("For full color experience, use a modern terminal like:")
			fmt.Println("- iTerm2")
			fmt.Println("- Alacritty")
			fmt.Println("- Windows Terminal")
			fmt.Println("- GNOME Terminal")
			fmt.Println("- Konsole")
		}

		// Reset at the end
		color.New(color.Reset).Print("")
	},
}

func supportsTrueColor() bool {
	return termenv.ColorProfile() == termenv.TrueColor
}
