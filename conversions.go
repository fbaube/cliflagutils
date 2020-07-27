package cliflagutils

import (
	"flag"
	"os"
	"strings"
	S "strings"
	// flag "github.com/spf13/pflag"
)

func FlagNameFromEnvarName(s string) string {
	s = S.ToLower(s)
	s = S.Replace(s, "_", "-", -1)
	return s
}

func EnvarNameFromFlagName(s string) string {
	s = S.ToUpper(s)
	s = S.Replace(s, "-", "_", -1)
	return s
}

// ParseEnvars is based on
// https://scene-si.org/2020/04/28/extending-pflag-with-environment-variables/
func ParseEnvars() error {
	for _, v := range os.Environ() {
		vals := strings.SplitN(v, "=", 2)
		flagName := FlagNameFromEnvarName(vals[0])
		var fn *flag.Flag
		if fn = flag.CommandLine.Lookup(flagName); fn == nil {
			continue
		}
		// This code is for flags, not pflags.
		flagOption := "--" + flagName
		if HasFlag(os.Args, flagOption) {
			continue
		}
		os.Args = append(os.Args, flagOption, vals[1])
		/* This code is for pflags, not flags.
		if fn == nil || fn.Changed {
			continue
		}
		if err := fn.Value.Set(vals[1]); err != nil {
			return err
		}
		*/
	}
	return nil
}

func HasFlag(haystack []string, needle string) bool {
	for _, v := range haystack {
		if strings.HasPrefix(v, needle) {
			return true
		}
	}
	return false
}
