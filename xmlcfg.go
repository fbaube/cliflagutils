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

	"github.com/fbaube/db"
	FU "github.com/fbaube/fileutils"
	MU "github.com/fbaube/miscutils"
	SU "github.com/fbaube/stringutils"
	WU "github.com/fbaube/wasmutils"
	XM "github.com/fbaube/xmlmodels"
)

var inArg, outArg, dbArg, xmlCatArg, xmlSchemasArg string

// XmlAppConfiguration can probably be used with various 3rd-party utilities.
type XmlAppConfiguration struct {
	AppName                                           string
	DBhandle                                          *db.MmmcDB
	Infile, Outfile, Dbdir, Xmlcatfile, Xmlschemasdir FU.PathProps // NOT ptr! Barfs at startup.
	RestPort                                          int
	// CLI flags
	FollowSymLinks, Pritt, DBdoImport, Help, Debug, GroupGenerated, Validate, DBdoZeroOut bool
	// Result of processing CLI arg for input file(s)
	SingleFile bool
	// Result of processing CLI args (-c, -s)
	*XM.XmlCatalogFile
}

var myAppName string

var multipleXmlCatalogFiles []*XM.XmlCatalogFile

// CA maybe should not be exported. Or should be generated
// on-the-fly instead of being a Singleton.
// // var CA XmlAppConfiguration

func myUsage() {
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
		"(e.g. ./Filnam.xml maps to ./Filenam.xml_gxml/Filenam.*)",
	"m": "Import input file(s) to database",
	"p": "Pretty-print to file with \"fmtd-\" prepended to file extension",
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
	flag.StringVarP(&outArg, "outfile", "o", "", UMM["o"])
	flag.StringVarP(&xmlCatArg, "catalog", "c", "", UMM["c"])
	flag.StringVarP(&dbArg, "db-dir", "d", "", UMM["d"])
	flag.StringVarP(&xmlSchemasArg, "search", "s", "", UMM["s"])
	flag.BoolVarP(&pXAC.DBdoImport, "import", "m", false, UMM["m"])
	flag.BoolVarP(&pXAC.FollowSymLinks, "symlinks", "L", true, UMM["L"])
	flag.BoolVarP(&pXAC.GroupGenerated, "group-gen", "g", false, UMM["g"])
	flag.BoolVarP(&pXAC.Pritt, "pretty", "p", true, UMM["p"])
	flag.BoolVarP(&pXAC.Debug, "debug", "D", false, UMM["D"])
	flag.BoolVarP(&pXAC.Help, "help", "h", false, UMM["h"])
	flag.BoolVarP(&pXAC.Validate, "validate", "v", false, UMM["v"])
	flag.BoolVarP(&pXAC.DBdoZeroOut, "zero-out", "z", false, UMM["z"])
	flag.IntVarP(&pXAC.RestPort, "rest-port", "r", 0, UMM["r"])
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
	MU.SessionLogger.Printf("%s failed: %s \n", myAppName, e)
	fmt.Fprintf(os.Stderr, "%s failed: %s \n", myAppName, e)
	MU.ErrorTrace(os.Stderr, e)
	os.Exit(1)
}

