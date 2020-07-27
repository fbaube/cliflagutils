package cliflagutils

import (
	// "flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	FP "path/filepath"

	flag "github.com/spf13/pflag"

	"errors"

	// XCU "github.com/fbaube/cliflagutils"
	"github.com/fbaube/db"
	FU "github.com/fbaube/fileutils"
	"github.com/fbaube/gparse"
	MU "github.com/fbaube/miscutils"
	SU "github.com/fbaube/stringutils"
	WU "github.com/fbaube/wasmutils"
	XM "github.com/fbaube/xmlmodels"
)

/*
func (f *FlagSet) StringVarP(p *string, name, shorthand string, value string, usage string)
func (f *FlagSet) BoolVarP(p *bool, name, shorthand string, value bool, usage string)
func (f *FlagSet) IntVarP(p *int, name, shorthand string, value int, usage string)
*/

var sOutRelFP, sXmlCatRelFP, sXmlCatSearchRelFP string

// XmlAppConfiguration can probably be used with various 3rd-party utilities.
type XmlAppConfiguration struct {
	AppName                       string
	DBdirPath                     string
	DBhandle                      *db.MmmcDB
	In, Out, XmlCat, XmlCatSearch FU.PathInfo // NOT ptr! Barfs at startup.
	RestPort                      int
	// CLI flags
	FollowSymLinks, Pritt, DBdoImport, Help, Debug, GroupGenerated, Validate, DBdoZeroOut bool
	// Result of processing CLI arg for input file(s)
	SingleFile bool
	// Result of processing CLI args (-c, -s)
	*gparse.XmlCatalogRecord
}

// var InFP, OutFP, XmlCatFP, XmlCatSearchFP string
var xmlCatalogRecords []*gparse.XmlCatalogRecord

// CA maybe should not be exported. Or should be generated
// on-the-fly instead of being a Singleton.
var CA XmlAppConfiguration

func myUsage() {
	//  Println(CA.AppName, "[-d] [-g] [-h] [-m] [-p] [-v] [-z] [-D] [-o outfile] [-d dbdir] Infile")
	fmt.Println(CA.AppName, "[-d] [-g] [-h] [-m] [-p] [-v] [-z] [-D] [-d dbdir] [-r port] Infile")
	fmt.Println("   Process mixed content XML, XHTML (XDITA), and Markdown (MDITA) input.")
	fmt.Println("   Infile is a single file or directory name; no wildcards (?,*).")
	fmt.Println("          If a directory, it is processed recursively.")
	fmt.Println("   Infile may be \"-\" for Stdin: input typed (or pasted) interactively")
	fmt.Println("          is written to file ./Stdin.xml for processing")
	flag.Usage()
}

// What are the filepaths we can ask for ?
// input file(s)/dir
// output file(s)/dir
// path to XML catalog file
// path to schema file(s)
// path to DB file

// For each DOCTYPE. also an XSL, thus XSL catalog file.
// Maybe also same for CSS.

// UMM is the Usage Message Map. We do this to provide (here) a quick
// option reference, and (below) to make the actual code clearer.
// e.g. commits := map[string]int { "rsc": 3711, "r":   2138, }
var UMM = map[string]string{
	// Simple BOOLs
	"h": "Show extended help message and exit",
	"g": "Group all generated files in same-named folder \n" +
		"(e.g. ./Filnam.xml maps to ./Filenam.xml_gxml/Filenam.*)",
	"m": "Import input file(s) to database",
	"p": "Pretty-print to file with \"fmtd-\" prepended to file extension",
	"v": "Validate input file(s)? (using xmllint) (with flag \"-c\" or \"-s\")",
	"z": "Zero out the database",
	"D": "Turn on debugging",
	"L": "Follow symbolic links in directory recursion",
	// All others
	"c": "XML `catalog_filepath` (do not use with \"-s\" flag)",
	"d": "Directory path of/for database mmmc.db",
	"r": "Run REST server on `port number`",
	"s": "Directory `path` to DTD schema file(s) (.dtd, .mod)",
}

