package ui

import (
	"fmt"
	"log"
	"strings"

	"github.com/superstarryeyes/bit/ansifonts"
)

const (
	LogoFont = "8bitfortress"
	LogoText = "GLLM"
)

func GetLogo(textColor string, gradientColor string, scale float64) string {
	font, err := ansifonts.LoadFont(LogoFont)
	if err != nil {
		return ""
	}

	options := ansifonts.RenderOptions{
		CharSpacing:            2,
		WordSpacing:            2,
		LineSpacing:            1,
		TextColor:              textColor,
		GradientColor:          gradientColor,
		UseGradient:            true,
		GradientDirection:      ansifonts.LeftRight,
		Alignment:              ansifonts.LeftAlign,
		ScaleFactor:            scale,
		ShadowEnabled:          false,
		ShadowHorizontalOffset: 0,
		ShadowVerticalOffset:   0,
		ShadowStyle:            ansifonts.MediumShade,
	}

	rendered := ansifonts.RenderTextWithOptions(LogoText, font, options)
	logo := strings.Builder{}
	for _, line := range rendered {
		logo.WriteString(line + "\n")
	}
	return logo.String()
}

func PrintLogo(textColor string, gradientColor string, scale float64) {
	font, err := ansifonts.LoadFont(LogoFont)
	if err != nil {
		log.Fatalf("Failed to load font: %v", err)
		return
	}

	options := ansifonts.RenderOptions{
		CharSpacing:            2,
		WordSpacing:            2,
		LineSpacing:            1,
		TextColor:              textColor,
		GradientColor:          gradientColor,
		UseGradient:            true,
		GradientDirection:      ansifonts.LeftRight,
		Alignment:              ansifonts.LeftAlign,
		ScaleFactor:            scale,
		ShadowEnabled:          false,
		ShadowHorizontalOffset: 0,
		ShadowVerticalOffset:   0,
		ShadowStyle:            ansifonts.MediumShade,
	}

	rendered := ansifonts.RenderTextWithOptions(LogoText, font, options)
	for _, line := range rendered {
		fmt.Println(line)
	}
}
