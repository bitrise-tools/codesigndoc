package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/bitrise-io/codesigndoc/codesign"
	"github.com/bitrise-io/codesigndoc/codesigndoc"
	"github.com/bitrise-io/codesigndoc/xamarin"
	"github.com/bitrise-io/go-utils/colorstring"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/stringutil"
	"github.com/bitrise-io/go-xamarin/analyzers/project"
	"github.com/bitrise-io/go-xamarin/analyzers/solution"
	"github.com/bitrise-io/go-xamarin/builder"
	"github.com/bitrise-io/go-xamarin/constants"
	"github.com/bitrise-io/goinp/goinp"
	"github.com/spf13/cobra"
)

// xamarinCmd represents the xamarin command
var xamarinCmd = &cobra.Command{
	Use:   "xamarin",
	Short: "Xamarin project scanner",
	Long:  `Scan a Xamarin project`,

	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          scanXamarinProject,
}

var (
	paramXamarinSolutionFilePath  = ""
	paramXamarinConfigurationName = ""
)

func init() {
	scanCmd.AddCommand(xamarinCmd)

	xamarinCmd.Flags().StringVar(&paramXamarinSolutionFilePath, "file", "", `Xamarin Solution file path`)
	xamarinCmd.Flags().StringVar(&paramXamarinConfigurationName, "config", "", `Xamarin Configuration Name (e.g.: "Release|iPhone")`)
}

func archivableSolutionConfigNames(projectsByID map[string]project.Model) []string {
	archivableSolutionConfigNameSet := map[string]bool{}
	for _, project := range projectsByID {
		if project.SDK != constants.SDKIOS || project.OutputType != "exe" {
			continue
		}

		var archivableProjectConfigNames []string
		for name, config := range project.Configs {
			if builder.IsDeviceArch(config.MtouchArchs...) {
				archivableProjectConfigNames = append(archivableProjectConfigNames, name)
			}

		}

		for solutionConfigName, projectConfigName := range project.ConfigMap {
			for _, archivableProjectConfigName := range archivableProjectConfigNames {
				if archivableProjectConfigName == projectConfigName {
					archivableSolutionConfigNameSet[solutionConfigName] = true
				}
			}
		}
	}

	archivableSolutionConfigNames := []string{}
	for configName := range archivableSolutionConfigNameSet {
		archivableSolutionConfigNames = append(archivableSolutionConfigNames, configName)
	}
	sort.Strings(archivableSolutionConfigNames)

	return archivableSolutionConfigNames
}

