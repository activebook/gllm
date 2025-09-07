package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(colorCmd)
}

var colorCmd = &cobra.Command{
	Use:   "color",
	Short: "Show some different colors of gllm output",
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
	colorTerm := os.Getenv("COLORTERM")
	return colorTerm == "truecolor" || colorTerm == "24bit"
}
