package codesign

import (
	"regexp"
	"strings"

	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-xcode/profileutil"
)

// ProvisioningProfile contains parsed data in the provisioning profile and the original profile file contents
type ProvisioningProfile struct {
	Info    profileutil.ProvisioningProfileInfoModel
	Content []byte
}

// profileExportFileName creates a file name for the given profile with pattern: uuid.escaped_profile_name.[mobileprovision|provisionprofile]
func profileExportFileName(info profileutil.ProvisioningProfileInfoModel, path string) string {
	replaceRexp, err := regexp.Compile("[^A-Za-z0-9_.-]")
	if err != nil {
		log.Warnf("Invalid regex, error: %s", err)
		return ""
	}
	safeTitle := replaceRexp.ReplaceAllString(info.Name, "")
	extension := ".mobileprovision"
	if strings.HasSuffix(path, ".provisionprofile") {
		extension = ".provisionprofile"
	}

	return info.UUID + "." + safeTitle + extension
}

// ProfileExportFileNameNoPath creates a file name for the given profile with pattern: uuid.escaped_profile_name.[mobileprovision|provisionprofile]
func ProfileExportFileNameNoPath(info profileutil.ProvisioningProfileInfoModel) string {
	replaceRexp, err := regexp.Compile("[^A-Za-z0-9_.-]")
	if err != nil {
		log.Warnf("Invalid regex, error: %s", err)
		return ""
	}
	safeTitle := replaceRexp.ReplaceAllString(info.Name, "")
	extension := ".mobileprovision"
	if info.Type == profileutil.ProfileTypeMacOs {
		extension = ".provisionprofile"
	}

	return info.UUID + "." + safeTitle + extension
}

// FilterLatestProfiles renmoves older versions of the same profile
func FilterLatestProfiles(profiles []profileutil.ProvisioningProfileInfoModel) []profileutil.ProvisioningProfileInfoModel {
	profilesByBundleIDAndName := map[string][]profileutil.ProvisioningProfileInfoModel{}
	for _, profile := range profiles {
		bundleID := profile.BundleID
		name := profile.Name
		bundleIDAndName := bundleID + name
		profs, ok := profilesByBundleIDAndName[bundleIDAndName]
		if !ok {
			profs = []profileutil.ProvisioningProfileInfoModel{}
		}
		profs = append(profs, profile)
		profilesByBundleIDAndName[bundleIDAndName] = profs
	}

	filteredProfiles := []profileutil.ProvisioningProfileInfoModel{}
	for _, profiles := range profilesByBundleIDAndName {
		var latestProfile *profileutil.ProvisioningProfileInfoModel
		for _, profile := range profiles {
			if latestProfile == nil || profile.ExpirationDate.After(latestProfile.ExpirationDate) {
				latestProfile = &profile
			}
		}
		filteredProfiles = append(filteredProfiles, *latestProfile)
	}
	return filteredProfiles
}
