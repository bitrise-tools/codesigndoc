package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-utils/stringutil"

	"github.com/bitrise-io/codesigndoc/codesign"
	"github.com/bitrise-io/codesigndoc/codesigndoc"
	"github.com/bitrise-io/codesigndoc/xcode"
	"github.com/bitrise-io/go-utils/colorstring"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-xcode/utility"
	"github.com/bitrise-io/goinp/goinp"
	"github.com/spf13/cobra"
)

// xcodeCmd represents the xcode command
var xcodeCmd = &cobra.Command{
	Use:   "xcode",
	Short: "Xcode project scanner",
	Long:  `Scan an Xcode project`,

	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          scanXcodeProject,
}

var (
	paramXcodeProjectFilePath string
	paramXcodeScheme          string
	paramXcodebuildSDK        string
)

func init() {
	scanCmd.AddCommand(xcodeCmd)

	xcodeCmd.Flags().StringVar(&paramXcodeProjectFilePath, "file", "", "Xcode Project/Workspace file path")
	xcodeCmd.Flags().StringVar(&paramXcodeScheme, "scheme", "", "Xcode Scheme")
	xcodeCmd.Flags().StringVar(&paramXcodebuildSDK, "xcodebuild-sdk", "", "xcodebuild -sdk param. If a value is specified for this flag it'll be passed to xcodebuild as the value of the -sdk flag. For more info about the values please see xcodebuild's -sdk flag docs. Example value: iphoneos")
}

func absOutputDir() (string, error) {
	confExportOutputDirPath := "./codesigndoc_exports"
	absExportOutputDirPath, err := pathutil.AbsPath(confExportOutputDirPath)
	log.Debugf("absExportOutputDirPath: %s", absExportOutputDirPath)
	if err != nil {
		return absExportOutputDirPath, fmt.Errorf("Failed to determine absolute path of export dir: %s", confExportOutputDirPath)
	}
	return absExportOutputDirPath, nil
}

func scanXcodeProject(cmd *cobra.Command, args []string) error {
	absExportOutputDirPath, err := absOutputDir()
	if err != nil {
		return err
	}

	// Output tools versions
	xcodebuildVersion, err := utility.GetXcodeVersion()
	if err != nil {
		return fmt.Errorf("failed to get Xcode (xcodebuild) version, error: %s", err)
	}
	fmt.Println()
	log.Infof("%s: %s (%s)", colorstring.Green("Xcode (xcodebuild) version"), xcodebuildVersion.Version, xcodebuildVersion.BuildVersion)
	fmt.Println()

	xcodeCmd := xcode.CommandModel{}

	projectPath := paramXcodeProjectFilePath
	if projectPath == "" {
		log.Infof("Scan the directory for project files")
		log.Warnf("You can specify the Xcode project/workscape file to scan with the --file flag.")

		//
		// Scan the directory for Xcode Project (.xcworkspace / .xcodeproject) file first
		// If can't find any, ask the user to drag-and-drop the file
		projpth, err := findXcodeProject()
		if err != nil {
			return err
		}

		projectPath = strings.Trim(strings.TrimSpace(projpth), "'\"")
	}
	log.Debugf("projectPath: %s", projectPath)
	xcodeCmd.ProjectFilePath = projectPath

	schemeToUse := paramXcodeScheme
	if schemeToUse == "" {
		fmt.Println()
		log.Printf("🔦  Scanning Schemes ...")
		schemes, err := xcodeCmd.ScanSchemes()
		if err != nil {
			return ArchiveError{toolXcode, "failed to scan Schemes: " + err.Error()}
		}
		log.Debugf("schemes: %v", schemes)

		if len(schemes) == 0 {
			return ArchiveError{toolXcode, "no schemes found"}
		} else if len(schemes) == 1 {
			schemeToUse = schemes[0]
		} else {
			fmt.Println()
			selectedScheme, err := goinp.SelectFromStringsWithDefault("Select the Scheme you usually use in Xcode", 1, schemes)
			if err != nil {
				return fmt.Errorf("failed to select Scheme: %s", err)
			}
			schemeToUse = selectedScheme
		}

		log.Debugf("selected scheme: %v", schemeToUse)
	}
	xcodeCmd.Scheme = schemeToUse

	if paramXcodebuildSDK != "" {
		xcodeCmd.SDK = paramXcodebuildSDK
	}

	fmt.Println()
	log.Printf("🔦  Running an Xcode Archive, to get all the required code signing settings...")
	var isLogFileWritten bool
	xcodebuildOutputFilePath := filepath.Join(absExportOutputDirPath, "xcodebuild-output.log")
	archivePath, xcodebuildOutput, err := xcodeCmd.GenerateArchive()

	if writeFiles == codesign.WriteFilesAlways ||
		writeFiles == codesign.WriteFilesFallback && err != nil { // save the xcodebuild output into a debug log file
		if err := os.MkdirAll(absExportOutputDirPath, 0700); err != nil {
			return fmt.Errorf("failed to create output directory, error: %s", err)
		}
		log.Infof("💡  "+colorstring.Yellow("Saving xcodebuild output into file")+": %s", xcodebuildOutputFilePath)
		if err := fileutil.WriteStringToFile(xcodebuildOutputFilePath, xcodebuildOutput); err != nil {
			log.Errorf("Failed to save xcodebuild output into file (%s), error: %s", xcodebuildOutputFilePath, err)
		} else {
			isLogFileWritten = true
		}
	}
	if err != nil {
		log.Warnf("Last lines of build log:")
		fmt.Println(stringutil.LastNLines(xcodebuildOutput, 15))
		fmt.Println()
		if isLogFileWritten {
			log.Warnf("Please check the logfile (%s) to see what caused the error", xcodebuildOutputFilePath)
			log.Warnf("and make sure that you can Archive this project from Xcode!")
			fmt.Println()
			log.Printf("Open the project: %s", xcodeCmd.ProjectFilePath)
			log.Printf("and Archive, using the Scheme: %s", xcodeCmd.Scheme)
			fmt.Println()
		}
		return ArchiveError{toolXcode, err.Error()}
	}

	// If certificatesOnly is set, CollectCodesignFiles returns an empty slice for profiles
	certificatesToExport, profilesToExport, err := codesigndoc.CollectCodesignFiles(archivePath, certificatesOnly)
	if err != nil {
		return err
	}
	exoprtResult, err := codesign.UploadAndWriteCodesignFiles(certificatesToExport,
		profilesToExport,
		isAskForPassword,
		codesign.WriteFilesConfig{
			WriteFiles:       writeFiles,
			AbsOutputDirPath: absExportOutputDirPath,
		},
		codesign.UploadConfig{
			PersonalAccessToken: personalAccessToken,
			AppSlug:             appSlug,
		})
	if err != nil {
		return err
	}

	printFinished(exoprtResult, absExportOutputDirPath)
	return nil
}