func scanXamarinProject(cmd *cobra.Command, args []string) error {
	absExportOutputDirPath, err := absOutputDir()
	if err != nil {
		return err
	}

	xamarinCmd := xamarin.CommandModel{}
	// --- Inputs ---

	// Xamarin Solution Path
	xamarinCmd.SolutionFilePath = paramXamarinSolutionFilePath
	if xamarinCmd.SolutionFilePath == "" {
		fmt.Println()
		log.Infof("Scan the directory for solution files")
		log.Warnf("You can specify the Xamarin Solution file to scan with the --file flag.")

		//
		// Scan the directory for Xamarin.Solution file first
		// If can't find any, ask the user to drag-and-drop the file
		var err error
		if xamarinCmd.SolutionFilePath, err = findXamarinSolution(); err != nil {
			return err
		}
	}
	log.Debugf("xamSolutionPth: %s", xamarinCmd.SolutionFilePath)

	xamSln, err := solution.New(xamarinCmd.SolutionFilePath, true)
	if err != nil {
		return fmt.Errorf("failed to analyze Xamarin solution: %s", err)
	}

	if enableVerboseLog {
		b, err := json.MarshalIndent(xamSln, "", "\t")
		if err == nil {
			log.Debugf("xamarin solution:\n%s", b)
		}
	}

	archivableSolutionConfigNames := archivableSolutionConfigNames(xamSln.ProjectMap)

	if len(archivableSolutionConfigNames) < 1 {
		return ArchiveError{toolXamarin, `no acceptable Configuration found in the provided Solution and Project, or none can be used for iOS "Archive for Publishing".`}
	}

	// Xamarin Configuration Name
	selectedXamarinConfigurationName := ""
	{
		if paramXamarinConfigurationName != "" {
			// configuration specified via flag/param
			for _, configName := range archivableSolutionConfigNames {
				if paramXamarinConfigurationName == configName {
					selectedXamarinConfigurationName = configName
					break
				}
			}
			if selectedXamarinConfigurationName == "" {
				return ArchiveError{toolXamarin, fmt.Sprintf("invalid Configuration specified (%s), either not found in the provided Solution and Project or it can't be used for iOS Archive.", paramXamarinConfigurationName)}
			}
		} else {
			// no configuration CLI param specified
			if len(archivableSolutionConfigNames) == 1 {
				selectedXamarinConfigurationName = archivableSolutionConfigNames[0]
			} else {
				fmt.Println()
				answerValue, err := goinp.SelectFromStringsWithDefault(`Select the Configuration Name you use for "Archive for Publishing" (usually Release|iPhone)`, 1, archivableSolutionConfigNames)
				if err != nil {
					return fmt.Errorf("failed to select Configuration: %s", err)
				}
				log.Debugf("selected configuration: %v", answerValue)
				selectedXamarinConfigurationName = answerValue
			}
		}
	}
	if selectedXamarinConfigurationName == "" {
		return ArchiveError{toolXamarin, `no acceptable Configuration found (it was empty) in the provided Solution and Project, or none can be used for iOS "Archive for Publishing".`}
	}
	if err := xamarinCmd.SetConfigurationPlatformCombination(selectedXamarinConfigurationName); err != nil {
		return fmt.Errorf("failed to set Configuration Platform combination for the command, error: %s", err)
	}

	fmt.Println()
	fmt.Println()
	log.Printf(`🔦  Running a Build, to get all the required code signing settings...`)
	logOutputFilePath := filepath.Join(absExportOutputDirPath, "xamarin-build-output.log")

	archivePath, logOutput, err := xamarinCmd.GenerateArchive()
	if writeFiles == codesign.WriteFilesAlways || writeFiles == codesign.WriteFilesFallback && err != nil { // save the xamarin output into a debug log file
		if err := os.MkdirAll(absExportOutputDirPath, 0700); err != nil {
			return fmt.Errorf("failed to create output directory, error: %s", err)
		}

		log.Infof("💡  "+colorstring.Yellow("Saving xamarin output into file")+": %s", logOutputFilePath)
		if err := fileutil.WriteStringToFile(logOutputFilePath, logOutput); err != nil {
			log.Errorf("Failed to save xamarin build output into file (%s), error: %s", logOutputFilePath, err)
		}
	}
	if err != nil {
		log.Warnf("Last lines of the build log:")
		fmt.Println(stringutil.LastNLines(logOutput, 15))

		log.Infof(colorstring.Yellow("Please check the build log to see what caused the error."))
		fmt.Println()

		log.Errorf("Build failed.")
		log.Infof(colorstring.Yellow("Open the project: ")+"%s", xamarinCmd.SolutionFilePath)
		log.Infof(colorstring.Yellow(`And do "Archive for Publishing", after selecting the Configuration+Platform: `)+"%s|%s", xamarinCmd.Configuration, xamarinCmd.Platform)
		fmt.Println()

		return ArchiveError{toolXamarin, "failed to run xamarin build command: " + err.Error()}
	}

	// If certificatesOnly is set, CollectCodesignFiles returns an empty slice for profiles
	certificatesToExport, profilesToExport, err := codesigndoc.CollectCodesignFiles(archivePath, certificatesOnly)
	if err != nil {
		return err
	}

	certificates, profiles, err := codesign.ExportCodesigningFiles(certificatesToExport, profilesToExport, isAskForPassword)
	if err != nil {
		return err
	}

	exportResult, err := codesign.UploadAndWriteCodesignFiles(certificates,
		profiles,
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

	printFinished(exportResult, absExportOutputDirPath)
	return nil
}
