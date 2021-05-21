package utility

import (
	"fmt"
	"path/filepath"

	"github.com/bitrise-io/go-utils/pathutil"
)

// FindFileInAppDir ...
func FindFileInAppDir(appDir, fileName string) (string, error) {
	filePth := filepath.Join(appDir, fileName)
	if exist, err := pathutil.IsPathExists(filePth); err != nil {
		return "", err
	} else if exist {
		return filePth, nil
	}
	// ---

	// It's somewhere else - let's find it!
	apps, err := pathutil.ListEntries(appDir, pathutil.ExtensionFilter(".app", true))
	if err != nil {
		return "", err
	}

	for _, app := range apps {
		pths, err := pathutil.ListEntries(app, pathutil.BaseFilter(fileName, true))
		if err != nil {
			return "", err
		}

		if len(pths) > 0 {
			return pths[0], nil
		}
	}
	// ---

	return "", fmt.Errorf("failed to find %s", fileName)
}
