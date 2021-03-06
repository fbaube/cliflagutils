package cliflagutils

import (
	// "flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	FP "path/filepath"

	flag "github.com/spf13/pflag"

	"errors"

	DU "github.com/fbaube/dbutils"
	FU "github.com/fbaube/fileutils"
	L "github.com/fbaube/mlog"

	// L "github.com/fbaube/mlog"
	SU "github.com/fbaube/stringutils"
	WU "github.com/fbaube/wasmutils"
	XU "github.com/fbaube/xmlutils"
)

// FIXME:
// hide a flag by specifying its name
// flags.MarkHidden("secretFlag")

/* FIXME:
// SetOutput sets the destination for usage and error messages.
// If output is nil, os.Stderr is used.
func (f *FlagSet) SetOutput(output io.Writer) {
	f.output = output
}
*/

var inArg, outArg, dbArg, gtokensArg, xmlCatArg, xmlSchemasArg string

// XmlAppConfiguration can probably be used with various 3rd-party utilities.
type XmlAppConfiguration struct {
	AppName  string
	DBhandle *DU.MmmcDB
	Infile, Outfile, Dbdir,
	Xmlcatfile, Xmlschemasdir FU.PathProps // NOT ptr! Barfs at startup.
	RestPort int
	// CLI flags
	FollowSymLinks, Pritt, DBdoImport, Help, Debug,
	GroupGenerated, GTokens, GTree, Validate, DBdoZeroOut bool
	// Result of processing CLI arg for input file(s)
	SingleFile bool
	// Result of processing CLI args (-c, -s)
	*XU.XmlCatalogFile
	PrittOutput io.Writer
}

var myAppName string

/*
func SetAppName(s string) {
	myAppName = s
}
*/

var multipleXmlCatalogFiles []*XU.XmlCatalogFile

// CA maybe should not be exported. Or should be generated
// on-the-fly instead of being a Singleton.
// // var CA XmlAppConfiguration

// MyUsage displays (1) the app name (or "(wasm)"), plus (2) a usage
// summary (see the func body), plus (3) the flags' usage message.
// TODO: Should not return info for flags that are Hidden (i.e. disabled).
func MyUsage() {
	//  Println(CA.AppName, "[-d] [-g] [-h] [-m] [-p] [-v] [-z] [-D] [-o outfile] [-d dbdir] Infile")
	fmt.Println(myAppName, "[-d] [-g] [-h] [-m] [-p] [-v] [-z] [-D] [-d dbdir] [-r port] Infile")
	fmt.Println("   Process mixed content XML, XHTML/XDITA, and Markdown/MDITA input.")
	fmt.Println("   Infile is a single file or directory name; no wildcards (?,*).")
	fmt.Println("          If a directory, it is processed recursively.")
	fmt.Println("   Infile may be \"-\" for Stdin: input typed (or pasted) interactively")
	fmt.Println("          is written to file ./Stdin.xml for processing")
	flag.Usage()
}

// For each DOCTYPE. also an XSL, thus XSL catalog file.
// Maybe also same for CSS.

// UMM is the Usage Message Map. We do this to provide (here) a quick
// option reference, and (below) to make the actual code clearer.
// e.g. commits := map[string]int { "rsc": 3711, "r":   2138, }
var UMM = map[string]string{
	// Simple BOOLs
	"h": "Show extended help message and exit",
	"g": "Group all generated files in same-named folder \n" +
		"(e.g. ./Filenam.xml maps to ./Filenam.xml_gxml/Filenam.*)",
	"m": "Import input file(s) to database",
	"p": "Pretty-print to file with \"fmtd-\" prepended to file extension",
	"t": "gTree written to file with \"gtr-\" prepended to file extension",
	"k": "gTokens written to file with \"gtk-\" prepended to file extension",
	"v": "Validate input file(s) (using xmllint) (with flag \"-c\" or \"-s\")",
	"z": "Zero out the database",
	"D": "Turn on debugging",
	"L": "Follow symbolic links in directory recursion",
	// All others
	"c": "XML `catalog_filepath` (do not use with \"-s\" flag)",
	"o": "`output_filepath` (possibly ignored, depending on command)",
	"d": "Database mmmc.db `directory_path`",
	"r": "Run REST server on `port_number`",
	"s": "DTD schema file(s) `directory_path` (.dtd, .mod)",
}

