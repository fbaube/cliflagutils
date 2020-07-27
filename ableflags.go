package cliflagutils

import (
	// "flag"
	"fmt"
	S "strings"

	flag "github.com/spf13/pflag"
)

func myFlagFunc(p *flag.Flag) {
	fmt.Printf("FLAG %v \n", *p)
	p.Hidden = true
}

func DisableAllFlags() {
	flag.CommandLine.VisitAll(disableFlag)
}
func disableFlag(p *flag.Flag) {
	p.Hidden = true
}
func EnableAllFlags() {
	flag.CommandLine.VisitAll(enableFlag)
}
func enableFlag(p *flag.Flag) {
	p.Hidden = false
}

var flagsToEnable, flagsToDisable string

func DisableFlags(s string) {
	flagsToDisable = s
	flag.CommandLine.VisitAll(maybeDisableFlag)
}
func maybeDisableFlag(p *flag.Flag) {
	var thisFlag = p.Shorthand
	if S.Contains(flagsToDisable, thisFlag) {
		p.Hidden = true
	}
}
func EnableFlags(s string) {
	flagsToEnable = s
	flag.CommandLine.VisitAll(maybeEnableFlag)
}
func maybeEnableFlag(p *flag.Flag) {
	var thisFlag = p.Shorthand
	if S.Contains(flagsToEnable, thisFlag) {
		p.Hidden = false
	}
}