func initVars() {
	// flag.StringVar(&CA.Out.RelFilePath, "o", "", // &CA.outArg, "o", "",
	// 	"Output file name (possibly ignored, depending on command)")
	// pflag:
	// flag.BoolVarP(&flagvar, "boolname", "b", true, "help message")
	flag.StringVarP(&sXmlCatRelFP, "catalog", "c", "", UMM["c"])
	flag.StringVarP(&CA.DBdirPath, "db-dir", "d", "", UMM["d"])
	flag.StringVarP(&sXmlCatSearchRelFP, "search", "s", "", UMM["s"])
	flag.BoolVarP(&CA.DBdoImport, "import", "m", false, UMM["m"])
	flag.BoolVarP(&CA.FollowSymLinks, "symlinks", "L", true, UMM["L"])
	flag.BoolVarP(&CA.GroupGenerated, "group-gen", "g", false, UMM["g"])
	flag.BoolVarP(&CA.Pritt, "pretty", "p", true, UMM["p"])
	flag.BoolVarP(&CA.Debug, "debug", "D", false, UMM["D"])
	flag.BoolVarP(&CA.Help, "help", "h", false, UMM["h"])
	flag.BoolVarP(&CA.Validate, "validate", "v", false, UMM["v"])
	flag.BoolVarP(&CA.DBdoZeroOut, "zero-out", "z", false, UMM["z"])
	flag.IntVarP(&CA.RestPort, "rest-port", "r", 0, UMM["r"])
	EnableAllFlags()
	// fmt.Printf("FLAGS %+v \n", flag.CommandLine)
	// func (f *FlagSet) VisitAll(fn func(*Flag))
	// flag.CommandLine.VisitAll(myFlagFunc)
}

// checkbarf simply aborts with an error message, if a
// serious (i.e. top-level) problem has been encountered.
func checkbarf(e error, s string) {
	if e == nil {
		return
	}
	MU.SessionLogger.Printf("%s failed: %s \n", CA.AppName, e)
	fmt.Fprintf(os.Stderr, "%s failed: %s \n", CA.AppName, e)
	MU.ErrorTrace(os.Stderr, e)
	os.Exit(1)
}

// ProcessArgs wants to be a generic CLI arguments for any XML-related command.
// If this is to be so, there should be a way to selectively disable commands
// that are inappropriate for the CLI command it is being integrated into.
func ProcessArgs(appName string, osArgs []string) (*XmlAppConfiguration, error) {
	initVars()
	DisableFlags("hDgpr")
	// Do not use logging until the invocation is sorted out.
	CA.AppName = appName
	var e error

	if !WU.IsWasm() {
		// == Figure out what CLI name we were called as ==
		osex, _ := os.Executable()
		// The call to FP.Clean(..) is needed (!!)
		println("==> Running:", FU.Enhomed(FP.Clean(osex)))
		// == Locate xmllint for doing XML validations ==
		xl, e := exec.LookPath("xmllint")
		if e != nil {
			xl = "not found"
			if CA.Validate {
				println("==> Validation is not possible: xmllint cannot be found")
			}
		}
		println("==> xmllint:", xl)
	}
	// == Examine CLI invocation flags ==
	flag.Parse()
	// FIXME - pos'l arg OR "-i" OR stdin OR "-"
	if len(osArgs) < 2 || nil == flag.Args() || 0 == len(flag.Args()) {
		println("==> Argument parsing failed. Did not specify input file(s)?")
		myUsage()
		os.Exit(1)
	}
	if CA.Debug {
		fmt.Printf("D=> Flags: debug:%s groupGen:%s help:%s "+
			"import:%s printty:%s validate:%s zeroOutDB:%s restPort:%d \n",
			SU.Yn(CA.Debug), SU.Yn(CA.GroupGenerated), // d g h m p v z r
			SU.Yn(CA.Help), SU.Yn(CA.DBdoImport), SU.Yn(CA.Pritt),
			SU.Yn(CA.Validate), SU.Yn(CA.DBdoZeroOut), CA.RestPort)
		fmt.Println("D=> CLI tail:", flag.Args())
	}

	// ===========================================
	//   PROCESS INPUT SPEC
	// ===========================================

	// Handle case where XML comes from standard input i.e. os.Stdin
	if flag.Args()[0] == "-" {
		if WU.IsWasm() {
			println("==> Trying to read from Stdin; press ^D right after a newline to end")
		} else {
			stat, e := os.Stdin.Stat()
			checkbarf(e, "Cannot Stat() Stdin")
			if (stat.Mode() & os.ModeCharDevice) != 0 {
				println("==> Reading from Stdin; press ^D right after a newline to end")
			} else {
				println("==> Reading Stdin from a file or pipe")
			}
		}
		// bb, e := ioutil.ReadAll(os.Stdin)
		stdIn := FU.GetStringFromStdin()
		checkbarf(e, "Cannot read from Stdin")
		e = ioutil.WriteFile("Stdin.xml", []byte(stdIn), 0666)
		checkbarf(e, "Cannot write to ./Stdin.xml")
		CA.In = *FU.NewPathInfo("Stdin.xml") // .RelFilePath = "Stdin.xml"

	} else {
		// ===========================================
		//   PROCESS INPUT SPEC (normal case) to get
		//   info about path, existence, and type
		// ===========================================
		// Process input-file(s) argument, which can be a relative filepath.
		CA.In = *FU.NewPathInfo(flag.Args()[0])
		// If the absolute path does not match the argument provided, inform the user.
		if CA.In.AbsFP() != flag.Args()[0] { // CA.In.RelFilePath { // CA.In.ArgFilePath {
			println("==> Input:", FU.Enhomed(CA.In.AbsFP()))
		}
		if CA.In.IsOkayDir() {
			println("    --> The input is a directory and will be processed recursively.")
		} else if CA.In.IsOkayFile() {
			println("    --> The input is a single file: extra info will be listed here.")
			CA.SingleFile = true
		} else {
			println("    --> The input is a type not understood.")
			return nil, fmt.Errorf("Bad type for input: " + CA.In.AbsFP())
		}
	}

	// ===========================================
	//   PROCESS ARGUMENTS to get complete info
	//   about path, existence, and type
	// ===========================================

	// Process output-file(s) argument, which can be a relative filepath.
	// CA.Out.ProcessFilePathArg(CA.Out.ArgFilePath)
	CA.Out = *FU.NewPathInfo(sOutRelFP) // CA.Out.RelFilePath)

	// Process database directory argument, which can be a relative filepath.
	// CA.DB.ProcessFilePathArg(CA.DBdirPath)

	// ====

	pCA := &CA
	e = pCA.ProcessDatabaseArgs()
	checkbarf(e, "Could not process DB directory argument(s)")
	e = pCA.ProcessCatalogArgs()
	checkbarf(e, "Could not process XML catalog argument(s)")
	return pCA, e
}

