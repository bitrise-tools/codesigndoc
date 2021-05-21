package xcarchive

import (
	"fmt"
	"path/filepath"

	"github.com/bitrise-io/go-xcode/plistutil"
	"github.com/bitrise-io/go-xcode/profileutil"

	"github.com/bitrise-io/go-utils/pathutil"
)

type macosBaseApplication struct {
	Path                string
	InfoPlist           plistutil.PlistData
	Entitlements        plistutil.PlistData
	ProvisioningProfile *profileutil.ProvisioningProfileInfoModel
}

// BundleIdentifier ...
func (app macosBaseApplication) BundleIdentifier() string {
	bundleID, _ := app.InfoPlist.GetString("CFBundleIdentifier")
	return bundleID
}

func newMacosBaseApplication(path string) (macosBaseApplication, error) {
	infoPlist := plistutil.PlistData{}
	{
		infoPlistPath := filepath.Join(path, "Contents/Info.plist")
		if exist, err := pathutil.IsPathExists(infoPlistPath); err != nil {
			return macosBaseApplication{}, fmt.Errorf("failed to check if Info.plist exists at: %s, error: %s", infoPlistPath, err)
		} else if !exist {
			return macosBaseApplication{}, fmt.Errorf("Info.plist not exists at: %s", infoPlistPath)
		}
		plist, err := plistutil.NewPlistDataFromFile(infoPlistPath)
		if err != nil {
			return macosBaseApplication{}, err
		}
		infoPlist = plist
	}

	var provisioningProfile *profileutil.ProvisioningProfileInfoModel
	{
		provisioningProfilePath := filepath.Join(path, "Contents/embedded.provisionprofile")
		if exist, err := pathutil.IsPathExists(provisioningProfilePath); err != nil {
			return macosBaseApplication{}, fmt.Errorf("failed to check if profile exists at: %s, error: %s", provisioningProfilePath, err)
		} else if exist {
			profile, err := profileutil.NewProvisioningProfileInfoFromFile(provisioningProfilePath)
			if err != nil {
				return macosBaseApplication{}, err
			}
			provisioningProfile = &profile
		}
	}

	entitlements := plistutil.PlistData{}
	{
		entitlementsPath := filepath.Join(path, "Contents/Resources/archived-expanded-entitlements.xcent")
		if exist, err := pathutil.IsPathExists(entitlementsPath); err != nil {
			return macosBaseApplication{}, fmt.Errorf("failed to check if entitlements exists at: %s, error: %s", entitlementsPath, err)
		} else if exist {
			plist, err := plistutil.NewPlistDataFromFile(entitlementsPath)
			if err != nil {
				return macosBaseApplication{}, err
			}
			entitlements = plist
		}
	}

	return macosBaseApplication{
		Path:                path,
		InfoPlist:           infoPlist,
		Entitlements:        entitlements,
		ProvisioningProfile: provisioningProfile,
	}, nil
}

// MacosExtension ...
type MacosExtension struct {
	macosBaseApplication
}

// NewMacosExtension ...
func NewMacosExtension(path string) (MacosExtension, error) {
	baseApp, err := newMacosBaseApplication(path)
	if err != nil {
		return MacosExtension{}, err
	}

	return MacosExtension{
		baseApp,
	}, nil
}

// MacosApplication ...
type MacosApplication struct {
	macosBaseApplication
	Extensions []MacosExtension
}

// NewMacosApplication ...
func NewMacosApplication(path string) (MacosApplication, error) {
	baseApp, err := newMacosBaseApplication(path)
	if err != nil {
		return MacosApplication{}, err
	}

	extensions := []MacosExtension{}
	{
		pattern := filepath.Join(pathutil.EscapeGlobPath(path), "Contents/PlugIns/*.appex")
		pths, err := filepath.Glob(pattern)
		if err != nil {
			return MacosApplication{}, fmt.Errorf("failed to search for watch application's extensions using pattern: %s, error: %s", pattern, err)
		}
		for _, pth := range pths {
			extension, err := NewMacosExtension(pth)
			if err != nil {
				return MacosApplication{}, err
			}

			extensions = append(extensions, extension)
		}
	}

	return MacosApplication{
		macosBaseApplication: baseApp,
		Extensions:           extensions,
	}, nil
}

