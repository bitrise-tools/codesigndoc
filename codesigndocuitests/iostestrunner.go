package codesigndocuitests

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-xcode/plistutil"
	"github.com/bitrise-io/go-xcode/profileutil"
)

// IOSTestRunner ...
type IOSTestRunner struct {
	Path                string
	InfoPlist           plistutil.PlistData
	Entitlements        plistutil.PlistData
	ProvisioningProfile profileutil.ProvisioningProfileInfoModel
}

// NewIOSTestRunners is the *-Runner.app which is generated with the xcodebuild build-for-testing command
func NewIOSTestRunners(path string) ([]*IOSTestRunner, error) {
	runnerPattern := filepath.Join(pathutil.EscapeGlobPath(path), "*-Runner.app")
	possibleTestRunnerPths, err := filepath.Glob(runnerPattern)
	if err != nil {
		return nil, err
	}

	if len(possibleTestRunnerPths) == 0 {
		return nil, fmt.Errorf("no Test-Runner.app found in %s", path)
	}

	var testRunners []*IOSTestRunner
	for _, testRunnerPath := range possibleTestRunnerPths {
		infoPlist := plistutil.PlistData{}
		{
			infoPlistPath := filepath.Join(testRunnerPath, "Info.plist")
			if exist, err := pathutil.IsPathExists(infoPlistPath); err != nil {
				return nil, fmt.Errorf("failed to check if Info.plist exists at: %s, error: %s", infoPlistPath, err)
			} else if !exist {
				return nil, fmt.Errorf("Info.plist not exists at: %s", infoPlistPath)
			}

			plist, err := plistutil.NewPlistDataFromFile(infoPlistPath)
			if err != nil {
				return nil, err
			}

			infoPlist = plist
		}

		provisioningProfile := profileutil.ProvisioningProfileInfoModel{}
		{
			provisioningProfilePath := filepath.Join(testRunnerPath, "embedded.mobileprovision")
			if exist, err := pathutil.IsPathExists(provisioningProfilePath); err != nil {
				return nil, fmt.Errorf("failed to check if profile exists at: %s, error: %s", provisioningProfilePath, err)
			} else if !exist {
				return nil, fmt.Errorf("profile not exists at: %s", provisioningProfilePath)
			}

			profile, err := profileutil.NewProvisioningProfileInfoFromFile(provisioningProfilePath)
			if err != nil {
				return nil, err
			}
			provisioningProfile = profile
		}

		entitlements := plistutil.PlistData{}
		{
			cmd := command.New("codesign", "-d", "--entitlements", "-", testRunnerPath)
			out, err := cmd.RunAndReturnTrimmedOutput()
			if err != nil {
				return nil, err
			}

			// The codesign -d --entitlements command's output contains unnecessary characters before the valid xml
			// We need to trim them before parsing the xml
			outSplit := strings.Split(out, "<?xml version")
			if len(outSplit) > 1 {
				out = outSplit[1]
			}

			entitlements, err = plistutil.NewPlistDataFromContent(out)
			if err != nil {
				return nil, err
			}
		}

		testRunners = append(testRunners, &IOSTestRunner{
			Path:                testRunnerPath,
			InfoPlist:           infoPlist,
			Entitlements:        entitlements,
			ProvisioningProfile: provisioningProfile,
		})
	}

	return testRunners, nil
}

// BundleIDEntitlementsMap ...
func (runner IOSTestRunner) BundleIDEntitlementsMap() map[string]plistutil.PlistData {
	bundleIDEntitlementsMap := map[string]plistutil.PlistData{}

	bundleID := strings.TrimSuffix(runner.ProvisioningProfile.BundleID, "-Runner")
	bundleIDEntitlementsMap[bundleID] = runner.ProvisioningProfile.Entitlements

	return bundleIDEntitlementsMap
}

// IsXcodeManaged ...
func (runner IOSTestRunner) IsXcodeManaged() bool {
	return runner.ProvisioningProfile.IsXcodeManaged()
}