func (pCA *XmlAppConfiguration) ProcessDatabaseArgs() error {
	var mustAccessTheDB, theDBexists bool
	var e error
	mustAccessTheDB = pCA.DBdoImport || pCA.DBdoZeroOut || pCA.DBdirPath != ""
	if !mustAccessTheDB {
		return nil
	}
	pCA.DBhandle, e = db.NewMmmcDB(pCA.DBdirPath)
	if e != nil {
		return fmt.Errorf("DB setup failure: %w", e)
	}
	theDBexists = CA.DBhandle.PathInfo.Exists()
	var s = "exists"
	if !theDBexists {
		s = "does not exist"
	}
	fmt.Printf("==> DB %s: %s\n", s, pCA.DBhandle.PathInfo.AbsFP())

	if pCA.DBdoZeroOut {
		println("    --> Zeroing out DB")
		pCA.DBhandle.MoveCurrentToBackup()
		pCA.DBhandle.ForceEmpty()
	} else {
		pCA.DBhandle.DupeCurrentToBackup()
		pCA.DBhandle.ForceExistDBandTables()
	}
	// spew.Dump(pCA.DBhandle)
	return nil
}

func (pCA *XmlAppConfiguration) ProcessCatalogArgs() error {
	var gotC, gotS bool
	gotC = ("" != sXmlCatRelFP)
	gotS = ("" != sXmlCatSearchRelFP)
	if !(gotC || gotS) {
		return nil
	}
	if gotC && gotS {
		return errors.New("mcfile.ConfArgs.ProcCatalArgs: cannot combine flags -c and -s")
	}
	if gotC { // -c
		// pCA.XmlCat.ProcessFilePathArg(CA.XmlCat.ArgFilePath)
		CA.XmlCat = *FU.NewPathInfo(sXmlCatRelFP)
		if !(pCA.XmlCat.IsOkayFile() && pCA.XmlCat.Size() > 0) {
			println("==> ERROR: XML catalog filepath is not file: " + pCA.XmlCat.AbsFP())
			return errors.New(fmt.Sprintf("mcfile.ConfArgs.ProcCatalArgs<%s:%s>",
				sXmlCatRelFP, CA.XmlCat.AbsFP()))
		}
		println("==> Catalog:", sXmlCatRelFP)
		if pCA.XmlCat.AbsFP() != sXmlCatRelFP {
			println("     --> i.e. ", FU.Enhomed(pCA.XmlCat.AbsFP()))
		}
	}
	if gotS { // -s
		pCA.XmlCatSearch = *FU.NewPathInfo(sXmlCatSearchRelFP)
		if !pCA.XmlCatSearch.IsOkayDir() {
			return errors.New("mcfile.ConfArgs.ProcCatalArgs: cannot open XML catalog directory: " +
				pCA.XmlCatSearch.AbsFP())
		}
	}
	var e error
	if gotS { // -s and not -c
		println("==> Schema(s):", sXmlCatSearchRelFP)
		pCA.XmlCatSearch = *FU.NewPathInfo(sXmlCatSearchRelFP)
		if CA.XmlCatSearch.AbsFP() != sXmlCatSearchRelFP {
			println("     --> i.e. ", FU.Enhomed(pCA.XmlCatSearch.AbsFP()))
		}
		if !pCA.XmlCatSearch.IsOkayDir() {
			println("==> ERROR: Schema path is not a readable directory: " +
				FU.Enhomed(pCA.XmlCatSearch.AbsFP()))
			return fmt.Errorf("mcfile.ConfArgs.ProcCatalArgs.abs<%s>: %w",
				pCA.XmlCatSearch.AbsFP(), e)
		}
	}
	// println(" ")

	// ==========================
	//   PROCESS XML CATALOG(S)
	// ==========================

	// IF user asked for a single catalog file
	if gotC && !gotS {
		CA.XmlCatalogRecord, e = gparse.NewXmlCatalogRecordFromFile(sXmlCatRelFP)
		if e != nil {
			println("==> ERROR: Can't find or process catalog file:", sXmlCatRelFP)
			println("    Error was:", e.Error())
			CA.XmlCatalogRecord = nil
			return fmt.Errorf("gxml.Confargs.NewXmlCatalogFromFile<%s>: %w", sXmlCatRelFP, e)
		}
		if CA.XmlCatalogRecord == nil ||
			len(CA.XmlCatalogRecord.XmlPublicIDsubrecords) == 0 {
			println("==> No valid entries in catalog file:", sXmlCatRelFP)
			CA.XmlCatalogRecord = nil
		}
		return nil
	}
	// IF user asked for a directory scan of schema files
	if gotS && !gotC {
		xmlCatalogRecords = make([]*gparse.XmlCatalogRecord, 0)
		fileNameToUse := "catalog.xml"
		if sXmlCatRelFP != "" {
			fileNameToUse = sXmlCatRelFP
		}
		filePathToUse := FU.AbsFilePath(".")
		if sXmlCatRelFP != "" {
			filePathToUse = FU.AbsFilePath(CA.XmlCatSearch.AbsFP())
		}
		fileNameList, e := filePathToUse.GatherNamedFiles(fileNameToUse)
		if e != nil {
			fmt.Printf("==> No valid files named <%s> found in+under catalog search path: %s \n",
				fileNameToUse, filePathToUse)
			println("    Error was:", e.Error())
			return fmt.Errorf(
				"gxml.Confargs.GatherNamedFilesForCatalog<%s:%s>: %w", fileNameToUse, filePathToUse, e)
		}
		// For every catalog file (usually just one)
		for _, filePathToUse = range fileNameList {
			var xmlCat *gparse.XmlCatalogRecord
			xmlCat, e = gparse.NewXmlCatalogRecordFromFile(filePathToUse.S())
			if e != nil {
				println("==> ERROR: Can't find or process catalog file:", filePathToUse)
				println("    Error was:", e.Error())
				continue
			}
			if xmlCat == nil || len(xmlCat.XmlPublicIDsubrecords) == 0 {
				println("==> No valid entries in catalog file:", filePathToUse)
				continue
			}
			xmlCatalogRecords = append(xmlCatalogRecords, xmlCat)
		}
		switch len(xmlCatalogRecords) {
		case 0:
			fmt.Printf("==> ERROR: No files named <%s> found in+under <%s>:",
				fileNameToUse, filePathToUse)
			CA.XmlCatalogRecord = nil
			return fmt.Errorf("gxml.Confargs.XmlCatalogs<%s:%s>: %w",
				fileNameToUse, filePathToUse, e)
		case 1:
			CA.XmlCatalogRecord = xmlCatalogRecords[0]
		default:
			// MERGE THEM ALL
			var xmlCat *gparse.XmlCatalogRecord
			CA.XmlCatalogRecord = new(gparse.XmlCatalogRecord)
			CA.XmlCatalogRecord.XmlPublicIDsubrecords =
				make([]XM.PIDSIDcatalogFileRecord, 0)
			for _, xmlCat = range xmlCatalogRecords {
				CA.XmlCatalogRecord.XmlPublicIDsubrecords =
					append(CA.XmlCatalogRecord.XmlPublicIDsubrecords,
						xmlCat.XmlPublicIDsubrecords...)
			}
		}
	}
	if CA.XmlCatalogRecord == nil ||
		CA.XmlCatalogRecord.XmlPublicIDsubrecords == nil ||
		len(CA.XmlCatalogRecord.XmlPublicIDsubrecords) == 0 {
		CA.XmlCatalogRecord = nil
		println("==> No valid catalog entries")
		return errors.New("gxml.Confargs.XmlCatalogs")
	}
	// println("==> Contents of XML catalog(s):")
	// print(CA.XmlCatalog.DString())
	fmt.Printf("==> XML catalog(s) yielded %d valid entries \n",
		len(CA.XmlCatalogRecord.XmlPublicIDsubrecords))

	// TODO:470 If import, create batch info ?
	return nil
}
