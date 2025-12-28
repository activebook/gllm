package service

import (
	"github.com/muesli/termenv"
)

var (
	// Status/Message colors
	errorColor   string
	successColor string
	warnColor    string
	infoColor    string
	debugColor   string

	// UI element colors
	cyanColor   string
	bbColor     string
	hiBlueColor string
	dimColor    string
	greyColor   string
	purpleColor string
	yellowColor string

	// Emphasis colors
	boldColor      string
	underlineColor string

	// Reset
	resetColor string
)

func init() {
	setupColors()
}

func setupColors() {
	p := termenv.ColorProfile()
	resetColor = "\033[0m"

	if p == termenv.TrueColor {
		errorColor = p.Color("#FF4500").Sequence(false)   // Orangered
		successColor = p.Color("#32CD32").Sequence(false) // Limegreen
		warnColor = p.Color("#FF8C00").Sequence(false)    // DarkOrange
		infoColor = p.Color("#00CED1").Sequence(false)    // DarkCyan
		debugColor = p.Color("#1E90FF").Sequence(false)   // DodgerBlue

		cyanColor = p.Color("#00FFFF").Sequence(false)
		bbColor = p.Color("#696969").Sequence(false)     // DimGray
		hiBlueColor = p.Color("#00BFFF").Sequence(false) // DeepSkyBlue
		dimColor = "\033[2m"
		greyColor = p.Color("#808080").Sequence(false)
		purpleColor = p.Color("#9370DB").Sequence(false) // MediumPurple
		yellowColor = p.Color("#FFD700").Sequence(false) // Gold
	} else if p >= termenv.ANSI256 {
		errorColor = "\033[31m"
		successColor = "\033[32m"
		warnColor = "\033[38;5;208m"
		infoColor = "\033[36m"
		debugColor = "\033[34m"

		cyanColor = "\033[36m"
		bbColor = "\033[90m"
		hiBlueColor = "\033[94m"
		dimColor = "\033[2m"
		greyColor = "\033[38;5;240m"
		purpleColor = "\033[38;5;141m"
		yellowColor = "\033[33m"
	} else {
		errorColor = "\033[31m"
		successColor = "\033[32m"
		warnColor = "\033[33m"
		infoColor = "\033[36m"
		debugColor = "\033[34m"

		cyanColor = "\033[36m"
		bbColor = "\033[90m"
		hiBlueColor = "\033[94m"
		dimColor = "\033[2m"
		greyColor = "\033[90m"
		purpleColor = "\033[35m"
		yellowColor = "\033[33m"
	}

	boldColor = "\033[1m"
	underlineColor = "\033[4m"
}