func initVars(pXAC *XmlAppConfiguration) {
	// f.StringVarP(&inArg, "infile", "i", "", UMM["i"])
	flag.StringVarP(&dbArg, "db-dir", "d", "", UMM["d"])
	flag.StringVarP(&outArg, "outfile", "o", "", UMM["o"])
	flag.StringVarP(&xmlCatArg, "catalog", "c", "", UMM["c"])
	flag.StringVarP(&xmlSchemasArg, "search", "s", "", UMM["s"])
	flag.BoolVarP(&pXAC.Help, "help", "h", false, UMM["h"])
	flag.BoolVarP(&pXAC.GTree, "gtree", "t", false, UMM["t"])
	flag.BoolVarP(&pXAC.Debug, "debug", "D", false, UMM["D"])
	flag.BoolVarP(&pXAC.Pritt, "pretty", "p", false, UMM["p"])
	flag.BoolVarP(&pXAC.GTokens, "gtokens", "k", false, UMM["k"])
	flag.BoolVarP(&pXAC.Validate, "validate", "v", false, UMM["v"])
	flag.BoolVarP(&pXAC.DBdoImport, "import", "m", false, UMM["m"])
	flag.BoolVarP(&pXAC.DBdoZeroOut, "zero-out", "z", false, UMM["z"])
	flag.BoolVarP(&pXAC.FollowSymLinks, "symlinks", "L", true, UMM["L"])
	flag.BoolVarP(&pXAC.GroupGenerated, "group-gen", "g", false, UMM["g"])
	flag.IntVarP(&pXAC.RestPort, "rest-port", "r", 0, UMM["r"])
	EnableAllFlags()
}

var calledCheckMustUsage bool

// CheckMustUsage returns a non-nil loggable error message if the caller
// should abort execution.
func CheckMustUsage() error {
	calledCheckMustUsage = true
	if WU.IsWasm() {
		myAppName = "(wasm)"
		return nil
	}
	// Figure out what CLI name we were called as
	myAppName, _ := os.Executable()
	// The call to FP.Clean(..) is needed (!!)
	// L.L.Dbg("Executing: " + FU.Tildotted(FP.Clean(myAppName)))
	myAppName = FP.Base(myAppName)
	pXAC.AppName = myAppName
	// Process CLI invocation flags
	flag.Parse()
	L.L.Dbg("Command tail: %+v", flag.Args())
	// FIXME - pos'l arg OR "-i" OR stdin OR "-"
	gotNoArgs := (len(os.Args) < 2)
	gotBadArgs := (nil == flag.Args() || 0 == len(flag.Args()))
	if !(gotNoArgs || gotBadArgs) {
		return nil
	}
	var err error
	if !gotNoArgs {
		err = errors.New("ERROR: Argument parsing failed. Did not specify input file(s)?")
		println(myAppName+":", err.Error())
	} else {
		err = errors.New("Nothing to do")
	}
	MyUsage()
	return err
}

// checkbarf simply aborts with an error message, if a
// serious (i.e. top-level) problem has been encountered.
func checkbarf(e error, s string) {
	if e == nil {
		return
	}
	// MU.SessionLogger.Printf("%s failed: %s \n", myAppName, e)
	// fmt.Fprintf(os.Stderr, "%s failed: %s \n", myAppName, e)
	// MU.ErrorTrace(os.Stderr, e)
	L.L.Panic("%s failed: %s", myAppName, e)
	L.L.Close()
	os.Exit(1)
}

// pXAC is a global predefined default XmlAppConfiguration.
var pXAC *XmlAppConfiguration

func init() {
	pXAC = new(XmlAppConfiguration)
	initVars(pXAC)
}

func GetXmlAppConfiguration() *XmlAppConfiguration {
	return pXAC
}

