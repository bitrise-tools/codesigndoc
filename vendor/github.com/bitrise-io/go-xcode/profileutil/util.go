package profileutil

import (
	"path/filepath"

	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/fullsailor/pkcs7"
)

// ProfileType ...
type ProfileType string

// ProfileTypeIos ...
const ProfileTypeIos ProfileType = "ios"

// ProfileTypeMacOs ...
const ProfileTypeMacOs ProfileType = "macOs"

// ProvProfileSystemDirPath ...
const ProvProfileSystemDirPath = "~/Library/MobileDevice/Provisioning Profiles"

// ProvisioningProfileFromContent ...
func ProvisioningProfileFromContent(content []byte) (*pkcs7.PKCS7, error) {
	return pkcs7.Parse(content)
}

// ProvisioningProfileFromFile ...
func ProvisioningProfileFromFile(pth string) (*pkcs7.PKCS7, error) {
	content, err := fileutil.ReadBytesFromFile(pth)
	if err != nil {
		return nil, err
	}
	return ProvisioningProfileFromContent(content)
}

// InstalledProvisioningProfiles ...
func InstalledProvisioningProfiles(profileType ProfileType) ([]*pkcs7.PKCS7, error) {
	ext := ".mobileprovision"
	if profileType == ProfileTypeMacOs {
		ext = ".provisionprofile"
	}

	absProvProfileDirPath, err := pathutil.AbsPath(ProvProfileSystemDirPath)
	if err != nil {
		return nil, err
	}

	pattern := filepath.Join(pathutil.EscapeGlobPath(absProvProfileDirPath), "*"+ext)
	pths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	profiles := []*pkcs7.PKCS7{}
	for _, pth := range pths {
		profile, err := ProvisioningProfileFromFile(pth)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

// FindProvisioningProfile ...
func FindProvisioningProfile(uuid string) (*pkcs7.PKCS7, string, error) {
	{
		iosProvisioningProfileExt := ".mobileprovision"
		absProvProfileDirPath, err := pathutil.AbsPath(ProvProfileSystemDirPath)
		if err != nil {
			return nil, "", err
		}

		pth := filepath.Join(absProvProfileDirPath, uuid+iosProvisioningProfileExt)
		if exist, err := pathutil.IsPathExists(pth); err != nil {
			return nil, "", err
		} else if exist {
			profile, err := ProvisioningProfileFromFile(pth)
			if err != nil {
				return nil, "", err
			}
			return profile, pth, nil
		}
	}

	{
		macOsProvisioningProfileExt := ".provisionprofile"
		absProvProfileDirPath, err := pathutil.AbsPath(ProvProfileSystemDirPath)
		if err != nil {
			return nil, "", err
		}

		pth := filepath.Join(absProvProfileDirPath, uuid+macOsProvisioningProfileExt)
		if exist, err := pathutil.IsPathExists(pth); err != nil {
			return nil, "", err
		} else if exist {
			profile, err := ProvisioningProfileFromFile(pth)
			if err != nil {
				return nil, "", err
			}
			return profile, pth, nil
		}
	}

	return nil, "", nil
}