// MacosArchive ...
type MacosArchive struct {
	Path        string
	InfoPlist   plistutil.PlistData
	Application MacosApplication
}

// NewMacosArchive ...
func NewMacosArchive(path string) (MacosArchive, error) {
	infoPlist := plistutil.PlistData{}
	{
		infoPlistPath := filepath.Join(path, "Info.plist")
		if exist, err := pathutil.IsPathExists(infoPlistPath); err != nil {
			return MacosArchive{}, fmt.Errorf("failed to check if Info.plist exists at: %s, error: %s", infoPlistPath, err)
		} else if !exist {
			return MacosArchive{}, fmt.Errorf("Info.plist not exists at: %s", infoPlistPath)
		}
		plist, err := plistutil.NewPlistDataFromFile(infoPlistPath)
		if err != nil {
			return MacosArchive{}, err
		}
		infoPlist = plist
	}

	application := MacosApplication{}
	{
		pattern := filepath.Join(pathutil.EscapeGlobPath(path), "Products/Applications/*.app")
		pths, err := filepath.Glob(pattern)
		if err != nil {
			return MacosArchive{}, err
		}

		appPath := ""
		if len(pths) > 0 {
			appPath = pths[0]
		} else {
			return MacosArchive{}, fmt.Errorf("failed to find main app, using pattern: %s", pattern)
		}

		app, err := NewMacosApplication(appPath)
		if err != nil {
			return MacosArchive{}, err
		}
		application = app
	}

	return MacosArchive{
		Path:        path,
		InfoPlist:   infoPlist,
		Application: application,
	}, nil
}

// IsXcodeManaged ...
func (archive MacosArchive) IsXcodeManaged() bool {
	if archive.Application.ProvisioningProfile != nil {
		return archive.Application.ProvisioningProfile.IsXcodeManaged()
	}
	return false
}

// SigningIdentity ...
func (archive MacosArchive) SigningIdentity() string {
	properties, found := archive.InfoPlist.GetMapStringInterface("ApplicationProperties")
	if found {
		identity, _ := properties.GetString("SigningIdentity")
		return identity
	}
	return ""
}

// BundleIDEntitlementsMap ...
func (archive MacosArchive) BundleIDEntitlementsMap() map[string]plistutil.PlistData {
	bundleIDEntitlementsMap := map[string]plistutil.PlistData{}

	bundleID := archive.Application.BundleIdentifier()
	bundleIDEntitlementsMap[bundleID] = archive.Application.Entitlements

	for _, plugin := range archive.Application.Extensions {
		bundleID := plugin.BundleIdentifier()
		bundleIDEntitlementsMap[bundleID] = plugin.Entitlements
	}

	return bundleIDEntitlementsMap
}

// BundleIDProfileInfoMap ...
func (archive MacosArchive) BundleIDProfileInfoMap() map[string]profileutil.ProvisioningProfileInfoModel {
	bundleIDProfileMap := map[string]profileutil.ProvisioningProfileInfoModel{}

	if archive.Application.ProvisioningProfile != nil {
		bundleID := archive.Application.BundleIdentifier()
		bundleIDProfileMap[bundleID] = *archive.Application.ProvisioningProfile
	}

	for _, plugin := range archive.Application.Extensions {
		if plugin.ProvisioningProfile != nil {
			bundleID := plugin.BundleIdentifier()
			bundleIDProfileMap[bundleID] = *plugin.ProvisioningProfile
		}
	}

	return bundleIDProfileMap
}

// FindDSYMs ...
func (archive MacosArchive) FindDSYMs() ([]string, []string, error) {
	return findDSYMs(archive.Path)
}