// NewXmlAppConfiguration processes CLI arguments for any XML-related command.
// It takes the CLI arguments as calling parameters, rather than accessing them
// directly itself, to facilitate testing, and enable running in-browser as wasm.
func NewXmlAppConfiguration(osArgs []string) (*XmlAppConfiguration, error) {
	if !calledCheckMustUsage {
		println("DEV: Please call CheckMustUsage()) before calling NewXmlAppConfiguration")
		e := CheckMustUsage()
		if e != nil {
			return nil, e
		}
	}
	var e error

	// If called from the CLI
	if !WU.IsWasm() {
		// Locate xmllint for doing XML validations
		xl, e := exec.LookPath("xmllint")
		if e != nil {
			if pXAC.Validate {
				L.L.Info("Validation is not possible: xmllint cannot be found")
			}
		}
		L.L.Info("xmllint found at: " + xl)
	}
	// Assert re. CLI invocation flags
	if len(osArgs) < 2 {
		L.L.Panic("CLI argument processing")
		L.L.Close()
		os.Exit(1)
	}
	L.L.Dbg("CLI tail: %+v", flag.Args())
	L.L.Dbg("CLI flags: debug:%s grpGen:%s help:%s "+
		"import:%s pritty:%s gtokens:%s gtree:%s validate:%s zeroOutDB:%s restPort:%d",
		SU.Yn(pXAC.Debug), SU.Yn(pXAC.GroupGenerated), SU.Yn(pXAC.Help),
		SU.Yn(pXAC.DBdoImport), SU.Yn(pXAC.Pritt), SU.Yn(pXAC.GTokens),
		SU.Yn(pXAC.GTree), SU.Yn(pXAC.Validate), SU.Yn(pXAC.DBdoZeroOut),
		pXAC.RestPort)

	// ===========================================
	//   PROCESS INPUT SPEC
	// ===========================================

	// Handle case where XML comes from standard input i.e. os.Stdin
	if flag.Args()[0] == "-" {
		if WU.IsWasm() {
			println("==> FIXME/wasm: Trying to read from Stdin; press ^D right after a newline to end")
		} else {
			stat, e := os.Stdin.Stat()
			checkbarf(e, "Cannot Stat() Stdin")
			if (stat.Mode() & os.ModeCharDevice) != 0 {
				println("==> Reading from Stdin; press ^D right after a newline to end")
			} else {
				L.L.Info("Reading Stdin from a file or pipe")
			}
		}
		// bb, e := ReadAll(os.Stdin)
		stdIn := FU.GetStringFromStdin()
		checkbarf(e, "Cannot read from Stdin")
		e = os.WriteFile("Stdin.xml", []byte(stdIn), 0666)
		checkbarf(e, "Cannot write to ./Stdin.xml")
		pXAC.Infile = *FU.NewPathProps("Stdin.xml") // .RelFilePath = "Stdin.xml"

	} else {
		// ===========================================
		//   PROCESS INPUT SPEC (normal case) to get
		//   info about path, existence, and type
		// ===========================================
		// Process input-file(s) argument, which can be a relative filepath.
		pXAC.Infile = *FU.NewPathProps(flag.Args()[0])
		// If the absolute path does not match the argument provided, inform the user.
		if pXAC.Infile.AbsFP.S() != flag.Args()[0] { // CA.In.RelFilePath { // CA.In.ArgFilePath {
			L.L.Info("Infilespec: " + FU.Tildotted(pXAC.Infile.AbsFP.S()))
		}
		if pXAC.Infile.IsOkayDir() {
			L.L.Info("The input is a directory and will be processed recursively.")
		} else if pXAC.Infile.IsOkayFile() {
			L.L.Info("The input is a single file: extra info will be listed here.")
			pXAC.SingleFile = true
		} else {
			L.L.Error("The input is a type not understood.")
			return nil, errors.New("Bad type for input: " + pXAC.Infile.AbsFP.S())
		}
	}

	// ===========================================
	//   PROCESS ARGUMENTS to get complete info
	//   about path, existence, and type
	// ===========================================

	// Process output-file(s) argument, which can be a relative filepath.
	// CA.Out.ProcessFilePathArg(CA.Out.ArgFilePath)
	pXAC.Outfile = *FU.NewPathProps(outArg) // CA.Out.RelFilePath)

	// Process database directory argument, which can be a relative filepath.
	// CA.DB.ProcessFilePathArg(CA.DBdirPath)

	// ====

	e = pXAC.ProcessDatabaseArgs()
	checkbarf(e, "Could not process DB directory argument(s)")
	// e = pXAC.ProcessCatalogArgs()
	L.L.Warning("XML catalog processing is temporariy disabled!")
	checkbarf(e, "Could not process XML catalog argument(s)")
	return pXAC, e
}

func (pXAC *XmlAppConfiguration) ProcessDatabaseArgs() error {
	var mustAccessTheDB, theDBexists bool
	var e error
	mustAccessTheDB = pXAC.DBdoImport || pXAC.DBdoZeroOut || dbArg != ""
	if !mustAccessTheDB {
		return nil
	}
	// NewMmmcDB(..) does not open or touch any files;
	// it only checks that the path is OK.
	pXAC.DBhandle, e = DU.NewMmmcDB(dbArg)
	if e != nil {
		return fmt.Errorf("DB path failure: %w", e)
	}
	theDBexists = pXAC.DBhandle.PathProps.Exists()
	var s = "exists"
	if !theDBexists {
		s = "does not exist"
	}
	L.L.Info("DB %s: %s", s, FU.Tildotted(pXAC.DBhandle.PathProps.AbsFP.S()))

	if pXAC.DBdoZeroOut {
		L.L.Progress("Zeroing out DB")
		if theDBexists {
			pXAC.DBhandle.MoveCurrentToBackup()
		}
		pXAC.DBhandle.ForceEmpty()
	} else {
		if theDBexists {
			pXAC.DBhandle.DupeCurrentToBackup()
		}
		pXAC.DBhandle.ForceExistDBandTables()
	}
	return nil
}
