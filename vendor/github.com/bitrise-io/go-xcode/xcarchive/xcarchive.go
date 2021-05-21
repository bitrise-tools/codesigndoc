package xcarchive

import (
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/ziputil"
	"github.com/bitrise-io/go-xcode/plistutil"
	"github.com/bitrise-io/go-xcode/utility"
)

// IsMacOS try to find the Contents dir under the .app/.
// If its finds it the archive is MacOs. If it does not the archive is iOS.
func IsMacOS(archPath string) (bool, error) {
	log.Debugf("Checking archive is MacOS or iOS")
	infoPlistPath := filepath.Join(archPath, "Info.plist")

	plist, err := plistutil.NewPlistDataFromFile(infoPlistPath)
	if err != nil {
		return false, err
	}

	appProperties, found := plist.GetMapStringInterface("ApplicationProperties")
	if !found {
		return false, err
	}

	applicationPath, found := appProperties.GetString("ApplicationPath")
	if !found {
		return false, err
	}

	applicationPath = filepath.Join(archPath, "Products", applicationPath)
	contentsPath := filepath.Join(applicationPath, "Contents")

	exist, err := pathutil.IsDirExists(contentsPath)
	if err != nil {
		return false, err
	}

	return exist, nil
}

// UnzipXcarchive ...
func UnzipXcarchive(xcarchivePth string) (string, error) {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("__xcarhive__")
	if err != nil {
		return "", err
	}

	return tmpDir, ziputil.UnZip(xcarchivePth, tmpDir)
}

// GetEmbeddedMobileProvisionPath ...
func GetEmbeddedMobileProvisionPath(xcarchivePth string) (string, error) {
	return utility.FindFileInAppDir(getAppSubfolder(xcarchivePth), "embedded.mobileprovision")
}

// GetEmbeddedInfoPlistPath ...
func GetEmbeddedInfoPlistPath(xcarchivePth string) (string, error) {
	return utility.FindFileInAppDir(getAppSubfolder(xcarchivePth), "Info.plist")
}

func getAppSubfolder(basepth string) string {
	return filepath.Join(basepth, "Products", "Applications")
}

func findDSYMs(archivePath string) ([]string, []string, error) {
	dsymsDirPth := filepath.Join(archivePath, "dSYMs")
	dsyms, err := pathutil.ListEntries(dsymsDirPth, pathutil.ExtensionFilter(".dsym", true))
	if err != nil {
		return []string{}, []string{}, err
	}

	appDSYMs := []string{}
	frameworkDSYMs := []string{}
	for _, dsym := range dsyms {
		if strings.HasSuffix(dsym, ".app.dSYM") {
			appDSYMs = append(appDSYMs, dsym)
		} else {
			frameworkDSYMs = append(frameworkDSYMs, dsym)
		}
	}

	return appDSYMs, frameworkDSYMs, nil
}