// NewXmlAppConfiguration processes CLI arguments for any XML-related command.
func NewXmlAppConfiguration(appName string, osArgs []string) (*XmlAppConfiguration, error) {
	var pXAC *XmlAppConfiguration
	pXAC = new(XmlAppConfiguration)
	initVars(pXAC)
	DisableFlags("hDgpr")
	// Do not use logging until the invocation is sorted out.
	myAppName = appName
	pXAC.AppName = appName
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
			if pXAC.Validate {
				println("==> Validation is not possible: xmllint cannot be found")
			}
		}
		println("==> xmllint:", xl)
	}
	// == Examine CLI invocation flags ==
	flag.Parse()
	fmt.Printf("CMD-TAIL: %+v \n", flag.Args())
	// FIXME - pos'l arg OR "-i" OR stdin OR "-"
	if len(osArgs) < 2 || nil == flag.Args() || 0 == len(flag.Args()) {
		println("==> Argument parsing failed. Did not specify input file(s)?")
		myUsage()
		os.Exit(1)
	}
	if pXAC.Debug {
		fmt.Printf("D=> Flags: debug:%s grpGen:%s help:%s "+
			"import:%s printty:%s validate:%s zeroOutDB:%s restPort:%d \n",
			SU.Yn(pXAC.Debug), SU.Yn(pXAC.GroupGenerated), // d g h m p v z r
			SU.Yn(pXAC.Help), SU.Yn(pXAC.DBdoImport), SU.Yn(pXAC.Pritt),
			SU.Yn(pXAC.Validate), SU.Yn(pXAC.DBdoZeroOut), pXAC.RestPort)
		fmt.Println("D=> CLI tail:", flag.Args())
	}

	// ===========================================
	//   PROCESS INPUT SPEC
	// ===========================================

	// Handle case where XML comes from standard input i.e. os.Stdin
	if flag.Args()[0] == "-" {
		if WU.IsWasm() {
			println("==> FIXME Trying to read from Stdin; press ^D right after a newline to end")
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
		pXAC.Infile = *FU.NewPathProps("Stdin.xml") // .RelFilePath = "Stdin.xml"

	} else {
		// ===========================================
		//   PROCESS INPUT SPEC (normal case) to get
		//   info about path, existence, and type
		// ===========================================
		// Process input-file(s) argument, which can be a relative filepath.
		pXAC.Infile = *FU.NewPathProps(flag.Args()[0])
		// If the absolute path does not match the argument provided, inform the user.
		if pXAC.Infile.AbsFP() != flag.Args()[0] { // CA.In.RelFilePath { // CA.In.ArgFilePath {
			println("==> Input:", FU.Enhomed(pXAC.Infile.AbsFP()))
		}
		if pXAC.Infile.IsOkayDir() {
			println("    --> The input is a directory and will be processed recursively.")
		} else if pXAC.Infile.IsOkayFile() {
			println("    --> The input is a single file: extra info will be listed here.")
			pXAC.SingleFile = true
		} else {
			println("    --> The input is a type not understood.")
			return nil, fmt.Errorf("Bad type for input: " + pXAC.Infile.AbsFP())
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
	e = pXAC.ProcessCatalogArgs()
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
	pXAC.DBhandle, e = db.NewMmmcDB(dbArg)
	if e != nil {
		return fmt.Errorf("DB setup failure: %w", e)
	}
	theDBexists = pXAC.DBhandle.PathProps.Exists()
	var s = "exists"
	if !theDBexists {
		s = "does not exist"
	}
	fmt.Printf("==> DB %s: %s\n", s, pXAC.DBhandle.PathProps.AbsFP())

	if pXAC.DBdoZeroOut {
		println("    --> Zeroing out DB")
		pXAC.DBhandle.MoveCurrentToBackup()
		pXAC.DBhandle.ForceEmpty()
	} else {
		pXAC.DBhandle.DupeCurrentToBackup()
		pXAC.DBhandle.ForceExistDBandTables()
	}
	// spew.Dump(pCA.DBhandle)
	return nil
}

func (pXAC *XmlAppConfiguration) ProcessCatalogArgs() error {
	var gotC, gotS bool
	gotC = ("" != xmlCatArg)
	gotS = ("" != xmlSchemasArg)
	if !(gotC || gotS) {
		return nil
	}
	if gotC && gotS {
		return errors.New("mcfile.ConfArgs.ProcCatalArgs: cannot combine flags -c and -s")
	}
	if gotC { // -c
		// pCA.XmlCat.ProcessFilePathArg(CA.XmlCat.ArgFilePath)
		pXAC.Xmlcatfile = *FU.NewPathProps(xmlCatArg)
		if !(pXAC.Xmlcatfile.IsOkayFile() && pXAC.Xmlcatfile.Size() > 0) {
			println("==> ERROR: XML catalog filepath is not file: " + pXAC.Xmlcatfile.AbsFP())
			return errors.New(fmt.Sprintf("mcfile.ConfArgs.ProcCatalArgs<%s:%s>",
				xmlCatArg, pXAC.Xmlcatfile.AbsFP()))
		}
		println("==> Catalog:", xmlCatArg)
		if pXAC.Xmlcatfile.AbsFP() != xmlCatArg {
			println("     --> i.e. ", FU.Enhomed(pXAC.Xmlcatfile.AbsFP()))
		}
	}
	if gotS { // -s
		pXAC.Xmlschemasdir = *FU.NewPathProps(xmlSchemasArg)
		if !pXAC.Xmlschemasdir.IsOkayDir() {
			return errors.New("mcfile.ConfArgs.ProcCatalArgs: cannot open XML catalog directory: " +
				pXAC.Xmlschemasdir.AbsFP())
		}
	}
	var e error
	if gotS { // -s and not -c
		println("==> Schema(s):", xmlSchemasArg)
		pXAC.Xmlschemasdir = *FU.NewPathProps(xmlSchemasArg)
		if pXAC.Xmlschemasdir.AbsFP() != xmlSchemasArg {
			println("     --> i.e. ", FU.Enhomed(pXAC.Xmlschemasdir.AbsFP()))
		}
		if !pXAC.Xmlschemasdir.IsOkayDir() {
			println("==> ERROR: Schema path is not a readable directory: " +
				FU.Enhomed(pXAC.Xmlschemasdir.AbsFP()))
			return fmt.Errorf("mcfile.ConfArgs.ProcCatalArgs.abs<%s>: %w",
				pXAC.Xmlschemasdir.AbsFP(), e)
		}
	}
	// println(" ")

	// ==========================
	//   PROCESS XML CATALOG(S)
	// ==========================

	// IF user asked for a single catalog file
	if gotC && !gotS {
		pXAC.XmlCatalogFile, e = XM.NewXmlCatalogFile(xmlCatArg)
		if e != nil {
			println("==> ERROR: Can't find or process catalog file:", xmlCatArg)
			println("    Error was:", e.Error())
			pXAC.XmlCatalogFile = nil
			return fmt.Errorf("gxml.Confargs.NewXmlCatalogFromFile<%s>: %w", xmlCatArg, e)
		}
		if pXAC.XmlCatalogFile == nil ||
			len(pXAC.XmlCatalogFile.XmlPublicIDsubrecords) == 0 {
			println("==> No valid entries in catalog file:", xmlCatArg)
			pXAC.XmlCatalogFile = nil
		}
		return nil
	}
	// IF user asked for a directory scan of schema files
	if gotS && !gotC {
		multipleXmlCatalogFiles = make([]*XM.XmlCatalogFile, 0)
		fileNameToUse := "catalog.xml"
		if xmlCatArg != "" {
			fileNameToUse = xmlCatArg
		}
		filePathToUse := FU.AbsFilePath(".")
		if xmlCatArg != "" {
			filePathToUse = FU.AbsFilePath(pXAC.Xmlschemasdir.AbsFP())
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
			var xmlCat *XM.XmlCatalogFile
			xmlCat, e = XM.NewXmlCatalogFile(filePathToUse.S())
			if e != nil {
				println("==> ERROR: Can't find or process catalog file:", filePathToUse)
				println("    Error was:", e.Error())
				continue
			}
			if xmlCat == nil || len(xmlCat.XmlPublicIDsubrecords) == 0 {
				println("==> No valid entries in catalog file:", filePathToUse)
				continue
			}
			multipleXmlCatalogFiles = append(multipleXmlCatalogFiles, xmlCat)
		}
		switch len(multipleXmlCatalogFiles) {
		case 0:
			fmt.Printf("==> ERROR: No files named <%s> found in+under <%s>:",
				fileNameToUse, filePathToUse)
			pXAC.XmlCatalogFile = nil
			return fmt.Errorf("gxml.Confargs.XmlCatalogs<%s:%s>: %w",
				fileNameToUse, filePathToUse, e)
		case 1:
			pXAC.XmlCatalogFile = multipleXmlCatalogFiles[0]
		default:
			// MERGE THEM ALL
			var xmlCat *XM.XmlCatalogFile
			pXAC.XmlCatalogFile = new(XM.XmlCatalogFile)
			pXAC.XmlCatalogFile.XmlPublicIDsubrecords =
				make([]XM.PIDSIDcatalogFileRecord, 0)
			for _, xmlCat = range multipleXmlCatalogFiles {
				pXAC.XmlCatalogFile.XmlPublicIDsubrecords =
					append(pXAC.XmlCatalogFile.XmlPublicIDsubrecords,
						xmlCat.XmlPublicIDsubrecords...)
			}
		}
	}
	if pXAC.XmlCatalogFile == nil ||
		pXAC.XmlCatalogFile.XmlPublicIDsubrecords == nil ||
		len(pXAC.XmlCatalogFile.XmlPublicIDsubrecords) == 0 {
		pXAC.XmlCatalogFile = nil
		println("==> No valid catalog entries")
		return errors.New("gxml.Confargs.XmlCatalogs")
	}
	// println("==> Contents of XML catalog(s):")
	// print(CA.XmlCatalog.DString())
	fmt.Printf("==> XML catalog(s) yielded %d valid entries \n",
		len(pXAC.XmlCatalogFile.XmlPublicIDsubrecords))

	// TODO:470 If import, create batch info ?
	return nil
}
